-- Migration: 004_channel_display_names
-- Description: Rename seeded channel to content-centric display (consumer-agnostic)
-- Leaves slug and redis_channel unchanged so existing subscribers are not broken.

UPDATE channels
SET name = 'Crime Feed',
    description = 'Aggregated crime content'
WHERE slug = 'streetcode_crime_feed';
