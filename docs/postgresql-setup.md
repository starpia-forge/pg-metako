# PostgreSQL Setup Guide

This guide covers PostgreSQL configuration for use with pg-metako, including master-slave replication setup.

## Prerequisites

- PostgreSQL 12 or later
- Network connectivity between master and slave nodes
- Sufficient disk space for WAL files and replication

## Master Configuration

### 1. Edit postgresql.conf

Add or modify the following settings in `postgresql.conf`:

```conf
# Replication settings
wal_level = replica
max_wal_senders = 3
wal_keep_segments = 64

# Connection settings
listen_addresses = '*'
port = 5432

# Memory settings (adjust based on your system)
shared_buffers = 256MB
effective_cache_size = 1GB

# Logging (optional but recommended)
log_statement = 'all'
log_destination = 'stderr'
logging_collector = on
log_directory = 'pg_log'
log_filename = 'postgresql-%Y-%m-%d_%H%M%S.log'
```

### 2. Edit pg_hba.conf

Add replication access for slave nodes in `pg_hba.conf`:

```conf
# Allow replication connections
host replication replicator 0.0.0.0/0 md5

# Allow application connections
host all postgres 0.0.0.0/0 md5
host all myapp 0.0.0.0/0 md5
```

**Security Note**: In production, replace `0.0.0.0/0` with specific IP ranges for better security.

### 3. Create Replication User

Connect to PostgreSQL and create a replication user:

```sql
-- Create replication user
CREATE USER replicator WITH REPLICATION ENCRYPTED PASSWORD 'replicator_password';

-- Grant necessary permissions
GRANT CONNECT ON DATABASE postgres TO replicator;
```

### 4. Restart PostgreSQL

Restart PostgreSQL to apply configuration changes:

```bash
# On systemd systems
sudo systemctl restart postgresql

# On other systems
sudo service postgresql restart
```

## Slave Configuration

### 1. Stop PostgreSQL on Slave

```bash
sudo systemctl stop postgresql
```

### 2. Remove Existing Data Directory

**Warning**: This will delete all existing data on the slave.

```bash
sudo rm -rf /var/lib/postgresql/12/main/*
```

### 3. Create Base Backup

Create a base backup from the master:

```bash
# Run as postgres user
sudo -u postgres pg_basebackup -h MASTER_IP -D /var/lib/postgresql/12/main -U replicator -P -v -R -W
```

Replace `MASTER_IP` with the actual IP address of your master server.

### 4. Configure Slave-Specific Settings

Edit `postgresql.conf` on the slave:

```conf
# Basic settings
listen_addresses = '*'
port = 5432

# Slave-specific settings
hot_standby = on
max_standby_streaming_delay = 30s
wal_receiver_status_interval = 10s
hot_standby_feedback = on
```

### 5. Configure Recovery Settings

The `-R` flag in pg_basebackup should have created a `standby.signal` file and configured `postgresql.auto.conf`. Verify the recovery settings:

```conf
# In postgresql.auto.conf (created by pg_basebackup -R)
primary_conninfo = 'host=MASTER_IP port=5432 user=replicator password=replicator_password'
```

### 6. Start PostgreSQL on Slave

```bash
sudo systemctl start postgresql
```

## Verification

### Check Master Status

On the master, verify replication is working:

```sql
-- Check replication status
SELECT * FROM pg_stat_replication;

-- Check WAL sender processes
SELECT pid, state, sent_lsn, write_lsn, flush_lsn, replay_lsn 
FROM pg_stat_replication;
```

### Check Slave Status

On the slave, verify it's receiving data:

```sql
-- Check if in recovery mode (should return true)
SELECT pg_is_in_recovery();

-- Check replication status
SELECT status, receive_start_lsn, received_lsn, last_msg_send_time, last_msg_receipt_time 
FROM pg_stat_wal_receiver;
```

### Test Replication

1. Create a test table on the master:
```sql
CREATE TABLE replication_test (id SERIAL PRIMARY KEY, data TEXT);
INSERT INTO replication_test (data) VALUES ('test data');
```

2. Check if the table appears on the slave:
```sql
SELECT * FROM replication_test;
```

## Multiple Slaves Setup

To set up multiple slaves, repeat the slave configuration process for each additional slave node.

### Considerations for Multiple Slaves

1. **WAL Senders**: Increase `max_wal_senders` on the master:
```conf
max_wal_senders = 5  # Adjust based on number of slaves
```

2. **Network Bandwidth**: Ensure sufficient network bandwidth for multiple replication streams.

3. **Monitoring**: Monitor replication lag across all slaves:
```sql
SELECT client_addr, state, sent_lsn, write_lsn, flush_lsn, replay_lsn,
       pg_wal_lsn_diff(sent_lsn, replay_lsn) AS lag_bytes
FROM pg_stat_replication;
```

## Performance Tuning

### Master Tuning

```conf
# WAL settings
wal_buffers = 16MB
checkpoint_completion_target = 0.9
max_wal_size = 2GB
min_wal_size = 1GB

# Connection settings
max_connections = 200
```

### Slave Tuning

```conf
# Read-only workload optimization
effective_cache_size = 2GB
random_page_cost = 1.1
seq_page_cost = 1.0

# Standby settings
max_standby_streaming_delay = 30s
max_standby_archive_delay = 30s
```

## Security Best Practices

### 1. Network Security

- Use SSL/TLS for replication connections
- Restrict access using specific IP ranges in pg_hba.conf
- Use VPN or private networks for replication traffic

### 2. SSL Configuration

Enable SSL in `postgresql.conf`:

```conf
ssl = on
ssl_cert_file = 'server.crt'
ssl_key_file = 'server.key'
ssl_ca_file = 'ca.crt'
```

Update replication connection string:

```conf
primary_conninfo = 'host=MASTER_IP port=5432 user=replicator password=replicator_password sslmode=require'
```

### 3. Authentication

Use strong passwords and consider certificate-based authentication:

```conf
# In pg_hba.conf
hostssl replication replicator 192.168.1.0/24 cert
```

## Monitoring and Maintenance

### Regular Monitoring

1. **Replication Lag**: Monitor lag between master and slaves
2. **WAL Files**: Monitor WAL file accumulation
3. **Connection Status**: Check replication connections
4. **Disk Space**: Monitor disk usage on all nodes

### Maintenance Tasks

1. **WAL Cleanup**: Ensure old WAL files are cleaned up
2. **Statistics Update**: Run ANALYZE on slaves periodically
3. **Connection Monitoring**: Monitor replication connections
4. **Backup Verification**: Test backup and recovery procedures

### Useful Monitoring Queries

```sql
-- Replication lag in seconds
SELECT EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp())) AS lag_seconds;

-- WAL file count
SELECT count(*) FROM pg_ls_waldir();

-- Active connections
SELECT count(*) FROM pg_stat_activity WHERE state = 'active';
```

## Troubleshooting Common Issues

### Replication Not Starting

1. Check network connectivity between master and slave
2. Verify replication user credentials
3. Check pg_hba.conf configuration
4. Review PostgreSQL logs for error messages

### High Replication Lag

1. Check network bandwidth and latency
2. Monitor master server load
3. Verify WAL settings on master
4. Check slave server performance

### Connection Issues

1. Verify firewall settings
2. Check PostgreSQL service status
3. Review connection limits (max_connections)
4. Validate SSL configuration if used

For more troubleshooting information, see the [Troubleshooting Guide](troubleshooting.md).