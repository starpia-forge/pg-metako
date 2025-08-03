package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// NodeRole represents the role of a database node
type NodeRole string

const (
	RoleMaster NodeRole = "master"
	RoleSlave  NodeRole = "slave"
)

// LoadBalancerAlgorithm represents the load balancing algorithm
type LoadBalancerAlgorithm string

const (
	AlgorithmRoundRobin     LoadBalancerAlgorithm = "round_robin"
	AlgorithmLeastConnected LoadBalancerAlgorithm = "least_connected"
)

// NodeConfig represents configuration for a single database node
type NodeConfig struct {
	Name     string   `yaml:"name"`
	Host     string   `yaml:"host"`
	Port     int      `yaml:"port"`
	Role     NodeRole `yaml:"role"`
	Username string   `yaml:"username"`
	Password string   `yaml:"password"`
	Database string   `yaml:"database"`
}

// HealthCheckConfig represents health check configuration
type HealthCheckConfig struct {
	Interval         time.Duration `yaml:"interval"`
	Timeout          time.Duration `yaml:"timeout"`
	FailureThreshold int           `yaml:"failure_threshold"`
}

// LoadBalancerConfig represents load balancer configuration
type LoadBalancerConfig struct {
	Algorithm    LoadBalancerAlgorithm `yaml:"algorithm"`
	ReadTimeout  time.Duration         `yaml:"read_timeout"`
	WriteTimeout time.Duration         `yaml:"write_timeout"`
}

// SecurityConfig represents security configuration
type SecurityConfig struct {
	TLSEnabled bool   `yaml:"tls_enabled"`
	CertFile   string `yaml:"cert_file"`
	KeyFile    string `yaml:"key_file"`
}

// Config represents the complete application configuration
type Config struct {
	Nodes        []NodeConfig       `yaml:"nodes"`
	HealthCheck  HealthCheckConfig  `yaml:"health_check"`
	LoadBalancer LoadBalancerConfig `yaml:"load_balancer"`
	Security     SecurityConfig     `yaml:"security"`
}

// LoadFromFile loads configuration from a YAML file
func LoadFromFile(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if len(c.Nodes) == 0 {
		return errors.New("at least one node must be configured")
	}

	// Check if there's at least one master node
	hasMaster := false
	for _, node := range c.Nodes {
		if node.Role == RoleMaster {
			hasMaster = true
			break
		}
	}

	if !hasMaster {
		return errors.New("at least one master node must be configured")
	}

	// Validate individual nodes
	for i, node := range c.Nodes {
		if err := node.validate(); err != nil {
			return fmt.Errorf("node %d validation failed: %w", i, err)
		}
	}

	return nil
}

// Validate validates a single node configuration (public method)
func (n *NodeConfig) Validate() error {
	return n.validate()
}

// validate validates a single node configuration
func (n *NodeConfig) validate() error {
	if n.Name == "" {
		return errors.New("node name cannot be empty")
	}
	if n.Host == "" {
		return errors.New("node host cannot be empty")
	}
	if n.Port <= 0 || n.Port > 65535 {
		return errors.New("node port must be between 1 and 65535")
	}
	if n.Role != RoleMaster && n.Role != RoleSlave {
		return errors.New("node role must be either 'master' or 'slave'")
	}
	if n.Username == "" {
		return errors.New("node username cannot be empty")
	}
	if n.Database == "" {
		return errors.New("node database cannot be empty")
	}
	return nil
}
