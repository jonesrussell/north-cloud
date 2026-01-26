-- Migration: Create index_metadata and migration_history tables
-- Description: Tables for tracking Elasticsearch index metadata and migration history
-- Version: 001
-- Date: 2025-12-23

-- Create index_metadata table
CREATE TABLE IF NOT EXISTS index_metadata (
    id SERIAL PRIMARY KEY,
    index_name VARCHAR(255) UNIQUE NOT NULL,
    index_type VARCHAR(50) NOT NULL,  -- raw_content, classified_content, article, page
    source_name VARCHAR(255),
    mapping_version VARCHAR(50) DEFAULT '1.0.0',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    status VARCHAR(50) DEFAULT 'active'  -- active, archived, deleted
);

-- Create indexes for index_metadata
CREATE INDEX IF NOT EXISTS idx_index_metadata_source ON index_metadata(source_name);
CREATE INDEX IF NOT EXISTS idx_index_metadata_type ON index_metadata(index_type);
CREATE INDEX IF NOT EXISTS idx_index_metadata_status ON index_metadata(status);

-- Create migration_history table
CREATE TABLE IF NOT EXISTS migration_history (
    id SERIAL PRIMARY KEY,
    index_name VARCHAR(255) NOT NULL,
    from_version VARCHAR(50),
    to_version VARCHAR(50),
    migration_type VARCHAR(50) NOT NULL,  -- create, update, delete
    status VARCHAR(50) NOT NULL,  -- pending, completed, failed
    error_message TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    completed_at TIMESTAMP
);

-- Create indexes for migration_history
CREATE INDEX IF NOT EXISTS idx_migration_history_index_name ON migration_history(index_name);
CREATE INDEX IF NOT EXISTS idx_migration_history_status ON migration_history(status);
CREATE INDEX IF NOT EXISTS idx_migration_history_created_at ON migration_history(created_at DESC);
