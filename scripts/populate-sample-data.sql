-- Sample Data Population Script
-- This script populates the database with comprehensive sample data for testing scenarios

\echo 'Starting sample data population...'

-- Clear existing data (be careful in production!)
TRUNCATE TABLE recommendation_metrics CASCADE;
TRUNCATE TABLE user_interactions CASCADE;
TRUNCATE TABLE user_profiles CASCADE;
TRUNCATE TABLE content_items CASCADE;
TRUNCATE TABLE content_jobs CASCADE;
TRUNCATE TABLE daily_metrics_summary CASCADE;

-- Reset sequences
ALTER SEQUENCE IF EXISTS content_items_id_seq RESTART WITH 1;
ALTER SEQUENCE IF EXISTS user_interactions_id_seq RESTART WITH 1;
ALTER SEQUENCE IF EXISTS recommendation_metrics_id_seq RESTART WITH 1;

\echo 'Populating content_items...'

-- Sample content items with different types and categories
INSERT INTO content_items (id, type, title, description, image_urls, metadata, categories, quality_score, active) VALUES
-- Electronics Products
('550e8400-e29b-41d4-a716-446655440001', 'product', 'iPhone 15 Pro', 'Latest Apple smartphone with advanced camera system', 
 ARRAY['https://example.com/iphone15pro.jpg'], 
 '{"brand": "Apple", "price": 999.99, "color": "Natural Titanium", "storage": "128GB"}',
 ARRAY['Electronics', 'Smartphones', 'Apple'], 0.95, true),

('550e8400-e29b-41d4-a716-446655440002', 'product', 'Samsung Galaxy S24 Ultra', 'Premium Android smartphone with S Pen', 
 ARRAY['https://example.com/galaxys24ultra.jpg'], 
 '{"brand": "Samsung", "price": 1199.99, "color": "Titanium Black", "storage": "256GB"}',
 ARRAY['Electronics', 'Smartphones', 'Samsung'], 0.92, true),

('550e8400-e29b-41d4-a716-446655440003', 'product', 'MacBook Pro 16"', 'Professional laptop with M3 Pro chip', 
 ARRAY['https://example.com/macbookpro16.jpg'], 
 '{"brand": "Apple", "price": 2499.99, "color": "Space Black", "memory": "18GB", "storage": "512GB"}',
 ARRAY['Electronics', 'Laptops', 'Apple'], 0.98, true),

('550e8400-e29b-41d4-a716-446655440004', 'product', 'Dell XPS 13', 'Ultrabook with Intel Core i7', 
 ARRAY['https://example.com/dellxps13.jpg'], 
 '{"brand": "Dell", "price": 1299.99, "color": "Platinum Silver", "memory": "16GB", "storage": "512GB"}',
 ARRAY['Electronics', 'Laptops', 'Dell'], 0.88, true),

('550e8400-e29b-41d4-a716-446655440005', 'product', 'Sony WH-1000XM5', 'Wireless noise-canceling headphones', 
 ARRAY['https://example.com/sonywh1000xm5.jpg'], 
 '{"brand": "Sony", "price": 399.99, "color": "Black", "battery_life": "30 hours"}',
 ARRAY['Electronics', 'Audio', 'Headphones'], 0.91, true),

-- Fashion Products
('550e8400-e29b-41d4-a716-446655440006', 'product', 'Nike Air Max 270', 'Comfortable running shoes', 
 ARRAY['https://example.com/nikeairmax270.jpg'], 
 '{"brand": "Nike", "price": 150.00, "color": "White/Black", "size": "US 10"}',
 ARRAY['Fashion', 'Shoes', 'Athletic'], 0.85, true),

('550e8400-e29b-41d4-a716-446655440007', 'product', 'Levis 501 Original Jeans', 'Classic straight-leg jeans', 
 ARRAY['https://example.com/levis501.jpg'], 
 '{"brand": "Levis", "price": 89.99, "color": "Dark Blue", "size": "32x32"}',
 ARRAY['Fashion', 'Clothing', 'Jeans'], 0.82, true),

-- Home & Garden
('550e8400-e29b-41d4-a716-446655440008', 'product', 'Dyson V15 Detect', 'Cordless vacuum cleaner with laser detection', 
 ARRAY['https://example.com/dysonv15.jpg'], 
 '{"brand": "Dyson", "price": 749.99, "color": "Yellow/Nickel", "battery_life": "60 minutes"}',
 ARRAY['Home & Garden', 'Appliances', 'Cleaning'], 0.94, true),

