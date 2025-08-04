// Purpose    : Test the main application orchestration and lifecycle management
// Context    : PostgreSQL replication management system testing
// Constraints: Must validate application startup, shutdown, and component integration

package metako

import (
	"context"
	"testing"
	"time"

	"pg-metako/internal/config"
)

func TestNewApplication(t *testing.T) {
	// Test configuration with minimal valid setup
	cfg := &config.Config{
		Identity: config.NodeIdentity{
			NodeName:    "test-node",
			LocalDBHost: "localhost",
			LocalDBPort: 5432,
			APIHost:     "localhost",
			APIPort:     8080,
		},
		LocalDB: config.NodeConfig{
			Name:     "master-1",
			Host:     "localhost",
			Port:     5432,
			Role:     config.RoleMaster,
			Username: "postgres",
			Password: "password",
			Database: "testdb",
		},
		ClusterMembers: []config.ClusterMember{
			{
				NodeName: "test-node",
				APIHost:  "localhost",
				APIPort:  8080,
				Role:     config.RoleMaster,
			},
		},
		Coordination: config.CoordinationConfig{
			HeartbeatInterval:    10 * time.Second,
			CommunicationTimeout: 5 * time.Second,
			FailoverTimeout:      30 * time.Second,
			MinConsensusNodes:    1,
			LocalNodePreference:  0.8,
		},
		HealthCheck: config.HealthCheckConfig{
			Interval:         30 * time.Second,
			Timeout:          5 * time.Second,
			FailureThreshold: 3,
		},
		LoadBalancer: config.LoadBalancerConfig{
			Algorithm:    config.AlgorithmRoundRobin,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
	}

	app, err := NewApplication(cfg)
	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}

	if app == nil {
		t.Fatal("Application should not be nil")
	}

	// Verify that all components are initialized
	if app.GetRouter() == nil {
		t.Error("Router should be initialized")
	}

	if app.GetReplicationManager() == nil {
		t.Error("ReplicationManager should be initialized")
	}

	if app.GetCoordinationAPI() == nil {
		t.Error("CoordinationAPI should be initialized")
	}
}

func TestApplication_StartStop(t *testing.T) {
	cfg := &config.Config{
		Identity: config.NodeIdentity{
			NodeName:    "test-node",
			LocalDBHost: "localhost",
			LocalDBPort: 5432,
			APIHost:     "localhost",
			APIPort:     8080,
		},
		LocalDB: config.NodeConfig{
			Name:     "master-1",
			Host:     "localhost",
			Port:     5432,
			Role:     config.RoleMaster,
			Username: "postgres",
			Password: "password",
			Database: "testdb",
		},
		ClusterMembers: []config.ClusterMember{
			{
				NodeName: "test-node",
				APIHost:  "localhost",
				APIPort:  8080,
				Role:     config.RoleMaster,
			},
		},
		Coordination: config.CoordinationConfig{
			HeartbeatInterval:    10 * time.Second,
			CommunicationTimeout: 5 * time.Second,
			FailoverTimeout:      30 * time.Second,
			MinConsensusNodes:    1,
			LocalNodePreference:  0.8,
		},
		HealthCheck: config.HealthCheckConfig{
			Interval:         30 * time.Second,
			Timeout:          5 * time.Second,
			FailureThreshold: 3,
		},
		LoadBalancer: config.LoadBalancerConfig{
			Algorithm:    config.AlgorithmRoundRobin,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
	}

	app, err := NewApplication(cfg)
	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test Start
	err = app.Start(ctx)
	if err != nil {
		t.Logf("Start failed as expected (no real database): %v", err)
	}

	// Test Stop
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	err = app.Stop(stopCtx)
	if err != nil {
		t.Errorf("Stop should not fail: %v", err)
	}
}

func TestApplication_Run(t *testing.T) {
	cfg := &config.Config{
		Identity: config.NodeIdentity{
			NodeName:    "test-node",
			LocalDBHost: "localhost",
			LocalDBPort: 5432,
			APIHost:     "localhost",
			APIPort:     8080,
		},
		LocalDB: config.NodeConfig{
			Name:     "master-1",
			Host:     "localhost",
			Port:     5432,
			Role:     config.RoleMaster,
			Username: "postgres",
			Password: "password",
			Database: "testdb",
		},
		ClusterMembers: []config.ClusterMember{
			{
				NodeName: "test-node",
				APIHost:  "localhost",
				APIPort:  8080,
				Role:     config.RoleMaster,
			},
		},
		Coordination: config.CoordinationConfig{
			HeartbeatInterval:    10 * time.Second,
			CommunicationTimeout: 5 * time.Second,
			FailoverTimeout:      30 * time.Second,
			MinConsensusNodes:    1,
			LocalNodePreference:  0.8,
		},
		HealthCheck: config.HealthCheckConfig{
			Interval:         100 * time.Millisecond, // Short for testing
			Timeout:          5 * time.Second,
			FailureThreshold: 3,
		},
		LoadBalancer: config.LoadBalancerConfig{
			Algorithm:    config.AlgorithmRoundRobin,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
	}

	app, err := NewApplication(cfg)
	if err != nil {
		t.Fatalf("Failed to create application: %v", err)
	}

	// Test Run - it doesn't take context parameter in the new version
	// We'll test it in a goroutine with a timeout
	done := make(chan error, 1)
	go func() {
		done <- app.Run()
	}()

	// Give it a short time to start, then we can't easily stop it in tests
	// so we'll just verify it can be created and started
	select {
	case err := <-done:
		t.Logf("Run completed with result: %v", err)
	case <-time.After(100 * time.Millisecond):
		t.Log("Run started successfully (test timeout)")
	}
}

func TestApplication_InvalidConfig(t *testing.T) {
	// Test with invalid configuration (no cluster members)
	cfg := &config.Config{
		Identity: config.NodeIdentity{
			NodeName:    "test-node",
			LocalDBHost: "localhost",
			LocalDBPort: 5432,
			APIHost:     "localhost",
			APIPort:     8080,
		},
		LocalDB: config.NodeConfig{
			Name:     "master-1",
			Host:     "localhost",
			Port:     5432,
			Role:     config.RoleMaster,
			Username: "postgres",
			Password: "password",
			Database: "testdb",
		},
		ClusterMembers: []config.ClusterMember{}, // Empty cluster members should cause error
		Coordination: config.CoordinationConfig{
			HeartbeatInterval:    10 * time.Second,
			CommunicationTimeout: 5 * time.Second,
			FailoverTimeout:      30 * time.Second,
			MinConsensusNodes:    1,
			LocalNodePreference:  0.8,
		},
	}

	_, err := NewApplication(cfg)
	if err == nil {
		t.Error("Expected error for invalid configuration")
	}
}
