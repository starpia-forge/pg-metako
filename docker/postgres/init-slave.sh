#!/bin/bash
# Purpose    : Initialize PostgreSQL slave for replication
# Context    : Docker container initialization script for standby server
# Constraints: Must configure standby mode and connect to master for replication

set -e

echo "Initializing PostgreSQL slave for replication..."

# Wait for master to be ready
echo "Waiting for master database to be ready..."
until pg_isready -h "$POSTGRES_MASTER_HOST" -p "$POSTGRES_MASTER_PORT" -U "$POSTGRES_USER"; do
  echo "Master not ready, waiting..."
  sleep 5
done

echo "Master database is ready, setting up slave..."

# Stop PostgreSQL if it's running
pg_ctl stop -D "$PGDATA" -m fast || true

# Remove existing data directory contents
rm -rf "$PGDATA"/*

# Create base backup from master
echo "Creating base backup from master..."
PGPASSWORD="$POSTGRES_REPLICATION_PASSWORD" pg_basebackup \
    -h "$POSTGRES_MASTER_HOST" \
    -p "$POSTGRES_MASTER_PORT" \
    -U "$POSTGRES_REPLICATION_USER" \
    -D "$PGDATA" \
    -Fp -Xs -P -R

# Create standby.signal file to indicate this is a standby server
touch "$PGDATA/standby.signal"

# Determine slot name based on container name
SLOT_NAME="slave1_slot"
if [[ "$HOSTNAME" == *"slave2"* ]]; then
    SLOT_NAME="slave2_slot"
fi

# Configure primary connection info
cat >> "$PGDATA/postgresql.auto.conf" <<EOF
# Standby configuration
primary_conninfo = 'host=$POSTGRES_MASTER_HOST port=$POSTGRES_MASTER_PORT user=$POSTGRES_REPLICATION_USER password=$POSTGRES_REPLICATION_PASSWORD application_name=$HOSTNAME'
primary_slot_name = '$SLOT_NAME'
promote_trigger_file = '/tmp/promote_trigger'
EOF

# Set proper permissions
chown -R postgres:postgres "$PGDATA"
chmod 700 "$PGDATA"

echo "PostgreSQL slave initialization completed successfully!"
echo "Connected to master: $POSTGRES_MASTER_HOST:$POSTGRES_MASTER_PORT"
echo "Using replication slot: $SLOT_NAME"
echo "Standby server is ready to start..."