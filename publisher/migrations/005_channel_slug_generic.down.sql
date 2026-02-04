-- Migration: 005_channel_slug_generic (rollback)
-- Restore original streetcode-specific slug

UPDATE channels
SET slug = 'streetcode_crime_feed'
WHERE slug = 'crime_feed';
