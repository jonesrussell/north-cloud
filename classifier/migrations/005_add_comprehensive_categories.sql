-- Migration: Add comprehensive classification categories
-- Description: Adds 25+ topic classification categories with initial keyword lists, expanding from 4 to comprehensive Microsoft/Bing-style taxonomy
-- Version: 005
-- Date: 2025-12-23

-- Insert comprehensive topic classification rules
-- Uses ON CONFLICT to avoid conflicts with existing rules (idempotent)

-- High Priority Rules (10) - Critical/Time-sensitive categories
INSERT INTO classification_rules (rule_name, rule_type, topic_name, keywords, min_confidence, priority, enabled) VALUES
    ('breaking_news_detection', 'topic', 'breaking_news', ARRAY[
        'breaking', 'urgent', 'developing', 'alert', 'emergency', 'just in', 'live',
        'breaking news', 'urgent news', 'developing story', 'news alert', 'flash',
        'latest', 'update', 'happening now', 'as it happens'
    ], 0.3, 10, TRUE),
    ('health_emergency_detection', 'topic', 'health_emergency', ARRAY[
        'pandemic', 'outbreak', 'epidemic', 'health crisis', 'public health emergency',
        'disease outbreak', 'contagious', 'quarantine', 'lockdown', 'health alert',
        'medical emergency', 'health warning', 'epidemic', 'plague', 'virus spread'
    ], 0.3, 10, TRUE)
ON CONFLICT (rule_name) DO NOTHING;