-- Video Content
('550e8400-e29b-41d4-a716-446655440009', 'video', 'iPhone 15 Pro Review', 'Comprehensive review of the latest iPhone', 
 ARRAY['https://example.com/iphone15review.jpg'], 
 '{"duration": 1200, "views": 150000, "likes": 12500, "channel": "TechReviewer"}',
 ARRAY['Technology', 'Reviews', 'Smartphones'], 0.89, true),

('550e8400-e29b-41d4-a716-446655440010', 'video', 'MacBook Pro M3 Unboxing', 'First look at the new MacBook Pro', 
 ARRAY['https://example.com/macbookunboxing.jpg'], 
 '{"duration": 900, "views": 89000, "likes": 7800, "channel": "UnboxingPro"}',
 ARRAY['Technology', 'Unboxing', 'Laptops'], 0.86, true),

-- Article Content
('550e8400-e29b-41d4-a716-446655440011', 'article', 'Best Smartphones of 2024', 'Comprehensive guide to top smartphones', 
 ARRAY['https://example.com/smartphones2024.jpg'], 
 '{"author": "Tech Writer", "read_time": 8, "published_date": "2024-01-15", "word_count": 2500}',
 ARRAY['Technology', 'Guides', 'Smartphones'], 0.93, true),

('550e8400-e29b-41d4-a716-446655440012', 'article', 'Fashion Trends Spring 2024', 'Latest fashion trends and styling tips', 
 ARRAY['https://example.com/fashiontrends2024.jpg'], 
 '{"author": "Fashion Expert", "read_time": 6, "published_date": "2024-02-01", "word_count": 1800}',
 ARRAY['Fashion', 'Trends', 'Style'], 0.87, true),

-- Books
('550e8400-e29b-41d4-a716-446655440013', 'product', 'The Psychology of Persuasion', 'Classic book on influence and persuasion', 
 ARRAY['https://example.com/psychologypersuasion.jpg'], 
 '{"author": "Robert Cialdini", "price": 16.99, "pages": 320, "publisher": "Harper Business"}',
 ARRAY['Books', 'Psychology', 'Business'], 0.96, true),

-- Sports & Outdoors
('550e8400-e29b-41d4-a716-446655440014', 'product', 'Patagonia Down Jacket', 'Lightweight down insulation jacket', 
 ARRAY['https://example.com/patagoniadown.jpg'], 
 '{"brand": "Patagonia", "price": 229.99, "color": "Navy Blue", "size": "Medium", "fill_power": "800"}',
 ARRAY['Sports & Outdoors', 'Clothing', 'Jackets'], 0.90, true),

-- Gaming
('550e8400-e29b-41d4-a716-446655440015', 'product', 'PlayStation 5', 'Next-generation gaming console', 
 ARRAY['https://example.com/ps5.jpg'], 
 '{"brand": "Sony", "price": 499.99, "storage": "825GB SSD", "color": "White"}',
 ARRAY['Gaming', 'Consoles', 'Sony'], 0.97, true);

\echo 'Populating user_profiles...'

-- Sample user profiles with different preference patterns
INSERT INTO user_profiles (user_id, explicit_preferences, behavior_patterns, demographics, interaction_count, last_interaction) VALUES
-- Tech enthusiast user
('123e4567-e89b-12d3-a456-426614174001', 
 '{"preferred_categories": ["Electronics", "Technology"], "price_range": {"min": 100, "max": 2000}, "brands": ["Apple", "Samsung", "Sony"]}',
 '{"avg_session_duration": 1200, "clicks_per_session": 15, "time_of_day": "evening", "device_type": "mobile"}',
 '{"age_range": "25-34", "gender": "male", "location": "San Francisco", "income_level": "high"}',
 45, NOW() - INTERVAL '2 hours'),

-- Fashion-focused user
('123e4567-e89b-12d3-a456-426614174002', 
 '{"preferred_categories": ["Fashion", "Beauty"], "price_range": {"min": 50, "max": 500}, "brands": ["Nike", "Adidas", "Levis"]}',
 '{"avg_session_duration": 900, "clicks_per_session": 20, "time_of_day": "afternoon", "device_type": "desktop"}',
 '{"age_range": "18-24", "gender": "female", "location": "New York", "income_level": "medium"}',
 32, NOW() - INTERVAL '1 day'),

