DROP INDEX IF EXISTS idx_sources_allow_source_discovery;
DROP INDEX IF EXISTS idx_sources_identity_key;

ALTER TABLE sources DROP COLUMN IF EXISTS template_hint;
ALTER TABLE sources DROP COLUMN IF EXISTS extraction_profile;
ALTER TABLE sources DROP COLUMN IF EXISTS identity_key;
ALTER TABLE sources DROP COLUMN IF EXISTS allow_source_discovery;
