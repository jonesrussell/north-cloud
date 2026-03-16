-- Rollback migration 013: restore original keyword arrays

BEGIN;

-- Restore original drug_crime keywords (from migration 007)
UPDATE classification_rules
SET keywords = ARRAY[
    'drug', 'drugs', 'narcotics', 'trafficking', 'dealer', 'possession',
    'cocaine', 'heroin', 'fentanyl', 'methamphetamine', 'meth', 'marijuana', 'cannabis', 'opioid',
    'drug bust', 'drug ring', 'cartel', 'smuggling', 'drug trafficking',
    'overdose', 'drug-related', 'controlled substance'
],
    updated_at = CURRENT_TIMESTAMP
WHERE rule_name = 'drug_crime_detection';

-- Restore original travel keywords (from migration 005)
UPDATE classification_rules
SET keywords = ARRAY[
    'trip', 'vacation', 'hotel', 'flight', 'destination', 'tourism', 'travel',
    'travel', 'trip', 'vacation', 'journey', 'tour', 'tourist', 'destination',
    'hotel', 'resort', 'flight', 'airline', 'airport', 'luggage', 'passport',
    'visa', 'cruise', 'beach', 'sightseeing', 'adventure', 'backpacking',
    'tourism', 'travel guide', 'itinerary', 'booking', 'reservation'
],
    updated_at = CURRENT_TIMESTAMP
WHERE rule_name = 'travel_detection';

COMMIT;
