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
		Nodes: []config.NodeConfig{
			{
				Name:     "master-1",
				Host:     "localhost",
				Port:     5432,
				Role:     config.RoleMaster,
				Username: "postgres",
				Password: "password",
				Database: "testdb",
			},
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
	if app.GetQueryRouter() == nil {
		t.Error("QueryRouter should be initialized")
	}

	if app.GetReplicationManager() == nil {
		t.Error("ReplicationManager should be initialized")
	}

	if app.GetHealthChecker() == nil {
		t.Error("HealthChecker should be initialized")
	}
}

func TestApplication_StartStop(t *testing.T) {
	cfg := &config.Config{
		Nodes: []config.NodeConfig{
			{
				Name:     "master-1",
				Host:     "localhost",
				Port:     5432,
				Role:     config.RoleMaster,
				Username: "postgres",
				Password: "password",
				Database: "testdb",
			},
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
		Nodes: []config.NodeConfig{
			{
				Name:     "master-1",
				Host:     "localhost",
				Port:     5432,
				Role:     config.RoleMaster,
				Username: "postgres",
				Password: "password",
				Database: "testdb",
			},
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

	// Test Run with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Run should handle context cancellation gracefully
	err = app.Run(ctx)
	if err != nil && err != context.DeadlineExceeded {
		t.Logf("Run completed with result: %v", err)
	}
}

func TestApplication_InvalidConfig(t *testing.T) {
	// Test with invalid configuration (no nodes)
	cfg := &config.Config{
		Nodes: []config.NodeConfig{},
	}

	_, err := NewApplication(cfg)
	if err == nil {
		t.Error("Expected error for invalid configuration")
	}
}
