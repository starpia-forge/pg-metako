package config

import (
	"errors"
	"os"
	"testing"
	"time"
)

func TestLoadConfigFromYAML(t *testing.T) {
	// Create a temporary YAML config file
	yamlContent := `
identity:
  node_name: "test-node"
  local_db_host: "localhost"
  local_db_port: 5432
  api_host: "localhost"
  api_port: 8080

local_db:
  name: "test-db"
  host: "localhost"
  port: 5432
  role: "master"
  username: "postgres"
  password: "password"
  database: "testdb"

cluster_members:
  - node_name: "test-node"
    api_host: "localhost"
    api_port: 8080
    role: "master"
  - node_name: "slave-node"
    api_host: "localhost"
    api_port: 8081
    role: "slave"

coordination:
  heartbeat_interval: "10s"
  communication_timeout: "5s"
  failover_timeout: "30s"
  min_consensus_nodes: 1
  local_node_preference: 0.8

health_check:
  interval: "30s"
  timeout: "5s"
  failure_threshold: 3

load_balancer:
  algorithm: "round_robin"
  read_timeout: "10s"
  write_timeout: "10s"

security:
  tls_enabled: true
  cert_file: "/path/to/cert.pem"
  key_file: "/path/to/key.pem"
`

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "config_test_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yamlContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Test loading configuration
	config, err := LoadFromFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify configuration values
	if len(config.ClusterMembers) != 2 {
		t.Errorf("Expected 2 cluster members, got %d", len(config.ClusterMembers))
	}

	if config.Identity.NodeName != "test-node" {
		t.Errorf("Expected node name to be 'test-node', got '%s'", config.Identity.NodeName)
	}

	if config.LocalDB.Role != RoleMaster {
		t.Errorf("Expected local DB role to be master, got %s", config.LocalDB.Role)
	}

	if config.ClusterMembers[0].NodeName != "test-node" {
		t.Errorf("Expected first cluster member name to be 'test-node', got '%s'", config.ClusterMembers[0].NodeName)
	}

	if config.Coordination.LocalNodePreference != 0.8 {
		t.Errorf("Expected local node preference to be 0.8, got %f", config.Coordination.LocalNodePreference)
	}

	if config.HealthCheck.Interval != 30*time.Second {
		t.Errorf("Expected health check interval to be 30s, got %v", config.HealthCheck.Interval)
	}

	if config.LoadBalancer.Algorithm != AlgorithmRoundRobin {
		t.Errorf("Expected load balancer algorithm to be round_robin, got %s", config.LoadBalancer.Algorithm)
	}

	if !config.Security.TLSEnabled {
		t.Error("Expected TLS to be enabled")
	}
}