-- Normal Priority Rules (5) - Common news categories
INSERT INTO classification_rules (rule_name, rule_type, topic_name, keywords, min_confidence, priority, enabled) VALUES
    ('business_detection', 'topic', 'business', ARRAY[
        'company', 'market', 'stock', 'economy', 'trade', 'finance', 'investment',
        'business', 'corporate', 'enterprise', 'industry', 'commercial', 'merger',
        'acquisition', 'revenue', 'profit', 'earnings', 'quarterly', 'shareholder',
        'CEO', 'executive', 'board', 'IPO', 'startup', 'venture capital'
    ], 0.4, 5, TRUE),
    ('technology_detection', 'topic', 'technology', ARRAY[
        'tech', 'software', 'app', 'digital', 'computer', 'internet', 'ai', 'innovation',
        'technology', 'software', 'hardware', 'device', 'smartphone', 'tablet',
        'laptop', 'cloud', 'cyber', 'hack', 'data', 'algorithm', 'machine learning',
        'artificial intelligence', 'blockchain', 'crypto', 'startup', 'innovation',
        'gadget', 'electronic', 'digital', 'online', 'web', 'platform'
    ], 0.4, 5, TRUE),
    ('health_detection', 'topic', 'health', ARRAY[
        'medical', 'doctor', 'hospital', 'treatment', 'medicine', 'wellness', 'fitness',
        'health', 'healthcare', 'medical', 'hospital', 'clinic', 'physician', 'nurse',
        'patient', 'diagnosis', 'treatment', 'therapy', 'medication', 'surgery',
        'disease', 'illness', 'symptom', 'recovery', 'wellness', 'fitness', 'exercise',
        'nutrition', 'diet', 'mental health', 'psychology', 'therapy'
    ], 0.4, 5, TRUE),
    ('entertainment_detection', 'topic', 'entertainment', ARRAY[
        'movie', 'film', 'tv', 'show', 'celebrity', 'music', 'concert', 'actor',
        'entertainment', 'movie', 'film', 'cinema', 'theater', 'television', 'TV',
        'series', 'episode', 'actor', 'actress', 'director', 'producer', 'celebrity',
        'star', 'famous', 'music', 'song', 'album', 'artist', 'singer', 'band',
        'concert', 'tour', 'award', 'oscar', 'grammy', 'emmy', 'premiere', 'red carpet'
    ], 0.4, 5, TRUE),
    ('science_detection', 'topic', 'science', ARRAY[
        'research', 'study', 'scientist', 'discovery', 'experiment', 'lab', 'study',
        'science', 'scientific', 'research', 'study', 'experiment', 'laboratory', 'lab',
        'scientist', 'researcher', 'discovery', 'finding', 'hypothesis', 'theory',
        'data', 'analysis', 'publication', 'journal', 'peer review', 'innovation',
        'breakthrough', 'study', 'observation', 'methodology'
    ], 0.4, 5, TRUE),
    ('education_detection', 'topic', 'education', ARRAY[
        'school', 'university', 'student', 'teacher', 'learning', 'education',
        'education', 'school', 'university', 'college', 'student', 'teacher',
        'professor', 'academic', 'learning', 'curriculum', 'course', 'degree',
        'diploma', 'graduation', 'tuition', 'scholarship', 'campus', 'classroom',
        'textbook', 'homework', 'exam', 'test', 'grade', 'semester', 'term'
    ], 0.4, 5, TRUE),
    ('weather_detection', 'topic', 'weather', ARRAY[
        'forecast', 'storm', 'temperature', 'climate', 'hurricane', 'tornado',
        'weather', 'forecast', 'temperature', 'rain', 'snow', 'storm', 'hurricane',
        'tornado', 'thunderstorm', 'lightning', 'wind', 'cloud', 'sunny', 'cloudy',
        'precipitation', 'drought', 'flood', 'climate', 'meteorology', 'meteorologist',
        'weather warning', 'weather alert', 'severe weather', 'heat wave', 'cold snap'
    ], 0.4, 5, TRUE),
    ('travel_detection', 'topic', 'travel', ARRAY[
        'trip', 'vacation', 'hotel', 'flight', 'destination', 'tourism', 'travel',
        'travel', 'trip', 'vacation', 'journey', 'tour', 'tourist', 'destination',
        'hotel', 'resort', 'flight', 'airline', 'airport', 'luggage', 'passport',
        'visa', 'cruise', 'beach', 'sightseeing', 'adventure', 'backpacking',
        'tourism', 'travel guide', 'itinerary', 'booking', 'reservation'
    ], 0.4, 5, TRUE),
    ('food_detection', 'topic', 'food', ARRAY[
        'restaurant', 'recipe', 'cooking', 'chef', 'cuisine', 'food', 'dining',
        'food', 'restaurant', 'cafe', 'dining', 'cuisine', 'recipe', 'cooking',
        'chef', 'kitchen', 'meal', 'dish', 'ingredient', 'flavor', 'taste',
        'menu', 'appetizer', 'entree', 'dessert', 'beverage', 'wine', 'beer',
        'bakery', 'grocery', 'farmers market', 'organic', 'vegan', 'vegetarian'
    ], 0.4, 5, TRUE),
    ('lifestyle_detection', 'topic', 'lifestyle', ARRAY[
        'fashion', 'style', 'trend', 'culture', 'lifestyle', 'personal',
        'lifestyle', 'fashion', 'style', 'trend', 'trendy', 'outfit', 'clothing',
        'designer', 'brand', 'culture', 'cultural', 'tradition', 'custom', 'habit',
        'routine', 'wellness', 'self-care', 'beauty', 'cosmetics', 'skincare',
        'personal', 'life', 'living', 'home', 'family', 'relationship'
    ], 0.4, 5, TRUE),
    ('automotive_detection', 'topic', 'automotive', ARRAY[
        'car', 'vehicle', 'auto', 'driving', 'road', 'traffic', 'vehicle',
        'car', 'automobile', 'vehicle', 'auto', 'truck', 'SUV', 'sedan',
        'driving', 'driver', 'road', 'highway', 'traffic', 'accident', 'crash',
        'dealership', 'manufacturer', 'brand', 'model', 'engine', 'fuel', 'gas',
        'electric', 'hybrid', 'autonomous', 'self-driving', 'parking', 'garage'
    ], 0.4, 5, TRUE),
    ('real_estate_detection', 'topic', 'real_estate', ARRAY[
        'property', 'house', 'home', 'real estate', 'mortgage', 'housing',
        'real estate', 'property', 'house', 'home', 'apartment', 'condo',
        'mortgage', 'loan', 'buyer', 'seller', 'realtor', 'agent', 'listing',
        'price', 'market', 'neighborhood', 'square feet', 'bedroom', 'bathroom',
        'renovation', 'remodel', 'construction', 'builder', 'developer', 'zoning'
    ], 0.4, 5, TRUE),
    ('finance_detection', 'topic', 'finance', ARRAY[
        'money', 'bank', 'financial', 'investment', 'credit', 'loan', 'savings',
        'finance', 'financial', 'money', 'bank', 'banking', 'account', 'savings',
        'checking', 'credit', 'loan', 'mortgage', 'interest', 'rate', 'investment',
        'portfolio', 'stock', 'bond', 'retirement', '401k', 'IRA', 'insurance',
        'tax', 'IRS', 'budget', 'expense', 'income', 'debt', 'credit card'
    ], 0.4, 5, TRUE),
    ('environment_detection', 'topic', 'environment', ARRAY[
        'climate', 'environment', 'pollution', 'green', 'sustainability', 'nature',
        'environment', 'environmental', 'climate', 'climate change', 'global warming',
        'pollution', 'air quality', 'water quality', 'green', 'sustainable', 'sustainability',
        'renewable', 'solar', 'wind', 'energy', 'carbon', 'emission', 'conservation',
        'wildlife', 'nature', 'ecosystem', 'habitat', 'endangered', 'extinction',
        'recycling', 'waste', 'plastic', 'ocean', 'forest', 'deforestation'
    ], 0.4, 5, TRUE),
    ('arts_detection', 'topic', 'arts', ARRAY[
        'art', 'artist', 'gallery', 'museum', 'painting', 'sculpture', 'creative',
        'art', 'artist', 'artistic', 'gallery', 'museum', 'exhibition', 'painting',
        'sculpture', 'drawing', 'sketch', 'portrait', 'landscape', 'abstract',
        'creative', 'creativity', 'design', 'illustration', 'photography', 'photo',
        'sculptor', 'painter', 'artwork', 'masterpiece', 'collection', 'auction'
    ], 0.4, 5, TRUE)
