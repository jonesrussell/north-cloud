-- Migration: 006_fix_crime_feed_rules (rollback)
-- WARNING: Rolling back this migration will cause ALL articles to be published
-- to the articles:crime channel, not just crime-related content

UPDATE channels
SET rules = jsonb_set(
    rules,
    '{include_topics}',
    '[]'
)
WHERE slug = 'crime_feed';
