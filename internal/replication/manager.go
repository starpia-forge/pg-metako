package replication

import (
	"context"
	"errors"
	"fmt"
	"pg-metako/internal/config"
	"pg-metako/internal/database"
	"pg-metako/internal/health"
	"sync"
	"time"
)

// ReplicationManager manages PostgreSQL replication and failover
type ReplicationManager struct {
	healthChecker    *health.HealthChecker
	masterNodes      map[string]*database.ConnectionManager
	slaveNodes       map[string]*database.ConnectionManager
	currentMaster    *database.ConnectionManager
	mu               sync.RWMutex
	failoverStopChan chan struct{}
	failoverRunning  bool
	failoverMu       sync.Mutex
}

// NewReplicationManager creates a new replication manager
func NewReplicationManager(healthChecker *health.HealthChecker) *ReplicationManager {
	return &ReplicationManager{
		healthChecker:    healthChecker,
		masterNodes:      make(map[string]*database.ConnectionManager),
		slaveNodes:       make(map[string]*database.ConnectionManager),
		failoverStopChan: make(chan struct{}),
	}
}

// AddNode adds a database node to the replication manager
func (rm *ReplicationManager) AddNode(connManager *database.ConnectionManager) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	nodeName := connManager.NodeName()
	role := connManager.Role()

	// Add to health checker
	if err := rm.healthChecker.AddNode(connManager); err != nil {
		return fmt.Errorf("failed to add node to health checker: %w", err)
	}

	// Add to appropriate node map
	switch role {
	case config.RoleMaster:
		rm.masterNodes[nodeName] = connManager
		// Set as current master if we don't have one
		if rm.currentMaster == nil {
			rm.currentMaster = connManager
		}
	case config.RoleSlave:
		rm.slaveNodes[nodeName] = connManager
	default:
		return fmt.Errorf("unknown node role: %s", role)
	}

	return nil
}

// RemoveNode removes a database node from the replication manager
func (rm *ReplicationManager) RemoveNode(nodeName string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Remove from health checker
	if err := rm.healthChecker.RemoveNode(nodeName); err != nil {
		return fmt.Errorf("failed to remove node from health checker: %w", err)
	}

	// Remove from node maps
	if connManager, exists := rm.masterNodes[nodeName]; exists {
		delete(rm.masterNodes, nodeName)
		// If this was the current master, clear it
		if rm.currentMaster == connManager {
			rm.currentMaster = nil
			// Try to set another master if available
			for _, master := range rm.masterNodes {
				rm.currentMaster = master
				break
			}
		}
	}

	if _, exists := rm.slaveNodes[nodeName]; exists {
		delete(rm.slaveNodes, nodeName)
	}

	return nil
}

// GetMasterNodes returns all master nodes
func (rm *ReplicationManager) GetMasterNodes() []*database.ConnectionManager {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	masters := make([]*database.ConnectionManager, 0, len(rm.masterNodes))
	for _, master := range rm.masterNodes {
		masters = append(masters, master)
	}

	return masters
}

// GetSlaveNodes returns all slave nodes
func (rm *ReplicationManager) GetSlaveNodes() []*database.ConnectionManager {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	slaves := make([]*database.ConnectionManager, 0, len(rm.slaveNodes))
	for _, slave := range rm.slaveNodes {
		slaves = append(slaves, slave)
	}

	return slaves
}

// GetCurrentMaster returns the current master node
func (rm *ReplicationManager) GetCurrentMaster() *database.ConnectionManager {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return rm.currentMaster
}

// PromoteSlave promotes a slave node to master
func (rm *ReplicationManager) PromoteSlave(ctx context.Context, slaveName string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Find the slave node
	slaveConn, exists := rm.slaveNodes[slaveName]
	if !exists {
		return fmt.Errorf("slave node %s not found", slaveName)
	}

	// In a real implementation, this would execute PostgreSQL commands to promote the slave
	// For now, we'll simulate the promotion by moving the node from slaves to masters

	// Remove from slaves
	delete(rm.slaveNodes, slaveName)

	// Add to masters
	rm.masterNodes[slaveName] = slaveConn

	// Set as current master
	rm.currentMaster = slaveConn

	// In a real implementation, you would:
	// 1. Stop replication on the slave
	// 2. Promote it to master (pg_promote() or similar)
	// 3. Update configuration
	// 4. Reconfigure other slaves to follow the new master

	return nil
}

