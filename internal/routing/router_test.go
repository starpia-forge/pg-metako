// Purpose    : Unit tests for query routing functionality with dependency injection
// Context    : PostgreSQL cluster query routing with read/write distribution and local node preference
// Constraints: Must test all routing logic, statistics, and maintain backward compatibility
package routing

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"pg-metako/internal/config"
	"pg-metako/internal/coordination"
	"pg-metako/internal/database"
)

// Mock implementations for testing

type mockConnectionManager struct {
	nodeName    string
	isHealthy   bool
	executeErr  error
	connectErr  error
	closeErr    error
	queryResult *database.QueryResult
}

func (m *mockConnectionManager) Connect(ctx context.Context) error {
	return m.connectErr
}

func (m *mockConnectionManager) Close() error {
	return m.closeErr
}

func (m *mockConnectionManager) ExecuteQuery(ctx context.Context, query string, args ...interface{}) (*database.QueryResult, error) {
	if m.executeErr != nil {
		return nil, m.executeErr
	}
	if m.queryResult != nil {
		return m.queryResult, nil
	}
	return &database.QueryResult{}, nil
}

func (m *mockConnectionManager) IsHealthy(ctx context.Context) bool {
	return m.isHealthy
}

func (m *mockConnectionManager) NodeName() string {
	return m.nodeName
}

type mockCoordinationAPI struct {
	clusterState coordination.ClusterState
}

func (m *mockCoordinationAPI) GetClusterState() coordination.ClusterState {
	return m.clusterState
}

type mockConnectionFactory struct {
	createErr  error
	connection ConnectionManager
}

func (m *mockConnectionFactory) CreateConnection(config config.NodeConfig) (ConnectionManager, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	return m.connection, nil
}

type mockRandomGenerator struct {
	float64Value float64
	intValue     int
}

func (m *mockRandomGenerator) Float64() float64 {
	return m.float64Value
}

func (m *mockRandomGenerator) Intn(n int) int {
	if m.intValue >= n {
		return 0
	}
	return m.intValue
}

type mockTimeProvider struct {
	currentTime time.Time
}

func (m *mockTimeProvider) Now() time.Time {
	return m.currentTime
}

type mockLogger struct {
	messages []string
}

func (m *mockLogger) Printf(format string, args ...interface{}) {
	// Store the message for verification in tests
	m.messages = append(m.messages, format)
}

// Test helper functions

func createTestConfig() *config.Config {
	return &config.Config{
		Identity: config.NodeIdentity{
			NodeName:    "test-node",
			LocalDBHost: "localhost",
			LocalDBPort: 5432,
			APIHost:     "localhost",
			APIPort:     8080,
		},
		LocalDB: config.NodeConfig{
			Name:     "test-node",
			Host:     "localhost",
			Port:     5432,
			Role:     config.RoleMaster,
			Username: "test",
			Password: "test",
			Database: "test",
		},
		ClusterMembers: []config.ClusterMember{
			{NodeName: "test-node", APIHost: "localhost", APIPort: 8080, Role: config.RoleMaster},
			{NodeName: "node2", APIHost: "localhost", APIPort: 8081, Role: config.RoleSlave},
			{NodeName: "node3", APIHost: "localhost", APIPort: 8082, Role: config.RoleSlave},
		},
		Coordination: config.CoordinationConfig{
			HeartbeatInterval:    10 * time.Second,
			CommunicationTimeout: 5 * time.Second,
			FailoverTimeout:      30 * time.Second,
			MinConsensusNodes:    2,
			LocalNodePreference:  0.8,
		},
		LoadBalancer: config.LoadBalancerConfig{
			Algorithm:    config.AlgorithmRoundRobin,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 5 * time.Second,
		},
	}
}

func createTestCoordinationAPI() CoordinationAPI {
	return &mockCoordinationAPI{
		clusterState: coordination.ClusterState{
			CurrentMaster: "test-node",
			Nodes: map[string]coordination.NodeStatus{
				"test-node": {IsHealthy: true, Role: string(config.RoleMaster)},
				"node2":     {IsHealthy: true, Role: string(config.RoleSlave)},
				"node3":     {IsHealthy: true, Role: string(config.RoleSlave)},
			},
		},
	}
}

func createTestRouter() (*Router, *mockLogger, *mockTimeProvider, *mockRandomGenerator) {
	cfg := createTestConfig()
	coordinationAPI := createTestCoordinationAPI()

	localConn := &mockConnectionManager{
		nodeName:  "test-node",
		isHealthy: true,
	}

	logger := &mockLogger{}
	timeProvider := &mockTimeProvider{currentTime: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)}
	randomGenerator := &mockRandomGenerator{float64Value: 0.5, intValue: 0}

	options := &RouterOptions{
		LocalConnection: localConn,
		Logger:          logger,
		TimeProvider:    timeProvider,
		RandomGenerator: randomGenerator,
	}

	router, _ := NewRouterWithOptions(cfg, coordinationAPI, options)
	return router, logger, timeProvider, randomGenerator
}

