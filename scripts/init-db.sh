#!/bin/bash
# QuotaLane Database Initialization Script for Docker
# This script waits for MySQL to be ready, then runs migrations and seeds data
# Usage: ./init-db.sh (called by docker-compose app service)

set -e

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Configuration from environment variables
DB_HOST="${DB_HOST:-mysql}"
DB_PORT="${DB_PORT:-3306}"
DB_USER="${DB_USER:-root}"
DB_PASSWORD="${MYSQL_ROOT_PASSWORD:-root}"
DB_NAME="${DB_NAME:-quotalane}"

echo "=========================================="
echo "QuotaLane Database Initialization"
echo "=========================================="
echo "Database Host: $DB_HOST:$DB_PORT"
echo "Database Name: $DB_NAME"
echo "=========================================="

# Wait for MySQL to be ready
echo "Waiting for MySQL to be ready..."
MAX_RETRIES=60
RETRY_COUNT=0

until nc -z -w1 "$DB_HOST" "$DB_PORT" 2>/dev/null; do
    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [ $RETRY_COUNT -ge $MAX_RETRIES ]; then
        echo "Error: MySQL did not become ready in time (waited ${MAX_RETRIES}s)"
        exit 1
    fi
    echo "MySQL is unavailable - sleeping (attempt $RETRY_COUNT/$MAX_RETRIES)"
    sleep 1
done

echo "✅ MySQL is ready!"

# Check if database is already initialized
echo ""
echo "Checking if database is already initialized..."
TABLE_COUNT=$(mysql -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" -p"$DB_PASSWORD" -D"$DB_NAME" -se "SHOW TABLES;" 2>/dev/null | wc -l | tr -d ' ')

if [ "$TABLE_COUNT" -gt 0 ]; then
    echo "ℹ️  Database already initialized ($TABLE_COUNT tables found). Skipping migrations."
else
    echo "📦 Database is empty. Running migrations..."
    # Pass DB_PASSWORD environment variable to migrate.sh
    if DB_PASSWORD="$DB_PASSWORD" bash "$SCRIPT_DIR/migrate.sh" up; then
        echo "✅ Migrations completed successfully"
    else
        echo "❌ Error: Migration failed"
        exit 1
    fi
fi

# Seed initial data (idempotent - will skip if data already exists)
echo ""
echo "Seeding initial data..."
if bash "$SCRIPT_DIR/seed.sh"; then
    echo "✅ Seeding completed successfully"
else
    echo "⚠️  Warning: Seeding failed (may already exist)"
    # Don't fail on seeding errors (data might already exist)
fi

echo ""
echo "=========================================="
echo "✅ Database initialization completed!"
echo "=========================================="
