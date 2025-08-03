package health

import (
	"context"
	"errors"
	"fmt"
	"pg-metako/internal/config"
	"pg-metako/internal/database"
	"sync"
	"time"
)

// NodeStatus represents the health status of a database node
type NodeStatus struct {
	NodeName     string
	IsHealthy    bool
	FailureCount int
	LastChecked  time.Time
	LastError    error
}

// HealthChecker monitors the health of database nodes
type HealthChecker struct {
	config       config.HealthCheckConfig
	nodes        map[string]*database.ConnectionManager
	statuses     map[string]*NodeStatus
	mu           sync.RWMutex
	stopChan     chan struct{}
	monitoring   bool
	monitoringMu sync.Mutex
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(config config.HealthCheckConfig) *HealthChecker {
	return &HealthChecker{
		config:   config,
		nodes:    make(map[string]*database.ConnectionManager),
		statuses: make(map[string]*NodeStatus),
		stopChan: make(chan struct{}),
	}
}

// AddNode adds a database node to be monitored
func (hc *HealthChecker) AddNode(connManager *database.ConnectionManager) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	nodeName := connManager.NodeName()

	// Check if node already exists
	if _, exists := hc.nodes[nodeName]; exists {
		return fmt.Errorf("node %s already exists", nodeName)
	}

	hc.nodes[nodeName] = connManager
	hc.statuses[nodeName] = &NodeStatus{
		NodeName:     nodeName,
		IsHealthy:    false,
		FailureCount: 0,
		LastChecked:  time.Time{},
		LastError:    nil,
	}

	return nil
}

// RemoveNode removes a database node from monitoring
func (hc *HealthChecker) RemoveNode(nodeName string) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if _, exists := hc.nodes[nodeName]; !exists {
		return fmt.Errorf("node %s not found", nodeName)
	}

	delete(hc.nodes, nodeName)
	delete(hc.statuses, nodeName)

	return nil
}

// GetNodeStatus returns the current health status of a node
func (hc *HealthChecker) GetNodeStatus(nodeName string) (*NodeStatus, error) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	status, exists := hc.statuses[nodeName]
	if !exists {
		return nil, fmt.Errorf("node %s not found", nodeName)
	}

	// Return a copy to avoid race conditions
	statusCopy := &NodeStatus{
		NodeName:     status.NodeName,
		IsHealthy:    status.IsHealthy,
		FailureCount: status.FailureCount,
		LastChecked:  status.LastChecked,
		LastError:    status.LastError,
	}

	return statusCopy, nil
}

// CheckHealth performs a health check on a specific node
func (hc *HealthChecker) CheckHealth(ctx context.Context, nodeName string) error {
	hc.mu.RLock()
	connManager, exists := hc.nodes[nodeName]
	if !exists {
		hc.mu.RUnlock()
		return fmt.Errorf("node %s not found", nodeName)
	}
	hc.mu.RUnlock()

	// Create a timeout context for the health check
	checkCtx, cancel := context.WithTimeout(ctx, hc.config.Timeout)
	defer cancel()

	// Perform the health check
	isHealthy := connManager.IsHealthy(checkCtx)

	// Update status
	hc.mu.Lock()
	defer hc.mu.Unlock()

	status := hc.statuses[nodeName]
	status.LastChecked = time.Now()

	if isHealthy {
		status.IsHealthy = true
		status.FailureCount = 0
		status.LastError = nil
	} else {
		status.FailureCount++
		status.LastError = errors.New("health check failed")

		// Mark as unhealthy if failure threshold is reached
		if status.FailureCount >= hc.config.FailureThreshold {
			status.IsHealthy = false
		}
	}

	if !isHealthy {
		return status.LastError
	}

	return nil
}

// StartMonitoring starts continuous health monitoring of all nodes
func (hc *HealthChecker) StartMonitoring(ctx context.Context) error {
	hc.monitoringMu.Lock()
	defer hc.monitoringMu.Unlock()

	if hc.monitoring {
		return errors.New("monitoring is already running")
	}

	hc.monitoring = true
	hc.stopChan = make(chan struct{})

	go hc.monitoringLoop(ctx)

	return nil
}

// StopMonitoring stops the continuous health monitoring
func (hc *HealthChecker) StopMonitoring() {
	hc.monitoringMu.Lock()
	defer hc.monitoringMu.Unlock()

	if !hc.monitoring {
		return
	}

	hc.monitoring = false
	close(hc.stopChan)
}

// monitoringLoop runs the continuous health monitoring
func (hc *HealthChecker) monitoringLoop(ctx context.Context) {
	ticker := time.NewTicker(hc.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-hc.stopChan:
			return
		case <-ticker.C:
			hc.checkAllNodes(ctx)
		}
	}
}

// checkAllNodes performs health checks on all monitored nodes
func (hc *HealthChecker) checkAllNodes(ctx context.Context) {
	hc.mu.RLock()
	nodeNames := make([]string, 0, len(hc.nodes))
	for nodeName := range hc.nodes {
		nodeNames = append(nodeNames, nodeName)
	}
	hc.mu.RUnlock()

	// Check each node concurrently
	var wg sync.WaitGroup
	for _, nodeName := range nodeNames {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			hc.CheckHealth(ctx, name)
		}(nodeName)
	}

	wg.Wait()
}

// GetAllStatuses returns the health status of all monitored nodes
func (hc *HealthChecker) GetAllStatuses() map[string]*NodeStatus {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	result := make(map[string]*NodeStatus)
	for nodeName, status := range hc.statuses {
		result[nodeName] = &NodeStatus{
			NodeName:     status.NodeName,
			IsHealthy:    status.IsHealthy,
			FailureCount: status.FailureCount,
			LastChecked:  status.LastChecked,
			LastError:    status.LastError,
		}
	}

	return result
}
