-- Remove duplicate content_hash rows, keeping the oldest entry per hash
DELETE FROM dictionary_entries
WHERE id NOT IN (
    SELECT DISTINCT ON (content_hash) id
    FROM dictionary_entries
    WHERE content_hash IS NOT NULL
    ORDER BY content_hash, created_at ASC
)
AND content_hash IS NOT NULL
AND id NOT IN (
    SELECT id FROM dictionary_entries WHERE content_hash IS NULL
);

DROP INDEX IF EXISTS idx_dictionary_entries_hash;
CREATE UNIQUE INDEX idx_dictionary_entries_hash ON dictionary_entries(content_hash)
    WHERE content_hash IS NOT NULL;