// Unit Tests

func TestNewRouter(t *testing.T) {
	cfg := createTestConfig()
	coordinationAPI := createTestCoordinationAPI()

	router, err := NewRouter(cfg, coordinationAPI)
	if err != nil {
		t.Fatalf("NewRouter should not return error: %v", err)
	}

	if router == nil {
		t.Fatal("Router should not be nil")
	}

	if router.config != cfg {
		t.Error("Router config should match provided config")
	}

	if router.coordinationAPI != coordinationAPI {
		t.Error("Router coordinationAPI should match provided coordinationAPI")
	}

	if router.localConnection == nil {
		t.Error("Router localConnection should not be nil")
	}

	if router.stats == nil {
		t.Error("Router stats should not be nil")
	}
}

func TestNewRouterWithOptions(t *testing.T) {
	cfg := createTestConfig()
	coordinationAPI := createTestCoordinationAPI()

	localConn := &mockConnectionManager{nodeName: "test-node", isHealthy: true}
	logger := &mockLogger{}
	timeProvider := &mockTimeProvider{currentTime: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)}

	options := &RouterOptions{
		LocalConnection: localConn,
		Logger:          logger,
		TimeProvider:    timeProvider,
	}

	router, err := NewRouterWithOptions(cfg, coordinationAPI, options)
	if err != nil {
		t.Fatalf("NewRouterWithOptions should not return error: %v", err)
	}

	if router.localConnection != localConn {
		t.Error("Router should use provided local connection")
	}

	if router.logger != logger {
		t.Error("Router should use provided logger")
	}

	if router.timeProvider != timeProvider {
		t.Error("Router should use provided time provider")
	}

	expectedTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	if !router.stats.LastQueryTime.Equal(expectedTime) {
		t.Errorf("Expected LastQueryTime %v, got %v", expectedTime, router.stats.LastQueryTime)
	}
}

func TestRouter_Initialize(t *testing.T) {
	router, _, _, _ := createTestRouter()

	ctx := context.Background()
	err := router.Initialize(ctx)
	if err != nil {
		t.Errorf("Initialize should not return error: %v", err)
	}
}

func TestRouter_DetectQueryType(t *testing.T) {
	router, _, _, _ := createTestRouter()

	tests := []struct {
		name     string
		query    string
		expected QueryType
	}{
		{"SELECT query", "SELECT * FROM users", QueryTypeRead},
		{"select query lowercase", "select id from users", QueryTypeRead},
		{"SHOW query", "SHOW TABLES", QueryTypeRead},
		{"DESCRIBE query", "DESCRIBE users", QueryTypeRead},
		{"EXPLAIN query", "EXPLAIN SELECT * FROM users", QueryTypeRead},
		{"INSERT query", "INSERT INTO users VALUES (1, 'test')", QueryTypeWrite},
		{"UPDATE query", "UPDATE users SET name = 'test'", QueryTypeWrite},
		{"DELETE query", "DELETE FROM users WHERE id = 1", QueryTypeWrite},
		{"CREATE query", "CREATE TABLE test (id INT)", QueryTypeWrite},
		{"DROP query", "DROP TABLE test", QueryTypeWrite},
		{"Empty query", "", QueryTypeWrite},
		{"Whitespace query", "   ", QueryTypeWrite},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := router.DetectQueryType(tt.query)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for query: %s", tt.expected, result, tt.query)
			}
		})
	}
}

