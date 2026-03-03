-- ============================================
-- Dev Environment: Multi-Database Init Script
-- ============================================
-- Creates all service databases in a single Postgres instance.
-- Runs once when the data directory is empty (docker-entrypoint-initdb.d).
-- Each service's migrations handle schema setup after the database exists.
-- ============================================

CREATE DATABASE source_manager;
CREATE DATABASE crawler;
CREATE DATABASE classifier;
CREATE DATABASE index_manager;
CREATE DATABASE publisher;
CREATE DATABASE pipeline;
CREATE DATABASE click_tracker;
