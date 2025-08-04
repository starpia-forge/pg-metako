package config

import (
	"errors"
	"fmt"
	"net"
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

// NodeIdentity represents the identity of the current pg-metako instance
type NodeIdentity struct {
	NodeName    string `yaml:"node_name"`     // Name of this node
	LocalDBHost string `yaml:"local_db_host"` // Host of local PostgreSQL instance
	LocalDBPort int    `yaml:"local_db_port"` // Port of local PostgreSQL instance
	APIHost     string `yaml:"api_host"`      // Host for inter-node API
	APIPort     int    `yaml:"api_port"`      // Port for inter-node API
}

// ClusterMember represents a member of the pg-metako cluster
type ClusterMember struct {
	NodeName string   `yaml:"node_name"`
	APIHost  string   `yaml:"api_host"`
	APIPort  int      `yaml:"api_port"`
	Role     NodeRole `yaml:"role"` // Role of the PostgreSQL instance on this node
}

// CoordinationConfig represents settings for inter-node coordination
type CoordinationConfig struct {
	// Heartbeat interval for cluster membership
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval"`

	// Timeout for inter-node communication
	CommunicationTimeout time.Duration `yaml:"communication_timeout"`

	// Failover coordination timeout
	FailoverTimeout time.Duration `yaml:"failover_timeout"`

	// Minimum nodes required for failover consensus
	MinConsensusNodes int `yaml:"min_consensus_nodes"`

	// Local node preference weight (0.0 to 1.0)
	LocalNodePreference float64 `yaml:"local_node_preference"`
}

// Config represents the complete application configuration for distributed deployment
type Config struct {
	// Node identity
	Identity NodeIdentity `yaml:"identity"`

	// Local PostgreSQL configuration
	LocalDB NodeConfig `yaml:"local_db"`

	// Cluster members for coordination
	ClusterMembers []ClusterMember `yaml:"cluster_members"`

	// Coordination settings
	Coordination CoordinationConfig `yaml:"coordination"`

	// Health check configuration
	HealthCheck HealthCheckConfig `yaml:"health_check"`

	// Load balancer configuration
	LoadBalancer LoadBalancerConfig `yaml:"load_balancer"`

	// Security configuration
	Security SecurityConfig `yaml:"security"`
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

// Validate validates the distributed configuration
func (c *Config) Validate() error {
	// Validate node identity
	if err := c.Identity.validate(); err != nil {
		return fmt.Errorf("identity validation failed: %w", err)
	}

	// Validate local database configuration
	if err := c.LocalDB.validate(); err != nil {
		return fmt.Errorf("local database validation failed: %w", err)
	}

	// Validate cluster members
	if len(c.ClusterMembers) == 0 {
		return errors.New("at least one cluster member must be configured")
	}

	for i, member := range c.ClusterMembers {
		if err := member.validate(); err != nil {
			return fmt.Errorf("cluster member %d validation failed: %w", i, err)
		}
	}

	// Validate coordination settings
	if err := c.Coordination.validate(); err != nil {
		return fmt.Errorf("coordination validation failed: %w", err)
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

// validate validates node identity
func (ni *NodeIdentity) validate() error {
	if ni.NodeName == "" {
		return errors.New("node name cannot be empty")
	}
	if ni.LocalDBHost == "" {
		return errors.New("local database host cannot be empty")
	}
	if ni.LocalDBPort <= 0 || ni.LocalDBPort > 65535 {
		return errors.New("local database port must be between 1 and 65535")
	}
	if ni.APIHost == "" {
		return errors.New("API host cannot be empty")
	}
	if ni.APIPort <= 0 || ni.APIPort > 65535 {
		return errors.New("API port must be between 1 and 65535")
	}
	return nil
}

// validate validates cluster member configuration
func (cm *ClusterMember) validate() error {
	if cm.NodeName == "" {
		return errors.New("cluster member node name cannot be empty")
	}
	if cm.APIHost == "" {
		return errors.New("cluster member API host cannot be empty")
	}
	if cm.APIPort <= 0 || cm.APIPort > 65535 {
		return errors.New("cluster member API port must be between 1 and 65535")
	}
	if cm.Role != RoleMaster && cm.Role != RoleSlave {
		return errors.New("cluster member role must be either 'master' or 'slave'")
	}
	return nil
}

// validate validates coordination configuration
func (cc *CoordinationConfig) validate() error {
	if cc.HeartbeatInterval <= 0 {
		return errors.New("heartbeat interval must be positive")
	}
	if cc.CommunicationTimeout <= 0 {
		return errors.New("communication timeout must be positive")
	}
	if cc.FailoverTimeout <= 0 {
		return errors.New("failover timeout must be positive")
	}
	if cc.MinConsensusNodes < 1 {
		return errors.New("minimum consensus nodes must be at least 1")
	}
	if cc.LocalNodePreference < 0.0 || cc.LocalNodePreference > 1.0 {
		return errors.New("local node preference must be between 0.0 and 1.0")
	}
	return nil
}

// IsLocalNode checks if the given node name is the local node
func (c *Config) IsLocalNode(nodeName string) bool {
	return c.Identity.NodeName == nodeName
}

// GetLocalDBConfig returns the local database configuration as a NodeConfig
func (c *Config) GetLocalDBConfig() NodeConfig {
	return c.LocalDB
}

// GetClusterMemberByName returns a cluster member by name
func (c *Config) GetClusterMemberByName(nodeName string) (*ClusterMember, error) {
	for _, member := range c.ClusterMembers {
		if member.NodeName == nodeName {
			return &member, nil
		}
	}
	return nil, fmt.Errorf("cluster member %s not found", nodeName)
}

// GetMasterNodes returns all cluster members with master role
func (c *Config) GetMasterNodes() []ClusterMember {
	var masters []ClusterMember
	for _, member := range c.ClusterMembers {
		if member.Role == RoleMaster {
			masters = append(masters, member)
		}
	}
	return masters
}

// GetSlaveNodes returns all cluster members with slave role
func (c *Config) GetSlaveNodes() []ClusterMember {
	var slaves []ClusterMember
	for _, member := range c.ClusterMembers {
		if member.Role == RoleSlave {
			slaves = append(slaves, member)
		}
	}
	return slaves
}

// DetectLocalNodeName attempts to detect the local node name based on network interfaces
func DetectLocalNodeName(clusterMembers []ClusterMember) (string, error) {
	// Get all local IP addresses
	localIPs, err := getLocalIPs()
	if err != nil {
		return "", fmt.Errorf("failed to get local IPs: %w", err)
	}

	// Check if any cluster member's API host matches a local IP
	for _, member := range clusterMembers {
		for _, localIP := range localIPs {
			if member.APIHost == localIP || member.APIHost == "localhost" || member.APIHost == "127.0.0.1" {
				return member.NodeName, nil
			}
		}
	}

	return "", errors.New("could not detect local node name from cluster members")
}

// getLocalIPs returns all local IP addresses
func getLocalIPs() ([]string, error) {
	var ips []string

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					ips = append(ips, ipnet.IP.String())
				}
			}
		}
	}

	// Always include localhost
	ips = append(ips, "127.0.0.1", "localhost")

	return ips, nil
}
