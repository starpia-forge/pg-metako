// Purpose    : Distributed query routing with local node preference
// Context    : Routes queries with preference for local PostgreSQL instance
// Constraints: Must maintain high availability while preferring local resources

package routing

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"pg-metako/internal/config"
	"pg-metako/internal/coordination"
	"pg-metako/internal/database"
	"pg-metako/internal/logger"
)

// DistributedRouter handles query routing in a distributed pg-metako deployment
type DistributedRouter struct {
	config            *config.DistributedConfig
	coordinationAPI   *coordination.CoordinationAPI
	localConnection   *database.ConnectionManager
	remoteConnections map[string]*database.ConnectionManager

	// Statistics
	stats *DistributedStats
	mu    sync.RWMutex

	// Connection tracking
	connectionCounts map[string]int64
}

// DistributedStats tracks routing statistics for distributed deployment
type DistributedStats struct {
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

// NodeConnection represents a connection to a remote node
type NodeConnection struct {
	NodeName   string
	Connection *database.ConnectionManager
	IsLocal    bool
	IsHealthy  bool
	LastUsed   time.Time
}

// NewDistributedRouter creates a new distributed router
func NewDistributedRouter(cfg *config.DistributedConfig, coordinationAPI *coordination.CoordinationAPI) (*DistributedRouter, error) {
	// Create local connection
	localConn, err := database.NewConnectionManager(cfg.LocalDB)
	if err != nil {
		return nil, fmt.Errorf("failed to create local connection: %w", err)
	}

	router := &DistributedRouter{
		config:            cfg,
		coordinationAPI:   coordinationAPI,
		localConnection:   localConn,
		remoteConnections: make(map[string]*database.ConnectionManager),
		connectionCounts:  make(map[string]int64),
		stats: &DistributedStats{
			NodeConnections: make(map[string]int64),
			LastQueryTime:   time.Now(),
			LocalPreference: cfg.Coordination.LocalNodePreference,
		},
	}

	return router, nil
}

// Initialize initializes the distributed router
func (dr *DistributedRouter) Initialize(ctx context.Context) error {
	// Connect to local database
	if err := dr.localConnection.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to local database: %w", err)
	}

	logger.Printf("Distributed router initialized with local node: %s", dr.config.Identity.NodeName)
	return nil
}

// RouteQuery routes a query with local node preference
func (dr *DistributedRouter) RouteQuery(ctx context.Context, query string, args ...interface{}) (*database.QueryResult, error) {
	queryType := dr.detectQueryType(query)

	// Update statistics
	atomic.AddInt64(&dr.stats.TotalQueries, 1)
	dr.stats.LastQueryTime = time.Now()

	var targetNode *database.ConnectionManager
	var isLocal bool
	var timeout time.Duration

	switch queryType {
	case QueryTypeRead:
		atomic.AddInt64(&dr.stats.ReadQueries, 1)
		targetNode, isLocal = dr.selectReadNode(ctx)
		timeout = dr.config.LoadBalancer.ReadTimeout
	case QueryTypeWrite:
		atomic.AddInt64(&dr.stats.WriteQueries, 1)
		targetNode, isLocal = dr.selectWriteNode(ctx)
		timeout = dr.config.LoadBalancer.WriteTimeout
	}

	if targetNode == nil {
		atomic.AddInt64(&dr.stats.FailedQueries, 1)
		return nil, fmt.Errorf("no available node for %s query", queryType.String())
	}

	// Update statistics based on routing decision
	if isLocal {
		atomic.AddInt64(&dr.stats.LocalQueries, 1)
	} else {
		atomic.AddInt64(&dr.stats.RemoteQueries, 1)
	}

	// Create timeout context
	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Update connection stats
	dr.mu.Lock()
	dr.stats.NodeConnections[targetNode.NodeName()]++
	dr.mu.Unlock()

	// Execute the query
	result, err := targetNode.ExecuteQuery(queryCtx, query, args...)
	if err != nil {
		atomic.AddInt64(&dr.stats.FailedQueries, 1)
		return nil, fmt.Errorf("query execution failed on node %s: %w", targetNode.NodeName(), err)
	}

	return result, nil
}

// selectReadNode selects a node for read operations with local preference
func (dr *DistributedRouter) selectReadNode(ctx context.Context) (*database.ConnectionManager, bool) {
	// Check if local node is healthy and available for reads
	if dr.isLocalNodeHealthy(ctx) {
		// Apply local preference probability
		if rand.Float64() < dr.config.Coordination.LocalNodePreference {
			logger.Printf("Routing read query to local node: %s", dr.config.Identity.NodeName)
			return dr.localConnection, true
		}
	}

	// If local node is not preferred or not healthy, try remote nodes
	remoteNode := dr.selectRemoteReadNode(ctx)
	if remoteNode != nil {
		logger.Printf("Routing read query to remote node: %s", remoteNode.NodeName())
		return remoteNode, false
	}

	// Fallback to local node if no remote nodes are available
	if dr.isLocalNodeHealthy(ctx) {
		logger.Printf("Fallback: routing read query to local node: %s", dr.config.Identity.NodeName)
		return dr.localConnection, true
	}

	return nil, false
}

