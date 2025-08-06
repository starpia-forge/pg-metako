// Purpose    : Configuration management for PostgreSQL cluster coordination and node identity
// Context    : Multi-node PostgreSQL cluster with master-slave replication and failover coordination
// Constraints: Must support both pair-mode (2 nodes) and cluster-mode (3+ nodes) configurations
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

// ClusterModeConfig represents settings for general distributed cluster coordination
type ClusterModeConfig struct {
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

// PairModeConfig represents settings for 2-node cluster coordination
type PairModeConfig struct {
	// Enable special handling for pair clusters (automatically enabled for 2-node clusters)
	Enable bool `yaml:"enable"`

	// Additional delay before executing failover in pair mode (safety measure)
	FailoverDelay time.Duration `yaml:"failover_delay"`

	// Number of consecutive health check failures required before failover in pair mode
	FailureThreshold int `yaml:"failure_threshold"`
}

// CoordinationConfig represents settings for inter-node coordination
type CoordinationConfig struct {
	// General distributed cluster settings
	ClusterMode ClusterModeConfig `yaml:"cluster_mode"`

	// Pair mode settings (for 2-node clusters)
	PairMode PairModeConfig `yaml:"pair_mode"`

	// Legacy fields for backward compatibility (will be deprecated)
	HeartbeatInterval    time.Duration `yaml:"heartbeat_interval,omitempty"`
	CommunicationTimeout time.Duration `yaml:"communication_timeout,omitempty"`
	FailoverTimeout      time.Duration `yaml:"failover_timeout,omitempty"`
	MinConsensusNodes    int           `yaml:"min_consensus_nodes,omitempty"`
	LocalNodePreference  float64       `yaml:"local_node_preference,omitempty"`
	EnablePairMode       bool          `yaml:"enable_pair_mode,omitempty"`
	PairFailoverDelay    time.Duration `yaml:"pair_failover_delay,omitempty"`
	PairFailureThreshold int           `yaml:"pair_failure_threshold,omitempty"`
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
	totalClusterSize := len(c.ClusterMembers) + 1 // +1 for local node
	if err := c.Coordination.validate(totalClusterSize); err != nil {
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
func (cc *CoordinationConfig) validate(totalClusterSize int) error {
	// Migrate legacy configuration to new structure if needed
	cc.migrateLegacyConfig()

	// Validate cluster mode configuration
	if cc.ClusterMode.HeartbeatInterval <= 0 {
		return errors.New("cluster_mode heartbeat_interval must be positive")
	}
	if cc.ClusterMode.CommunicationTimeout <= 0 {
		return errors.New("cluster_mode communication_timeout must be positive")
	}
	if cc.ClusterMode.FailoverTimeout <= 0 {
		return errors.New("cluster_mode failover_timeout must be positive")
	}
	if cc.ClusterMode.MinConsensusNodes < 1 {
		return errors.New("cluster_mode min_consensus_nodes must be at least 1")
	}
	if cc.ClusterMode.LocalNodePreference < 0.0 || cc.ClusterMode.LocalNodePreference > 1.0 {
		return errors.New("cluster_mode local_node_preference must be between 0.0 and 1.0")
	}

	// Auto-detect and enable pair mode for 2-node clusters
	if totalClusterSize == 2 {
		cc.PairMode.Enable = true
		if cc.ClusterMode.MinConsensusNodes > 1 {
			cc.ClusterMode.MinConsensusNodes = 1 // Auto-adjust for pair mode
		}
		// Set default values if not specified
		if cc.PairMode.FailoverDelay == 0 {
			cc.PairMode.FailoverDelay = 10 * time.Second
		}
		if cc.PairMode.FailureThreshold == 0 {
			cc.PairMode.FailureThreshold = 3
		}
	}

	// Validate pair mode configuration
	if cc.PairMode.Enable {
		if totalClusterSize != 2 {
			return fmt.Errorf("pair mode can only be enabled for exactly 2 nodes (pair cluster), got %d", totalClusterSize)
		}
		if cc.ClusterMode.MinConsensusNodes != 1 {
			return errors.New("min_consensus_nodes must be 1 when pair mode is enabled")
		}
		if cc.PairMode.FailoverDelay < 0 {
			return errors.New("pair_mode failover_delay must be non-negative")
		}
		if cc.PairMode.FailureThreshold < 1 {
			return errors.New("pair_mode failure_threshold must be at least 1")
		}
	} else {
		// Standard validation for multi-node clusters
		if cc.ClusterMode.MinConsensusNodes > totalClusterSize {
			return fmt.Errorf("min_consensus_nodes (%d) cannot exceed total cluster size (%d)", cc.ClusterMode.MinConsensusNodes, totalClusterSize)
		}
	}

	return nil
}

// migrateLegacyConfig migrates legacy flat configuration to new nested structure
func (cc *CoordinationConfig) migrateLegacyConfig() {
	// Only migrate if new structure is empty and legacy fields are present
	if cc.ClusterMode.HeartbeatInterval == 0 && cc.HeartbeatInterval > 0 {
		cc.ClusterMode.HeartbeatInterval = cc.HeartbeatInterval
	}
	if cc.ClusterMode.CommunicationTimeout == 0 && cc.CommunicationTimeout > 0 {
		cc.ClusterMode.CommunicationTimeout = cc.CommunicationTimeout
	}
	if cc.ClusterMode.FailoverTimeout == 0 && cc.FailoverTimeout > 0 {
		cc.ClusterMode.FailoverTimeout = cc.FailoverTimeout
	}
	if cc.ClusterMode.MinConsensusNodes == 0 && cc.MinConsensusNodes > 0 {
		cc.ClusterMode.MinConsensusNodes = cc.MinConsensusNodes
	}
	if cc.ClusterMode.LocalNodePreference == 0.0 && cc.LocalNodePreference > 0.0 {
		cc.ClusterMode.LocalNodePreference = cc.LocalNodePreference
	}
	if !cc.PairMode.Enable && cc.EnablePairMode {
		cc.PairMode.Enable = cc.EnablePairMode
	}
	if cc.PairMode.FailoverDelay == 0 && cc.PairFailoverDelay > 0 {
		cc.PairMode.FailoverDelay = cc.PairFailoverDelay
	}
	if cc.PairMode.FailureThreshold == 0 && cc.PairFailureThreshold > 0 {
		cc.PairMode.FailureThreshold = cc.PairFailureThreshold
	}
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

// IsPairMode returns true if the cluster is configured for pair mode
func (c *Config) IsPairMode() bool {
	// Ensure migration has happened
	c.Coordination.migrateLegacyConfig()
	return c.Coordination.PairMode.Enable
}

// GetTotalClusterSize returns the total number of nodes in the cluster
func (c *Config) GetTotalClusterSize() int {
	return len(c.ClusterMembers) + 1 // +1 for local node
}

// GetPairFailoverDelay returns the failover delay for pair mode
func (c *Config) GetPairFailoverDelay() time.Duration {
	// Ensure migration has happened
	c.Coordination.migrateLegacyConfig()
	if c.Coordination.PairMode.FailoverDelay == 0 {
		return 10 * time.Second // Default delay
	}
	return c.Coordination.PairMode.FailoverDelay
}

// GetPairFailureThreshold returns the failure threshold for pair mode
func (c *Config) GetPairFailureThreshold() int {
	// Ensure migration has happened
	c.Coordination.migrateLegacyConfig()
	if c.Coordination.PairMode.FailureThreshold == 0 {
		return 3 // Default threshold
	}
	return c.Coordination.PairMode.FailureThreshold
}

// GetHeartbeatInterval returns the heartbeat interval for cluster coordination
func (c *Config) GetHeartbeatInterval() time.Duration {
	// Ensure migration has happened
	c.Coordination.migrateLegacyConfig()
	return c.Coordination.ClusterMode.HeartbeatInterval
}

// GetCommunicationTimeout returns the communication timeout for cluster coordination
func (c *Config) GetCommunicationTimeout() time.Duration {
	// Ensure migration has happened
	c.Coordination.migrateLegacyConfig()
	return c.Coordination.ClusterMode.CommunicationTimeout
}

// GetFailoverTimeout returns the failover timeout for cluster coordination
func (c *Config) GetFailoverTimeout() time.Duration {
	// Ensure migration has happened
	c.Coordination.migrateLegacyConfig()
	return c.Coordination.ClusterMode.FailoverTimeout
}

// GetMinConsensusNodes returns the minimum consensus nodes for cluster coordination
func (c *Config) GetMinConsensusNodes() int {
	// Ensure migration has happened
	c.Coordination.migrateLegacyConfig()
	return c.Coordination.ClusterMode.MinConsensusNodes
}

// GetLocalNodePreference returns the local node preference for cluster coordination
func (c *Config) GetLocalNodePreference() float64 {
	// Ensure migration has happened
	c.Coordination.migrateLegacyConfig()
	return c.Coordination.ClusterMode.LocalNodePreference
}

// NetworkInterface defines an interface for network operations to enable testing
type NetworkInterface interface {
	GetLocalIPs() ([]string, error)
}

// DefaultNetworkInterface implements NetworkInterface using real network calls
type DefaultNetworkInterface struct{}

// GetLocalIPs returns all local IP addresses using real network interfaces
func (d *DefaultNetworkInterface) GetLocalIPs() ([]string, error) {
	return getLocalIPs()
}

// DetectLocalNodeName attempts to detect the local node name based on network interfaces
func DetectLocalNodeName(clusterMembers []ClusterMember) (string, error) {
	return DetectLocalNodeNameWithInterface(clusterMembers, &DefaultNetworkInterface{})
}

// DetectLocalNodeNameWithInterface allows dependency injection for testing
func DetectLocalNodeNameWithInterface(clusterMembers []ClusterMember, netInterface NetworkInterface) (string, error) {
	// Get all local IP addresses
	localIPs, err := netInterface.GetLocalIPs()
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