-- Home improvement enthusiast
('123e4567-e89b-12d3-a456-426614174003', 
 '{"preferred_categories": ["Home & Garden", "Tools"], "price_range": {"min": 200, "max": 1000}, "brands": ["Dyson", "Black & Decker"]}',
 '{"avg_session_duration": 1800, "clicks_per_session": 8, "time_of_day": "weekend", "device_type": "tablet"}',
 '{"age_range": "35-44", "gender": "male", "location": "Chicago", "income_level": "high"}',
 28, NOW() - INTERVAL '3 days'),

-- Budget-conscious user
('123e4567-e89b-12d3-a456-426614174004', 
 '{"preferred_categories": ["Books", "Electronics"], "price_range": {"min": 10, "max": 200}, "brands": ["generic", "budget"]}',
 '{"avg_session_duration": 600, "clicks_per_session": 25, "time_of_day": "morning", "device_type": "mobile"}',
 '{"age_range": "18-24", "gender": "non-binary", "location": "Austin", "income_level": "low"}',
 67, NOW() - INTERVAL '6 hours'),

-- Gaming enthusiast
('123e4567-e89b-12d3-a456-426614174005', 
 '{"preferred_categories": ["Gaming", "Electronics"], "price_range": {"min": 50, "max": 800}, "brands": ["Sony", "Microsoft", "Nintendo"]}',
 '{"avg_session_duration": 2400, "clicks_per_session": 12, "time_of_day": "night", "device_type": "desktop"}',
 '{"age_range": "25-34", "gender": "male", "location": "Seattle", "income_level": "medium"}',
 89, NOW() - INTERVAL '4 hours'),

-- Outdoor enthusiast
('123e4567-e89b-12d3-a456-426614174006', 
 '{"preferred_categories": ["Sports & Outdoors", "Fashion"], "price_range": {"min": 100, "max": 600}, "brands": ["Patagonia", "North Face", "REI"]}',
 '{"avg_session_duration": 1500, "clicks_per_session": 10, "time_of_day": "morning", "device_type": "mobile"}',
 '{"age_range": "25-34", "gender": "female", "location": "Denver", "income_level": "high"}',
 41, NOW() - INTERVAL '1 day'),

-- New user with minimal data
('123e4567-e89b-12d3-a456-426614174007', 
 '{}',
 '{"avg_session_duration": 300, "clicks_per_session": 5, "time_of_day": "afternoon", "device_type": "mobile"}',
 '{"age_range": "35-44", "gender": "female", "location": "Miami", "income_level": "medium"}',
 3, NOW() - INTERVAL '30 minutes'),

-- Power user with diverse interests
('123e4567-e89b-12d3-a456-426614174008', 
 '{"preferred_categories": ["Electronics", "Books", "Fashion", "Home & Garden"], "price_range": {"min": 20, "max": 1500}}',
 '{"avg_session_duration": 1800, "clicks_per_session": 30, "time_of_day": "evening", "device_type": "desktop"}',
 '{"age_range": "45-54", "gender": "male", "location": "Boston", "income_level": "high"}',
 156, NOW() - INTERVAL '8 hours');

\echo 'Populating user_interactions...'

-- Generate realistic user interactions
INSERT INTO user_interactions (user_id, item_id, interaction_type, value, duration, session_id, context, timestamp) VALUES
-- Tech enthusiast interactions
('123e4567-e89b-12d3-a456-426614174001', '550e8400-e29b-41d4-a716-446655440001', 'rating', 5.0, NULL, 
 '550e8400-e29b-41d4-a716-446655440101', '{"page": "product_detail", "referrer": "search"}', NOW() - INTERVAL '2 hours'),
('123e4567-e89b-12d3-a456-426614174001', '550e8400-e29b-41d4-a716-446655440003', 'view', NULL, 180, 
 '550e8400-e29b-41d4-a716-446655440101', '{"page": "product_detail", "referrer": "recommendations"}', NOW() - INTERVAL '2 hours 15 minutes'),
('123e4567-e89b-12d3-a456-426614174001', '550e8400-e29b-41d4-a716-446655440005', 'like', 1.0, NULL, 
 '550e8400-e29b-41d4-a716-446655440101', '{"page": "product_detail", "referrer": "category"}', NOW() - INTERVAL '2 hours 30 minutes'),

