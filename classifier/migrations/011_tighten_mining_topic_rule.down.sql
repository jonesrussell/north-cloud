-- Migration 011: Revert mining topic rule to original broad keywords and threshold

BEGIN;

UPDATE classification_rules
SET
    keywords = ARRAY[
        'mining', 'miner', 'mine', 'mineral', 'minerals',
        'exploration', 'drilling', 'drill', 'drilled', 'prospect', 'prospecting',
        'ore', 'orebody', 'deposit', 'deposits', 'geology', 'geologist', 'geological',
        'resource', 'resources', 'reserve', 'reserves', 'tonne', 'tonnes',
        'assay', 'assays', 'grade', 'grades', 'intercept', 'intercepts',
        'gold', 'silver', 'copper', 'zinc', 'nickel', 'lithium', 'uranium',
        'pit', 'underground', 'open-pit', 'tailings', 'smelter', 'refinery',
        'metallurgy', 'metallurgical', 'concentrate', 'extraction'
    ],
    min_confidence = 0.3,
    updated_at = CURRENT_TIMESTAMP
WHERE rule_name = 'mining_detection';

COMMIT;
