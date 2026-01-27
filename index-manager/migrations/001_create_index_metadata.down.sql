-- Rollback: Drop index_metadata and migration_history tables
-- WARNING: This will permanently delete all index metadata and migration history

-- Drop indexes for migration_history
DROP INDEX IF EXISTS idx_migration_history_created_at;
DROP INDEX IF EXISTS idx_migration_history_status;
DROP INDEX IF EXISTS idx_migration_history_index_name;

-- Drop migration_history table
DROP TABLE IF EXISTS migration_history;

-- Drop indexes for index_metadata
DROP INDEX IF EXISTS idx_index_metadata_status;
DROP INDEX IF EXISTS idx_index_metadata_type;
DROP INDEX IF EXISTS idx_index_metadata_source;

-- Drop index_metadata table
DROP TABLE IF EXISTS index_metadata;
