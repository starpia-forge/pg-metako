// Purpose    : Tests for distributed configuration management
// Context    : Validates distributed config loading and node identity detection
// Constraints: Must ensure configuration validation and proper node setup

package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDistributedFromFile(t *testing.T) {
	// Create a temporary config file
	configContent := `
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

coordination:
  heartbeat_interval: "10s"
  communication_timeout: "5s"
  failover_timeout: "30s"
  min_consensus_nodes: 1
  local_node_preference: 0.8

health_check:
  interval: "15s"
  timeout: "5s"
  failure_threshold: 3

load_balancer:
  algorithm: "round_robin"
  read_timeout: "10s"
  write_timeout: "10s"

security:
  tls_enabled: false
  cert_file: ""
  key_file: ""
`

	// Write to temporary file
	tmpFile, err := os.CreateTemp("", "distributed-config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()

	// Test loading configuration
	cfg, err := LoadDistributedFromFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load distributed config: %v", err)
	}

	// Validate configuration
	if cfg.Identity.NodeName != "test-node" {
		t.Errorf("Expected node name 'test-node', got '%s'", cfg.Identity.NodeName)
	}

	if cfg.LocalDB.Role != RoleMaster {
		t.Errorf("Expected local DB role 'master', got '%s'", cfg.LocalDB.Role)
	}

	if len(cfg.ClusterMembers) != 1 {
		t.Errorf("Expected 1 cluster member, got %d", len(cfg.ClusterMembers))
	}

	if cfg.Coordination.LocalNodePreference != 0.8 {
		t.Errorf("Expected local node preference 0.8, got %f", cfg.Coordination.LocalNodePreference)
	}
}

func TestDistributedConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      DistributedConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: DistributedConfig{
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
			},
			expectError: false,
		},
		{
			name: "empty node name",
			config: DistributedConfig{
				Identity: NodeIdentity{
					NodeName:    "",
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
			name: "no cluster members",
			config: DistributedConfig{
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
				ClusterMembers: []ClusterMember{},
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
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestIsLocalNode(t *testing.T) {
	cfg := &DistributedConfig{
		Identity: NodeIdentity{
			NodeName: "test-node",
		},
	}

	if !cfg.IsLocalNode("test-node") {
		t.Error("Expected IsLocalNode to return true for local node")
	}

	if cfg.IsLocalNode("other-node") {
		t.Error("Expected IsLocalNode to return false for other node")
	}
}

func TestGetClusterMemberByName(t *testing.T) {
	cfg := &DistributedConfig{
		ClusterMembers: []ClusterMember{
			{
				NodeName: "node1",
				APIHost:  "host1",
				APIPort:  8080,
				Role:     RoleMaster,
			},
			{
				NodeName: "node2",
				APIHost:  "host2",
				APIPort:  8080,
				Role:     RoleSlave,
			},
		},
	}

	// Test existing member
	member, err := cfg.GetClusterMemberByName("node1")
	if err != nil {
		t.Fatalf("Expected to find node1, got error: %v", err)
	}
	if member.APIHost != "host1" {
		t.Errorf("Expected API host 'host1', got '%s'", member.APIHost)
	}

	// Test non-existing member
	_, err = cfg.GetClusterMemberByName("node3")
	if err == nil {
		t.Error("Expected error for non-existing node")
	}
}

func TestGetMasterAndSlaveNodes(t *testing.T) {
	cfg := &DistributedConfig{
		ClusterMembers: []ClusterMember{
			{
				NodeName: "master1",
				Role:     RoleMaster,
			},
			{
				NodeName: "slave1",
				Role:     RoleSlave,
			},
			{
				NodeName: "slave2",
				Role:     RoleSlave,
			},
		},
	}

	masters := cfg.GetMasterNodes()
	if len(masters) != 1 {
		t.Errorf("Expected 1 master node, got %d", len(masters))
	}
	if masters[0].NodeName != "master1" {
		t.Errorf("Expected master node 'master1', got '%s'", masters[0].NodeName)
	}

	slaves := cfg.GetSlaveNodes()
	if len(slaves) != 2 {
		t.Errorf("Expected 2 slave nodes, got %d", len(slaves))
	}
}

// Test loading 2-node configuration files
func TestLoad2NodeDistributedConfigs(t *testing.T) {
	testCases := []struct {
		name                 string
		configFile           string
		expectedNode         string
		expectedRole         NodeRole
		expectedMinConsensus int
	}{
		{
			name:                 "2-node master config",
			configFile:           "../../configs/distributed-2node-node1.yaml",
			expectedNode:         "node1",
			expectedRole:         RoleMaster,
			expectedMinConsensus: 1,
		},
		{
			name:                 "2-node slave config",
			configFile:           "../../configs/distributed-2node-node2.yaml",
			expectedNode:         "node2",
			expectedRole:         RoleSlave,
			expectedMinConsensus: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := LoadDistributedFromFile(tc.configFile)
			if err != nil {
				t.Fatalf("Failed to load 2-node config %s: %v", tc.configFile, err)
			}

			// Validate node identity
			if cfg.Identity.NodeName != tc.expectedNode {
				t.Errorf("Expected node name %s, got %s", tc.expectedNode, cfg.Identity.NodeName)
			}

			// Validate local DB role
			if cfg.LocalDB.Role != tc.expectedRole {
				t.Errorf("Expected local DB role %s, got %s", tc.expectedRole, cfg.LocalDB.Role)
			}

			// Validate 2-node specific settings
			if cfg.Coordination.MinConsensusNodes != tc.expectedMinConsensus {
				t.Errorf("Expected min_consensus_nodes %d, got %d",
					tc.expectedMinConsensus, cfg.Coordination.MinConsensusNodes)
			}

			// Validate cluster size
			if len(cfg.ClusterMembers) != 2 {
				t.Errorf("Expected 2 cluster members, got %d", len(cfg.ClusterMembers))
			}

			// Validate local node preference
			if cfg.Coordination.LocalNodePreference != 0.8 {
				t.Errorf("Expected local node preference 0.8, got %f",
					cfg.Coordination.LocalNodePreference)
			}

			// Validate that both master and slave roles are present in cluster
			hasmaster := false
			hasSlave := false
			for _, member := range cfg.ClusterMembers {
				if member.Role == RoleMaster {
					hasmaster = true
				}
				if member.Role == RoleSlave {
					hasSlave = true
				}
			}

			if !hasmaster {
				t.Error("Expected at least one master node in cluster members")
			}
			if !hasSlave {
				t.Error("Expected at least one slave node in cluster members")
			}
		})
	}
}

// Test 2-node configuration validation
func Test2NodeConfigValidation(t *testing.T) {
	// Test valid 2-node configuration
	validConfig := &DistributedConfig{
		Identity: NodeIdentity{
			NodeName:    "node1",
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
			{NodeName: "node1", APIHost: "localhost", APIPort: 8080, Role: RoleMaster},
			{NodeName: "node2", APIHost: "localhost", APIPort: 8081, Role: RoleSlave},
		},
		Coordination: CoordinationConfig{
			HeartbeatInterval:    10 * time.Second,
			CommunicationTimeout: 5 * time.Second,
			FailoverTimeout:      30 * time.Second,
			MinConsensusNodes:    1, // Key 2-node setting
			LocalNodePreference:  0.8,
		},
		HealthCheck: HealthCheckConfig{
			Interval:         15 * time.Second,
			Timeout:          5 * time.Second,
			FailureThreshold: 3,
		},
		LoadBalancer: LoadBalancerConfig{
			Algorithm:    AlgorithmRoundRobin,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
		Security: SecurityConfig{
			TLSEnabled: false,
			CertFile:   "",
			KeyFile:    "",
		},
	}

	err := validConfig.Validate()
	if err != nil {
		t.Errorf("Expected valid 2-node configuration to pass validation, got error: %v", err)
	}

	// Verify key 2-node settings
	if validConfig.Coordination.MinConsensusNodes != 1 {
		t.Errorf("Expected min_consensus_nodes to be 1 for 2-node setup, got: %d",
			validConfig.Coordination.MinConsensusNodes)
	}

	if len(validConfig.ClusterMembers) != 2 {
		t.Errorf("Expected 2 cluster members, got: %d", len(validConfig.ClusterMembers))
	}
}

// Test 2-node cluster member operations
func Test2NodeClusterOperations(t *testing.T) {
	cfg := &DistributedConfig{
		Identity: NodeIdentity{
			NodeName: "node1",
		},
		ClusterMembers: []ClusterMember{
			{NodeName: "node1", APIHost: "host1", APIPort: 8080, Role: RoleMaster},
			{NodeName: "node2", APIHost: "host2", APIPort: 8080, Role: RoleSlave},
		},
	}

	// Test getting master nodes
	masters := cfg.GetMasterNodes()
	if len(masters) != 1 {
		t.Errorf("Expected 1 master node in 2-node setup, got %d", len(masters))
	}
	if masters[0].NodeName != "node1" {
		t.Errorf("Expected master node to be node1, got %s", masters[0].NodeName)
	}

	// Test getting slave nodes
	slaves := cfg.GetSlaveNodes()
	if len(slaves) != 1 {
		t.Errorf("Expected 1 slave node in 2-node setup, got %d", len(slaves))
	}
	if slaves[0].NodeName != "node2" {
		t.Errorf("Expected slave node to be node2, got %s", slaves[0].NodeName)
	}

	// Test getting cluster member by name
	member, err := cfg.GetClusterMemberByName("node1")
	if err != nil {
		t.Fatalf("Expected to find node1, got error: %v", err)
	}
	if member.Role != RoleMaster {
		t.Errorf("Expected node1 to be master, got role: %s", member.Role)
	}

	member, err = cfg.GetClusterMemberByName("node2")
	if err != nil {
		t.Fatalf("Expected to find node2, got error: %v", err)
	}
	if member.Role != RoleSlave {
		t.Errorf("Expected node2 to be slave, got role: %s", member.Role)
	}

	// Test local node identification
	if !cfg.IsLocalNode("node1") {
		t.Error("Expected IsLocalNode to return true for node1")
	}
	if cfg.IsLocalNode("node2") {
		t.Error("Expected IsLocalNode to return false for node2")
	}
}

// Test 2-node configuration edge cases
func Test2NodeConfigEdgeCases(t *testing.T) {
	// Test configuration with min_consensus_nodes = 0 (should fail)
	invalidConfig := &DistributedConfig{
		Identity: NodeIdentity{
			NodeName:    "node1",
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
			{NodeName: "node1", APIHost: "localhost", APIPort: 8080, Role: RoleMaster},
			{NodeName: "node2", APIHost: "localhost", APIPort: 8081, Role: RoleSlave},
		},
		Coordination: CoordinationConfig{
			HeartbeatInterval:    10 * time.Second,
			CommunicationTimeout: 5 * time.Second,
			FailoverTimeout:      30 * time.Second,
			MinConsensusNodes:    0, // Invalid: should be at least 1
			LocalNodePreference:  0.8,
		},
	}

	err := invalidConfig.Validate()
	if err == nil {
		t.Error("Expected configuration with min_consensus_nodes=0 to fail validation")
	}

	// Test configuration with local_node_preference > 1.0 (should fail)
	invalidConfig.Coordination.MinConsensusNodes = 1
	invalidConfig.Coordination.LocalNodePreference = 1.5 // Invalid: should be <= 1.0

	err = invalidConfig.Validate()
	if err == nil {
		t.Error("Expected configuration with local_node_preference>1.0 to fail validation")
	}
}
