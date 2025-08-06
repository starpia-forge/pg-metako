## Languages | 언어

- [English](README.md) (Current)
- [한국어](README_ko.md)

---

# pg-metako

A distributed, high-availability PostgreSQL cluster management system written in Go.

## Overview

pg-metako is a distributed PostgreSQL cluster management system that provides:

- **Distributed Architecture**: Each PostgreSQL node runs its own pg-metako instance for true distributed coordination
- **High Availability**: Automatic failover with distributed consensus and coordination
- **Intelligent Routing**: Query routing with local node preference and load balancing
- **Health Monitoring**: Continuous health checks with configurable thresholds across the cluster
- **Scalability**: Support for N-node distributed PostgreSQL clusters
- **Coordination**: Inter-node communication and consensus for failover decisions
- **Observability**: Comprehensive logging and metrics across all nodes

## Features

### Core Functionality
- ✅ Distributed PostgreSQL cluster management (minimum 2 nodes)
- ✅ Master-Slave architecture with distributed consensus for promotion
- ✅ Continuous health checking across all cluster nodes
- ✅ Automatic failover with distributed coordination and consensus
- ✅ Automatic Pair Mode: Seamless automatic failover for 2-node clusters
- ✅ Intelligent query routing with local node preference
- ✅ Load balancing algorithms (Round-Robin, Least Connections)
- ✅ Inter-node communication and coordination API

### Configuration Management
- ✅ YAML configuration files
- ✅ Dynamic configuration reloading
- ✅ Comprehensive validation

### Reliability & Monitoring
- ✅ Graceful handling of network partitions
- ✅ Comprehensive logging for all operations
- ✅ Connection statistics and monitoring
- ✅ Periodic cluster status reporting

## Quick Start

### Prerequisites

- Go 1.24 or later (for local development)
- Docker and Docker Compose (for containerized deployment)
- PostgreSQL 12+ with replication configured (for local development)

### Docker Deployment (Recommended)

The easiest way to run pg-metako is using Docker Compose:

1. Clone the repository:
```bash
git clone <repository-url>
cd pg-metako
```

2. Setup environment and start services:
```bash
./scripts/deploy.sh start
```

3. Check service status:
```bash
./scripts/deploy.sh status
```

### Local Development

1. Clone the repository:
```bash
git clone <repository-url>
cd pg-metako
```

2. Build the application:
```bash
# Using Makefile (recommended)
make build

# Or using Go directly
go build -o bin/pg-metako ./cmd/pg-metako
```

3. Run with example configuration:
```bash
./bin/pg-metako --config configs/config.yaml
```

### Configuration

Create a YAML configuration file for distributed deployment:

