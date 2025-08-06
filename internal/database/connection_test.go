package database

import (
	"context"
	"pg-metako/internal/config"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
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

func TestIsSelectQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected bool
	}{
		{"uppercase SELECT", "SELECT * FROM users", true},
		{"lowercase select", "select * from users", true},
		{"SELECT with leading spaces", "   SELECT id FROM users", true},
		{"SELECT with tabs", "\t\tSELECT name FROM users", true},
		{"SELECT with newlines", "\n\nSELECT * FROM users", true},
		{"mixed case Select", "Select * from users", false},
		{"INSERT query", "INSERT INTO users VALUES (1, 'test')", false},
		{"UPDATE query", "UPDATE users SET name = 'test'", false},
		{"DELETE query", "DELETE FROM users WHERE id = 1", false},
		{"empty query", "", false},
		{"short query", "SEL", false},
		{"exactly 6 chars uppercase", "SELECT", true},
		{"exactly 6 chars lowercase", "select", true},
		{"query starting with S but not SELECT", "SHOW TABLES", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSelectQuery(tt.query)
			if result != tt.expected {
				t.Errorf("isSelectQuery(%q) = %v, expected %v", tt.query, result, tt.expected)
			}
		})
	}
}

func TestNewConnectionManager_InvalidConfig(t *testing.T) {
	// Test with invalid config that should fail validation
	nodeConfig := config.NodeConfig{
		Name: "", // Empty name should cause validation to fail
	}

	_, err := NewConnectionManager(nodeConfig)
	if err == nil {
		t.Error("Expected error for invalid config, got nil")
	}
}

func TestConnectionManager_ExecuteQuery_NotConnected(t *testing.T) {
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

	ctx := context.Background()

	// Test executing query when not connected
	_, err = manager.ExecuteQuery(ctx, "SELECT 1")
	if err == nil {
		t.Error("Expected error when executing query without connection, got nil")
	}

	expectedMsg := "database not connected"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestConnectionManager_ExecuteSelectQuery_WithMock(t *testing.T) {
	// Create a mock database
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

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

	// Manually set the database connection for testing
	manager.db = db
	manager.connected = true

	ctx := context.Background()

	// Set up mock expectations for a SELECT query
	rows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(1, "test1").
		AddRow(2, "test2")

	mock.ExpectQuery("SELECT \\* FROM users").WillReturnRows(rows)

	// Execute the query
	result, err := manager.ExecuteQuery(ctx, "SELECT * FROM users")
	if err != nil {
		t.Fatalf("Failed to execute query: %v", err)
	}

	// Verify results
	if result == nil {
		t.Fatal("Result should not be nil")
	}

	if len(result.Rows) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(result.Rows))
	}

	if result.Affected != 2 {
		t.Errorf("Expected affected count 2, got %d", result.Affected)
	}

	// Verify all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Mock expectations were not met: %v", err)
	}
}

func TestConnectionManager_ExecuteModifyQuery_WithMock(t *testing.T) {
	// Create a mock database
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

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

	// Manually set the database connection for testing
	manager.db = db
	manager.connected = true

	ctx := context.Background()

	// Set up mock expectations for an INSERT query
	mock.ExpectExec("INSERT INTO users").WillReturnResult(sqlmock.NewResult(1, 1))

	// Execute the query
	result, err := manager.ExecuteQuery(ctx, "INSERT INTO users (name) VALUES ('test')")
	if err != nil {
		t.Fatalf("Failed to execute query: %v", err)
	}

	// Verify results
	if result == nil {
		t.Fatal("Result should not be nil")
	}

	if result.Rows != nil {
		t.Error("Expected Rows to be nil for modify query")
	}

	if result.Affected != 1 {
		t.Errorf("Expected affected count 1, got %d", result.Affected)
	}

	// Verify all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Mock expectations were not met: %v", err)
	}
}

func TestConnectionManager_IsHealthy_WithMock(t *testing.T) {
	// Create a mock database
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

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

	ctx := context.Background()

	// Test when not connected
	if manager.IsHealthy(ctx) {
		t.Error("Expected IsHealthy to return false when not connected")
	}

	// Manually set the database connection for testing
	manager.db = db
	manager.connected = true

	// Set up mock expectations for ping
	mock.ExpectPing()

	// Test when connected and healthy
	if !manager.IsHealthy(ctx) {
		t.Error("Expected IsHealthy to return true when connected and ping succeeds")
	}

	// Verify all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Mock expectations were not met: %v", err)
	}
}
