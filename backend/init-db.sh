#!/bin/bash
set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    -- Create application database
    CREATE DATABASE audit;
    
    -- Grant permissions
    GRANT ALL PRIVILEGES ON DATABASE audit TO $POSTGRES_USER;
EOSQL

echo "Database audit created successfully"
