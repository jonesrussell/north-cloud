-- Drop trigger
DROP TRIGGER IF EXISTS update_rules_updated_at ON classification_rules;

-- Drop function (only if not used by other tables)
-- Note: This function may be used by other tables, so we check first
-- DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop indexes
DROP INDEX IF EXISTS idx_rules_priority;
DROP INDEX IF EXISTS idx_rules_enabled;
DROP INDEX IF EXISTS idx_rules_topic;
DROP INDEX IF EXISTS idx_rules_type;

-- Drop table
DROP TABLE IF EXISTS classification_rules;

