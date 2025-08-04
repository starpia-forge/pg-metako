# Distributed Deployment Guide

This guide covers production deployment of pg-metako in distributed mode where each PostgreSQL node runs its own pg-metako instance.

## Overview

For production environments, deploy pg-metako in distributed mode where each PostgreSQL node runs its own pg-metako instance. This provides true high availability and distributed coordination.

## Setup Steps

### 1. Prepare Each Node

Install PostgreSQL and pg-metako on each node:

```bash
# On each node, install PostgreSQL and configure replication
# Build and install pg-metako binary
make build
sudo cp bin/pg-metako /usr/local/bin/
```

### 2. Configure Each Node

Create appropriate distributed configuration for each node:

**Node 1 (Master) - /etc/pg-metako/config.yaml**:
```yaml
# Node identity - identifies this pg-metako instance
identity:
  node_name: "pg-metako-node1"
  local_db_host: "localhost"
  local_db_port: 5432
  api_host: "0.0.0.0"
  api_port: 8080

# Local PostgreSQL database configuration
local_db:
  name: "postgres-node1"
  host: "localhost"
  port: 5432
  role: "master"
  username: "postgres"
  password: "secure_password"
  database: "myapp"

# Other pg-metako nodes in the cluster
cluster_members:
  - node_name: "pg-metako-node2"
    api_host: "192.168.1.102"
    api_port: 8080
    role: "slave"
  - node_name: "pg-metako-node3"
    api_host: "192.168.1.103"
    api_port: 8080
    role: "slave"

# Coordination settings for distributed consensus
coordination:
  heartbeat_interval: "10s"        # Heartbeat interval for cluster membership
  communication_timeout: "5s"     # Timeout for inter-node communication
  failover_timeout: "30s"         # Failover coordination timeout
  min_consensus_nodes: 2          # Minimum nodes required for failover consensus
  local_node_preference: 0.8      # Local node preference weight (0.0 to 1.0)

# Health check configuration
health_check:
  interval: "30s"          # How often to check node health
  timeout: "5s"            # Timeout for each health check
  failure_threshold: 3     # Consecutive failures before marking unhealthy

# Load balancer configuration
load_balancer:
  algorithm: "round_robin" # Options: round_robin, least_connected
  read_timeout: "10s"      # Timeout for read queries
  write_timeout: "10s"     # Timeout for write queries

# Security configuration
security:
  tls_enabled: true
  cert_file: "/path/to/cert.pem"
  key_file: "/path/to/key.pem"
```

**Node 2 (Slave) - /etc/pg-metako/config.yaml**:
```yaml
identity:
  node_name: "pg-metako-node2"
  local_db_host: "localhost"
  local_db_port: 5432
  api_host: "0.0.0.0"
  api_port: 8080

local_db:
  name: "postgres-node2"
  host: "localhost"
  port: 5432
  role: "slave"
  username: "postgres"
  password: "secure_password"
  database: "myapp"

cluster_members:
  - node_name: "pg-metako-node1"
    api_host: "192.168.1.101"
    api_port: 8080
    role: "master"
  - node_name: "pg-metako-node3"
    api_host: "192.168.1.103"
    api_port: 8080
    role: "slave"

# ... (same coordination, health_check, load_balancer, security sections)
```

### 3. Start pg-metako on Each Node

Use systemd or similar service manager:

```bash
# Create systemd service file
sudo tee /etc/systemd/system/pg-metako.service > /dev/null <<EOF
[Unit]
Description=pg-metako PostgreSQL cluster management
After=network.target postgresql.service
Requires=postgresql.service

[Service]
Type=simple
User=postgres
Group=postgres
ExecStart=/usr/local/bin/pg-metako --config /etc/pg-metako/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# Start and enable the service
sudo systemctl daemon-reload
sudo systemctl start pg-metako
sudo systemctl enable pg-metako
```

### 4. Verify Cluster Status

Check logs on any node:

```bash
# Check logs
sudo journalctl -u pg-metako -f

# Check service status
sudo systemctl status pg-metako
```

## Distributed Architecture Benefits

- **True High Availability**: No single point of failure
- **Local Query Processing**: Queries processed locally when possible
- **Distributed Consensus**: Coordinated failover decisions
- **Scalable Architecture**: Easy to add/remove nodes

## Distributed Coordination

- **Cluster Membership**: Each node maintains awareness of all other pg-metako nodes
- **Heartbeat System**: Regular heartbeats ensure cluster health and detect node failures
- **Consensus Protocol**: Distributed consensus for failover decisions and master promotion
- **Local Node Preference**: Queries are preferentially routed to local PostgreSQL instances when possible

## Distributed Failover Process

1. **Failure Detection**: Multiple pg-metako nodes detect master failure through health checks
2. **Consensus Initiation**: Nodes communicate to reach consensus on master failure
3. **Leader Election**: Distributed consensus selects the best candidate for promotion based on:
   - PostgreSQL replication lag
   - Node health and availability
   - Local node preference weights
4. **Coordinated Promotion**: The selected node promotes its local PostgreSQL instance to master
5. **Cluster Reconfiguration**: All nodes update their routing to use the new master
6. **Replication Restart**: Remaining slaves are reconfigured to follow the new master
7. **State Synchronization**: Cluster state is synchronized across all pg-metako nodes

## Monitoring and Maintenance

### Health Monitoring

Each node provides health status through:
- Local health checks of PostgreSQL instance
- Inter-node communication status
- Cluster membership awareness

### Log Monitoring

Monitor logs on each node:
```bash
# Follow logs in real-time
sudo journalctl -u pg-metako -f

# View recent logs
sudo journalctl -u pg-metako --since "1 hour ago"
```

### Adding New Nodes

1. Install PostgreSQL and pg-metako on the new node
2. Configure PostgreSQL replication from current master
3. Create pg-metako configuration with updated cluster_members
4. Update existing nodes' configurations to include the new node
5. Restart pg-metako services to apply new configuration

### Removing Nodes

1. Stop pg-metako service on the node to be removed
2. Update cluster_members configuration on remaining nodes
3. Restart pg-metako services on remaining nodes
4. Remove PostgreSQL replication configuration if needed