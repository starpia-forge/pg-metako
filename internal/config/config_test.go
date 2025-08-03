package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfigFromYAML(t *testing.T) {
	// Create a temporary YAML config file
	yamlContent := `
nodes:
  - name: "master-1"
    host: "localhost"
    port: 5432
    role: "master"
    username: "postgres"
    password: "password"
    database: "testdb"
  - name: "slave-1"
    host: "localhost"
    port: 5433
    role: "slave"
    username: "postgres"
    password: "password"
    database: "testdb"

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
	if len(config.Nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(config.Nodes))
	}

	if config.Nodes[0].Name != "master-1" {
		t.Errorf("Expected first node name to be 'master-1', got '%s'", config.Nodes[0].Name)
	}

	if config.Nodes[0].Role != RoleMaster {
		t.Errorf("Expected first node role to be master, got %s", config.Nodes[0].Role)
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

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "valid config",
			config: &Config{
				Nodes: []NodeConfig{
					{
						Name:     "master-1",
						Host:     "localhost",
						Port:     5432,
						Role:     RoleMaster,
						Username: "postgres",
						Password: "password",
						Database: "testdb",
					},
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
			name: "no master node",
			config: &Config{
				Nodes: []NodeConfig{
					{
						Name:     "slave-1",
						Host:     "localhost",
						Port:     5432,
						Role:     RoleSlave,
						Username: "postgres",
						Password: "password",
						Database: "testdb",
					},
				},
			},
			expectError: true,
		},
		{
			name: "empty nodes",
			config: &Config{
				Nodes: []NodeConfig{},
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