-- Fashion user interactions
('123e4567-e89b-12d3-a456-426614174002', '550e8400-e29b-41d4-a716-446655440006', 'rating', 4.0, NULL, 
 '550e8400-e29b-41d4-a716-446655440102', '{"page": "product_detail", "referrer": "search"}', NOW() - INTERVAL '1 day'),
('123e4567-e89b-12d3-a456-426614174002', '550e8400-e29b-41d4-a716-446655440007', 'view', NULL, 120, 
 '550e8400-e29b-41d4-a716-446655440102', '{"page": "product_detail", "referrer": "recommendations"}', NOW() - INTERVAL '1 day 5 minutes'),
('123e4567-e89b-12d3-a456-426614174002', '550e8400-e29b-41d4-a716-446655440012', 'view', NULL, 300, 
 '550e8400-e29b-41d4-a716-446655440102', '{"page": "article", "referrer": "homepage"}', NOW() - INTERVAL '1 day 10 minutes'),

-- Home improvement user interactions
('123e4567-e89b-12d3-a456-426614174003', '550e8400-e29b-41d4-a716-446655440008', 'rating', 5.0, NULL, 
 '550e8400-e29b-41d4-a716-446655440103', '{"page": "product_detail", "referrer": "search"}', NOW() - INTERVAL '3 days'),
('123e4567-e89b-12d3-a456-426614174003', '550e8400-e29b-41d4-a716-446655440008', 'view', NULL, 240, 
 '550e8400-e29b-41d4-a716-446655440103', '{"page": "product_detail", "referrer": "category"}', NOW() - INTERVAL '3 days 10 minutes'),

-- Budget user interactions
('123e4567-e89b-12d3-a456-426614174004', '550e8400-e29b-41d4-a716-446655440013', 'rating', 4.0, NULL, 
 '550e8400-e29b-41d4-a716-446655440104', '{"page": "product_detail", "referrer": "search"}', NOW() - INTERVAL '6 hours'),
('123e4567-e89b-12d3-a456-426614174004', '550e8400-e29b-41d4-a716-446655440011', 'view', NULL, 480, 
 '550e8400-e29b-41d4-a716-446655440104', '{"page": "article", "referrer": "recommendations"}', NOW() - INTERVAL '6 hours 15 minutes'),

-- Gaming user interactions
('123e4567-e89b-12d3-a456-426614174005', '550e8400-e29b-41d4-a716-446655440015', 'rating', 5.0, NULL, 
 '550e8400-e29b-41d4-a716-446655440105', '{"page": "product_detail", "referrer": "search"}', NOW() - INTERVAL '4 hours'),
('123e4567-e89b-12d3-a456-426614174005', '550e8400-e29b-41d4-a716-446655440009', 'view', NULL, 720, 
 '550e8400-e29b-41d4-a716-446655440105', '{"page": "video", "referrer": "recommendations"}', NOW() - INTERVAL '4 hours 30 minutes'),

-- Outdoor user interactions
('123e4567-e89b-12d3-a456-426614174006', '550e8400-e29b-41d4-a716-446655440014', 'rating', 4.0, NULL, 
 '550e8400-e29b-41d4-a716-446655440106', '{"page": "product_detail", "referrer": "category"}', NOW() - INTERVAL '1 day'),
('123e8400-e29b-12d3-a456-426614174006', '550e8400-e29b-41d4-a716-446655440006', 'like', 1.0, NULL, 
 '550e8400-e29b-41d4-a716-446655440106', '{"page": "product_detail", "referrer": "recommendations"}', NOW() - INTERVAL '1 day 20 minutes'),

-- New user interactions (limited)
('123e4567-e89b-12d3-a456-426614174007', '550e8400-e29b-41d4-a716-446655440001', 'view', NULL, 60, 
 '550e8400-e29b-41d4-a716-446655440107', '{"page": "product_detail", "referrer": "homepage"}', NOW() - INTERVAL '30 minutes'),

-- Power user interactions (diverse)
('123e4567-e89b-12d3-a456-426614174008', '550e8400-e29b-41d4-a716-446655440001', 'rating', 4.0, NULL, 
 '550e8400-e29b-41d4-a716-446655440108', '{"page": "product_detail", "referrer": "search"}', NOW() - INTERVAL '8 hours'),
