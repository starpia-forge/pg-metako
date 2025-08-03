package database

import (
	"context"
	"pg-metako/internal/config"
	"testing"
	"time"
)

func TestNewConnectionManager(t *testing.T) {
	nodeConfig := config.NodeConfig{
		Name:     "test-node",
		Host:     "localhost",
		Port:     5432,
		Role:     config.RoleMaster,
		Username: "postgres",
		Password: "password",
		Database: "testdb",
	}

	manager, err := NewConnectionManager(nodeConfig)
	if err != nil {
		t.Fatalf("Failed to create connection manager: %v", err)
	}

	if manager == nil {
		t.Fatal("Connection manager should not be nil")
	}

	if manager.NodeName() != "test-node" {
		t.Errorf("Expected node name 'test-node', got '%s'", manager.NodeName())
	}

	if manager.Role() != config.RoleMaster {
		t.Errorf("Expected role master, got %s", manager.Role())
	}
}

func TestConnectionManager_Connect(t *testing.T) {
	nodeConfig := config.NodeConfig{
		Name:     "test-node",
		Host:     "localhost",
		Port:     5432,
		Role:     config.RoleMaster,
		Username: "postgres",
		Password: "password",
		Database: "testdb",
	}

	manager, err := NewConnectionManager(nodeConfig)
	if err != nil {
		t.Fatalf("Failed to create connection manager: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// This should fail initially since we don't have a real database
	err = manager.Connect(ctx)
	if err == nil {
		t.Log("Connection succeeded (database is available)")
	} else {
		t.Logf("Connection failed as expected (no database): %v", err)
	}
}

func TestConnectionManager_IsHealthy(t *testing.T) {
	nodeConfig := config.NodeConfig{
		Name:     "test-node",
		Host:     "localhost",
		Port:     5432,
		Role:     config.RoleMaster,
		Username: "postgres",
		Password: "password",
		Database: "testdb",
	}

	manager, err := NewConnectionManager(nodeConfig)
	if err != nil {
		t.Fatalf("Failed to create connection manager: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Initially should be unhealthy since not connected
	if manager.IsHealthy(ctx) {
		t.Error("Manager should be unhealthy when not connected")
	}
}

func TestConnectionManager_ExecuteQuery(t *testing.T) {
	nodeConfig := config.NodeConfig{
		Name:     "test-node",
		Host:     "localhost",
		Port:     5432,
		Role:     config.RoleMaster,
		Username: "postgres",
		Password: "password",
		Database: "testdb",
	}

	manager, err := NewConnectionManager(nodeConfig)
	if err != nil {
		t.Fatalf("Failed to create connection manager: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test executing a simple query
	query := "SELECT 1"
	result, err := manager.ExecuteQuery(ctx, query)
	if err != nil {
		t.Logf("Query execution failed as expected (no database): %v", err)
	} else {
		t.Logf("Query executed successfully: %v", result)
	}
}

func TestConnectionManager_Close(t *testing.T) {
	nodeConfig := config.NodeConfig{
		Name:     "test-node",
		Host:     "localhost",
		Port:     5432,
		Role:     config.RoleMaster,
		Username: "postgres",
		Password: "password",
		Database: "testdb",
	}

	manager, err := NewConnectionManager(nodeConfig)
	if err != nil {
		t.Fatalf("Failed to create connection manager: %v", err)
	}

	// Should be able to close without error
	err = manager.Close()
	if err != nil {
		t.Errorf("Failed to close connection manager: %v", err)
	}

	// Should be able to close multiple times without error
	err = manager.Close()
	if err != nil {
		t.Errorf("Failed to close connection manager second time: %v", err)
	}
}
