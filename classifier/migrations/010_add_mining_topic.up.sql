-- Migration 010: Add mining topic classification rule
-- Adds a single "mining" topic for detecting mining industry news

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
    'mining_detection',
    'topic',
    'mining',
    ARRAY[
        -- Core mining terms
        'mining', 'miner', 'mine', 'mineral', 'minerals',
        -- Exploration and development
        'exploration', 'drilling', 'drill', 'drilled', 'prospect', 'prospecting',
        -- Geology
        'ore', 'orebody', 'deposit', 'deposits', 'geology', 'geologist', 'geological',
        -- Resources and reserves
        'resource', 'resources', 'reserve', 'reserves', 'tonne', 'tonnes',
        -- Assay and grade
        'assay', 'assays', 'grade', 'grades', 'intercept', 'intercepts',
        -- Key commodities (overlap with orewire commodities)
        'gold', 'silver', 'copper', 'zinc', 'nickel', 'lithium', 'uranium',
        -- Mining operations
        'pit', 'underground', 'open-pit', 'tailings', 'smelter', 'refinery',
        -- Industry terms
        'metallurgy', 'metallurgical', 'concentrate', 'extraction'
    ],
    0.3,  -- 30% confidence threshold (same as other topics)
    7,    -- Priority 7 (medium - between crime sub-categories and general topics)
    true,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
) ON CONFLICT (rule_name) DO NOTHING;

COMMIT;