func TestQueryType_String(t *testing.T) {
	tests := []struct {
		queryType QueryType
		expected  string
	}{
		{QueryTypeRead, "READ"},
		{QueryTypeWrite, "WRITE"},
		{QueryType(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.queryType.String()
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestRouter_RouteQuery_ReadQuery(t *testing.T) {
	router, logger, _, randomGen := createTestRouter()

	// Set random generator to prefer local node
	randomGen.float64Value = 0.5 // Less than LocalNodePreference (0.8)

	ctx := context.Background()
	query := "SELECT * FROM users"

	result, err := router.RouteQuery(ctx, query)
	if err != nil {
		t.Errorf("RouteQuery should not return error: %v", err)
	}

	if result == nil {
		t.Error("RouteQuery should return a result")
	}

	// Check statistics
	stats := router.GetStats()
	if stats.TotalQueries != 1 {
		t.Errorf("Expected TotalQueries 1, got %d", stats.TotalQueries)
	}
	if stats.ReadQueries != 1 {
		t.Errorf("Expected ReadQueries 1, got %d", stats.ReadQueries)
	}
	if stats.LocalQueries != 1 {
		t.Errorf("Expected LocalQueries 1, got %d", stats.LocalQueries)
	}

	// Check that logger was called
	if len(logger.messages) == 0 {
		t.Error("Expected logger to be called")
	}
}

func TestRouter_RouteQuery_WriteQuery(t *testing.T) {
	router, logger, _, _ := createTestRouter()

	ctx := context.Background()
	query := "INSERT INTO users VALUES (1, 'test')"

	result, err := router.RouteQuery(ctx, query)
	if err != nil {
		t.Errorf("RouteQuery should not return error: %v", err)
	}

	if result == nil {
		t.Error("RouteQuery should return a result")
	}

	// Check statistics
	stats := router.GetStats()
	if stats.TotalQueries != 1 {
		t.Errorf("Expected TotalQueries 1, got %d", stats.TotalQueries)
	}
	if stats.WriteQueries != 1 {
		t.Errorf("Expected WriteQueries 1, got %d", stats.WriteQueries)
	}
	if stats.LocalQueries != 1 {
		t.Errorf("Expected LocalQueries 1, got %d", stats.LocalQueries)
	}

	// Check that logger was called
	if len(logger.messages) == 0 {
		t.Error("Expected logger to be called")
	}
}

func TestRouter_RouteQuery_NoAvailableNode(t *testing.T) {
	cfg := createTestConfig()
	coordinationAPI := createTestCoordinationAPI()

	// Create unhealthy local connection
	localConn := &mockConnectionManager{
		nodeName:  "test-node",
		isHealthy: false,
	}

	options := &RouterOptions{
		LocalConnection: localConn,
	}

	router, err := NewRouterWithOptions(cfg, coordinationAPI, options)
	if err != nil {
		t.Fatalf("NewRouterWithOptions should not return error: %v", err)
	}

	ctx := context.Background()
	query := "SELECT * FROM users"

	result, err := router.RouteQuery(ctx, query)
	if err == nil {
		t.Error("RouteQuery should return error when no nodes available")
	}
	if result != nil {
		t.Error("RouteQuery should return nil result when no nodes available")
	}

	if !strings.Contains(err.Error(), "no available node") {
		t.Errorf("Expected error to contain 'no available node', got: %v", err)
	}

	// Check failed queries statistic
	stats := router.GetStats()
	if stats.FailedQueries != 1 {
		t.Errorf("Expected FailedQueries 1, got %d", stats.FailedQueries)
	}
}

func TestRouter_RouteQuery_ExecutionError(t *testing.T) {
	cfg := createTestConfig()
	coordinationAPI := createTestCoordinationAPI()

	// Create connection that fails execution
	localConn := &mockConnectionManager{
		nodeName:   "test-node",
		isHealthy:  true,
		executeErr: errors.New("execution failed"),
	}

	options := &RouterOptions{
		LocalConnection: localConn,
	}

	router, err := NewRouterWithOptions(cfg, coordinationAPI, options)
	if err != nil {
		t.Fatalf("NewRouterWithOptions should not return error: %v", err)
	}

	ctx := context.Background()
	query := "SELECT * FROM users"

	result, err := router.RouteQuery(ctx, query)
	if err == nil {
		t.Error("RouteQuery should return error when execution fails")
	}
	if result != nil {
		t.Error("RouteQuery should return nil result when execution fails")
	}

	if !strings.Contains(err.Error(), "query execution failed") {
		t.Errorf("Expected error to contain 'query execution failed', got: %v", err)
	}

	// Check failed queries statistic
	stats := router.GetStats()
	if stats.FailedQueries != 1 {
		t.Errorf("Expected FailedQueries 1, got %d", stats.FailedQueries)
	}
}

func TestRouter_GetStats(t *testing.T) {
	router, _, timeProvider, _ := createTestRouter()

	// Execute some queries to generate statistics
	ctx := context.Background()
	router.RouteQuery(ctx, "SELECT * FROM users")
	router.RouteQuery(ctx, "INSERT INTO users VALUES (1, 'test')")

	stats := router.GetStats()
	if stats == nil {
		t.Fatal("GetStats should not return nil")
	}

	if stats.TotalQueries != 2 {
		t.Errorf("Expected TotalQueries 2, got %d", stats.TotalQueries)
	}
	if stats.ReadQueries != 1 {
		t.Errorf("Expected ReadQueries 1, got %d", stats.ReadQueries)
	}
	if stats.WriteQueries != 1 {
		t.Errorf("Expected WriteQueries 1, got %d", stats.WriteQueries)
	}
	if stats.LocalQueries != 2 {
		t.Errorf("Expected LocalQueries 2, got %d", stats.LocalQueries)
	}
	if stats.LocalPreference != 0.8 {
		t.Errorf("Expected LocalPreference 0.8, got %f", stats.LocalPreference)
	}

	expectedTime := timeProvider.currentTime
	if !stats.LastQueryTime.Equal(expectedTime) {
		t.Errorf("Expected LastQueryTime %v, got %v", expectedTime, stats.LastQueryTime)
	}
}

func TestRouter_ResetStats(t *testing.T) {
	router, _, timeProvider, _ := createTestRouter()

	// Execute some queries to generate statistics
	ctx := context.Background()
	router.RouteQuery(ctx, "SELECT * FROM users")
	router.RouteQuery(ctx, "INSERT INTO users VALUES (1, 'test')")

	// Update time provider for reset
	newTime := time.Date(2023, 1, 2, 12, 0, 0, 0, time.UTC)
	timeProvider.currentTime = newTime

	router.ResetStats()

	stats := router.GetStats()
	if stats.TotalQueries != 0 {
		t.Errorf("Expected TotalQueries 0 after reset, got %d", stats.TotalQueries)
	}
	if stats.ReadQueries != 0 {
		t.Errorf("Expected ReadQueries 0 after reset, got %d", stats.ReadQueries)
	}
	if stats.WriteQueries != 0 {
		t.Errorf("Expected WriteQueries 0 after reset, got %d", stats.WriteQueries)
	}
	if stats.LocalQueries != 0 {
		t.Errorf("Expected LocalQueries 0 after reset, got %d", stats.LocalQueries)
	}
	if stats.RemoteQueries != 0 {
		t.Errorf("Expected RemoteQueries 0 after reset, got %d", stats.RemoteQueries)
	}
	if stats.FailedQueries != 0 {
		t.Errorf("Expected FailedQueries 0 after reset, got %d", stats.FailedQueries)
	}

	if !stats.LastQueryTime.Equal(newTime) {
		t.Errorf("Expected LastQueryTime %v after reset, got %v", newTime, stats.LastQueryTime)
	}
}

func TestRouter_IsLocalNodeMaster(t *testing.T) {
	tests := []struct {
		name     string
		role     config.NodeRole
		expected bool
	}{
		{"Master node", config.RoleMaster, true},
		{"Slave node", config.RoleSlave, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createTestConfig()
			cfg.LocalDB.Role = tt.role
			coordinationAPI := createTestCoordinationAPI()

			router, err := NewRouter(cfg, coordinationAPI)
			if err != nil {
				t.Fatalf("NewRouter should not return error: %v", err)
			}

			result := router.IsLocalNodeMaster()
			if result != tt.expected {
				t.Errorf("Expected IsLocalNodeMaster %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestRouter_GetClusterState(t *testing.T) {
	router, _, _, _ := createTestRouter()

	clusterState := router.GetClusterState()
	if clusterState.CurrentMaster != "test-node" {
		t.Errorf("Expected CurrentMaster 'test-node', got '%s'", clusterState.CurrentMaster)
	}

	if len(clusterState.Nodes) != 3 {
		t.Errorf("Expected 3 nodes in cluster state, got %d", len(clusterState.Nodes))
	}
}

func TestRouter_GetLocalConnection(t *testing.T) {
	router, _, _, _ := createTestRouter()

	conn := router.GetLocalConnection()
	// Note: This returns nil because the mock doesn't match *database.ConnectionManager type
	// This is expected behavior for the legacy compatibility method with mocks
	if conn != nil {
		t.Error("GetLocalConnection should return nil with mock connection (type assertion fails)")
	}
}

func TestRouter_Close(t *testing.T) {
	router, _, _, _ := createTestRouter()

	err := router.Close()
	if err != nil {
		t.Errorf("Close should not return error: %v", err)
	}
}

// Legacy compatibility tests

func TestRouter_SelectReadNode(t *testing.T) {
	router, _, _, _ := createTestRouter()

	node := router.SelectReadNode()
	// Note: This returns nil because the mock doesn't match *database.ConnectionManager type
	// This is expected behavior for the legacy compatibility method with mocks
	if node != nil {
		t.Error("SelectReadNode should return nil with mock connection (type assertion fails)")
	}
}

func TestRouter_SelectWriteNode(t *testing.T) {
	router, _, _, _ := createTestRouter()

	node := router.SelectWriteNode()
	// Note: This returns nil because the mock doesn't match *database.ConnectionManager type
	// This is expected behavior for the legacy compatibility method with mocks
	if node != nil {
		t.Error("SelectWriteNode should return nil with mock connection (type assertion fails)")
	}
}

func TestRouter_GetConnectionStats(t *testing.T) {
	router, _, _, _ := createTestRouter()

	stats := router.GetConnectionStats()
	if stats == nil {
		t.Error("GetConnectionStats should not return nil")
	}

	// Should be the same as GetStats
	expectedStats := router.GetStats()
	if stats.TotalQueries != expectedStats.TotalQueries {
		t.Error("GetConnectionStats should return same data as GetStats")
	}
}