ON CONFLICT (rule_name) DO NOTHING;

-- Low Priority Rules (1-3) - Niche categories
INSERT INTO classification_rules (rule_name, rule_type, topic_name, keywords, min_confidence, priority, enabled) VALUES
    ('pets_detection', 'topic', 'pets', ARRAY[
        'pet', 'dog', 'cat', 'animal', 'veterinary', 'pet care',
        'pet', 'pets', 'dog', 'puppy', 'cat', 'kitten', 'animal', 'animals',
        'veterinary', 'vet', 'veterinarian', 'pet care', 'pet food', 'pet store',
        'adoption', 'rescue', 'shelter', 'breed', 'training', 'grooming', 'walk',
        'leash', 'collar', 'toy', 'treat', 'vaccination', 'spay', 'neuter'
    ], 0.4, 2, TRUE),
    ('gaming_detection', 'topic', 'gaming', ARRAY[
        'game', 'video game', 'gaming', 'esports', 'console', 'gamer',
        'gaming', 'game', 'video game', 'gamer', 'console', 'playstation', 'xbox',
        'nintendo', 'PC game', 'mobile game', 'esports', 'tournament', 'streaming',
        'twitch', 'youtube gaming', 'level', 'quest', 'character', 'player',
        'multiplayer', 'online', 'MMO', 'RPG', 'FPS', 'strategy', 'indie game'
    ], 0.4, 2, TRUE),
    ('shopping_detection', 'topic', 'shopping', ARRAY[
        'store', 'shop', 'retail', 'purchase', 'buy', 'sale', 'discount',
        'shopping', 'store', 'shop', 'retail', 'mall', 'boutique', 'purchase',
        'buy', 'sale', 'discount', 'coupon', 'deal', 'offer', 'price', 'cost',
        'checkout', 'cart', 'basket', 'online shopping', 'e-commerce', 'amazon',
        'delivery', 'shipping', 'return', 'refund', 'warranty', 'review'
    ], 0.4, 2, TRUE),
    ('home_garden_detection', 'topic', 'home_garden', ARRAY[
        'home', 'garden', 'furniture', 'decor', 'interior', 'landscaping',
        'home', 'house', 'garden', 'gardening', 'yard', 'lawn', 'furniture',
        'decor', 'decoration', 'interior', 'interior design', 'renovation',
        'remodel', 'landscaping', 'landscape', 'plant', 'flower', 'tree',
        'vegetable', 'herb', 'soil', 'compost', 'seed', 'sprinkler', 'fence',
        'patio', 'deck', 'outdoor', 'indoor', 'room', 'kitchen', 'bathroom'
    ], 0.4, 2, TRUE),
    ('recreation_detection', 'topic', 'recreation', ARRAY[
        'hobby', 'recreation', 'leisure', 'activity', 'outdoor', 'fun',
        'recreation', 'recreational', 'hobby', 'hobbies', 'leisure', 'activity',
        'outdoor', 'indoor', 'fun', 'entertainment', 'pastime', 'sport',
        'fishing', 'hunting', 'camping', 'hiking', 'biking', 'running', 'jogging',
        'swimming', 'boating', 'sailing', 'skiing', 'snowboarding', 'photography',
        'reading', 'writing', 'crafting', 'knitting', 'sewing', 'woodworking'
    ], 0.4, 1, TRUE)
ON CONFLICT (rule_name) DO NOTHING;

-- Comments
COMMENT ON TABLE classification_rules IS 'Rules for classifying content by type and topic - now includes comprehensive Microsoft/Bing-style taxonomy';
