#!/bin/sh
set -e

echo "Waiting for database to be ready..."
until PGPASSWORD="${POSTGRES_PASSWORD}" psql -h "${POSTGRES_HOST:-postgres-classifier}" -p "${POSTGRES_PORT:-5432}" -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-classifier}" -c "SELECT 1" > /dev/null 2>&1; do
  echo "Database is unavailable - sleeping"
  sleep 1
done

echo "Database is ready. Running migrations..."

# Run migrations in order (they use IF NOT EXISTS so they're mostly idempotent)
# Note: Some INSERT statements may fail if data already exists - that's OK
for migration in migrations/001_create_rules.sql \
                migrations/002_create_source_reputation.sql \
                migrations/003_create_classification_history.sql \
                migrations/004_create_ml_models.sql \
                migrations/005_add_comprehensive_categories.sql \
                migrations/006_remove_is_crime_related.sql; do
  if [ -f "$migration" ]; then
    echo "Running migration: $migration"
    # Run migration and ignore errors (migrations are mostly idempotent)
    PGPASSWORD="${POSTGRES_PASSWORD}" psql -h "${POSTGRES_HOST:-postgres-classifier}" \
      -p "${POSTGRES_PORT:-5432}" \
      -U "${POSTGRES_USER:-postgres}" \
      -d "${POSTGRES_DB:-classifier}" \
      -f "$migration" 2>&1 | grep -v -E "(already exists|duplicate key)" || true
  fi
done

echo "Migrations complete. Starting classifier service..."
exec ./classifier "$@"

