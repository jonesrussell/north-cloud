-- ============================================
-- PostgreSQL Initialization Script
-- ============================================
-- This script runs when PostgreSQL container is first initialized
-- It's mounted in docker-compose.yml as /docker-entrypoint-initdb.d/init.sql
-- 
-- Note: This runs only once when the data directory is empty
-- ============================================

-- Create extensions that might be useful across databases
-- (Each service will create its own database, but we can set up common extensions here)

-- Enable useful extensions
-- Note: These need to be created per-database, so each service should
-- create them in their own migration files. This file is mainly for
-- demonstration and can be used for shared initialization logic.

-- Example: Log initialization
DO $$
BEGIN
    RAISE NOTICE 'PostgreSQL initialization script executed';
    RAISE NOTICE 'Current database: %', current_database();
END $$;

-- ============================================
-- Optional: Create a shared database for cross-service queries
-- (Only use if absolutely necessary - prefer service-specific databases)
-- ============================================

-- CREATE DATABASE north_cloud_shared;
-- \c north_cloud_shared;
-- CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================
-- Notes for Service-Specific Setup:
-- ============================================
-- 
-- Each service should handle its own database setup:
-- 
-- 1. source-manager: Creates 'source_manager' database
--    - Run migrations from ./source-manager/migrations/
-- 
-- 2. crawler: Creates 'crawler' database  
--    - Run migrations via crawler's migration system
-- 
-- 3. streetcode: Creates 'streetcode' database
--    - Drupal handles its own schema via install script
-- 
-- ============================================

