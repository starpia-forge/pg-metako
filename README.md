## Languages | 언어

- [English](README.md) (Current)
- [한국어](README_ko.md)

---

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

## Docker Deployment

### Overview

pg-metako provides a complete Docker-based deployment solution with:
- PostgreSQL master and slave containers with automatic replication setup
- pg-metako application container
- Optional pgAdmin for database management
- Persistent data volumes
- Easy configuration through environment variables

### Quick Docker Setup

1. **Setup environment**:
```bash
./scripts/deploy.sh setup
```

2. **Edit configuration** (optional):
```bash
# Edit .env file to customize passwords and ports
nano .env
```

3. **Start all services**:
```bash
./scripts/deploy.sh start
```

4. **Check status**:
```bash
./scripts/deploy.sh status
```

### Docker Services

The Docker Compose setup includes:

- **postgres-master**: PostgreSQL master database (port 5432)
- **postgres-slave1**: PostgreSQL slave database (port 5433)
- **postgres-slave2**: PostgreSQL slave database (port 5434)
- **pg-metako**: Main application container (port 8080)
- **pgadmin**: Database management interface (port 8081, optional)

### Environment Configuration

Copy `.env.example` to `.env` and customize:

```bash
# PostgreSQL Configuration
POSTGRES_DB=myapp
POSTGRES_USER=postgres
POSTGRES_PASSWORD=your_secure_password

# Replication Configuration
POSTGRES_REPLICATION_USER=replicator
POSTGRES_REPLICATION_PASSWORD=your_replication_password

# Port Configuration
POSTGRES_MASTER_PORT=5432
POSTGRES_SLAVE1_PORT=5433
POSTGRES_SLAVE2_PORT=5434
PG_METAKO_PORT=8080
```

### Deployment Script Commands

The `scripts/deploy.sh` script provides easy management:

```bash
# Setup environment file
./scripts/deploy.sh setup

# Start all services
./scripts/deploy.sh start

# Stop all services
./scripts/deploy.sh stop

# Restart services
./scripts/deploy.sh restart

# Show service status
./scripts/deploy.sh status

# Show logs (all services)
./scripts/deploy.sh logs

# Show logs for specific service
./scripts/deploy.sh logs pg-metako

# Start with pgAdmin
./scripts/deploy.sh admin

# Clean up all containers and volumes
./scripts/deploy.sh cleanup
```

### Data Persistence

Docker volumes ensure data persistence:
- `postgres_master_data`: Master database data
- `postgres_slave1_data`: Slave 1 database data
- `postgres_slave2_data`: Slave 2 database data
- `pg_metako_logs`: Application logs
- `pgadmin_data`: pgAdmin configuration

### Accessing Services

Once deployed, services are available at:
- **PostgreSQL Master**: `localhost:5432`
- **PostgreSQL Slave 1**: `localhost:5433`
- **PostgreSQL Slave 2**: `localhost:5434`
- **pg-metako Application**: `localhost:8080`
- **pgAdmin** (if enabled): `http://localhost:8081`

### Docker Troubleshooting

**Services won't start**:
```bash
# Check Docker is running
docker info

# Check service logs
./scripts/deploy.sh logs

# Restart services
./scripts/deploy.sh restart
```

**Database connection issues**:
```bash
# Check PostgreSQL master status
docker-compose exec postgres-master pg_isready -U postgres

# Check replication status
docker-compose exec postgres-master psql -U postgres -c "SELECT * FROM pg_stat_replication;"
```

**Reset everything**:
```bash
# Clean up and start fresh
./scripts/deploy.sh cleanup
./scripts/deploy.sh start
```

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

The application provides comprehensive structured logging in JSON format:

```
{"time":"2025-08-03T22:41:20.552941+09:00","level":"INFO","msg":"Loading configuration from configs/example.yaml"}
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

### Project Structure

The project follows the [golang-standards/project-layout](https://github.com/golang-standards/project-layout):

```
├── cmd/pg-metako/          # Main application
├── internal/               # Private application packages
│   ├── config/            # Configuration management
│   ├── database/          # Database connection handling
│   ├── health/            # Health checking
│   ├── metako/            # Main application orchestration
│   ├── replication/       # Replication and failover
│   └── routing/           # Query routing and load balancing
├── configs/               # Configuration examples
├── docs/                  # Documentation
├── bin/                   # Compiled binaries
├── go.mod                 # Go module definition
├── go.sum                 # Go module checksums
└── README.md              # Project documentation
```

### Makefile

The project includes a comprehensive Makefile for build automation. View all available targets:

```bash
make help
```

#### Common Commands

**Build and Test:**
```bash
make build          # Build the application
make test           # Run all tests
make test-coverage  # Run tests with coverage report
make check          # Run format, vet, and tests
```

**Development:**
```bash
make setup          # Setup development environment
make dev-setup      # Setup with additional development tools
make fmt            # Format Go code
make vet            # Run go vet
make lint           # Run linter (requires golangci-lint)
```

**Docker:**
```bash
make docker-build   # Build Docker image
make docker-run     # Run Docker container
make docker-compose-up    # Start services with Docker Compose
make docker-compose-down  # Stop services with Docker Compose
```

**Cross-platform Builds:**
```bash
make build-all      # Build for all platforms
make build-linux    # Build for Linux
make build-windows  # Build for Windows
make build-darwin   # Build for macOS
```

**Release and Cleanup:**
```bash
make release        # Create release build
make clean          # Clean build artifacts
make ci             # Run CI pipeline
```

### Testing

Run all tests:
```bash
# Using Makefile (recommended)
make test

# Or using Go directly
go test ./...
```

Run tests with verbose output:
```bash
# Using Makefile
make test-verbose

# Or using Go directly
go test -v ./...
```

Run tests for a specific package:
```bash
go test -v ./internal/config
```

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

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.