// selectWriteNode selects a node for write operations
func (dr *DistributedRouter) selectWriteNode(ctx context.Context) (*database.ConnectionManager, bool) {
	// For write operations, we need to route to the current master
	clusterState := dr.coordinationAPI.GetClusterState()

	// Check if local node is the master
	if dr.config.LocalDB.Role == config.RoleMaster && dr.isLocalNodeHealthy(ctx) {
		logger.Printf("Routing write query to local master: %s", dr.config.Identity.NodeName)
		return dr.localConnection, true
	}

	// If local node is not master, find the current master from cluster state
	currentMaster := clusterState.CurrentMaster
	if currentMaster == "" {
		// Try to determine master from cluster members
		for _, member := range dr.config.ClusterMembers {
			if member.Role == config.RoleMaster {
				if nodeStatus, exists := clusterState.Nodes[member.NodeName]; exists && nodeStatus.IsHealthy {
					currentMaster = member.NodeName
					break
				}
			}
		}
	}

	if currentMaster != "" && currentMaster != dr.config.Identity.NodeName {
		// Route to remote master (this would require establishing remote connections)
		logger.Printf("Write query requires routing to remote master: %s", currentMaster)
		// For now, return nil as remote connections are not fully implemented
		return nil, false
	}

	return nil, false
}

// selectRemoteReadNode selects a remote node for read operations
func (dr *DistributedRouter) selectRemoteReadNode(ctx context.Context) *database.ConnectionManager {
	clusterState := dr.coordinationAPI.GetClusterState()

	// Get healthy remote nodes that can serve reads
	var healthyRemoteNodes []string
	for nodeName, nodeStatus := range clusterState.Nodes {
		if nodeName != dr.config.Identity.NodeName && nodeStatus.IsHealthy && nodeStatus.LocalDBHealthy {
			healthyRemoteNodes = append(healthyRemoteNodes, nodeName)
		}
	}

	if len(healthyRemoteNodes) == 0 {
		return nil
	}

	// Simple round-robin selection for now
	selectedNode := healthyRemoteNodes[rand.Intn(len(healthyRemoteNodes))]

	// Note: In a full implementation, we would maintain connections to remote nodes
	// For now, we return nil as remote connections are not fully implemented
	logger.Printf("Would route to remote node: %s (remote connections not implemented)", selectedNode)
	return nil
}

// isLocalNodeHealthy checks if the local node is healthy
func (dr *DistributedRouter) isLocalNodeHealthy(ctx context.Context) bool {
	return dr.localConnection.IsHealthy(ctx)
}

// detectQueryType analyzes a SQL query to determine if it's a read or write operation
func (dr *DistributedRouter) detectQueryType(query string) QueryType {
	// Reuse the existing query type detection logic
	qr := &QueryRouter{} // Temporary instance just for the method
	return qr.DetectQueryType(query)
}

// GetStats returns the current routing statistics
func (dr *DistributedRouter) GetStats() *DistributedStats {
	dr.mu.RLock()
	defer dr.mu.RUnlock()

	// Calculate local preference ratio
	totalQueries := atomic.LoadInt64(&dr.stats.TotalQueries)
	localQueries := atomic.LoadInt64(&dr.stats.LocalQueries)

	if totalQueries > 0 {
		dr.stats.LocalPreference = float64(localQueries) / float64(totalQueries)
	}

	return dr.stats
}

// UpdateRemoteConnections updates the list of remote connections based on cluster state
func (dr *DistributedRouter) UpdateRemoteConnections(ctx context.Context) error {
	clusterState := dr.coordinationAPI.GetClusterState()

	dr.mu.Lock()
	defer dr.mu.Unlock()

	// Remove connections to nodes that are no longer in the cluster
	for nodeName := range dr.remoteConnections {
		if _, exists := clusterState.Nodes[nodeName]; !exists {
			if conn := dr.remoteConnections[nodeName]; conn != nil {
				// Close connection (if connection manager had a Close method)
				logger.Printf("Removing connection to node: %s", nodeName)
			}
			delete(dr.remoteConnections, nodeName)
		}
	}

	// Add connections to new healthy nodes
	for nodeName, nodeStatus := range clusterState.Nodes {
		if nodeName == dr.config.Identity.NodeName {
			continue // Skip local node
		}

		if _, exists := dr.remoteConnections[nodeName]; !exists && nodeStatus.IsHealthy {
			// In a full implementation, we would create connections to remote databases
			// This would require knowing the database connection details for remote nodes
			logger.Printf("Would create connection to remote node: %s", nodeName)
		}
	}

	return nil
}

// Close closes all connections
func (dr *DistributedRouter) Close() error {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	// Close remote connections
	for nodeName, conn := range dr.remoteConnections {
		if conn != nil {
			logger.Printf("Closing connection to remote node: %s", nodeName)
			// Note: database.ConnectionManager doesn't have a Close method in current implementation
		}
	}

	logger.Printf("Distributed router closed")
	return nil
}

// GetLocalConnection returns the local database connection
func (dr *DistributedRouter) GetLocalConnection() *database.ConnectionManager {
	return dr.localConnection
}

// IsLocalNodeMaster checks if the local node is currently the master
func (dr *DistributedRouter) IsLocalNodeMaster() bool {
	return dr.config.LocalDB.Role == config.RoleMaster
}

// GetClusterState returns the current cluster state from coordination API
func (dr *DistributedRouter) GetClusterState() coordination.ClusterState {
	return dr.coordinationAPI.GetClusterState()
}
