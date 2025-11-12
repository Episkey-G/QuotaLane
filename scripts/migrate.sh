#!/bin/bash
# QuotaLane Database Migration Script
# Usage: ./migrate.sh [up|down] [steps]

set -e

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MIGRATIONS_DIR="$PROJECT_ROOT/migrations"

# Load database configuration from config file
CONFIG_FILE="$PROJECT_ROOT/configs/config.yaml"

# Extract database configuration from YAML (simple parsing)
# Expected format in config.yaml:
#   data:
#     database:
#       driver: mysql
#       dsn: "user:password@tcp(localhost:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local"

# Default values
DB_USER="${DB_USER:-root}"
DB_PASSWORD="${DB_PASSWORD:-}"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-3306}"
DB_NAME="${DB_NAME:-quotalane}"

# Try to read from config file if it exists
if [ -f "$CONFIG_FILE" ]; then
    echo "Reading database configuration from $CONFIG_FILE..."
    # This is a simple grep-based extraction. In production, use yq or similar.
    DSN=$(grep -A 5 'database:' "$CONFIG_FILE" | grep 'dsn:' | sed 's/.*dsn: *"\(.*\)".*/\1/' || echo "")
    if [ -n "$DSN" ]; then
        # Use the DSN directly if found
        DB_CONNECTION="mysql://$DSN"
    fi
fi

# If DSN not found in config, construct from environment or defaults
if [ -z "$DB_CONNECTION" ]; then
    if [ -n "$DB_PASSWORD" ]; then
        DB_CONNECTION="mysql://$DB_USER:$DB_PASSWORD@tcp($DB_HOST:$DB_PORT)/$DB_NAME?charset=utf8mb4&parseTime=True&multiStatements=true"
    else
        DB_CONNECTION="mysql://$DB_USER@tcp($DB_HOST:$DB_PORT)/$DB_NAME?charset=utf8mb4&parseTime=True&multiStatements=true"
    fi
fi

# Check if migrate CLI is installed
if ! command -v migrate &> /dev/null; then
    echo "Error: golang-migrate CLI not found"
    echo "Please install it: go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest"
    exit 1
fi

# Migration command
COMMAND="${1:-up}"
STEPS="${2:-}"

echo "==================================="
echo "QuotaLane Database Migration"
echo "==================================="
echo "Migrations directory: $MIGRATIONS_DIR"
echo "Database: $DB_NAME"
echo "Command: $COMMAND ${STEPS}"
echo "==================================="

# Run migration
if [ -n "$STEPS" ]; then
    migrate -path "$MIGRATIONS_DIR" -database "$DB_CONNECTION" "$COMMAND" "$STEPS"
else
    migrate -path "$MIGRATIONS_DIR" -database "$DB_CONNECTION" "$COMMAND"
fi

echo "Migration completed successfully!"
