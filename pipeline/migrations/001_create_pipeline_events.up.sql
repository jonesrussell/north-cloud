-- Migration: Create pipeline event-sourcing schema
-- Description: Articles, pipeline stages, and partitioned event log
-- Version: 001
-- Date: 2026-02-10

-- ============================================================
-- articles — canonical article identity
-- ============================================================
CREATE TABLE IF NOT EXISTS articles (
    url         TEXT        PRIMARY KEY,
    url_hash    CHAR(64)    UNIQUE NOT NULL,
    domain      TEXT        NOT NULL,
    source_name TEXT        NOT NULL,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_articles_source ON articles(source_name);
CREATE INDEX IF NOT EXISTS idx_articles_hash   ON articles(url_hash);
CREATE INDEX IF NOT EXISTS idx_articles_domain ON articles(domain);

-- ============================================================
-- pipeline_stage — enum for pipeline stages
-- ============================================================
CREATE TYPE pipeline_stage AS ENUM (
    'crawled',
    'indexed',
    'classified',
    'routed',
    'published'
);

-- ============================================================
-- stage_ordering — deterministic sort order for stages
-- ============================================================
CREATE TABLE IF NOT EXISTS stage_ordering (
    stage      pipeline_stage PRIMARY KEY,
    sort_order SMALLINT       UNIQUE NOT NULL
);

INSERT INTO stage_ordering (stage, sort_order) VALUES
    ('crawled',    1),
    ('indexed',    2),
    ('classified', 3),
    ('routed',     4),
    ('published',  5);

-- ============================================================
-- pipeline_events — partitioned event log
-- ============================================================
CREATE TABLE IF NOT EXISTS pipeline_events (
    id                      BIGSERIAL,
    article_url             TEXT            NOT NULL REFERENCES articles(url),
    stage                   pipeline_stage  NOT NULL,
    occurred_at             TIMESTAMPTZ     NOT NULL,
    received_at             TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    service_name            TEXT            NOT NULL,
    metadata                JSONB,
    metadata_schema_version SMALLINT        NOT NULL DEFAULT 1,
    idempotency_key         TEXT,
    PRIMARY KEY (id, occurred_at),
    UNIQUE (idempotency_key, occurred_at)
) PARTITION BY RANGE (occurred_at);

-- Partitions for 2026 Q1–Q2
CREATE TABLE pipeline_events_2026_01 PARTITION OF pipeline_events
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');

CREATE TABLE pipeline_events_2026_02 PARTITION OF pipeline_events
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');

CREATE TABLE pipeline_events_2026_03 PARTITION OF pipeline_events
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');

CREATE TABLE pipeline_events_2026_04 PARTITION OF pipeline_events
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');

-- Indexes on the partitioned table (propagate to all partitions)
CREATE INDEX IF NOT EXISTS idx_events_stage_time ON pipeline_events(stage, occurred_at);
CREATE INDEX IF NOT EXISTS idx_events_article    ON pipeline_events(article_url);
CREATE INDEX IF NOT EXISTS idx_events_occurred   ON pipeline_events(occurred_at);