('123e4567-e89b-12d3-a456-426614174008', '550e8400-e29b-41d4-a716-446655440013', 'rating', 5.0, NULL, 
 '550e8400-e29b-41d4-a716-446655440108', '{"page": "product_detail", "referrer": "recommendations"}', NOW() - INTERVAL '8 hours 15 minutes'),
('123e4567-e89b-12d3-a456-426614174008', '550e8400-e29b-41d4-a716-446655440008', 'view', NULL, 200, 
 '550e8400-e29b-41d4-a716-446655440108', '{"page": "product_detail", "referrer": "category"}', NOW() - INTERVAL '8 hours 30 minutes');

\echo 'Populating content_jobs...'

-- Sample content ingestion jobs
INSERT INTO content_jobs (job_id, status, progress, total_items, processed_items, failed_items, estimated_time, error_message) VALUES
('550e8400-e29b-41d4-a716-446655440201', 'completed', 100, 15, 15, 0, 300, NULL),
('550e8400-e29b-41d4-a716-446655440202', 'processing', 75, 20, 15, 1, 120, NULL),
('550e8400-e29b-41d4-a716-446655440203', 'failed', 50, 10, 5, 5, NULL, 'Network timeout during embedding generation'),
('550e8400-e29b-41d4-a716-446655440204', 'queued', 0, 25, 0, 0, 600, NULL);

\echo 'Populating recommendation_metrics...'

-- Sample recommendation metrics for business analytics
INSERT INTO recommendation_metrics (user_id, item_id, recommendation_id, event_type, algorithm_used, position_in_list, confidence_score, session_id, context, user_tier, content_category, timestamp) VALUES
-- Tech user metrics
('123e4567-e89b-12d3-a456-426614174001', '550e8400-e29b-41d4-a716-446655440001', '550e8400-e29b-41d4-a716-446655440301', 'impression', 'semantic_search', 1, 0.95, '550e8400-e29b-41d4-a716-446655440101', '{"page": "homepage"}', 'premium', 'Electronics', NOW() - INTERVAL '2 hours'),
('123e4567-e89b-12d3-a456-426614174001', '550e8400-e29b-41d4-a716-446655440001', '550e8400-e29b-41d4-a716-446655440301', 'click', 'semantic_search', 1, 0.95, '550e8400-e29b-41d4-a716-446655440101', '{"page": "homepage"}', 'premium', 'Electronics', NOW() - INTERVAL '2 hours'),
('123e4567-e89b-12d3-a456-426614174001', '550e8400-e29b-41d4-a716-446655440001', '550e8400-e29b-41d4-a716-446655440301', 'conversion', 'semantic_search', 1, 0.95, '550e8400-e29b-41d4-a716-446655440101', '{"page": "homepage"}', 'premium', 'Electronics', NOW() - INTERVAL '1 hour 45 minutes'),

-- Fashion user metrics
('123e4567-e89b-12d3-a456-426614174002', '550e8400-e29b-41d4-a716-446655440006', '550e8400-e29b-41d4-a716-446655440302', 'impression', 'collaborative_filtering', 2, 0.87, '550e8400-e29b-41d4-a716-446655440102', '{"page": "category"}', 'free', 'Fashion', NOW() - INTERVAL '1 day'),
('123e4567-e89b-12d3-a456-426614174002', '550e8400-e29b-41d4-a716-446655440006', '550e8400-e29b-41d4-a716-446655440302', 'click', 'collaborative_filtering', 2, 0.87, '550e8400-e29b-41d4-a716-446655440102', '{"page": "category"}', 'free', 'Fashion', NOW() - INTERVAL '1 day'),

-- Gaming user metrics
('123e4567-e89b-12d3-a456-426614174005', '550e8400-e29b-41d4-a716-446655440015', '550e8400-e29b-41d4-a716-446655440303', 'impression', 'pagerank', 1, 0.92, '550e8400-e29b-41d4-a716-446655440105', '{"page": "search"}', 'premium', 'Gaming', NOW() - INTERVAL '4 hours'),
('123e4567-e89b-12d3-a456-426614174005', '550e8400-e29b-41d4-a716-446655440015', '550e8400-e29b-41d4-a716-446655440303', 'click', 'pagerank', 1, 0.92, '550e8400-e29b-41d4-a716-446655440105', '{"page": "search"}', 'premium', 'Gaming', NOW() - INTERVAL '4 hours'),
('123e4567-e89b-12d3-a456-426614174005', '550e8400-e29b-41d4-a716-446655440015', '550e8400-e29b-41d4-a716-446655440303', 'conversion', 'pagerank', 1, 0.92, '550e8400-e29b-41d4-a716-446655440105', '{"page": "search"}', 'premium', 'Gaming', NOW() - INTERVAL '3 hours 30 minutes'),

