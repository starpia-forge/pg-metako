package config

import (
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
