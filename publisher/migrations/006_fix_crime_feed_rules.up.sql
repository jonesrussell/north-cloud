-- Migration: 006_fix_crime_feed_rules
-- Description: Fix Crime Feed channel to only include crime-related topics
-- Root cause: include_topics was manually cleared to [], causing ALL articles to match
-- and be published to articles:crime channel (should only be crime-related content)

UPDATE channels
SET rules = jsonb_set(
    rules,
    '{include_topics}',
    '["violent_crime", "property_crime", "drug_crime", "organized_crime", "criminal_justice"]'
)
WHERE slug = 'crime_feed';
