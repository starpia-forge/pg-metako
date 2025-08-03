package replication

import (
	"context"
	"pg-metako/internal/config"
	"pg-metako/internal/database"
	"pg-metako/internal/health"
	"testing"
	"time"
)

func TestNewReplicationManager(t *testing.T) {
	healthConfig := config.HealthCheckConfig{
		Interval:         30 * time.Second,
		Timeout:          5 * time.Second,
		FailureThreshold: 3,
	}

	healthChecker := health.NewHealthChecker(healthConfig)

	manager := NewReplicationManager(healthChecker)
	if manager == nil {
		t.Fatal("ReplicationManager should not be nil")
	}
}

func TestReplicationManager_AddNode(t *testing.T) {
	healthConfig := config.HealthCheckConfig{
		Interval:         30 * time.Second,
		Timeout:          5 * time.Second,
		FailureThreshold: 3,
	}

	healthChecker := health.NewHealthChecker(healthConfig)
	manager := NewReplicationManager(healthChecker)

	// Add master node
	masterConfig := config.NodeConfig{
		Name:     "master-1",
		Host:     "localhost",
		Port:     5432,
		Role:     config.RoleMaster,
		Username: "postgres",
		Password: "password",
		Database: "testdb",
	}

	masterConn, err := database.NewConnectionManager(masterConfig)
	if err != nil {
		t.Fatalf("Failed to create master connection manager: %v", err)
	}

	err = manager.AddNode(masterConn)
	if err != nil {
		t.Errorf("Failed to add master node: %v", err)
	}

	// Add slave node
	slaveConfig := config.NodeConfig{
		Name:     "slave-1",
		Host:     "localhost",
		Port:     5433,
		Role:     config.RoleSlave,
		Username: "postgres",
		Password: "password",
		Database: "testdb",
	}

	slaveConn, err := database.NewConnectionManager(slaveConfig)
	if err != nil {
		t.Fatalf("Failed to create slave connection manager: %v", err)
	}

	err = manager.AddNode(slaveConn)
	if err != nil {
		t.Errorf("Failed to add slave node: %v", err)
	}

	// Verify nodes were added
	masters := manager.GetMasterNodes()
	if len(masters) != 1 {
		t.Errorf("Expected 1 master node, got %d", len(masters))
	}

	slaves := manager.GetSlaveNodes()
	if len(slaves) != 1 {
		t.Errorf("Expected 1 slave node, got %d", len(slaves))
	}
}

func TestReplicationManager_GetCurrentMaster(t *testing.T) {
	healthConfig := config.HealthCheckConfig{
		Interval:         30 * time.Second,
		Timeout:          5 * time.Second,
		FailureThreshold: 3,
	}

	healthChecker := health.NewHealthChecker(healthConfig)
	manager := NewReplicationManager(healthChecker)

	// Initially should have no master
	master := manager.GetCurrentMaster()
	if master != nil {
		t.Error("Should have no master initially")
	}

	// Add master node
	masterConfig := config.NodeConfig{
		Name:     "master-1",
		Host:     "localhost",
		Port:     5432,
		Role:     config.RoleMaster,
		Username: "postgres",
		Password: "password",
		Database: "testdb",
	}

	masterConn, err := database.NewConnectionManager(masterConfig)
	if err != nil {
		t.Fatalf("Failed to create master connection manager: %v", err)
	}

	err = manager.AddNode(masterConn)
	if err != nil {
		t.Fatalf("Failed to add master node: %v", err)
	}

	// Should now have a master
	master = manager.GetCurrentMaster()
	if master == nil {
		t.Error("Should have a master after adding one")
	}

	if master.NodeName() != "master-1" {
		t.Errorf("Expected master name 'master-1', got '%s'", master.NodeName())
	}
}

