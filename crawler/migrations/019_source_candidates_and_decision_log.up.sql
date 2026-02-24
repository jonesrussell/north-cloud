-- Source Candidate Pipeline: candidates and decision log for automatic source discovery.

CREATE TABLE IF NOT EXISTS source_candidates (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    canonical_url       TEXT NOT NULL,
    identity_key        VARCHAR(512) NOT NULL,
    referring_source_id VARCHAR(36) NOT NULL,
    enrichment          JSONB,
    risk_score         DOUBLE PRECISION NOT NULL DEFAULT 0,
    risk_reasons        JSONB NOT NULL DEFAULT '[]',
    status              VARCHAR(20) NOT NULL DEFAULT 'pending',
    approved_at         TIMESTAMP WITH TIME ZONE,
    approved_by         VARCHAR(255),
    created_source_id   VARCHAR(36),
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_candidate_status CHECK (status IN ('pending', 'approved', 'rejected', 'processing'))
);

CREATE INDEX idx_source_candidates_identity_key ON source_candidates (identity_key);
CREATE INDEX idx_source_candidates_status ON source_candidates (status);
CREATE INDEX idx_source_candidates_referring_source ON source_candidates (referring_source_id);
CREATE UNIQUE INDEX idx_source_candidates_identity_pending ON source_candidates (identity_key) WHERE status = 'pending';

CREATE TRIGGER update_source_candidates_updated_at BEFORE UPDATE ON source_candidates
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Decision log: every resolution, risk score, approval, creation, frontier seed (deterministic audit).
CREATE TABLE IF NOT EXISTS discovery_decision_log (
    id          BIGSERIAL PRIMARY KEY,
    occurred_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    stage       VARCHAR(64) NOT NULL,
    reason      TEXT NOT NULL,
    inputs      JSONB NOT NULL DEFAULT '{}',
    outputs     JSONB,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_discovery_decision_log_occurred_at ON discovery_decision_log (occurred_at DESC);
CREATE INDEX idx_discovery_decision_log_stage ON discovery_decision_log (stage);
