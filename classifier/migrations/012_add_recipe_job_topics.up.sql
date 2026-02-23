-- Add recipe and job topic keyword rules for fallback detection
-- when Schema.org structured data is not present.
INSERT INTO classification_rules (rule_name, rule_type, topic_name, keywords, min_confidence, priority, enabled)
VALUES
    ('recipe_detection', 'topic', 'recipe', ARRAY[
        'recipe', 'ingredients', 'prep time', 'cook time', 'servings', 'tablespoon',
        'preheat', 'bake at'
    ], 0.5, 80, true),
    ('jobs_detection', 'topic', 'jobs', ARRAY[
        'job posting', 'apply now', 'salary', 'qualifications', 'employment type',
        'full-time', 'part-time', 'hiring', 'career opportunity'
    ], 0.5, 80, true);