// HandleFailover handles automatic failover when master fails
func (rm *ReplicationManager) HandleFailover(ctx context.Context) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Check if current master is healthy
	if rm.currentMaster != nil {
		status, err := rm.healthChecker.GetNodeStatus(rm.currentMaster.NodeName())
		if err == nil && status.IsHealthy {
			return nil // Master is healthy, no failover needed
		}
	}

	// Find a healthy slave to promote
	var candidateSlave *database.ConnectionManager
	var candidateName string

	for slaveName, slaveConn := range rm.slaveNodes {
		status, err := rm.healthChecker.GetNodeStatus(slaveName)
		if err == nil && status.IsHealthy {
			candidateSlave = slaveConn
			candidateName = slaveName
			break
		}
	}

	if candidateSlave == nil {
		return errors.New("no healthy slave available for promotion")
	}

	// Promote the candidate slave
	if err := rm.promoteSlave(ctx, candidateName, candidateSlave); err != nil {
		return fmt.Errorf("failed to promote slave %s: %w", candidateName, err)
	}

	// Reconfigure remaining slaves to follow the new master
	newMasterHost := "localhost" // In real implementation, get from connection config
	newMasterPort := 5432        // In real implementation, get from connection config

	if err := rm.reconfigureSlaves(ctx, newMasterHost, newMasterPort); err != nil {
		return fmt.Errorf("failed to reconfigure slaves: %w", err)
	}

	return nil
}

// promoteSlave internal method to promote a slave (without locking)
func (rm *ReplicationManager) promoteSlave(ctx context.Context, slaveName string, slaveConn *database.ConnectionManager) error {
	// Remove from slaves
	delete(rm.slaveNodes, slaveName)

	// Add to masters
	rm.masterNodes[slaveName] = slaveConn

	// Set as current master
	rm.currentMaster = slaveConn

	return nil
}

// ReconfigureSlaves reconfigures all slave nodes to follow a new master
func (rm *ReplicationManager) ReconfigureSlaves(ctx context.Context, masterHost string, masterPort int) error {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return rm.reconfigureSlaves(ctx, masterHost, masterPort)
}

// reconfigureSlaves internal method to reconfigure slaves (without locking)
func (rm *ReplicationManager) reconfigureSlaves(ctx context.Context, masterHost string, masterPort int) error {
	// In a real implementation, this would:
	// 1. Stop replication on each slave
	// 2. Update primary_conninfo to point to the new master
	// 3. Restart replication

	for slaveName := range rm.slaveNodes {
		// Simulate reconfiguration
		_ = slaveName
		_ = masterHost
		_ = masterPort

		// In real implementation:
		// query := fmt.Sprintf("SELECT pg_reload_conf(); ALTER SYSTEM SET primary_conninfo = 'host=%s port=%d user=replicator';", masterHost, masterPort)
		// _, err := slaveConn.ExecuteQuery(ctx, query)
		// if err != nil {
		//     return fmt.Errorf("failed to reconfigure slave %s: %w", slaveName, err)
		// }
	}

	return nil
}

// StartFailoverMonitoring starts automatic failover monitoring
func (rm *ReplicationManager) StartFailoverMonitoring(ctx context.Context) error {
	rm.failoverMu.Lock()
	defer rm.failoverMu.Unlock()

	if rm.failoverRunning {
		return errors.New("failover monitoring is already running")
	}

	rm.failoverRunning = true
	rm.failoverStopChan = make(chan struct{})

	go rm.failoverMonitoringLoop(ctx)

	return nil
}

// StopFailoverMonitoring stops automatic failover monitoring
func (rm *ReplicationManager) StopFailoverMonitoring() {
	rm.failoverMu.Lock()
	defer rm.failoverMu.Unlock()

	if !rm.failoverRunning {
		return
	}

	rm.failoverRunning = false
	close(rm.failoverStopChan)
}

// failoverMonitoringLoop runs the failover monitoring loop
func (rm *ReplicationManager) failoverMonitoringLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second) // Check every 10 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-rm.failoverStopChan:
			return
		case <-ticker.C:
			if err := rm.HandleFailover(ctx); err != nil {
				// In a real implementation, this would be logged
				_ = err
			}
		}
	}
}

// GetNodeStatus returns the health status of a node
func (rm *ReplicationManager) GetNodeStatus(nodeName string) (*health.NodeStatus, error) {
	return rm.healthChecker.GetNodeStatus(nodeName)
}

// GetAllNodeStatuses returns the health status of all nodes
func (rm *ReplicationManager) GetAllNodeStatuses() map[string]*health.NodeStatus {
	return rm.healthChecker.GetAllStatuses()
}
