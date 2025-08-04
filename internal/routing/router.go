// Purpose    : Unified query routing with distributed functionality as default
// Context    : Handles query routing in distributed pg-metako deployment with local node preference
// Constraints: Must maintain compatibility with existing interfaces while prioritizing distributed features

package routing

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"pg-metako/internal/config"
	"pg-metako/internal/coordination"
	"pg-metako/internal/database"
	"pg-metako/internal/logger"
)

// QueryType represents the type of SQL query
type QueryType int

const (
	QueryTypeRead QueryType = iota
	QueryTypeWrite
)

// String returns the string representation of QueryType
func (qt QueryType) String() string {
	switch qt {
	case QueryTypeRead:
		return "READ"
	case QueryTypeWrite:
		return "WRITE"
	default:
		return "UNKNOWN"
	}
}

// Router handles query routing in a distributed pg-metako deployment
type Router struct {
	config            *config.Config
	coordinationAPI   *coordination.CoordinationAPI
	localConnection   *database.ConnectionManager
	remoteConnections map[string]*database.ConnectionManager

	// Statistics
	stats *Stats
	mu    sync.RWMutex

	// Connection tracking
	connectionCounts map[string]int64
}

// Stats tracks routing statistics for distributed deployment
type Stats struct {
	TotalQueries    int64            `json:"total_queries"`
	LocalQueries    int64            `json:"local_queries"`
	RemoteQueries   int64            `json:"remote_queries"`
	ReadQueries     int64            `json:"read_queries"`
	WriteQueries    int64            `json:"write_queries"`
	FailedQueries   int64            `json:"failed_queries"`
	LastQueryTime   time.Time        `json:"last_query_time"`
	NodeConnections map[string]int64 `json:"node_connections"`
	LocalPreference float64          `json:"local_preference_ratio"`
}

// NewRouter creates a new distributed router
func NewRouter(cfg *config.Config, coordinationAPI *coordination.CoordinationAPI) (*Router, error) {
	// Create local connection
	localConn, err := database.NewConnectionManager(cfg.LocalDB)
	if err != nil {
		return nil, fmt.Errorf("failed to create local connection: %w", err)
	}

	router := &Router{
		config:            cfg,
		coordinationAPI:   coordinationAPI,
		localConnection:   localConn,
		remoteConnections: make(map[string]*database.ConnectionManager),
		connectionCounts:  make(map[string]int64),
		stats: &Stats{
			NodeConnections: make(map[string]int64),
			LastQueryTime:   time.Now(),
			LocalPreference: cfg.Coordination.LocalNodePreference,
		},
	}

	return router, nil
}

// Initialize initializes the distributed router
func (r *Router) Initialize(ctx context.Context) error {
	// Connect to local database
	if err := r.localConnection.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to local database: %w", err)
	}

	logger.Printf("Router initialized with local node: %s", r.config.Identity.NodeName)
	return nil
}

// RouteQuery routes a query with local node preference
func (r *Router) RouteQuery(ctx context.Context, query string, args ...interface{}) (*database.QueryResult, error) {
	queryType := r.DetectQueryType(query)

	// Update statistics
	atomic.AddInt64(&r.stats.TotalQueries, 1)
	r.stats.LastQueryTime = time.Now()

	var targetNode *database.ConnectionManager
	var isLocal bool
	var timeout time.Duration

	switch queryType {
	case QueryTypeRead:
		atomic.AddInt64(&r.stats.ReadQueries, 1)
		targetNode, isLocal = r.selectReadNode(ctx)
		timeout = r.config.LoadBalancer.ReadTimeout
	case QueryTypeWrite:
		atomic.AddInt64(&r.stats.WriteQueries, 1)
		targetNode, isLocal = r.selectWriteNode(ctx)
		timeout = r.config.LoadBalancer.WriteTimeout
	}

	if targetNode == nil {
		atomic.AddInt64(&r.stats.FailedQueries, 1)
		return nil, fmt.Errorf("no available node for %s query", queryType.String())
	}

	// Update statistics based on routing decision
	if isLocal {
		atomic.AddInt64(&r.stats.LocalQueries, 1)
	} else {
		atomic.AddInt64(&r.stats.RemoteQueries, 1)
	}

	// Create timeout context
	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute query
	result, err := targetNode.ExecuteQuery(queryCtx, query, args...)
	if err != nil {
		atomic.AddInt64(&r.stats.FailedQueries, 1)
		return nil, fmt.Errorf("query execution failed: %w", err)
	}

	return result, nil
}

// selectReadNode selects the best node for read queries with local preference
func (r *Router) selectReadNode(ctx context.Context) (*database.ConnectionManager, bool) {
	// Check if local node is healthy and can handle reads
	if r.isLocalNodeHealthy(ctx) {
		// Apply local node preference
		if rand.Float64() < r.config.Coordination.LocalNodePreference {
			logger.Printf("Routing read query to local node: %s", r.config.Identity.NodeName)
			return r.localConnection, true
		}
	}

	// Try to find a healthy remote read node
	if remoteNode := r.selectRemoteReadNode(ctx); remoteNode != nil {
		logger.Printf("Routing read query to remote node: %s", remoteNode.NodeName())
		return remoteNode, false
	}

	// Fallback to local node if available
	if r.isLocalNodeHealthy(ctx) {
		logger.Printf("Fallback: routing read query to local node: %s", r.config.Identity.NodeName)
		return r.localConnection, true
	}

	return nil, false
}

