-- Migration 014: Add indigenous topic classification rule
-- Adds "indigenous" to the topics[] array so content can be filtered via
-- /api/v1/search?topics[]=indigenous. Complements the existing Layer 7
-- indigenous classifier (which populates the nested indigenous object).
--
-- Keywords drawn from indigenous_rules.go. Single tokens use exact-token
-- matching; multi-word phrases use substring matching.

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
        -- Core terms (low false-positive risk)
        'indigenous', 'aboriginal',
        -- North American Indigenous peoples (unique, no false positives)
        'anishinaabe', 'anishinaabemowin', 'ojibwe', 'ojibwa', 'chippewa',
        'cree', 'mohawk', 'haudenosaunee', 'dene', 'inuit', 'inuk',
        'métis', 'metis',
        -- Multi-word phrases (substring match — specific enough to avoid false positives)
        'first nations', 'first nation', 'indigenous peoples', 'indigenous community',
        'native american', 'truth and reconciliation', 'residential school',
        'treaty rights', 'land rights', 'land claim', 'band council',
        'knowledge keeper', 'self-determination',
        -- Cultural terms (unique to indigenous contexts)
        'midewiwin', 'powwow', 'potlatch', 'smudging', 'sweat lodge',
        -- Oceania (multi-word to avoid single-token false positives)
        'aboriginal australian', 'torres strait islander',
        -- Americas (Spanish/Portuguese)
        'pueblos indígenas', 'comunidad indígena',
        -- French
        'peuples autochtones', 'premières nations', 'droits autochtones',
        -- Nordic
        'sami people'
    ],
    0.3,  -- 30% confidence threshold (matches peer topic rules (mining, crime) —
          -- indigenous has dedicated hybrid classifier so this rule is a coarse topic tag)
    8,    -- Priority 8 (above mining at 7, below crime sub-categories)
    true,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
) ON CONFLICT (rule_name) DO NOTHING;

COMMIT;
