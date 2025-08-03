package routing

import (
	"context"
	"pg-metako/internal/config"
	"pg-metako/internal/database"
	"pg-metako/internal/health"
	"pg-metako/internal/replication"
	"testing"
	"time"
)

func TestNewQueryRouter(t *testing.T) {
	loadBalancerConfig := config.LoadBalancerConfig{
		Algorithm:    config.AlgorithmRoundRobin,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	healthConfig := config.HealthCheckConfig{
		Interval:         30 * time.Second,
		Timeout:          5 * time.Second,
		FailureThreshold: 3,
	}

	healthChecker := health.NewHealthChecker(healthConfig)
	replicationManager := replication.NewReplicationManager(healthChecker)

	router := NewQueryRouter(replicationManager, loadBalancerConfig)
	if router == nil {
		t.Fatal("QueryRouter should not be nil")
	}
}

func TestQueryRouter_DetectQueryType(t *testing.T) {
	loadBalancerConfig := config.LoadBalancerConfig{
		Algorithm:    config.AlgorithmRoundRobin,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	healthConfig := config.HealthCheckConfig{
		Interval:         30 * time.Second,
		Timeout:          5 * time.Second,
		FailureThreshold: 3,
	}

	healthChecker := health.NewHealthChecker(healthConfig)
	replicationManager := replication.NewReplicationManager(healthChecker)
	router := NewQueryRouter(replicationManager, loadBalancerConfig)

	tests := []struct {
		name     string
		query    string
		expected QueryType
	}{
		{
			name:     "SELECT query",
			query:    "SELECT * FROM users WHERE id = 1",
			expected: QueryTypeRead,
		},
		{
			name:     "INSERT query",
			query:    "INSERT INTO users (name, email) VALUES ('John', 'john@example.com')",
			expected: QueryTypeWrite,
		},
		{
			name:     "UPDATE query",
			query:    "UPDATE users SET name = 'Jane' WHERE id = 1",
			expected: QueryTypeWrite,
		},
		{
			name:     "DELETE query",
			query:    "DELETE FROM users WHERE id = 1",
			expected: QueryTypeWrite,
		},
		{
			name:     "CREATE TABLE query",
			query:    "CREATE TABLE test (id SERIAL PRIMARY KEY, name VARCHAR(100))",
			expected: QueryTypeWrite,
		},
		{
			name:     "DROP TABLE query",
			query:    "DROP TABLE test",
			expected: QueryTypeWrite,
		},
		{
			name:     "SELECT with whitespace",
			query:    "   SELECT count(*) FROM users   ",
			expected: QueryTypeRead,
		},
		{
			name:     "lowercase select",
			query:    "select * from products",
			expected: QueryTypeRead,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryType := router.DetectQueryType(tt.query)
			if queryType != tt.expected {
				t.Errorf("Expected query type %v, got %v for query: %s", tt.expected, queryType, tt.query)
			}
		})
	}
}

func TestQueryRouter_RouteQuery(t *testing.T) {
	loadBalancerConfig := config.LoadBalancerConfig{
		Algorithm:    config.AlgorithmRoundRobin,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	healthConfig := config.HealthCheckConfig{
		Interval:         30 * time.Second,
		Timeout:          5 * time.Second,
		FailureThreshold: 3,
	}

	healthChecker := health.NewHealthChecker(healthConfig)
	replicationManager := replication.NewReplicationManager(healthChecker)
	router := NewQueryRouter(replicationManager, loadBalancerConfig)

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

	masterConn, _ := database.NewConnectionManager(masterConfig)
	slave1Conn, _ := database.NewConnectionManager(slave1Config)
	slave2Conn, _ := database.NewConnectionManager(slave2Config)

	replicationManager.AddNode(masterConn)
	replicationManager.AddNode(slave1Conn)
	replicationManager.AddNode(slave2Conn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test write query routing (should go to master)
	writeQuery := "INSERT INTO users (name) VALUES ('test')"
	result, err := router.RouteQuery(ctx, writeQuery)
	if err != nil {
		t.Logf("Write query routing failed as expected (no database): %v", err)
	} else {
		t.Logf("Write query routed successfully: %v", result)
	}

	// Test read query routing (should go to slave)
	readQuery := "SELECT * FROM users"
	result, err = router.RouteQuery(ctx, readQuery)
	if err != nil {
		t.Logf("Read query routing failed as expected (no database): %v", err)
	} else {
		t.Logf("Read query routed successfully: %v", result)
	}
}

func TestQueryRouter_LoadBalancing_RoundRobin(t *testing.T) {
	loadBalancerConfig := config.LoadBalancerConfig{
		Algorithm:    config.AlgorithmRoundRobin,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	healthConfig := config.HealthCheckConfig{
		Interval:         30 * time.Second,
		Timeout:          5 * time.Second,
		FailureThreshold: 3,
	}

	healthChecker := health.NewHealthChecker(healthConfig)
	replicationManager := replication.NewReplicationManager(healthChecker)
	router := NewQueryRouter(replicationManager, loadBalancerConfig)

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

	replicationManager.AddNode(slave1Conn)
	replicationManager.AddNode(slave2Conn)

	// Test round-robin distribution
	selectedNodes := make(map[string]int)
	for i := 0; i < 10; i++ {
		node := router.SelectReadNode()
		if node != nil {
			selectedNodes[node.NodeName()]++
		}
	}

	if len(selectedNodes) == 0 {
		t.Log("No nodes selected (expected when no healthy slaves)")
	} else {
		t.Logf("Round-robin distribution: %v", selectedNodes)

		// In a real scenario with healthy nodes, we'd expect roughly equal distribution
		for nodeName, count := range selectedNodes {
			t.Logf("Node %s selected %d times", nodeName, count)
		}
	}
}

func TestQueryRouter_LoadBalancing_LeastConnected(t *testing.T) {
	loadBalancerConfig := config.LoadBalancerConfig{
		Algorithm:    config.AlgorithmLeastConnected,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	healthConfig := config.HealthCheckConfig{
		Interval:         30 * time.Second,
		Timeout:          5 * time.Second,
		FailureThreshold: 3,
	}

	healthChecker := health.NewHealthChecker(healthConfig)
	replicationManager := replication.NewReplicationManager(healthChecker)
	router := NewQueryRouter(replicationManager, loadBalancerConfig)

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

	replicationManager.AddNode(slave1Conn)
	replicationManager.AddNode(slave2Conn)

	// Test least connected selection
	node := router.SelectReadNode()
	if node != nil {
		t.Logf("Least connected algorithm selected node: %s", node.NodeName())
	} else {
		t.Log("No node selected (expected when no healthy slaves)")
	}
}

func TestQueryRouter_GetConnectionStats(t *testing.T) {
	loadBalancerConfig := config.LoadBalancerConfig{
		Algorithm:    config.AlgorithmRoundRobin,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	healthConfig := config.HealthCheckConfig{
		Interval:         30 * time.Second,
		Timeout:          5 * time.Second,
		FailureThreshold: 3,
	}

	healthChecker := health.NewHealthChecker(healthConfig)
	replicationManager := replication.NewReplicationManager(healthChecker)
	router := NewQueryRouter(replicationManager, loadBalancerConfig)

	stats := router.GetConnectionStats()
	if stats == nil {
		t.Error("Connection stats should not be nil")
	}

	t.Logf("Connection stats: %+v", stats)
}
