# Docker Deployment Guide

This guide covers Docker-based deployment options for pg-metako.

## Overview

pg-metako provides two deployment models:

1. **Single-Instance Deployment (Development/Testing)**: One pg-metako instance manages multiple PostgreSQL containers
2. **Distributed Deployment (Production)**: Each PostgreSQL node runs its own pg-metako instance

The current Docker Compose setup provides a **single-instance deployment** for development and testing:
- PostgreSQL master and slave containers with automatic replication setup
- Single pg-metako application container managing all PostgreSQL instances
- Optional pgAdmin for database management
- Persistent data volumes
- Easy configuration through environment variables

**Note**: For production use, consider distributed deployment where each PostgreSQL node runs its own pg-metako instance for true high availability and distributed coordination.

## Quick Docker Setup

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

## Docker Services

The Docker Compose setup includes:

- **postgres-master**: PostgreSQL master database (port 5432)
- **postgres-slave1**: PostgreSQL slave database (port 5433)
- **postgres-slave2**: PostgreSQL slave database (port 5434)
- **pg-metako**: Main application container (port 8080)
- **pgadmin**: Database management interface (port 8081, optional)

## Environment Configuration

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

## Deployment Script Commands

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

## Data Persistence

Docker volumes ensure data persistence:
- `postgres_master_data`: Master database data
- `postgres_slave1_data`: Slave 1 database data
- `postgres_slave2_data`: Slave 2 database data
- `pg_metako_logs`: Application logs
- `pgadmin_data`: pgAdmin configuration

## Accessing Services

Once deployed, services are available at:
- **PostgreSQL Master**: `localhost:5432`
- **PostgreSQL Slave 1**: `localhost:5433`
- **PostgreSQL Slave 2**: `localhost:5434`
- **pg-metako Application**: `localhost:8080`
- **pgAdmin** (if enabled): `http://localhost:8081`

## Docker Troubleshooting

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