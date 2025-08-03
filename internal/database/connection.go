package database

import (
	"context"
	"database/sql"
	"fmt"
	"pg-metako/internal/config"
	"sync"

	_ "github.com/lib/pq"
)

// ConnectionManager manages database connections for a single PostgreSQL node
type ConnectionManager struct {
	nodeConfig config.NodeConfig
	db         *sql.DB
	mu         sync.RWMutex
	connected  bool
}

// QueryResult represents the result of a database query
type QueryResult struct {
	Rows     []map[string]interface{}
	Affected int64
}

// NewConnectionManager creates a new connection manager for a PostgreSQL node
func NewConnectionManager(nodeConfig config.NodeConfig) (*ConnectionManager, error) {
	if err := nodeConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid node configuration: %w", err)
	}

	return &ConnectionManager{
		nodeConfig: nodeConfig,
		connected:  false,
	}, nil
}

// NodeName returns the name of the node
func (cm *ConnectionManager) NodeName() string {
	return cm.nodeConfig.Name
}

// Role returns the role of the node
func (cm *ConnectionManager) Role() config.NodeRole {
	return cm.nodeConfig.Role
}

// Connect establishes a connection to the PostgreSQL database
func (cm *ConnectionManager) Connect(ctx context.Context) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.connected && cm.db != nil {
		return nil // Already connected
	}

	// Build connection string
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cm.nodeConfig.Host,
		cm.nodeConfig.Port,
		cm.nodeConfig.Username,
		cm.nodeConfig.Password,
		cm.nodeConfig.Database,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}

	// Test the connection
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	cm.db = db
	cm.connected = true

	return nil
}

// IsHealthy checks if the database connection is healthy
func (cm *ConnectionManager) IsHealthy(ctx context.Context) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if !cm.connected || cm.db == nil {
		return false
	}

	// Perform a simple ping to check health
	if err := cm.db.PingContext(ctx); err != nil {
		return false
	}

	return true
}

// ExecuteQuery executes a SQL query and returns the result
func (cm *ConnectionManager) ExecuteQuery(ctx context.Context, query string, args ...interface{}) (*QueryResult, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if !cm.connected || cm.db == nil {
		return nil, fmt.Errorf("database not connected")
	}

	// Check if it's a SELECT query or a modification query
	if isSelectQuery(query) {
		return cm.executeSelectQuery(ctx, query, args...)
	}

	return cm.executeModifyQuery(ctx, query, args...)
}

// executeSelectQuery executes a SELECT query
func (cm *ConnectionManager) executeSelectQuery(ctx context.Context, query string, args ...interface{}) (*QueryResult, error) {
	rows, err := cm.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute select query: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	var result []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = values[i]
		}
		result = append(result, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return &QueryResult{
		Rows:     result,
		Affected: int64(len(result)),
	}, nil
}

// executeModifyQuery executes INSERT, UPDATE, DELETE queries
func (cm *ConnectionManager) executeModifyQuery(ctx context.Context, query string, args ...interface{}) (*QueryResult, error) {
	result, err := cm.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute modify query: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to get affected rows: %w", err)
	}

	return &QueryResult{
		Rows:     nil,
		Affected: affected,
	}, nil
}

// Close closes the database connection
func (cm *ConnectionManager) Close() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.db != nil {
		err := cm.db.Close()
		cm.db = nil
		cm.connected = false
		return err
	}

	return nil
}

// isSelectQuery checks if the query is a SELECT statement
func isSelectQuery(query string) bool {
	// Simple check - in a real implementation, you might want a more sophisticated parser
	trimmed := query
	for i, r := range query {
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			trimmed = query[i:]
			break
		}
	}

	if len(trimmed) >= 6 {
		return trimmed[:6] == "SELECT" || trimmed[:6] == "select"
	}

	return false
}
