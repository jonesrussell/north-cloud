-- Migration 014: Remove indigenous topic classification rule (rollback)

BEGIN;

DELETE FROM classification_rules WHERE rule_name = 'indigenous_detection';

COMMIT;
