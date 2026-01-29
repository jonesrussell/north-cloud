-- Dead Letter Queue for failed classifications
-- Provides automatic retry with exponential backoff

CREATE TABLE IF NOT EXISTS dead_letter_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    content_id VARCHAR(255) NOT NULL,
    source_name VARCHAR(255) NOT NULL,
    index_name VARCHAR(255) NOT NULL,
    error_message TEXT NOT NULL,
    error_code VARCHAR(50),
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 5,
    next_retry_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_attempt_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    -- Prevent duplicate entries for same content
    CONSTRAINT unique_content_in_dlq UNIQUE (content_id)
);

-- Index for retry worker: find retryable items efficiently
CREATE INDEX idx_dlq_next_retry ON dead_letter_queue(next_retry_at)
    WHERE retry_count < max_retries;

-- Index for monitoring by source
CREATE INDEX idx_dlq_source ON dead_letter_queue(source_name);

-- Index for finding exhausted items
CREATE INDEX idx_dlq_exhausted ON dead_letter_queue(retry_count, max_retries)
    WHERE retry_count >= max_retries;

-- Index for error code analysis
CREATE INDEX idx_dlq_error_code ON dead_letter_queue(error_code)
    WHERE error_code IS NOT NULL;

COMMENT ON TABLE dead_letter_queue IS 'Failed classifications awaiting retry with exponential backoff';
COMMENT ON COLUMN dead_letter_queue.error_code IS 'Categorized error: ES_TIMEOUT, ES_UNAVAILABLE, RULE_PANIC, QUALITY_ERROR, UNKNOWN';
COMMENT ON COLUMN dead_letter_queue.next_retry_at IS 'When this entry becomes eligible for retry (exponential backoff)';