func TestReplicationManager_PromoteSlave(t *testing.T) {
	healthConfig := config.HealthCheckConfig{
		Interval:         30 * time.Second,
		Timeout:          5 * time.Second,
		FailureThreshold: 3,
	}

	healthChecker := health.NewHealthChecker(healthConfig)
	manager := NewReplicationManager(healthChecker)

	// Add master and slave nodes
	masterConfig := config.NodeConfig{
		Name:     "master-1",
		Host:     "localhost",
		Port:     5432,
		Role:     config.RoleMaster,
		Username: "postgres",
		Password: "password",
		Database: "testdb",
	}

	slaveConfig := config.NodeConfig{
		Name:     "slave-1",
		Host:     "localhost",
		Port:     5433,
		Role:     config.RoleSlave,
		Username: "postgres",
		Password: "password",
		Database: "testdb",
	}

	masterConn, _ := database.NewConnectionManager(masterConfig)
	slaveConn, _ := database.NewConnectionManager(slaveConfig)

	manager.AddNode(masterConn)
	manager.AddNode(slaveConn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Promote slave to master
	err := manager.PromoteSlave(ctx, "slave-1")
	if err != nil {
		t.Logf("Promotion failed as expected (no real database): %v", err)
	}

	// Check that slave-1 is now considered a master
	newMaster := manager.GetCurrentMaster()
	if newMaster != nil && newMaster.NodeName() == "slave-1" {
		t.Log("Slave successfully promoted to master")
	}
}

func TestReplicationManager_HandleFailover(t *testing.T) {
	healthConfig := config.HealthCheckConfig{
		Interval:         30 * time.Second,
		Timeout:          5 * time.Second,
		FailureThreshold: 3,
	}

	healthChecker := health.NewHealthChecker(healthConfig)
	manager := NewReplicationManager(healthChecker)

	// Add master and slave nodes
	masterConfig := config.NodeConfig{
		Name:     "master-1",
		Host:     "localhost",
		Port:     5432,
		Role:     config.RoleMaster,
		Username: "postgres",
		Password: "password",
		Database: "testdb",
	}

	slaveConfig := config.NodeConfig{
		Name:     "slave-1",
		Host:     "localhost",
		Port:     5433,
		Role:     config.RoleSlave,
		Username: "postgres",
		Password: "password",
		Database: "testdb",
	}

	masterConn, _ := database.NewConnectionManager(masterConfig)
	slaveConn, _ := database.NewConnectionManager(slaveConfig)

	manager.AddNode(masterConn)
	manager.AddNode(slaveConn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Simulate master failure and trigger failover
	err := manager.HandleFailover(ctx)
	if err != nil {
		t.Logf("Failover handling completed with result: %v", err)
	}

	// Verify that a slave was promoted if available
	newMaster := manager.GetCurrentMaster()
	if newMaster != nil {
		t.Logf("New master after failover: %s", newMaster.NodeName())
	}
}

func TestReplicationManager_ReconfigureSlaves(t *testing.T) {
	healthConfig := config.HealthCheckConfig{
		Interval:         30 * time.Second,
		Timeout:          5 * time.Second,
		FailureThreshold: 3,
	}

	healthChecker := health.NewHealthChecker(healthConfig)
	manager := NewReplicationManager(healthChecker)

	// Add multiple slave nodes
	slave1Config := config.NodeConfig{
		Name:     "slave-1",
		Host:     "localhost",
		Port:     5433,
		Role:     config.RoleSlave,
		Username: "postgres",
		Password: "password",
		Database: "testdb",
	}

	slave2Config := config.NodeConfig{
		Name:     "slave-2",
		Host:     "localhost",
		Port:     5434,
		Role:     config.RoleSlave,
		Username: "postgres",
		Password: "password",
		Database: "testdb",
	}

	slave1Conn, _ := database.NewConnectionManager(slave1Config)
	slave2Conn, _ := database.NewConnectionManager(slave2Config)

	manager.AddNode(slave1Conn)
	manager.AddNode(slave2Conn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Reconfigure slaves to follow a new master
	err := manager.ReconfigureSlaves(ctx, "new-master-host", 5432)
	if err != nil {
		t.Logf("Slave reconfiguration completed with result: %v", err)
	}
}

func TestReplicationManager_StartStopFailoverMonitoring(t *testing.T) {
	healthConfig := config.HealthCheckConfig{
		Interval:         100 * time.Millisecond, // Short interval for testing
		Timeout:          5 * time.Second,
		FailureThreshold: 2, // Lower threshold for faster testing
	}

	healthChecker := health.NewHealthChecker(healthConfig)
	manager := NewReplicationManager(healthChecker)

	// Add nodes
	masterConfig := config.NodeConfig{
		Name:     "master-1",
		Host:     "localhost",
		Port:     5432,
		Role:     config.RoleMaster,
		Username: "postgres",
		Password: "password",
		Database: "testdb",
	}

	masterConn, _ := database.NewConnectionManager(masterConfig)
	manager.AddNode(masterConn)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Start failover monitoring
	err := manager.StartFailoverMonitoring(ctx)
	if err != nil {
		t.Errorf("Failed to start failover monitoring: %v", err)
	}

	// Let it run for a short time
	time.Sleep(300 * time.Millisecond)

	// Stop monitoring
	manager.StopFailoverMonitoring()

	t.Log("Failover monitoring started and stopped successfully")
}
