-- Migration: Remove comprehensive classification categories
-- Description: Removes the comprehensive topic categories added in migration 005

-- Delete the comprehensive topic rules (keep original 4 rules)
DELETE FROM classification_rules WHERE rule_name IN (
    'breaking_news_detection',
    'health_emergency_detection',
    'business_detection',
    'technology_detection',
    'health_detection',
    'entertainment_detection',
    'science_detection',
    'education_detection',
    'weather_detection',
    'travel_detection',
    'food_detection',
    'lifestyle_detection',
    'automotive_detection',
    'real_estate_detection',
    'finance_detection',
    'environment_detection',
    'arts_detection',
    'pets_detection',
    'gaming_detection',
    'shopping_detection',
    'home_garden_detection',
    'recreation_detection'
);

-- Restore original comment
COMMENT ON TABLE classification_rules IS 'Rules for classifying content by type and topic';

