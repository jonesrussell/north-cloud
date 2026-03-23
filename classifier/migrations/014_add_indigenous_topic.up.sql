-- Migration 014: Add indigenous topic classification rule
-- Adds "indigenous" to the topics[] array so content can be filtered via
-- /api/v1/search?topics[]=indigenous. Complements the existing Layer 7
-- indigenous classifier (which populates the nested indigenous object).
--
-- Keywords drawn from indigenous_rules.go — single tokens that work with
-- the TopicClassifier's whole-token matching algorithm.

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
        -- Core terms (English)
        'indigenous', 'aboriginal', 'autochtone',
        -- North American Indigenous peoples
        'anishinaabe', 'anishinaabemowin', 'ojibwe', 'ojibwa', 'chippewa',
        'cree', 'mohawk', 'haudenosaunee', 'dene', 'inuit', 'inuk',
        -- Canadian context
        'métis', 'metis', 'reconciliation', 'residential',
        -- Governance and rights
        'treaty', 'sovereignty', 'self-determination',
        -- Cultural terms
        'midewiwin', 'powwow', 'potlatch', 'smudging', 'sweat lodge',
        -- Oceania
        'maori', 'māori', 'aboriginal australian',
        -- Americas (Spanish/Portuguese)
        'indígena', 'indigena',
        -- French
        'autochtones', 'premières nations',
        -- Organizations and institutions
        'tribal', 'band council', 'elder', 'elders',
        'knowledge keeper', 'land claim', 'reserve'
    ],
    0.5,  -- 50% confidence threshold (same as mining after tightening)
    8,    -- Priority 8 (above mining at 7, below crime sub-categories)
    true,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
) ON CONFLICT (rule_name) DO NOTHING;

COMMIT;
