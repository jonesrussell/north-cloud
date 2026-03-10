CREATE TABLE people (
    id              VARCHAR(36)  PRIMARY KEY,
    community_id    VARCHAR(36)  NOT NULL REFERENCES communities(id) ON DELETE CASCADE,

    -- Identity
    name            VARCHAR(255) NOT NULL,
    slug            VARCHAR(255) NOT NULL,
    role            VARCHAR(100) NOT NULL,
    role_title      VARCHAR(255),

    -- Contact
    email           TEXT,
    phone           VARCHAR(50),

    -- Term
    term_start      DATE,
    term_end        DATE,
    is_current      BOOLEAN      NOT NULL DEFAULT true,

    -- Provenance
    data_source     VARCHAR(50)  NOT NULL DEFAULT 'manual',
    source_url      TEXT,
    verified        BOOLEAN      NOT NULL DEFAULT false,
    verified_at     TIMESTAMP,

    -- Lifecycle
    created_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT uq_people_community_name_role UNIQUE (community_id, name, role)
);

CREATE INDEX idx_people_community ON people(community_id);
CREATE INDEX idx_people_role ON people(role);
CREATE INDEX idx_people_current ON people(is_current) WHERE is_current = true;

CREATE TRIGGER set_people_updated_at
    BEFORE UPDATE ON people
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE people_history (
    id              VARCHAR(36)  PRIMARY KEY,
    person_id       VARCHAR(36)  NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    community_id    VARCHAR(36)  NOT NULL REFERENCES communities(id) ON DELETE CASCADE,
    name            VARCHAR(255) NOT NULL,
    role            VARCHAR(100) NOT NULL,
    term_start      DATE,
    term_end        DATE,
    data_source     VARCHAR(50),
    source_url      TEXT,
    archived_at     TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_people_history_community ON people_history(community_id);
CREATE INDEX idx_people_history_person ON people_history(person_id);
