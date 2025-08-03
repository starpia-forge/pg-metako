# pg-metako

A high-availability PostgreSQL replication management system written in Go.

## Overview

pg-metako is a robust PostgreSQL replication management system that provides:

- **High Availability**: Automatic failover when master nodes fail
- **Load Balancing**: Intelligent query routing with multiple algorithms
- **Health Monitoring**: Continuous health checks with configurable thresholds
- **Scalability**: Support for N-node replication setups
- **Observability**: Comprehensive logging and metrics

## Features

### Core Functionality
- ✅ N-node PostgreSQL replication support (minimum 2 nodes)
- ✅ Master-Slave architecture with automatic promotion
- ✅ Continuous health checking with configurable intervals
- ✅ Automatic failover with configurable failure thresholds
- ✅ Query routing (writes to master, reads to slaves)
- ✅ Load balancing algorithms (Round-Robin, Least Connections)

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

- Go 1.24 or later
- PostgreSQL 12+ with replication configured

### Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd pg-metako
```

2. Build the application:
```bash
go build -o bin/pg-metako ./cmd/pg-metako
```

3. Run with example configuration:
```bash
./bin/pg-metako --config configs/example.yaml
```

### Configuration

Create a YAML configuration file (see `configs/example.yaml`):

```yaml
# Database nodes configuration
nodes:
  - name: "master-1"
    host: "localhost"
    port: 5432
    role: "master"
    username: "postgres"
    password: "password"
    database: "myapp"
  
  - name: "slave-1"
    host: "localhost"
    port: 5433
    role: "slave"
    username: "postgres"
    password: "password"
    database: "myapp"

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

### Components

1. **Configuration Manager**: Loads and validates YAML configuration
2. **Database Connection Manager**: Manages PostgreSQL connections
3. **Health Checker**: Monitors node health with configurable intervals
4. **Replication Manager**: Handles failover and slave promotion
5. **Query Router**: Routes queries based on type and load balancing

### Query Routing

- **Write Queries**: Always routed to the current master node
  - `INSERT`, `UPDATE`, `DELETE`, `CREATE`, `DROP`, `ALTER`, etc.
- **Read Queries**: Distributed across healthy slave nodes
  - `SELECT`, `SHOW`, `DESCRIBE`, `EXPLAIN`, `WITH`

### Load Balancing Algorithms

1. **Round-Robin**: Distributes queries evenly across all healthy slaves
2. **Least Connected**: Routes to the slave with the fewest active connections

### Failover Process

1. Health checker detects master failure (consecutive failures exceed threshold)
2. Replication manager selects a healthy slave for promotion
3. Slave is promoted to master (PostgreSQL promotion commands)
4. Remaining slaves are reconfigured to follow the new master
5. Query router updates routing to use the new master

## Usage

### Command Line Options

```bash
./bin/pg-metako [options]

Options:
  --config string    Path to configuration file (default "configs/example.yaml")
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

The application provides comprehensive logging:

```
2025/08/03 15:46:51 Loading configuration from configs/example.yaml
2025/08/03 15:46:51 Configuration loaded successfully with 3 nodes
2025/08/03 15:46:51 Added node master-1 (master) to cluster
2025/08/03 15:46:51 Added node slave-1 (slave) to cluster
2025/08/03 15:46:51 Health monitoring started
2025/08/03 15:46:51 Failover monitoring started
2025/08/03 15:46:51 pg-metako started successfully
```

Periodic status reports include:
- Current master node
- Health status of all nodes
- Query statistics (total, reads, writes, failures)
- Overall cluster health

## Development

### Project Structure

The project follows the [golang-standards/project-layout](https://github.com/golang-standards/project-layout):

```
├── cmd/pg-metako/          # Main application
├── internal/pkg/           # Private application packages
│   ├── config/            # Configuration management
│   ├── database/          # Database connection handling
│   ├── health/            # Health checking
│   ├── replication/       # Replication and failover
│   └── routing/           # Query routing and load balancing
├── configs/               # Configuration examples
├── docs/                  # Documentation
└── examples/              # Usage examples
```

### Testing

Run all tests:
```bash
go test ./...
```

Run tests with verbose output:
```bash
go test -v ./...
```

Run tests for a specific package:
```bash
go test -v ./internal/pkg/config
```

### Development Principles

The project follows Test-Driven Development (TDD) and Kent Beck's "Tidy First" principles:
- Red-Green-Refactor cycle
- Comprehensive test coverage
- Clean, readable code
- Separation of concerns

## PostgreSQL Setup

### Master Configuration

Add to `postgresql.conf`:
```
wal_level = replica
max_wal_senders = 3
wal_keep_segments = 64
```

Add to `pg_hba.conf`:
```
host replication replicator 0.0.0.0/0 md5
```

### Slave Configuration

Create `recovery.conf`:
```
standby_mode = 'on'
primary_conninfo = 'host=master_ip port=5432 user=replicator'
```

## Troubleshooting

### Common Issues

1. **Connection refused**: Check PostgreSQL is running and accessible
2. **Authentication failed**: Verify username/password in configuration
3. **No healthy slaves**: Check slave node connectivity and replication status
4. **Failover not working**: Verify failure threshold and health check settings

### Logs

All operations are logged with timestamps and context:
- Configuration loading and validation
- Node health status changes
- Failover events and slave promotions
- Query routing decisions
- Connection statistics

## Contributing

1. Fork the repository
2. Create a feature branch
3. Write tests for new functionality
4. Implement the feature following TDD principles
5. Ensure all tests pass
6. Submit a pull request

## License

[Add your license information here]

## Support

[Add support information here]