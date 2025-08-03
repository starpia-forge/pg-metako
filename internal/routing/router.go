package routing

import (
	"context"
	"fmt"
	"pg-metako/internal/config"
	"pg-metako/internal/database"
	"pg-metako/internal/replication"
	"strings"
	"sync"
	"sync/atomic"
	"time"
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

// ConnectionStats represents connection statistics for monitoring
type ConnectionStats struct {
	TotalQueries    int64
	ReadQueries     int64
	WriteQueries    int64
	FailedQueries   int64
	NodeConnections map[string]int64
	LastQueryTime   time.Time
}

// QueryRouter handles query routing and load balancing
type QueryRouter struct {
	replicationManager *replication.ReplicationManager
	config             config.LoadBalancerConfig
	roundRobinCounter  int64
	connectionCounts   map[string]int64
	stats              *ConnectionStats
	mu                 sync.RWMutex
}

// NewQueryRouter creates a new query router
func NewQueryRouter(replicationManager *replication.ReplicationManager, config config.LoadBalancerConfig) *QueryRouter {
	return &QueryRouter{
		replicationManager: replicationManager,
		config:             config,
		connectionCounts:   make(map[string]int64),
		stats: &ConnectionStats{
			NodeConnections: make(map[string]int64),
			LastQueryTime:   time.Now(),
		},
	}
}

// DetectQueryType analyzes a SQL query to determine if it's a read or write operation
func (qr *QueryRouter) DetectQueryType(query string) QueryType {
	// Trim whitespace and convert to uppercase for analysis
	trimmed := strings.TrimSpace(query)
	if len(trimmed) == 0 {
		return QueryTypeRead // Default to read for empty queries
	}

	// Convert to uppercase for comparison
	upper := strings.ToUpper(trimmed)

	// Check for read operations
	readKeywords := []string{"SELECT", "SHOW", "DESCRIBE", "EXPLAIN", "WITH"}
	for _, keyword := range readKeywords {
		if strings.HasPrefix(upper, keyword) {
			return QueryTypeRead
		}
	}

	// Everything else is considered a write operation
	// This includes: INSERT, UPDATE, DELETE, CREATE, DROP, ALTER, TRUNCATE, etc.
	return QueryTypeWrite
}

// RouteQuery routes a query to the appropriate database node
func (qr *QueryRouter) RouteQuery(ctx context.Context, query string, args ...interface{}) (*database.QueryResult, error) {
	queryType := qr.DetectQueryType(query)

	// Update statistics
	atomic.AddInt64(&qr.stats.TotalQueries, 1)
	qr.stats.LastQueryTime = time.Now()

	var targetNode *database.ConnectionManager
	var timeout time.Duration

	switch queryType {
	case QueryTypeRead:
		atomic.AddInt64(&qr.stats.ReadQueries, 1)
		targetNode = qr.SelectReadNode()
		timeout = qr.config.ReadTimeout
	case QueryTypeWrite:
		atomic.AddInt64(&qr.stats.WriteQueries, 1)
		targetNode = qr.SelectWriteNode()
		timeout = qr.config.WriteTimeout
	}

	if targetNode == nil {
		atomic.AddInt64(&qr.stats.FailedQueries, 1)
		return nil, fmt.Errorf("no available node for %s query", queryType.String())
	}

	// Create timeout context
	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Update connection stats
	qr.mu.Lock()
	qr.stats.NodeConnections[targetNode.NodeName()]++
	qr.mu.Unlock()

	// Execute the query
	result, err := targetNode.ExecuteQuery(queryCtx, query, args...)
	if err != nil {
		atomic.AddInt64(&qr.stats.FailedQueries, 1)
		return nil, fmt.Errorf("query execution failed on node %s: %w", targetNode.NodeName(), err)
	}

	return result, nil
}

// SelectReadNode selects a slave node for read operations using the configured load balancing algorithm
func (qr *QueryRouter) SelectReadNode() *database.ConnectionManager {
	slaves := qr.replicationManager.GetSlaveNodes()
	if len(slaves) == 0 {
		// Fallback to master if no slaves available
		return qr.replicationManager.GetCurrentMaster()
	}

	// Filter healthy slaves
	healthySlaves := qr.filterHealthyNodes(slaves)
	if len(healthySlaves) == 0 {
		// Fallback to master if no healthy slaves
		return qr.replicationManager.GetCurrentMaster()
	}

	switch qr.config.Algorithm {
	case config.AlgorithmRoundRobin:
		return qr.selectRoundRobin(healthySlaves)
	case config.AlgorithmLeastConnected:
		return qr.selectLeastConnected(healthySlaves)
	default:
		return qr.selectRoundRobin(healthySlaves)
	}
}

// SelectWriteNode selects the master node for write operations
func (qr *QueryRouter) SelectWriteNode() *database.ConnectionManager {
	master := qr.replicationManager.GetCurrentMaster()
	if master == nil {
		return nil
	}

	// Check if master is healthy
	status, err := qr.replicationManager.GetNodeStatus(master.NodeName())
	if err != nil || !status.IsHealthy {
		return nil
	}

	return master
}

// filterHealthyNodes filters nodes to return only healthy ones
func (qr *QueryRouter) filterHealthyNodes(nodes []*database.ConnectionManager) []*database.ConnectionManager {
	var healthy []*database.ConnectionManager

	for _, node := range nodes {
		status, err := qr.replicationManager.GetNodeStatus(node.NodeName())
		if err == nil && status.IsHealthy {
			healthy = append(healthy, node)
		}
	}

	return healthy
}

// selectRoundRobin implements round-robin load balancing
func (qr *QueryRouter) selectRoundRobin(nodes []*database.ConnectionManager) *database.ConnectionManager {
	if len(nodes) == 0 {
		return nil
	}

	// Atomic increment and get the counter
	counter := atomic.AddInt64(&qr.roundRobinCounter, 1)
	index := int((counter - 1) % int64(len(nodes)))

	return nodes[index]
}

// selectLeastConnected implements least-connected load balancing
func (qr *QueryRouter) selectLeastConnected(nodes []*database.ConnectionManager) *database.ConnectionManager {
	if len(nodes) == 0 {
		return nil
	}

	qr.mu.RLock()
	defer qr.mu.RUnlock()

	var selectedNode *database.ConnectionManager
	minConnections := int64(-1)

	for _, node := range nodes {
		connections := qr.stats.NodeConnections[node.NodeName()]
		if minConnections == -1 || connections < minConnections {
			minConnections = connections
			selectedNode = node
		}
	}

	return selectedNode
}

// GetConnectionStats returns current connection statistics
func (qr *QueryRouter) GetConnectionStats() *ConnectionStats {
	qr.mu.RLock()
	defer qr.mu.RUnlock()

	// Create a copy to avoid race conditions
	statsCopy := &ConnectionStats{
		TotalQueries:    atomic.LoadInt64(&qr.stats.TotalQueries),
		ReadQueries:     atomic.LoadInt64(&qr.stats.ReadQueries),
		WriteQueries:    atomic.LoadInt64(&qr.stats.WriteQueries),
		FailedQueries:   atomic.LoadInt64(&qr.stats.FailedQueries),
		NodeConnections: make(map[string]int64),
		LastQueryTime:   qr.stats.LastQueryTime,
	}

	// Copy node connections
	for nodeName, count := range qr.stats.NodeConnections {
		statsCopy.NodeConnections[nodeName] = count
	}

	return statsCopy
}

// ResetStats resets connection statistics
func (qr *QueryRouter) ResetStats() {
	qr.mu.Lock()
	defer qr.mu.Unlock()

	atomic.StoreInt64(&qr.stats.TotalQueries, 0)
	atomic.StoreInt64(&qr.stats.ReadQueries, 0)
	atomic.StoreInt64(&qr.stats.WriteQueries, 0)
	atomic.StoreInt64(&qr.stats.FailedQueries, 0)

	// Clear node connections
	qr.stats.NodeConnections = make(map[string]int64)
	qr.stats.LastQueryTime = time.Now()
}

// UpdateLoadBalancerConfig updates the load balancer configuration
func (qr *QueryRouter) UpdateLoadBalancerConfig(config config.LoadBalancerConfig) {
	qr.mu.Lock()
	defer qr.mu.Unlock()

	qr.config = config
}

// GetAvailableNodes returns information about available nodes
func (qr *QueryRouter) GetAvailableNodes() map[string]bool {
	result := make(map[string]bool)

	// Check master
	master := qr.replicationManager.GetCurrentMaster()
	if master != nil {
		status, err := qr.replicationManager.GetNodeStatus(master.NodeName())
		result[master.NodeName()] = err == nil && status.IsHealthy
	}

	// Check slaves
	slaves := qr.replicationManager.GetSlaveNodes()
	for _, slave := range slaves {
		status, err := qr.replicationManager.GetNodeStatus(slave.NodeName())
		result[slave.NodeName()] = err == nil && status.IsHealthy
	}

	return result
}
