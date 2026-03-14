CREATE TABLE IF NOT EXISTS travel_time_cache (
    id SERIAL PRIMARY KEY,
    origin_community_id TEXT NOT NULL REFERENCES communities(id),
    destination_community_id TEXT NOT NULL REFERENCES communities(id),
    transport_mode VARCHAR(20) NOT NULL DEFAULT 'car',
    duration_seconds INTEGER NOT NULL,
    distance_meters INTEGER NOT NULL,
    computed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(origin_community_id, destination_community_id, transport_mode)
);

CREATE INDEX idx_travel_time_origin ON travel_time_cache(origin_community_id);
CREATE INDEX idx_travel_time_destination ON travel_time_cache(destination_community_id);
