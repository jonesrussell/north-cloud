-- Add metadata fields for structured (non-crawled) sources.
-- These fields are nullable because crawled sources don't need them.
ALTER TABLE sources ADD COLUMN data_format TEXT;
ALTER TABLE sources ADD COLUMN update_frequency TEXT;
ALTER TABLE sources ADD COLUMN license_type TEXT;
ALTER TABLE sources ADD COLUMN attribution_text TEXT;

COMMENT ON COLUMN sources.data_format IS 'Data format: json, csv, rss, html, api';
COMMENT ON COLUMN sources.update_frequency IS 'How often source updates: daily, weekly, monthly, realtime';
COMMENT ON COLUMN sources.license_type IS 'License: open, cc-by, cc-by-sa, restricted, unknown';
COMMENT ON COLUMN sources.attribution_text IS 'Required attribution text for hosted content';
