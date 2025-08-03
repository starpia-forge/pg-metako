#!/bin/bash
# Purpose    : Initialize PostgreSQL master for replication
# Context    : Docker container initialization script
# Constraints: Must create replication user and configure master for streaming replication

set -e

# Wait for PostgreSQL to be ready
until pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB"; do
  echo "Waiting for PostgreSQL to be ready..."
  sleep 2
done

echo "Initializing PostgreSQL master for replication..."

# Create replication user if it doesn't exist
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    DO \$\$
    BEGIN
        IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = '$POSTGRES_REPLICATION_USER') THEN
            CREATE ROLE $POSTGRES_REPLICATION_USER WITH REPLICATION PASSWORD '$POSTGRES_REPLICATION_PASSWORD' LOGIN;
            GRANT CONNECT ON DATABASE $POSTGRES_DB TO $POSTGRES_REPLICATION_USER;
        END IF;
    END
    \$\$;
EOSQL

# Create replication slot for each slave
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    SELECT pg_create_physical_replication_slot('slave1_slot') WHERE NOT EXISTS (
        SELECT 1 FROM pg_replication_slots WHERE slot_name = 'slave1_slot'
    );
    SELECT pg_create_physical_replication_slot('slave2_slot') WHERE NOT EXISTS (
        SELECT 1 FROM pg_replication_slots WHERE slot_name = 'slave2_slot'
    );
EOSQL

# Create a test table for demonstration
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE TABLE IF NOT EXISTS test_replication (
        id SERIAL PRIMARY KEY,
        message TEXT,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    
    INSERT INTO test_replication (message) VALUES 
        ('Master initialized at ' || CURRENT_TIMESTAMP),
        ('Replication setup complete');
EOSQL

echo "PostgreSQL master initialization completed successfully!"
echo "Replication user: $POSTGRES_REPLICATION_USER"
echo "Replication slots created: slave1_slot, slave2_slot"