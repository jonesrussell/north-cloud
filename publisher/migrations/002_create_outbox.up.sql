-- Transactional Outbox for guaranteed Redis publishing
-- Implements the outbox pattern for reliable event delivery

CREATE TABLE IF NOT EXISTS classified_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Content identification
    content_id VARCHAR(255) NOT NULL,
    source_name VARCHAR(255) NOT NULL,
    index_name VARCHAR(255) NOT NULL,

    -- Routing information
    content_type VARCHAR(50) NOT NULL,
    topics TEXT[] NOT NULL DEFAULT '{}',
    quality_score INTEGER NOT NULL,
    is_crime_related BOOLEAN NOT NULL DEFAULT FALSE,
    crime_subcategory VARCHAR(50),

    -- Denormalized content for publishing (avoids ES round-trip)
    title TEXT NOT NULL,
    body TEXT,
    url TEXT NOT NULL,
    published_date TIMESTAMP WITH TIME ZONE,

    -- Outbox metadata
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 5,
    error_message TEXT,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    published_at TIMESTAMP WITH TIME ZONE,
    next_retry_at TIMESTAMP WITH TIME ZONE,

    -- Idempotency: prevent duplicate outbox entries
    CONSTRAINT unique_outbox_content UNIQUE (content_id)
);

-- Index for outbox worker: pending items ready to publish
CREATE INDEX idx_outbox_pending ON classified_outbox(created_at)
    WHERE status = 'pending';

-- Index for retry worker: failed items ready for retry
CREATE INDEX idx_outbox_retry ON classified_outbox(next_retry_at)
    WHERE status = 'failed' AND retry_count < max_retries;

-- Index for routing queries
CREATE INDEX idx_outbox_routing ON classified_outbox(content_type, source_name)
    WHERE status = 'pending';

-- Index for crime-related content (high priority publishing)
CREATE INDEX idx_outbox_crime ON classified_outbox(created_at)
    WHERE status = 'pending' AND is_crime_related = TRUE;

-- Cleanup index for old published entries
CREATE INDEX idx_outbox_cleanup ON classified_outbox(published_at)
    WHERE status = 'published';

-- Index for status monitoring
CREATE INDEX idx_outbox_status ON classified_outbox(status);

COMMENT ON TABLE classified_outbox IS 'Transactional outbox for guaranteed Redis Pub/Sub publishing';
COMMENT ON COLUMN classified_outbox.status IS 'pending=awaiting publish, publishing=in-flight, published=done, failed=retry needed';
COMMENT ON COLUMN classified_outbox.crime_subcategory IS 'violent_crime, property_crime, drug_crime, organized_crime, criminal_justice';
