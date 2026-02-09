-- Migration 010: Remove mining topic classification rule (rollback)

BEGIN;

DELETE FROM classification_rules WHERE rule_name = 'mining_detection';

COMMIT;
