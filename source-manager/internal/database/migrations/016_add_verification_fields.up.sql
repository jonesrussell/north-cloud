ALTER TABLE people ADD COLUMN verification_confidence REAL;
ALTER TABLE people ADD COLUMN verification_issues JSONB DEFAULT '[]';

ALTER TABLE band_offices ADD COLUMN verification_confidence REAL;
ALTER TABLE band_offices ADD COLUMN verification_issues JSONB DEFAULT '[]';
