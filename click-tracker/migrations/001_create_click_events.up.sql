CREATE TABLE click_events (
    id               BIGSERIAL PRIMARY KEY,
    query_id         VARCHAR(32)  NOT NULL,
    result_id        VARCHAR(128) NOT NULL,
    position         SMALLINT     NOT NULL,
    page             SMALLINT     NOT NULL DEFAULT 1,
    destination_hash VARCHAR(64)  NOT NULL,
    session_id       VARCHAR(32),
    user_agent_hash  VARCHAR(12),
    generated_at     TIMESTAMPTZ  NOT NULL,
    clicked_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
) PARTITION BY RANGE (clicked_at);

CREATE TABLE click_events_default PARTITION OF click_events DEFAULT;

CREATE INDEX idx_click_events_query_id   ON click_events (query_id);
CREATE INDEX idx_click_events_result_id  ON click_events (result_id);
CREATE INDEX idx_click_events_position   ON click_events (position);
CREATE INDEX idx_click_events_clicked_at ON click_events (clicked_at);
