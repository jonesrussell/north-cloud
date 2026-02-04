-- Migration: 005_channel_slug_generic
-- Description: Rename streetcode-specific slug to generic crime_feed
-- The publisher is consumer-agnostic; channel slugs should not reference consumers.

UPDATE channels
SET slug = 'crime_feed'
WHERE slug = 'streetcode_crime_feed';
