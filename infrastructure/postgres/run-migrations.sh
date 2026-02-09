#!/bin/bash
# Runs all .up.sql migration files from /migrations on the current database.
# Mount this into /docker-entrypoint-initdb.d/ for automatic schema init.
# Only runs on first container start (when data directory is empty).

set -e

if [ ! -d /migrations ]; then
    echo "No /migrations directory found, skipping."
    exit 0
fi

for f in $(ls /migrations/*.up.sql 2>/dev/null | sort); do
    echo "Running migration: $(basename "$f")"
    psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -f "$f"
done

echo "All migrations applied."
