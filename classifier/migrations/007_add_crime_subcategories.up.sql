-- Migration 007: Add crime sub-category classification rules
-- Phase 1: Core crime categories (violent, property, drug, organized, criminal_justice)
-- Disables original generic "crime" rule in favor of specific sub-categories

BEGIN;

-- Disable the original generic "crime" rule
-- Forcing explicit categorization via specific sub-categories
UPDATE classification_rules
SET enabled = false,
    updated_at = CURRENT_TIMESTAMP
WHERE rule_name = 'crime_detection'
  AND rule_type = 'topic';

-- 1. Violent Crime (Priority 10 - Highest)
-- Includes: gang violence, murder, assault, shootings, domestic violence
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
    'violent_crime_detection',
    'topic',
    'violent_crime',
    ARRAY[
        -- Core violent crimes
        'murder', 'homicide', 'assault', 'shooting', 'stabbing', 'killing', 'manslaughter',
        -- Violence keywords
        'attack', 'attacked', 'weapon', 'armed', 'gunman', 'shooter', 'fight', 'fighting', 'beating',
        -- Gang-related violence
        'gang', 'gang violence', 'drive-by', 'turf war', 'gang member', 'gang activity',
        -- Additional violent crimes
        'domestic violence', 'sexual assault', 'rape', 'kidnapping', 'abduction', 'hostage'
    ],
    0.3,  -- 30% confidence threshold
    10,   -- Highest priority
    true,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
);

-- 2. Property Crime (Priority 9)
-- Includes: theft, burglary, auto theft, vandalism, arson
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
    'property_crime_detection',
    'topic',
    'property_crime',
    ARRAY[
        -- Core property crimes
        'theft', 'robbery', 'burglary', 'stolen', 'shoplifting', 'larceny',
        -- Breaking and entering
        'break-in', 'breaking and entering', 'trespassing', 'vandalism',
        -- Vehicle-related
        'carjacking', 'auto theft', 'stolen car', 'stolen vehicle', 'vehicle theft',
        -- Additional property crimes
        'arson', 'graffiti', 'property damage', 'looting'
    ],
    0.3,
    9,
    true,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
);

-- 3. Drug Crime (Priority 9)
-- Includes: trafficking, possession, drug busts, narcotics
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
    'drug_crime_detection',
    'topic',
    'drug_crime',
    ARRAY[
        -- Core drug crime
        'drug', 'drugs', 'narcotics', 'trafficking', 'dealer', 'possession',
        -- Specific substances
        'cocaine', 'heroin', 'fentanyl', 'methamphetamine', 'meth', 'marijuana', 'cannabis', 'opioid',
        -- Operations
        'drug bust', 'drug ring', 'cartel', 'smuggling', 'drug trafficking',
        -- Related
        'overdose', 'drug-related', 'controlled substance'
    ],
    0.3,
    9,
    true,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
);

-- 4. Organized Crime (Priority 9)
-- Includes: cartels, racketeering, mafia, crime syndicates
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
    'organized_crime_detection',
    'topic',
    'organized_crime',
    ARRAY[
        -- Core organized crime
        'organized crime', 'mafia', 'mob', 'cartel', 'crime syndicate',
        -- Operations
        'racketeering', 'extortion', 'money laundering', 'trafficking ring', 'human trafficking',
        -- RICO and legal terms
        'RICO', 'RICO charges', 'organized criminal enterprise',
        -- Related
        'crime family', 'kingpin', 'crime boss', 'criminal organization'
    ],
    0.3,
    9,
    true,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
);

-- 5. Criminal Justice (Priority 5 - Context Category)
-- Cross-cutting category for court cases, arrests, legal proceedings
-- Lower priority as it often appears alongside other crime categories
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
    'criminal_justice_detection',
    'topic',
    'criminal_justice',
    ARRAY[
        -- Core legal process
        'court', 'trial', 'conviction', 'sentence', 'sentencing', 'verdict',
        -- Court proceedings
        'arraignment', 'indictment', 'plea deal', 'hearing', 'appeal',
        -- Outcomes
        'prison', 'jail', 'incarceration', 'parole', 'probation',
        -- Investigation and enforcement
        'warrant', 'arrest', 'arrested', 'investigation', 'detective', 'prosecution', 'prosecutor',
        -- Legal roles
        'judge', 'jury', 'defense attorney', 'public defender'
    ],
    0.3,
    5,  -- Lower priority (context category)
    true,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
);

COMMIT;
