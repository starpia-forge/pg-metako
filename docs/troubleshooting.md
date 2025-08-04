# Troubleshooting Guide

This guide covers common issues and solutions when working with pg-metako.

## General Issues

### Application Won't Start

**Symptoms:**
- pg-metako exits immediately after startup
- Error messages about configuration or dependencies

**Solutions:**

1. **Check configuration file**:
```bash
# Validate configuration syntax
./bin/pg-metako --config configs/config.yaml --validate

# Check file permissions
ls -la configs/config.yaml
```

2. **Verify PostgreSQL connectivity**:
```bash
# Test connection to local database
psql -h localhost -p 5432 -U postgres -d myapp

# Test connection to remote nodes
psql -h 192.168.1.102 -p 5432 -U postgres -d myapp
```

3. **Check log files**:
```bash
# View application logs
tail -f /var/log/pg-metako/app.log

# Check system logs
journalctl -u pg-metako -f
```

### Configuration Issues

**Invalid YAML syntax:**
```bash
# Use a YAML validator
python -c "import yaml; yaml.safe_load(open('configs/config.yaml'))"

# Or use online YAML validators
```

**Missing required fields:**
- Ensure all required configuration sections are present
- Check the [distributed deployment guide](distributed-deployment.md) for complete configuration examples

## Database Connection Issues

### Connection Refused

**Symptoms:**
- "connection refused" errors in logs
- Unable to connect to PostgreSQL instances

**Solutions:**

1. **Check PostgreSQL service status**:
```bash
# Check if PostgreSQL is running
sudo systemctl status postgresql

# Start PostgreSQL if stopped
sudo systemctl start postgresql
```

2. **Verify network connectivity**:
```bash
# Test network connectivity
telnet DATABASE_HOST 5432

# Check firewall rules
sudo ufw status
sudo iptables -L
```

3. **Check PostgreSQL configuration**:
```bash
# Verify listen_addresses in postgresql.conf
grep listen_addresses /etc/postgresql/*/main/postgresql.conf

# Check pg_hba.conf for access rules
sudo cat /etc/postgresql/*/main/pg_hba.conf
```

### Authentication Failed

**Symptoms:**
- "authentication failed" errors
- Password or user-related errors

**Solutions:**

1. **Verify credentials**:
```bash
# Test credentials manually
psql -h HOST -p PORT -U USERNAME -d DATABASE

# Check user exists and has proper permissions
psql -c "SELECT usename, usesuper, userepl FROM pg_user WHERE usename='USERNAME';"
```

2. **Check pg_hba.conf**:
```bash
# Ensure proper authentication method
# Example for md5 authentication:
host all postgres 0.0.0.0/0 md5
```

3. **Reset password if needed**:
```sql
-- Connect as superuser and reset password
ALTER USER postgres PASSWORD 'new_password';
```

## Replication Issues

### Replication Not Working

**Symptoms:**
- Slaves not receiving updates from master
- Replication lag warnings in logs

**Solutions:**

1. **Check replication status on master**:
```sql
-- View active replication connections
SELECT * FROM pg_stat_replication;

-- Check WAL sender processes
SELECT pid, state, sent_lsn, write_lsn, flush_lsn, replay_lsn 
FROM pg_stat_replication;
```

2. **Check replication status on slave**:
```sql
-- Verify slave is in recovery mode
SELECT pg_is_in_recovery();

-- Check WAL receiver status
SELECT * FROM pg_stat_wal_receiver;
```

3. **Verify replication user**:
```sql
-- Check replication user exists and has proper permissions
SELECT usename, usesuper, userepl FROM pg_user WHERE usename='replicator';
```

### High Replication Lag

**Symptoms:**
- Significant delay between master and slave updates
- Lag warnings in monitoring

**Solutions:**

1. **Check network performance**:
```bash
# Test network latency
ping SLAVE_HOST

# Test bandwidth
iperf3 -c SLAVE_HOST
```

2. **Monitor system resources**:
```bash
# Check CPU and memory usage
top
htop

# Check disk I/O
iostat -x 1
```

3. **Tune PostgreSQL settings**:
```conf
# In postgresql.conf on master
wal_buffers = 16MB
max_wal_size = 2GB

# On slave
max_standby_streaming_delay = 30s
```

## Failover Issues

### Failover Not Triggering

**Symptoms:**
- Master failure detected but no failover occurs
- Slaves not being promoted

**Solutions:**

1. **Check health check configuration**:
```yaml
health_check:
  interval: "30s"
  timeout: "5s"
  failure_threshold: 3  # Ensure this is reasonable
```

2. **Verify cluster consensus**:
```bash
# Check logs for consensus messages
grep -i "consensus\|failover" /var/log/pg-metako/app.log
```

3. **Check minimum consensus nodes**:
```yaml
coordination:
  min_consensus_nodes: 2  # Ensure enough nodes are available
```

### Failed Promotion

**Symptoms:**
- Promotion attempts fail
- Slaves remain in read-only mode

**Solutions:**

1. **Check slave readiness**:
```sql
-- Ensure slave is caught up
SELECT pg_last_wal_receive_lsn(), pg_last_wal_replay_lsn();
```

2. **Verify promotion permissions**:
```bash
# Check if pg-metako has necessary system permissions
sudo -u postgres pg_promote
```

3. **Check for conflicts**:
```bash
# Look for lock files or other conflicts
ls -la /var/lib/postgresql/*/main/
```