```yaml
# Node identity - identifies this pg-metako instance
identity:
  node_name: "pg-metako-node1"    # Unique name for this node
  local_db_host: "localhost"      # Host of local PostgreSQL instance
  local_db_port: 5432            # Port of local PostgreSQL instance
  api_host: "0.0.0.0"            # Host for inter-node API
  api_port: 8080                 # Port for inter-node API

# Local PostgreSQL database configuration
local_db:
  name: "postgres-node1"
  host: "localhost"
  port: 5432
  role: "master"                 # Role of local PostgreSQL instance
  username: "postgres"
  password: "password"
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
  cluster_mode:
    heartbeat_interval: "10s"
    communication_timeout: "5s"
    failover_timeout: "30s"
    min_consensus_nodes: 2
    local_node_preference: 0.8
  pair_mode:
    enable: true
    failover_delay: "10s"
    failure_threshold: 3

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

## Architecture

### Distributed Architecture

pg-metako uses a distributed architecture where each PostgreSQL node runs its own pg-metako instance. This provides true distributed coordination and eliminates single points of failure.

### Components

1. **Configuration Manager**: Loads and validates distributed YAML configuration
2. **Local Database Connection Manager**: Manages connection to the local PostgreSQL instance
3. **Health Checker**: Monitors health of all cluster nodes with configurable intervals
4. **Distributed Replication Manager**: Handles failover coordination and consensus across nodes
5. **Coordination API**: Provides inter-node communication and cluster membership management
6. **Unified Router**: Routes queries with local node preference and intelligent load balancing

### Distributed Coordination

- **Cluster Membership**: Each node maintains awareness of all other pg-metako nodes
- **Heartbeat System**: Regular heartbeats ensure cluster health and detect node failures
- **Consensus Protocol**: Distributed consensus for failover decisions and master promotion
- **Local Node Preference**: Queries are preferentially routed to local PostgreSQL instances when possible

### Query Routing

- **Write Queries**: Always routed to the current master node
  - `INSERT`, `UPDATE`, `DELETE`, `CREATE`, `DROP`, `ALTER`, etc.
- **Read Queries**: Distributed across healthy slave nodes
  - `SELECT`, `SHOW`, `DESCRIBE`, `EXPLAIN`, `WITH`

### Load Balancing Algorithms

1. **Round-Robin**: Distributes queries evenly across all healthy slaves
2. **Least Connected**: Routes to the slave with the fewest active connections

### Distributed Failover Process

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

## Deployment

pg-metako supports two deployment models:

1. **Docker Deployment (Development/Testing)**: Quick setup using Docker Compose
2. **Distributed Deployment (Production)**: Each PostgreSQL node runs its own pg-metako instance

### Quick Docker Setup

For development and testing, use Docker Compose:

```bash
# Setup and start all services
./scripts/deploy.sh start

# Check status
./scripts/deploy.sh status
```

📖 **For detailed Docker deployment instructions, see [Docker Deployment Guide](docs/docker-deployment.md)**

### Production Deployment

For production environments, deploy pg-metako in distributed mode where each PostgreSQL node runs its own pg-metako instance for true high availability.

📖 **For detailed production deployment instructions, see [Distributed Deployment Guide](docs/distributed-deployment.md)**

## Usage

### Command Line Options

```bash
./bin/pg-metako [options]

Options:
  --config string    Path to configuration file (default "configs/config.yaml")
  --version         Show version information
```

### Running the Application

1. **Start with default configuration**:
```bash
./bin/pg-metako
```

2. **Start with custom configuration**:
```bash
./bin/pg-metako --config /path/to/config.yaml
```

3. **Check version**:
```bash
./bin/pg-metako --version
```

### Monitoring

The application provides comprehensive structured logging in JSON format:

```
{"time":"2025-08-03T22:41:20.552941+09:00","level":"INFO","msg":"Loading configuration from configs/config.yaml"}
{"time":"2025-08-03T22:41:20.553267+09:00","level":"INFO","msg":"Configuration loaded successfully with 3 nodes"}
{"time":"2025-08-03T22:41:20.553269+09:00","level":"INFO","msg":"Added node master-1 (master) to cluster"}
{"time":"2025-08-03T22:41:20.553271+09:00","level":"INFO","msg":"Added node slave-1 (slave) to cluster"}
{"time":"2025-08-03T22:41:20.553276+09:00","level":"INFO","msg":"Health monitoring started"}
{"time":"2025-08-03T22:41:20.553277+09:00","level":"INFO","msg":"Failover monitoring started"}
{"time":"2025-08-03T22:41:20.553278+09:00","level":"INFO","msg":"Application started successfully"}
```

Periodic status reports include:
- Current master node
- Health status of all nodes
- Query statistics (total, reads, writes, failures)
- Overall cluster health

## Development

### Quick Start

```bash
# Build the application
make build

# Run tests
make test

# Run with example configuration
./bin/pg-metako --config configs/config.yaml
```

📖 **For detailed development setup, project structure, testing, and contribution guidelines, see [Development Guide](docs/development.md)**

## PostgreSQL Setup

For PostgreSQL master-slave replication configuration:

📖 **For detailed PostgreSQL setup instructions, see [PostgreSQL Setup Guide](docs/postgresql-setup.md)**

## Troubleshooting

For common issues and solutions:

📖 **For comprehensive troubleshooting information, see [Troubleshooting Guide](docs/troubleshooting.md)**

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.