func TestCoordinationConfigStructure(t *testing.T) {
	// Test the new nested coordination configuration structure
	yamlContent := `
identity:
  node_name: "test-node"
  local_db_host: "localhost"
  local_db_port: 5432
  api_host: "localhost"
  api_port: 8080

local_db:
  name: "test-db"
  host: "localhost"
  port: 5432
  role: "master"
  username: "postgres"
  password: "password"
  database: "testdb"

cluster_members:
  - node_name: "test-node-2"
    api_host: "192.168.1.102"
    api_port: 8080
    role: "slave"
  - node_name: "test-node-3"
    api_host: "192.168.1.103"
    api_port: 8080
    role: "slave"

coordination:
  cluster_mode:
    heartbeat_interval: "10s"
    communication_timeout: "5s"
    failover_timeout: "30s"
    min_consensus_nodes: 2
    local_node_preference: 0.8
  pair_mode:
    enable: false
    failover_delay: "10s"
    failure_threshold: 3

health_check:
  interval: "30s"
  timeout: "5s"
  failure_threshold: 3

load_balancer:
  algorithm: "round_robin"
  read_timeout: "10s"
  write_timeout: "10s"

security:
  tls_enabled: false
`

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yamlContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Load configuration
	config, err := LoadFromFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test cluster_mode fields
	if config.Coordination.ClusterMode.HeartbeatInterval != 10*time.Second {
		t.Errorf("Expected cluster_mode heartbeat_interval to be 10s, got %v", config.Coordination.ClusterMode.HeartbeatInterval)
	}
	if config.Coordination.ClusterMode.CommunicationTimeout != 5*time.Second {
		t.Errorf("Expected cluster_mode communication_timeout to be 5s, got %v", config.Coordination.ClusterMode.CommunicationTimeout)
	}
	if config.Coordination.ClusterMode.FailoverTimeout != 30*time.Second {
		t.Errorf("Expected cluster_mode failover_timeout to be 30s, got %v", config.Coordination.ClusterMode.FailoverTimeout)
	}
	if config.Coordination.ClusterMode.MinConsensusNodes != 2 {
		t.Errorf("Expected cluster_mode min_consensus_nodes to be 2, got %d", config.Coordination.ClusterMode.MinConsensusNodes)
	}
	if config.Coordination.ClusterMode.LocalNodePreference != 0.8 {
		t.Errorf("Expected cluster_mode local_node_preference to be 0.8, got %f", config.Coordination.ClusterMode.LocalNodePreference)
	}

	// Test pair_mode fields
	if config.Coordination.PairMode.Enable {
		t.Error("Expected pair_mode enable to be false")
	}
	if config.Coordination.PairMode.FailoverDelay != 10*time.Second {
		t.Errorf("Expected pair_mode failover_delay to be 10s, got %v", config.Coordination.PairMode.FailoverDelay)
	}
	if config.Coordination.PairMode.FailureThreshold != 3 {
		t.Errorf("Expected pair_mode failure_threshold to be 3, got %d", config.Coordination.PairMode.FailureThreshold)
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "valid config",
			config: &Config{
				Identity: NodeIdentity{
					NodeName:    "test-node",
					LocalDBHost: "localhost",
					LocalDBPort: 5432,
					APIHost:     "localhost",
					APIPort:     8080,
				},
				LocalDB: NodeConfig{
					Name:     "test-db",
					Host:     "localhost",
					Port:     5432,
					Role:     RoleMaster,
					Username: "postgres",
					Password: "password",
					Database: "testdb",
				},
				ClusterMembers: []ClusterMember{
					{
						NodeName: "test-node",
						APIHost:  "localhost",
						APIPort:  8080,
						Role:     RoleMaster,
					},
				},
				Coordination: CoordinationConfig{
					HeartbeatInterval:    10 * time.Second,
					CommunicationTimeout: 5 * time.Second,
					FailoverTimeout:      30 * time.Second,
					MinConsensusNodes:    1,
					LocalNodePreference:  0.8,
				},
				HealthCheck: HealthCheckConfig{
					Interval:         30 * time.Second,
					Timeout:          5 * time.Second,
					FailureThreshold: 3,
				},
			},
			expectError: false,
		},
		{
			name: "invalid node identity",
			config: &Config{
				Identity: NodeIdentity{
					NodeName:    "", // Empty node name should cause error
					LocalDBHost: "localhost",
					LocalDBPort: 5432,
					APIHost:     "localhost",
					APIPort:     8080,
				},
				LocalDB: NodeConfig{
					Name:     "test-db",
					Host:     "localhost",
					Port:     5432,
					Role:     RoleMaster,
					Username: "postgres",
					Password: "password",
					Database: "testdb",
				},
				ClusterMembers: []ClusterMember{
					{
						NodeName: "test-node",
						APIHost:  "localhost",
						APIPort:  8080,
						Role:     RoleMaster,
					},
				},
				Coordination: CoordinationConfig{
					HeartbeatInterval:    10 * time.Second,
					CommunicationTimeout: 5 * time.Second,
					FailoverTimeout:      30 * time.Second,
					MinConsensusNodes:    1,
					LocalNodePreference:  0.8,
				},
			},
			expectError: true,
		},
		{
			name: "empty cluster members",
			config: &Config{
				Identity: NodeIdentity{
					NodeName:    "test-node",
					LocalDBHost: "localhost",
					LocalDBPort: 5432,
					APIHost:     "localhost",
					APIPort:     8080,
				},
				LocalDB: NodeConfig{
					Name:     "test-db",
					Host:     "localhost",
					Port:     5432,
					Role:     RoleMaster,
					Username: "postgres",
					Password: "password",
					Database: "testdb",
				},
				ClusterMembers: []ClusterMember{}, // Empty cluster members should cause error
				Coordination: CoordinationConfig{
					HeartbeatInterval:    10 * time.Second,
					CommunicationTimeout: 5 * time.Second,
					FailoverTimeout:      30 * time.Second,
					MinConsensusNodes:    1,
					LocalNodePreference:  0.8,
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError && err == nil {
				t.Error("Expected validation error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no validation error, got: %v", err)
			}
		})
	}
}