-- Additional impression/click data for analytics
('123e4567-e89b-12d3-a456-426614174003', '550e8400-e29b-41d4-a716-446655440008', '550e8400-e29b-41d4-a716-446655440304', 'impression', 'semantic_search', 1, 0.89, '550e8400-e29b-41d4-a716-446655440103', '{"page": "homepage"}', 'premium', 'Home & Garden', NOW() - INTERVAL '3 days'),
('123e4567-e89b-12d3-a456-426614174004', '550e8400-e29b-41d4-a716-446655440013', '550e8400-e29b-41d4-a716-446655440305', 'impression', 'collaborative_filtering', 3, 0.76, '550e8400-e29b-41d4-a716-446655440104', '{"page": "recommendations"}', 'free', 'Books', NOW() - INTERVAL '6 hours'),
('123e4567-e89b-12d3-a456-426614174006', '550e8400-e29b-41d4-a716-446655440014', '550e8400-e29b-41d4-a716-446655440306', 'impression', 'pagerank', 2, 0.84, '550e8400-e29b-41d4-a716-446655440106', '{"page": "category"}', 'premium', 'Sports & Outdoors', NOW() - INTERVAL '1 day');

\echo 'Generating sample embeddings...'

-- Generate sample embeddings (normally these would be generated by ML models)
-- Using random vectors for demonstration - in production these would be actual embeddings
UPDATE content_items SET embedding = (
    SELECT ARRAY(
        SELECT random()::float4 
        FROM generate_series(1, 768)
    )::vector(768)
) WHERE embedding IS NULL;

-- Generate sample user preference vectors
UPDATE user_profiles SET preference_vector = (
    SELECT ARRAY(
        SELECT (random() - 0.5)::float4 
        FROM generate_series(1, 768)
    )::vector(768)
) WHERE preference_vector IS NULL;

\echo 'Creating additional test scenarios...'

-- Add some items with missing embeddings for testing fallback scenarios
INSERT INTO content_items (id, type, title, description, categories, quality_score, active, embedding) VALUES
('550e8400-e29b-41d4-a716-446655440016', 'product', 'Test Product No Embedding', 'Product without embedding for testing', 
 ARRAY['Test'], 0.5, true, NULL);

-- Add inactive content for testing filtering
INSERT INTO content_items (id, type, title, description, categories, quality_score, active) VALUES
('550e8400-e29b-41d4-a716-446655440017', 'product', 'Inactive Product', 'This product is inactive', 
 ARRAY['Test'], 0.3, false);

\echo 'Updating interaction counts...'

-- Update user profile interaction counts based on actual interactions
UPDATE user_profiles 
SET interaction_count = (
    SELECT COUNT(*) 
    FROM user_interactions 
    WHERE user_interactions.user_id = user_profiles.user_id
),
last_interaction = (
    SELECT MAX(timestamp) 
    FROM user_interactions 
    WHERE user_interactions.user_id = user_profiles.user_id
);

\echo 'Sample data population complete!'

-- Display summary statistics
\echo 'Summary of populated data:'
SELECT 'content_items' as table_name, COUNT(*) as row_count FROM content_items
UNION ALL
SELECT 'user_profiles', COUNT(*) FROM user_profiles
UNION ALL
SELECT 'user_interactions', COUNT(*) FROM user_interactions
UNION ALL
SELECT 'recommendation_metrics', COUNT(*) FROM recommendation_metrics
UNION ALL
SELECT 'content_jobs', COUNT(*) FROM content_jobs;

\echo 'Content by type:'
SELECT type, COUNT(*) as count FROM content_items GROUP BY type;

\echo 'User interaction types:'
SELECT interaction_type, COUNT(*) as count FROM user_interactions GROUP BY interaction_type;

\echo 'Recommendation metrics by algorithm:'
SELECT algorithm_used, COUNT(*) as count FROM recommendation_metrics GROUP BY algorithm_used;