-- Migration 013: Fix topic keyword rules
-- 1. drug_crime: replace generic "trafficking" with drug-specific compound terms
-- 2. travel: remove ambiguous soft keywords (destination, trip, visa, passport)
-- See: docs/superpowers/specs/2026-03-16-classifier-topic-content-type-fixes-design.md

BEGIN;

-- Fix drug_crime: remove generic "trafficking", add drug-specific compound terms
UPDATE classification_rules
SET keywords = ARRAY[
    'drug', 'drugs', 'narcotics', 'dealer', 'possession',
    'cocaine', 'heroin', 'fentanyl', 'methamphetamine', 'meth', 'marijuana', 'cannabis', 'opioid',
    'drug bust', 'drug ring', 'cartel', 'smuggling', 'drug trafficking',
    'narcotics trafficking', 'fentanyl trafficking', 'cocaine trafficking', 'meth trafficking',
    'overdose', 'drug-related', 'controlled substance'
],
    updated_at = CURRENT_TIMESTAMP
WHERE rule_name = 'drug_crime_detection';

-- Fix travel: remove ambiguous soft keywords
UPDATE classification_rules
SET keywords = ARRAY[
    'vacation', 'hotel', 'flight', 'tourism', 'travel',
    'journey', 'tour', 'tourist',
    'resort', 'airline', 'airport', 'luggage',
    'cruise', 'beach', 'sightseeing', 'adventure', 'backpacking',
    'travel guide', 'itinerary', 'booking', 'reservation'
],
    updated_at = CURRENT_TIMESTAMP
WHERE rule_name = 'travel_detection';

COMMIT;
