-- Leads export for Claudriel pipeline (GET /api/leads)
CREATE TABLE claudriel_leads (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title VARCHAR(512) NOT NULL DEFAULT '',
    description TEXT,
    contact_name VARCHAR(255),
    contact_email VARCHAR(255),
    url TEXT,
    closing_date VARCHAR(64),
    budget VARCHAR(64),
    sector VARCHAR(128),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_claudriel_leads_created_at ON claudriel_leads (created_at DESC);
