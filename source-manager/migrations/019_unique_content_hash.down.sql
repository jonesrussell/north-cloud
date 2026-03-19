DROP INDEX IF EXISTS idx_dictionary_entries_hash;
CREATE INDEX idx_dictionary_entries_hash ON dictionary_entries(content_hash);
