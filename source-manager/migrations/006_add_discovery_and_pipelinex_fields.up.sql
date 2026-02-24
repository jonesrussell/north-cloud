-- Add automatic source discovery and PipelineX-related fields.
-- allow_source_discovery: when true, outlinks from this source may feed the Source Candidate Pipeline (only if global discovery is enabled).
-- identity_key: logical source identity (e.g. host + path or platform:tenant); used by Source Identity Resolver; not equal to hostname.
-- extraction_profile: optional JSON for PipelineX domain-aware extraction (selectors, template hint).
-- template_hint: optional string for PipelineX template inference (e.g. "substack", "wordpress").
ALTER TABLE sources ADD COLUMN IF NOT EXISTS allow_source_discovery BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE sources ADD COLUMN IF NOT EXISTS identity_key VARCHAR(512);
ALTER TABLE sources ADD COLUMN IF NOT EXISTS extraction_profile JSONB;
ALTER TABLE sources ADD COLUMN IF NOT EXISTS template_hint VARCHAR(255);

CREATE INDEX IF NOT EXISTS idx_sources_identity_key ON sources(identity_key) WHERE identity_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_sources_allow_source_discovery ON sources(allow_source_discovery) WHERE allow_source_discovery = true;
