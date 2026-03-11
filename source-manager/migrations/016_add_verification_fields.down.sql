ALTER TABLE people DROP COLUMN IF EXISTS verification_confidence;
ALTER TABLE people DROP COLUMN IF EXISTS verification_issues;

ALTER TABLE band_offices DROP COLUMN IF EXISTS verification_confidence;
ALTER TABLE band_offices DROP COLUMN IF EXISTS verification_issues;
