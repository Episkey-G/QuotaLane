#!/bin/bash
# QuotaLane Database Seed Script
# Usage: ./seed.sh

set -e

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SEED_FILE="$SCRIPT_DIR/seed_plans.sql"

# Load database configuration from config file
CONFIG_FILE="$PROJECT_ROOT/configs/config.yaml"

# Default values
DB_USER="${DB_USER:-root}"
DB_PASSWORD="${DB_PASSWORD:-root}"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-3306}"
DB_NAME="${DB_NAME:-quotalane}"

# Try to read from config file if it exists
if [ -f "$CONFIG_FILE" ]; then
    echo "Reading database configuration from $CONFIG_FILE..."
    # Extract database name from DSN
    DSN=$(grep -A 5 'database:' "$CONFIG_FILE" | grep 'source:' | sed 's/.*source: *"\?\([^"]*\)"\?.*/\1/' || echo "")
    if [ -n "$DSN" ]; then
        # Parse DSN to extract database name
        DB_NAME=$(echo "$DSN" | sed 's/.*\/\([^?]*\).*/\1/')
        # Extract credentials if present
        if [[ "$DSN" =~ ^([^:]+):([^@]+)@ ]]; then
            DB_USER="${BASH_REMATCH[1]}"
            DB_PASSWORD="${BASH_REMATCH[2]}"
        fi
    fi
fi

echo "==================================="
echo "QuotaLane Database Seeding"
echo "==================================="
echo "Seed file: $SEED_FILE"
echo "Database: $DB_NAME"
echo "Host: $DB_HOST:$DB_PORT"
echo "==================================="

# Check if seed file exists
if [ ! -f "$SEED_FILE" ]; then
    echo "Error: Seed file not found: $SEED_FILE"
    exit 1
fi

# Check if mysql client is installed
if ! command -v mysql &> /dev/null; then
    echo "Error: mysql client not found"
    echo "Please install MySQL client"
    exit 1
fi

# Execute seed SQL
echo "Seeding plans data..."
if [ -n "$DB_PASSWORD" ]; then
    mysql -u"$DB_USER" -p"$DB_PASSWORD" -h"$DB_HOST" -P"$DB_PORT" "$DB_NAME" < "$SEED_FILE"
else
    mysql -u"$DB_USER" -h"$DB_HOST" -P"$DB_PORT" "$DB_NAME" < "$SEED_FILE"
fi

if [ $? -eq 0 ]; then
    echo "==================================="
    echo "Seeding completed successfully!"
    echo "Inserted 5 default plans:"
    echo "  - Starter   ($9.99/month, $10 credit)"
    echo "  - Basic     ($29.99/month, $50 credit)"
    echo "  - Professional ($99.99/month, $200 credit)"
    echo "  - Flagship  ($299.99/month, $800 credit)"
    echo "  - Exclusive ($999.99/month, $5000 credit)"
    echo "==================================="
else
    echo "Error: Seeding failed"
    exit 1
fi