// Test utility functions
func TestConfig_IsLocalNode(t *testing.T) {
	config := &Config{
		Identity: NodeIdentity{
			NodeName: "test-node",
		},
	}

	tests := []struct {
		name     string
		nodeName string
		expected bool
	}{
		{"matching node name", "test-node", true},
		{"non-matching node name", "other-node", false},
		{"empty node name", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.IsLocalNode(tt.nodeName)
			if result != tt.expected {
				t.Errorf("IsLocalNode(%q) = %v, expected %v", tt.nodeName, result, tt.expected)
			}
		})
	}
}

func TestConfig_GetLocalDBConfig(t *testing.T) {
	expectedConfig := NodeConfig{
		Name:     "test-db",
		Host:     "localhost",
		Port:     5432,
		Role:     RoleMaster,
		Username: "postgres",
		Password: "password",
		Database: "testdb",
	}

	config := &Config{
		LocalDB: expectedConfig,
	}

	result := config.GetLocalDBConfig()
	if result != expectedConfig {
		t.Errorf("GetLocalDBConfig() = %+v, expected %+v", result, expectedConfig)
	}
}

func TestConfig_GetClusterMemberByName(t *testing.T) {
	config := &Config{
		ClusterMembers: []ClusterMember{
			{NodeName: "node1", APIHost: "host1", APIPort: 8080, Role: RoleMaster},
			{NodeName: "node2", APIHost: "host2", APIPort: 8081, Role: RoleSlave},
		},
	}

	tests := []struct {
		name        string
		nodeName    string
		expectError bool
		expected    *ClusterMember
	}{
		{
			name:        "existing node",
			nodeName:    "node1",
			expectError: false,
			expected:    &ClusterMember{NodeName: "node1", APIHost: "host1", APIPort: 8080, Role: RoleMaster},
		},
		{
			name:        "non-existing node",
			nodeName:    "node3",
			expectError: true,
			expected:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := config.GetClusterMemberByName(tt.nodeName)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				if result != nil {
					t.Errorf("Expected nil result but got %+v", result)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result == nil {
					t.Error("Expected non-nil result but got nil")
				} else if *result != *tt.expected {
					t.Errorf("GetClusterMemberByName(%q) = %+v, expected %+v", tt.nodeName, *result, *tt.expected)
				}
			}
		})
	}
}

func TestConfig_GetMasterNodes(t *testing.T) {
	config := &Config{
		ClusterMembers: []ClusterMember{
			{NodeName: "master1", APIHost: "host1", APIPort: 8080, Role: RoleMaster},
			{NodeName: "slave1", APIHost: "host2", APIPort: 8081, Role: RoleSlave},
			{NodeName: "master2", APIHost: "host3", APIPort: 8082, Role: RoleMaster},
			{NodeName: "slave2", APIHost: "host4", APIPort: 8083, Role: RoleSlave},
		},
	}

	masters := config.GetMasterNodes()
	if len(masters) != 2 {
		t.Errorf("Expected 2 master nodes, got %d", len(masters))
	}

	expectedMasters := map[string]bool{"master1": true, "master2": true}
	for _, master := range masters {
		if !expectedMasters[master.NodeName] {
			t.Errorf("Unexpected master node: %s", master.NodeName)
		}
		if master.Role != RoleMaster {
			t.Errorf("Expected master role for %s, got %s", master.NodeName, master.Role)
		}
	}
}

func TestConfig_GetSlaveNodes(t *testing.T) {
	config := &Config{
		ClusterMembers: []ClusterMember{
			{NodeName: "master1", APIHost: "host1", APIPort: 8080, Role: RoleMaster},
			{NodeName: "slave1", APIHost: "host2", APIPort: 8081, Role: RoleSlave},
			{NodeName: "master2", APIHost: "host3", APIPort: 8082, Role: RoleMaster},
			{NodeName: "slave2", APIHost: "host4", APIPort: 8083, Role: RoleSlave},
		},
	}

	slaves := config.GetSlaveNodes()
	if len(slaves) != 2 {
		t.Errorf("Expected 2 slave nodes, got %d", len(slaves))
	}

	expectedSlaves := map[string]bool{"slave1": true, "slave2": true}
	for _, slave := range slaves {
		if !expectedSlaves[slave.NodeName] {
			t.Errorf("Unexpected slave node: %s", slave.NodeName)
		}
		if slave.Role != RoleSlave {
			t.Errorf("Expected slave role for %s, got %s", slave.NodeName, slave.Role)
		}
	}
}

