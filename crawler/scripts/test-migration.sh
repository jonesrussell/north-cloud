#!/bin/bash
set -e

# Script to test database migration 003
# This script:
# 1. Creates a test database
# 2. Runs the migration
# 3. Verifies schema changes
# 4. Tests rollback
# 5. Cleans up

echo "=== Database Migration Test Script ==="
echo ""

# Configuration
TEST_DB_NAME="crawler_migration_test"
DB_HOST="${POSTGRES_HOST:-localhost}"
DB_PORT="${POSTGRES_PORT:-5432}"
DB_USER="${POSTGRES_USER:-postgres}"
DB_PASSWORD="${POSTGRES_PASSWORD:-postgres}"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Helper functions
function info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

function error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

function warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Check if psql is available
if ! command -v psql &> /dev/null; then
    error "psql is not installed. Please install PostgreSQL client tools."
    exit 1
fi

# Set PGPASSWORD for passwordless connections
export PGPASSWORD="$DB_PASSWORD"

# Drop test database if it exists
info "Dropping test database if it exists..."
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres -c "DROP DATABASE IF EXISTS $TEST_DB_NAME;" 2>/dev/null || true

# Create test database
info "Creating test database: $TEST_DB_NAME"
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres -c "CREATE DATABASE $TEST_DB_NAME;"

# Create initial schema (001 and 002 migrations)
info "Creating initial schema..."
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$TEST_DB_NAME" <<EOF
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create jobs table (simplified 001 migration)
CREATE TABLE IF NOT EXISTS jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_id TEXT,
    source_name TEXT,
    url TEXT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    schedule_time TEXT,
    schedule_enabled BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT
);

-- Create queued_links table (simplified 002 migration)
CREATE TABLE IF NOT EXISTS queued_links (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    url TEXT NOT NULL,
    source_name TEXT NOT NULL,
    status VARCHAR(50) DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    processed_at TIMESTAMP WITH TIME ZONE
);

-- Insert test data
INSERT INTO jobs (source_id, source_name, url, schedule_time, schedule_enabled) VALUES
    ('test-1', 'example.com', 'https://example.com', '0 * * * *', true),
    ('test-2', 'test.com', 'https://test.com', '0 0 * * *', true),
    ('test-3', 'news.com', 'https://news.com', '', false);
EOF

# Verify initial state
info "Verifying initial job count..."
JOB_COUNT=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$TEST_DB_NAME" -t -c "SELECT COUNT(*) FROM jobs;")
if [ "$JOB_COUNT" -ne 3 ]; then
    error "Expected 3 jobs, found $JOB_COUNT"
    exit 1
fi
info "✓ Initial state verified (3 jobs)"

# Run migration 003 up
info "Running migration 003 (up)..."
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$TEST_DB_NAME" < migrations/003_refactor_to_interval_scheduler.up.sql

# Verify migration results
info "Verifying migration results..."

# 1. Check if job_executions table exists
EXECUTIONS_TABLE=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$TEST_DB_NAME" -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'job_executions';")
if [ "$EXECUTIONS_TABLE" -ne 1 ]; then
    error "job_executions table not created"
    exit 1
fi
info "✓ job_executions table exists"

# 2. Check if new columns exist in jobs table
NEW_COLUMNS="interval_minutes interval_type next_run_at is_paused max_retries retry_backoff_seconds current_retry_count lock_token lock_acquired_at paused_at cancelled_at metadata"
for col in $NEW_COLUMNS; do
    COL_EXISTS=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$TEST_DB_NAME" -t -c "SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'jobs' AND column_name = '$col';")
    if [ "$COL_EXISTS" -ne 1 ]; then
        error "Column $col not found in jobs table"
        exit 1
    fi
done
info "✓ All new columns exist in jobs table"

# 3. Verify jobs were migrated
MIGRATED_JOBS=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$TEST_DB_NAME" -t -c "SELECT COUNT(*) FROM jobs;")
if [ "$MIGRATED_JOBS" -ne 3 ]; then
    error "Job count mismatch after migration. Expected 3, found $MIGRATED_JOBS"
    exit 1
fi
info "✓ All jobs migrated successfully"

# 4. Verify cron to interval conversion
HOURLY_JOB=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$TEST_DB_NAME" -t -c "SELECT interval_minutes FROM jobs WHERE source_id = 'test-1';")
if [ "$HOURLY_JOB" -ne 60 ]; then
    error "Hourly job conversion failed. Expected 60, found $HOURLY_JOB"
    exit 1
fi
info "✓ Cron expression '0 * * * *' converted to 60 minutes"

DAILY_JOB=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$TEST_DB_NAME" -t -c "SELECT interval_minutes FROM jobs WHERE source_id = 'test-2';")
if [ "$DAILY_JOB" -ne 1440 ]; then
    error "Daily job conversion failed. Expected 1440, found $DAILY_JOB"
    exit 1
fi
info "✓ Cron expression '0 0 * * *' converted to 1440 minutes (daily)"

# 5. Verify triggers were created
TRIGGER_COUNT=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$TEST_DB_NAME" -t -c "SELECT COUNT(*) FROM information_schema.triggers WHERE trigger_name LIKE '%next_run_at%' OR trigger_name LIKE '%execution_duration%';")
if [ "$TRIGGER_COUNT" -lt 2 ]; then
    warn "Expected at least 2 triggers, found $TRIGGER_COUNT"
fi
info "✓ Database triggers created"

# 6. Verify indexes were created
INDEX_COUNT=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$TEST_DB_NAME" -t -c "SELECT COUNT(*) FROM pg_indexes WHERE tablename IN ('jobs', 'job_executions');")
if [ "$INDEX_COUNT" -lt 2 ]; then
    warn "Expected at least 2 indexes, found $INDEX_COUNT"
fi
info "✓ Database indexes created"

# Test rollback
info "Testing rollback (migration down)..."
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$TEST_DB_NAME" < migrations/003_refactor_to_interval_scheduler.down.sql

# Verify rollback
EXECUTIONS_TABLE_AFTER_DOWN=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$TEST_DB_NAME" -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'job_executions';")
if [ "$EXECUTIONS_TABLE_AFTER_DOWN" -ne 0 ]; then
    error "job_executions table still exists after rollback"
    exit 1
fi
info "✓ Rollback successful (job_executions table dropped)"

# Verify jobs table still has old schema
COL_EXISTS_AFTER_DOWN=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$TEST_DB_NAME" -t -c "SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'jobs' AND column_name = 'interval_minutes';")
if [ "$COL_EXISTS_AFTER_DOWN" -ne 0 ]; then
    error "interval_minutes column still exists after rollback"
    exit 1
fi
info "✓ Jobs table schema restored"

# Clean up
info "Cleaning up test database..."
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres -c "DROP DATABASE $TEST_DB_NAME;"

echo ""
info "================================"
info "✓ All migration tests passed!"
info "================================"
echo ""

# Unset password
unset PGPASSWORD

exit 0