## Docker Issues

### Services Won't Start

**Symptoms:**
- Docker containers exit immediately
- Services fail health checks

**Solutions:**

1. **Check Docker daemon**:
```bash
# Verify Docker is running
docker info

# Check Docker service status
sudo systemctl status docker
```

2. **Examine container logs**:
```bash
# Check logs for specific service
docker-compose logs pg-metako
docker-compose logs postgres-master

# Follow logs in real-time
docker-compose logs -f
```

3. **Verify resource availability**:
```bash
# Check available disk space
df -h

# Check memory usage
free -h

# Check if ports are available
netstat -tlnp | grep :5432
```

### Database Connection Issues in Docker

**Symptoms:**
- pg-metako can't connect to PostgreSQL containers
- Network connectivity issues between containers

**Solutions:**

1. **Check Docker network**:
```bash
# List Docker networks
docker network ls

# Inspect network configuration
docker network inspect pg-metako_pg-metako-network
```

2. **Verify service names**:
```bash
# Check if services can resolve each other
docker-compose exec pg-metako nslookup postgres-master
```

3. **Check environment variables**:
```bash
# Verify environment variables are set correctly
docker-compose exec pg-metako env | grep POSTGRES
```

### Volume Mount Issues

**Symptoms:**
- Data not persisting between container restarts
- Permission denied errors

**Solutions:**

1. **Check volume mounts**:
```bash
# List Docker volumes
docker volume ls

# Inspect volume details
docker volume inspect postgres_master_data
```

2. **Fix permissions**:
```bash
# Fix ownership if needed
sudo chown -R 999:999 /var/lib/docker/volumes/postgres_master_data/_data
```

## Performance Issues

### High CPU Usage

**Symptoms:**
- pg-metako consuming excessive CPU
- System becomes unresponsive

**Solutions:**

1. **Profile the application**:
```bash
# Use pprof for Go applications
go tool pprof http://localhost:8080/debug/pprof/profile
```

2. **Check query patterns**:
```sql
-- Identify slow queries
SELECT query, mean_time, calls 
FROM pg_stat_statements 
ORDER BY mean_time DESC 
LIMIT 10;
```

3. **Tune configuration**:
```yaml
health_check:
  interval: "60s"  # Increase interval to reduce load
```

### High Memory Usage

**Symptoms:**
- Memory usage continuously growing
- Out of memory errors

**Solutions:**

1. **Monitor memory usage**:
```bash
# Check memory usage patterns
ps aux | grep pg-metako
top -p $(pgrep pg-metako)
```

2. **Check for memory leaks**:
```bash
# Use memory profiling
go tool pprof http://localhost:8080/debug/pprof/heap
```

3. **Tune PostgreSQL connections**:
```yaml
# Limit connection pool size
load_balancer:
  max_connections: 50
```

## Network Issues

### Inter-node Communication Failures

**Symptoms:**
- Nodes can't communicate with each other
- Coordination API errors

**Solutions:**

1. **Check network connectivity**:
```bash
# Test API connectivity between nodes
curl -v http://NODE_IP:8080/health

# Check firewall rules
sudo ufw status
```

2. **Verify API configuration**:
```yaml
identity:
  api_host: "0.0.0.0"  # Ensure binding to all interfaces
  api_port: 8080
```

3. **Check DNS resolution**:
```bash
# Verify hostname resolution
nslookup NODE_HOSTNAME
dig NODE_HOSTNAME
```

## Monitoring and Debugging

### Enable Debug Logging

```yaml
# In configuration file
logging:
  level: "debug"
  format: "json"
```

```bash
# Set environment variable
export LOG_LEVEL=debug
./bin/pg-metako --config configs/config.yaml
```

### Useful Monitoring Commands

```bash
# Monitor system resources
htop
iotop
nethogs

# Monitor PostgreSQL
psql -c "SELECT * FROM pg_stat_activity;"
psql -c "SELECT * FROM pg_stat_replication;"

# Monitor network connections
netstat -tlnp | grep pg-metako
ss -tlnp | grep pg-metako
```

### Log Analysis

```bash
# Search for errors
grep -i error /var/log/pg-metako/app.log

# Monitor real-time logs
tail -f /var/log/pg-metako/app.log | grep -i "error\|warn\|fail"

# Analyze connection patterns
grep "connection" /var/log/pg-metako/app.log | tail -20
```

## Getting Help

### Collecting Debug Information

When reporting issues, collect the following information:

1. **System Information**:
```bash
uname -a
cat /etc/os-release
```

2. **pg-metako Version**:
```bash
./bin/pg-metako --version
```

3. **Configuration** (sanitized):
```bash
# Remove passwords before sharing
cat configs/config.yaml | sed 's/password:.*/password: [REDACTED]/'
```

4. **Logs**:
```bash
# Recent logs with timestamps
tail -100 /var/log/pg-metako/app.log
```

5. **PostgreSQL Status**:
```sql
SELECT version();
SELECT * FROM pg_stat_replication;
```

### Common Log Messages

**Normal Operations**:
- "Configuration loaded successfully"
- "Health monitoring started"
- "Coordination API started"

**Warning Signs**:
- "Failed to connect to database"
- "Health check failed"
- "Replication lag detected"

**Critical Issues**:
- "Failover initiated"
- "Master promotion failed"
- "Cluster consensus lost"

For additional help, check the project documentation or create an issue with the collected debug information.