func TestConfig_GetTotalClusterSize(t *testing.T) {
	tests := []struct {
		name           string
		clusterMembers []ClusterMember
		expectedSize   int
	}{
		{
			name:           "empty cluster",
			clusterMembers: []ClusterMember{},
			expectedSize:   1, // +1 for local node
		},
		{
			name: "single member cluster",
			clusterMembers: []ClusterMember{
				{NodeName: "node1", APIHost: "host1", APIPort: 8080, Role: RoleMaster},
			},
			expectedSize: 2, // 1 member + 1 local node
		},
		{
			name: "multi-member cluster",
			clusterMembers: []ClusterMember{
				{NodeName: "node1", APIHost: "host1", APIPort: 8080, Role: RoleMaster},
				{NodeName: "node2", APIHost: "host2", APIPort: 8081, Role: RoleSlave},
				{NodeName: "node3", APIHost: "host3", APIPort: 8082, Role: RoleSlave},
			},
			expectedSize: 4, // 3 members + 1 local node
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				ClusterMembers: tt.clusterMembers,
			}
			result := config.GetTotalClusterSize()
			if result != tt.expectedSize {
				t.Errorf("GetTotalClusterSize() = %d, expected %d", result, tt.expectedSize)
			}
		})
	}
}

// Test coordination getter functions
func TestConfig_IsPairMode(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected bool
	}{
		{
			name: "pair mode enabled",
			config: &Config{
				Coordination: CoordinationConfig{
					PairMode: PairModeConfig{
						Enable: true,
					},
				},
			},
			expected: true,
		},
		{
			name: "pair mode disabled",
			config: &Config{
				Coordination: CoordinationConfig{
					PairMode: PairModeConfig{
						Enable: false,
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.IsPairMode()
			if result != tt.expected {
				t.Errorf("IsPairMode() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestConfig_GetPairFailoverDelay(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected time.Duration
	}{
		{
			name: "custom failover delay",
			config: &Config{
				Coordination: CoordinationConfig{
					PairMode: PairModeConfig{
						FailoverDelay: 15 * time.Second,
					},
				},
			},
			expected: 15 * time.Second,
		},
		{
			name: "default failover delay",
			config: &Config{
				Coordination: CoordinationConfig{
					PairMode: PairModeConfig{
						FailoverDelay: 0, // Should use default
					},
				},
			},
			expected: 10 * time.Second, // Default value
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetPairFailoverDelay()
			if result != tt.expected {
				t.Errorf("GetPairFailoverDelay() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestConfig_GetPairFailureThreshold(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected int
	}{
		{
			name: "custom failure threshold",
			config: &Config{
				Coordination: CoordinationConfig{
					PairMode: PairModeConfig{
						FailureThreshold: 5,
					},
				},
			},
			expected: 5,
		},
		{
			name: "default failure threshold",
			config: &Config{
				Coordination: CoordinationConfig{
					PairMode: PairModeConfig{
						FailureThreshold: 0, // Should use default
					},
				},
			},
			expected: 3, // Default value
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetPairFailureThreshold()
			if result != tt.expected {
				t.Errorf("GetPairFailureThreshold() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestConfig_GetHeartbeatInterval(t *testing.T) {
	config := &Config{
		Coordination: CoordinationConfig{
			ClusterMode: ClusterModeConfig{
				HeartbeatInterval: 5 * time.Second,
			},
		},
	}

	result := config.GetHeartbeatInterval()
	expected := 5 * time.Second
	if result != expected {
		t.Errorf("GetHeartbeatInterval() = %v, expected %v", result, expected)
	}
}

func TestConfig_GetCommunicationTimeout(t *testing.T) {
	config := &Config{
		Coordination: CoordinationConfig{
			ClusterMode: ClusterModeConfig{
				CommunicationTimeout: 3 * time.Second,
			},
		},
	}

	result := config.GetCommunicationTimeout()
	expected := 3 * time.Second
	if result != expected {
		t.Errorf("GetCommunicationTimeout() = %v, expected %v", result, expected)
	}
}

func TestConfig_GetFailoverTimeout(t *testing.T) {
	config := &Config{
		Coordination: CoordinationConfig{
			ClusterMode: ClusterModeConfig{
				FailoverTimeout: 25 * time.Second,
			},
		},
	}

	result := config.GetFailoverTimeout()
	expected := 25 * time.Second
	if result != expected {
		t.Errorf("GetFailoverTimeout() = %v, expected %v", result, expected)
	}
}

func TestConfig_GetMinConsensusNodes(t *testing.T) {
	config := &Config{
		Coordination: CoordinationConfig{
			ClusterMode: ClusterModeConfig{
				MinConsensusNodes: 2,
			},
		},
	}

	result := config.GetMinConsensusNodes()
	expected := 2
	if result != expected {
		t.Errorf("GetMinConsensusNodes() = %v, expected %v", result, expected)
	}
}

func TestConfig_GetLocalNodePreference(t *testing.T) {
	config := &Config{
		Coordination: CoordinationConfig{
			ClusterMode: ClusterModeConfig{
				LocalNodePreference: 0.75,
			},
		},
	}

	result := config.GetLocalNodePreference()
	expected := 0.75
	if result != expected {
		t.Errorf("GetLocalNodePreference() = %v, expected %v", result, expected)
	}
}

// Mock network interface for testing
type MockNetworkInterface struct {
	ips []string
	err error
}

func (m *MockNetworkInterface) GetLocalIPs() ([]string, error) {
	return m.ips, m.err
}

func TestDetectLocalNodeNameWithInterface(t *testing.T) {
	tests := []struct {
		name           string
		clusterMembers []ClusterMember
		mockIPs        []string
		mockError      error
		expectError    bool
		expected       string
	}{
		{
			name: "match by IP address",
			clusterMembers: []ClusterMember{
				{NodeName: "node1", APIHost: "192.168.1.100", APIPort: 8080, Role: RoleMaster},
				{NodeName: "node2", APIHost: "192.168.1.101", APIPort: 8081, Role: RoleSlave},
			},
			mockIPs:     []string{"192.168.1.100", "10.0.0.1"},
			mockError:   nil,
			expectError: false,
			expected:    "node1",
		},
		{
			name: "match by localhost",
			clusterMembers: []ClusterMember{
				{NodeName: "node1", APIHost: "192.168.1.100", APIPort: 8080, Role: RoleMaster},
				{NodeName: "node3", APIHost: "localhost", APIPort: 8082, Role: RoleSlave},
			},
			mockIPs:     []string{"10.0.0.1", "127.0.0.1", "localhost"},
			mockError:   nil,
			expectError: false,
			expected:    "node3",
		},
		{
			name: "no match found",
			clusterMembers: []ClusterMember{
				{NodeName: "node1", APIHost: "192.168.1.100", APIPort: 8080, Role: RoleMaster},
				{NodeName: "node2", APIHost: "192.168.1.101", APIPort: 8081, Role: RoleSlave},
			},
			mockIPs:     []string{"10.0.0.1", "172.16.0.1"}, // No localhost or matching IPs
			mockError:   nil,
			expectError: true,
			expected:    "",
		},
		{
			name: "network interface error",
			clusterMembers: []ClusterMember{
				{NodeName: "node1", APIHost: "192.168.1.100", APIPort: 8080, Role: RoleMaster},
			},
			mockIPs:     nil,
			mockError:   errors.New("network error"),
			expectError: true,
			expected:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockInterface := &MockNetworkInterface{
				ips: tt.mockIPs,
				err: tt.mockError,
			}

			result, err := DetectLocalNodeNameWithInterface(tt.clusterMembers, mockInterface)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("DetectLocalNodeNameWithInterface() = %q, expected %q", result, tt.expected)
				}
			}
		})
	}
}

func TestDetectLocalNodeName(t *testing.T) {
	// Test the wrapper function that uses DefaultNetworkInterface
	// This will use real network interfaces, so we just test that it doesn't panic
	clusterMembers := []ClusterMember{
		{NodeName: "node1", APIHost: "localhost", APIPort: 8080, Role: RoleMaster},
	}

	// This should not panic, but may or may not find a match depending on the system
	_, err := DetectLocalNodeName(clusterMembers)
	// We don't assert on the result since it depends on the actual network configuration
	// We just ensure it doesn't panic and returns some result
	if err != nil {
		t.Logf("DetectLocalNodeName returned error (expected on some systems): %v", err)
	}
}
