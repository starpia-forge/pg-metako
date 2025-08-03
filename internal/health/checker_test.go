package health

import (
	"context"
	"pg-metako/internal/config"
	"pg-metako/internal/database"
	"testing"
	"time"
)

func TestNewHealthChecker(t *testing.T) {
	healthConfig := config.HealthCheckConfig{
		Interval:         30 * time.Second,
		Timeout:          5 * time.Second,
		FailureThreshold: 3,
	}

	checker := NewHealthChecker(healthConfig)
	if checker == nil {
		t.Fatal("HealthChecker should not be nil")
	}
}

func TestHealthChecker_AddNode(t *testing.T) {
	healthConfig := config.HealthCheckConfig{
		Interval:         30 * time.Second,
		Timeout:          5 * time.Second,
		FailureThreshold: 3,
	}

	checker := NewHealthChecker(healthConfig)

	nodeConfig := config.NodeConfig{
		Name:     "test-node",
		Host:     "localhost",
		Port:     5432,
		Role:     config.RoleMaster,
		Username: "postgres",
		Password: "password",
		Database: "testdb",
	}

	connManager, err := database.NewConnectionManager(nodeConfig)
	if err != nil {
		t.Fatalf("Failed to create connection manager: %v", err)
	}

	err = checker.AddNode(connManager)
	if err != nil {
		t.Errorf("Failed to add node to health checker: %v", err)
	}

	// Adding the same node again should return an error
	err = checker.AddNode(connManager)
	if err == nil {
		t.Error("Expected error when adding duplicate node")
	}
}

func TestHealthChecker_RemoveNode(t *testing.T) {
	healthConfig := config.HealthCheckConfig{
		Interval:         30 * time.Second,
		Timeout:          5 * time.Second,
		FailureThreshold: 3,
	}

	checker := NewHealthChecker(healthConfig)

	nodeConfig := config.NodeConfig{
		Name:     "test-node",
		Host:     "localhost",
		Port:     5432,
		Role:     config.RoleMaster,
		Username: "postgres",
		Password: "password",
		Database: "testdb",
	}

	connManager, err := database.NewConnectionManager(nodeConfig)
	if err != nil {
		t.Fatalf("Failed to create connection manager: %v", err)
	}

	// Add node first
	err = checker.AddNode(connManager)
	if err != nil {
		t.Fatalf("Failed to add node: %v", err)
	}

	// Remove node
	err = checker.RemoveNode("test-node")
	if err != nil {
		t.Errorf("Failed to remove node: %v", err)
	}

	// Removing non-existent node should return error
	err = checker.RemoveNode("non-existent")
	if err == nil {
		t.Error("Expected error when removing non-existent node")
	}
}

func TestHealthChecker_GetNodeStatus(t *testing.T) {
	healthConfig := config.HealthCheckConfig{
		Interval:         30 * time.Second,
		Timeout:          5 * time.Second,
		FailureThreshold: 3,
	}

	checker := NewHealthChecker(healthConfig)

	nodeConfig := config.NodeConfig{
		Name:     "test-node",
		Host:     "localhost",
		Port:     5432,
		Role:     config.RoleMaster,
		Username: "postgres",
		Password: "password",
		Database: "testdb",
	}

	connManager, err := database.NewConnectionManager(nodeConfig)
	if err != nil {
		t.Fatalf("Failed to create connection manager: %v", err)
	}

	err = checker.AddNode(connManager)
	if err != nil {
		t.Fatalf("Failed to add node: %v", err)
	}

	status, err := checker.GetNodeStatus("test-node")
	if err != nil {
		t.Errorf("Failed to get node status: %v", err)
	}

	if status == nil {
		t.Error("Node status should not be nil")
	}

	if status.NodeName != "test-node" {
		t.Errorf("Expected node name 'test-node', got '%s'", status.NodeName)
	}

	// Initially should be unhealthy since not connected
	if status.IsHealthy {
		t.Error("Node should be unhealthy initially")
	}
}

func TestHealthChecker_CheckHealth(t *testing.T) {
	healthConfig := config.HealthCheckConfig{
		Interval:         30 * time.Second,
		Timeout:          5 * time.Second,
		FailureThreshold: 3,
	}

	checker := NewHealthChecker(healthConfig)

	nodeConfig := config.NodeConfig{
		Name:     "test-node",
		Host:     "localhost",
		Port:     5432,
		Role:     config.RoleMaster,
		Username: "postgres",
		Password: "password",
		Database: "testdb",
	}

	connManager, err := database.NewConnectionManager(nodeConfig)
	if err != nil {
		t.Fatalf("Failed to create connection manager: %v", err)
	}

	err = checker.AddNode(connManager)
	if err != nil {
		t.Fatalf("Failed to add node: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Perform health check
	err = checker.CheckHealth(ctx, "test-node")
	if err != nil {
		t.Logf("Health check failed as expected (no database): %v", err)
	}

	// Check status after health check
	status, err := checker.GetNodeStatus("test-node")
	if err != nil {
		t.Errorf("Failed to get node status after health check: %v", err)
	}

	if status.FailureCount == 0 {
		t.Error("Failure count should be incremented after failed health check")
	}
}

func TestHealthChecker_StartStopMonitoring(t *testing.T) {
	healthConfig := config.HealthCheckConfig{
		Interval:         100 * time.Millisecond, // Short interval for testing
		Timeout:          5 * time.Second,
		FailureThreshold: 3,
	}

	checker := NewHealthChecker(healthConfig)

	nodeConfig := config.NodeConfig{
		Name:     "test-node",
		Host:     "localhost",
		Port:     5432,
		Role:     config.RoleMaster,
		Username: "postgres",
		Password: "password",
		Database: "testdb",
	}

	connManager, err := database.NewConnectionManager(nodeConfig)
	if err != nil {
		t.Fatalf("Failed to create connection manager: %v", err)
	}

	err = checker.AddNode(connManager)
	if err != nil {
		t.Fatalf("Failed to add node: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Start monitoring
	err = checker.StartMonitoring(ctx)
	if err != nil {
		t.Errorf("Failed to start monitoring: %v", err)
	}

	// Let it run for a short time
	time.Sleep(300 * time.Millisecond)

	// Stop monitoring
	checker.StopMonitoring()

	// Check that some health checks were performed
	status, err := checker.GetNodeStatus("test-node")
	if err != nil {
		t.Errorf("Failed to get node status: %v", err)
	}

	if status.LastChecked.IsZero() {
		t.Error("LastChecked should be set after monitoring")
	}
}
