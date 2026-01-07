-- Migration 007 Rollback: Remove crime sub-category classification rules
-- Re-enables the original generic "crime" rule

BEGIN;

-- Delete all crime sub-category rules created in migration 007
DELETE FROM classification_rules
WHERE rule_name IN (
    'violent_crime_detection',
    'property_crime_detection',
    'drug_crime_detection',
    'organized_crime_detection',
    'criminal_justice_detection'
);

-- Re-enable the original generic "crime" rule
UPDATE classification_rules
SET enabled = true,
    updated_at = CURRENT_TIMESTAMP
WHERE rule_name = 'crime_detection'
  AND rule_type = 'topic';

COMMIT;
