CREATE TABLE IF NOT EXISTS content (
    id          TEXT PRIMARY KEY,
    type        TEXT NOT NULL,
    title       TEXT,
    body        TEXT,
    summary     TEXT,
    url         TEXT,
    images      JSONB DEFAULT '[]'::jsonb,
    tags        JSONB DEFAULT '[]'::jsonb,
    project     TEXT NOT NULL,
    metadata    JSONB DEFAULT '{}'::jsonb,
    source      TEXT NOT NULL,
    published   BOOLEAN NOT NULL DEFAULT false,
    scheduled_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS accounts (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL UNIQUE,
    platform      TEXT NOT NULL,
    project       TEXT NOT NULL,
    enabled       BOOLEAN NOT NULL DEFAULT true,
    credentials   BYTEA,
    token_expiry  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS deliveries (
    id            TEXT PRIMARY KEY,
    content_id    TEXT NOT NULL REFERENCES content(id),
    platform      TEXT NOT NULL,
    account       TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'pending',
    platform_id   TEXT,
    platform_url  TEXT,
    error         TEXT,
    attempts      INT NOT NULL DEFAULT 0,
    max_attempts  INT NOT NULL DEFAULT 3,
    next_retry_at TIMESTAMPTZ,
    last_error_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivered_at  TIMESTAMPTZ,
    UNIQUE(content_id, platform, account)
);

CREATE INDEX idx_deliveries_retry ON deliveries (status, next_retry_at);
CREATE INDEX idx_deliveries_content ON deliveries (content_id);
CREATE INDEX idx_content_scheduled ON content (scheduled_at) WHERE scheduled_at IS NOT NULL AND published = false;
