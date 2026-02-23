-- Add recipe and job topic keyword rules for fallback detection
-- when Schema.org structured data is not present.
INSERT INTO classification_rules (type, topic, keywords, priority, min_confidence, enabled)
VALUES
    ('topic', 'recipe', '["recipe","ingredients","prep time","cook time","servings","tablespoon","preheat","bake at"]', 80, 0.5, true),
    ('topic', 'jobs', '["job posting","apply now","salary","qualifications","employment type","full-time","part-time","hiring","career opportunity"]', 80, 0.5, true);
