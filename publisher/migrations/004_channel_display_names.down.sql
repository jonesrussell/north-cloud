-- Revert display names to original seeded values

UPDATE channels
SET name = 'StreetCode Crime Feed',
    description = 'Aggregated crime content for StreetCode'
WHERE slug = 'streetcode_crime_feed';
