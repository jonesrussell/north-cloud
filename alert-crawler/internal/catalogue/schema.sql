-- schema.sql: canonical DDL for the alert-crawler SQLite catalogue.
-- Applied via embedded migrations in store.go; do not run this directly.

CREATE TABLE IF NOT EXISTS poll_checkpoint (
    source_id            TEXT    NOT NULL,
    feed_url             TEXT    NOT NULL,
    last_polled_at       TEXT    NOT NULL,  -- RFC 3339
    last_etag            TEXT    NOT NULL DEFAULT '',
    last_modified        TEXT    NOT NULL DEFAULT '',
    last_status          INTEGER NOT NULL DEFAULT 0,
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (source_id, feed_url)
);

CREATE TABLE IF NOT EXISTS alert_catalogue (
    source_id    TEXT    NOT NULL,
    alert_id     TEXT    NOT NULL,
    last_seen_at TEXT    NOT NULL,  -- RFC 3339
    is_active    INTEGER NOT NULL DEFAULT 1,  -- 1=true, 0=false
    content_hash TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (source_id, alert_id)
);

CREATE INDEX IF NOT EXISTS idx_catalogue_active_seen
    ON alert_catalogue (source_id, is_active, last_seen_at);