// selectWriteNode selects the best node for write queries (must be master)
func (r *Router) selectWriteNode(ctx context.Context) (*database.ConnectionManager, bool) {
	// Check if local node is master and healthy
	if r.IsLocalNodeMaster() && r.isLocalNodeHealthy(ctx) {
		logger.Printf("Routing write query to local master: %s", r.config.Identity.NodeName)
		return r.localConnection, true
	}

	// Find current master from cluster state
	clusterState := r.GetClusterState()
	currentMaster := clusterState.CurrentMaster

	if currentMaster == "" {
		return nil, false
	}

	// If current master is not local, we need to route to remote master
	if currentMaster != r.config.Identity.NodeName {
		logger.Printf("Write query requires routing to remote master: %s", currentMaster)
		// In a real implementation, we would establish connection to remote master
		// For now, return nil to indicate remote routing is needed
		return nil, false
	}

	return nil, false
}

// selectRemoteReadNode selects a healthy remote node for read queries
func (r *Router) selectRemoteReadNode(ctx context.Context) *database.ConnectionManager {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Get cluster state to find available nodes
	clusterState := r.GetClusterState()

	// Find healthy slave nodes first, then masters
	var candidates []string
	for nodeName, status := range clusterState.Nodes {
		if nodeName != r.config.Identity.NodeName && status.IsHealthy {
			candidates = append(candidates, nodeName)
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	// Simple round-robin selection for now
	selectedNode := candidates[rand.Intn(len(candidates))]
	if conn, exists := r.remoteConnections[selectedNode]; exists {
		return conn
	}

	return nil
}

// isLocalNodeHealthy checks if the local node is healthy
func (r *Router) isLocalNodeHealthy(ctx context.Context) bool {
	return r.localConnection != nil && r.localConnection.IsHealthy(ctx)
}

// DetectQueryType detects the type of query (read or write)
func (r *Router) DetectQueryType(query string) QueryType {
	// This is the same logic from the original QueryRouter
	query = strings.ToUpper(strings.TrimSpace(query))

	if strings.HasPrefix(query, "SELECT") ||
		strings.HasPrefix(query, "SHOW") ||
		strings.HasPrefix(query, "DESCRIBE") ||
		strings.HasPrefix(query, "EXPLAIN") {
		return QueryTypeRead
	}

	return QueryTypeWrite
}

// GetStats returns the current routing statistics
func (r *Router) GetStats() *Stats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Create a copy of stats to avoid race conditions
	statsCopy := *r.stats
	statsCopy.NodeConnections = make(map[string]int64)
	for k, v := range r.stats.NodeConnections {
		statsCopy.NodeConnections[k] = v
	}

	return &statsCopy
}

// UpdateRemoteConnections updates connections to remote cluster members
func (r *Router) UpdateRemoteConnections(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Update connections for each cluster member
	for _, member := range r.config.ClusterMembers {
		if member.NodeName == r.config.Identity.NodeName {
			continue // Skip local node
		}

		// Check if we need to create or update connection
		if _, exists := r.remoteConnections[member.NodeName]; !exists {
			// Create connection configuration for remote node
			remoteConfig := config.NodeConfig{
				Name:     member.NodeName,
				Host:     member.APIHost, // Use API host for now
				Port:     5432,           // Default PostgreSQL port
				Role:     member.Role,
				Username: r.config.LocalDB.Username, // Use same credentials
				Password: r.config.LocalDB.Password,
				Database: r.config.LocalDB.Database,
			}

			// Create connection manager
			connMgr, err := database.NewConnectionManager(remoteConfig)
			if err != nil {
				logger.Printf("Failed to create connection to %s: %v", member.NodeName, err)
				continue
			}

			r.remoteConnections[member.NodeName] = connMgr
		}
	}

	return nil
}

// Close closes all connections
func (r *Router) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Close local connection
	if r.localConnection != nil {
		if err := r.localConnection.Close(); err != nil {
			logger.Printf("Error closing local connection: %v", err)
		}
	}

	// Close remote connections
	for nodeName, conn := range r.remoteConnections {
		if err := conn.Close(); err != nil {
			logger.Printf("Error closing connection to %s: %v", nodeName, err)
		}
	}

	return nil
}

// GetLocalConnection returns the local database connection
func (r *Router) GetLocalConnection() *database.ConnectionManager {
	return r.localConnection
}

// IsLocalNodeMaster checks if the local node is currently the master
func (r *Router) IsLocalNodeMaster() bool {
	return r.config.LocalDB.Role == config.RoleMaster
}

// GetClusterState returns the current cluster state
func (r *Router) GetClusterState() coordination.ClusterState {
	if r.coordinationAPI != nil {
		return r.coordinationAPI.GetClusterState()
	}
	return coordination.ClusterState{}
}

// Legacy compatibility methods for existing code

// SelectReadNode provides compatibility with the old QueryRouter interface
func (r *Router) SelectReadNode() *database.ConnectionManager {
	ctx := context.Background()
	node, _ := r.selectReadNode(ctx)
	return node
}

// SelectWriteNode provides compatibility with the old QueryRouter interface
func (r *Router) SelectWriteNode() *database.ConnectionManager {
	ctx := context.Background()
	node, _ := r.selectWriteNode(ctx)
	return node
}

// GetConnectionStats provides compatibility with the old QueryRouter interface
func (r *Router) GetConnectionStats() *Stats {
	return r.GetStats()
}

// ResetStats resets routing statistics
func (r *Router) ResetStats() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.stats.TotalQueries = 0
	r.stats.LocalQueries = 0
	r.stats.RemoteQueries = 0
	r.stats.ReadQueries = 0
	r.stats.WriteQueries = 0
	r.stats.FailedQueries = 0
	r.stats.NodeConnections = make(map[string]int64)
	r.stats.LastQueryTime = time.Now()
}
