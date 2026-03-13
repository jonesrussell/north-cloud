CREATE TABLE IF NOT EXISTS dictionary_entries (
    id                        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lemma                     TEXT NOT NULL,
    word_class                TEXT,
    word_class_normalized     TEXT,
    definitions               JSONB NOT NULL DEFAULT '[]',
    inflections               JSONB NOT NULL DEFAULT '{}',
    examples                  JSONB NOT NULL DEFAULT '[]',
    word_family               JSONB NOT NULL DEFAULT '[]',
    media                     JSONB NOT NULL DEFAULT '[]',
    attribution               TEXT,
    license                   TEXT NOT NULL DEFAULT 'CC BY-NC-SA 4.0',
    consent_public_display    BOOLEAN NOT NULL DEFAULT FALSE,
    consent_ai_training       BOOLEAN NOT NULL DEFAULT FALSE,
    consent_derivative_works  BOOLEAN NOT NULL DEFAULT FALSE,
    content_hash              TEXT,
    source_url                TEXT,
    created_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_dictionary_entries_lemma ON dictionary_entries(lemma);
CREATE INDEX idx_dictionary_entries_consent ON dictionary_entries(consent_public_display);
CREATE INDEX idx_dictionary_entries_hash ON dictionary_entries(content_hash);

-- Full-text search index
ALTER TABLE dictionary_entries ADD COLUMN search_vector TSVECTOR
    GENERATED ALWAYS AS (
        to_tsvector('english', coalesce(lemma, ''))
    ) STORED;
CREATE INDEX idx_dictionary_entries_fts ON dictionary_entries USING GIN(search_vector);
