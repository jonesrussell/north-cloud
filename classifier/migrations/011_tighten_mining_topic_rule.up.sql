-- Migration 011: Tighten mining topic classification rule
-- Raise min_confidence from 0.3 to 0.5 and remove ambiguous keywords
-- that cause false positives (e.g. "gold", "silver", "resource", "grade").

BEGIN;

UPDATE classification_rules
SET
    keywords = ARRAY[
        -- Core mining terms
        'mining', 'miner', 'mine',
        -- Exploration and development
        'exploration', 'drilling', 'drill', 'drilled', 'prospect', 'prospecting',
        -- Ore and assay
        'ore', 'orebody', 'assay', 'assays', 'intercept', 'intercepts',
        -- Mining-specific commodities
        'lithium', 'uranium',
        -- Mining operations
        'open-pit', 'tailings', 'smelter', 'refinery',
        -- Industry terms
        'metallurgy', 'metallurgical', 'concentrate'
    ],
    min_confidence = 0.5,
    updated_at = CURRENT_TIMESTAMP
WHERE rule_name = 'mining_detection';

COMMIT;
