-- Migration 014: Add indigenous topic classification rule
-- Adds "indigenous" to topic detection so search queries with topics[]=indigenous
-- return results. Previously indigenous content was only in the nested indigenous
-- object and never appeared in topics[]. Keywords are a focused subset of the
-- core patterns from indigenous_rules.go.

BEGIN;

INSERT INTO classification_rules (
    rule_name,
    rule_type,
    topic_name,
    keywords,
    min_confidence,
    priority,
    enabled,
    created_at,
    updated_at
) VALUES (
    'indigenous_detection',
    'topic',
    'indigenous',
    ARRAY[
        -- Core identity terms (English, North America)
        'indigenous', 'first nations', 'anishinaabe', 'ojibwe', 'ojibwa',
        'métis', 'metis', 'inuit', 'inuk', 'aboriginal',
        -- Institutional / rights
        'treaty rights', 'land rights', 'residential school',
        'truth and reconciliation', 'self-determination',
        -- Governance
        'band council', 'tribal sovereignty', 'tribal nation',
        -- Cultural
        'powwow', 'midewiwin', 'potlatch', 'sweat lodge',
        -- Languages
        'anishinaabemowin', 'inuktitut', 'cree',
        -- French
        'autochtone', 'premières nations', 'peuples autochtones',
        -- Oceania
        'māori', 'maori', 'tangata whenua',
        -- Other global
        'native hawaiian', 'sami people',
        -- Key acronyms / movements
        'mmiwg', 'land back'
    ],
    0.3,  -- 30% confidence threshold (matches peer topic rules (mining, crime) —
          -- indigenous has dedicated hybrid classifier so this rule is a coarse topic tag)
    10,   -- Priority 10 (same as issue recommendation)
    true,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
) ON CONFLICT (rule_name) DO NOTHING;

COMMIT;
