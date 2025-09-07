-- Data Integrity Validation Script
-- This script validates data integrity and relationships between database tables

\echo 'Starting data integrity validation...'

-- Check for orphaned records
\echo 'Checking for orphaned user_interactions...'
SELECT 
    'Orphaned user_interactions (invalid item_id)' as check_name,
    COUNT(*) as count
FROM user_interactions ui
LEFT JOIN content_items ci ON ui.item_id = ci.id
WHERE ui.item_id IS NOT NULL AND ci.id IS NULL;

-- Check for missing embeddings
\echo 'Checking embedding coverage...'
SELECT 
    'Content items missing embeddings' as check_name,
    type,
    COUNT(*) as total_items,
    COUNT(embedding) as items_with_embeddings,
    COUNT(*) - COUNT(embedding) as missing_embeddings,
    ROUND(COUNT(embedding)::numeric / COUNT(*) * 100, 2) as coverage_percentage
FROM content_items 
GROUP BY type
ORDER BY coverage_percentage;

-- Check user profile consistency
\echo 'Checking user profile consistency...'
SELECT 
    'User profile interaction count consistency' as check_name,
    up.user_id,
    up.interaction_count as profile_count,
    COUNT(ui.id) as actual_interactions,
    up.interaction_count - COUNT(ui.id) as difference
FROM user_profiles up
LEFT JOIN user_interactions ui ON up.user_id = ui.user_id
GROUP BY up.user_id, up.interaction_count
HAVING up.interaction_count != COUNT(ui.id)
ORDER BY ABS(up.interaction_count - COUNT(ui.id)) DESC;

-- Check last interaction timestamp consistency
\echo 'Checking last interaction timestamp consistency...'
SELECT 
    'Last interaction timestamp consistency' as check_name,
    up.user_id,
    up.last_interaction as profile_last_interaction,
    MAX(ui.timestamp) as actual_last_interaction,
    up.last_interaction - MAX(ui.timestamp) as time_difference
FROM user_profiles up
LEFT JOIN user_interactions ui ON up.user_id = ui.user_id
GROUP BY up.user_id, up.last_interaction
HAVING up.last_interaction IS DISTINCT FROM MAX(ui.timestamp)
ORDER BY ABS(EXTRACT(EPOCH FROM (up.last_interaction - MAX(ui.timestamp)))) DESC;

-- Check recommendation metrics consistency
\echo 'Checking recommendation metrics data quality...'
SELECT 
    'Recommendation metrics with invalid item references' as check_name,
    COUNT(*) as count
FROM recommendation_metrics rm
LEFT JOIN content_items ci ON rm.item_id = ci.id
WHERE rm.item_id IS NOT NULL AND ci.id IS NULL;

-- Check for invalid confidence scores
SELECT 
    'Invalid confidence scores (outside 0-1 range)' as check_name,
    COUNT(*) as count
FROM recommendation_metrics
WHERE confidence_score < 0 OR confidence_score > 1;

-- Check for invalid quality scores
SELECT 
    'Invalid quality scores (outside 0-1 range)' as check_name,
    COUNT(*) as count
FROM content_items
WHERE quality_score < 0 OR quality_score > 1;

-- Check vector dimensions
\echo 'Checking vector dimensions...'
SELECT 
    'Content items with incorrect embedding dimensions' as check_name,
    COUNT(*) as count
FROM content_items
WHERE embedding IS NOT NULL 
    AND array_length(embedding::float4[], 1) != 768;

SELECT 
    'User profiles with incorrect preference vector dimensions' as check_name,
    COUNT(*) as count
FROM user_profiles
WHERE preference_vector IS NOT NULL 
    AND array_length(preference_vector::float4[], 1) != 768;

-- Check for duplicate content
\echo 'Checking for potential duplicate content...'
SELECT 
    'Potential duplicate content (same title)' as check_name,
    title,
    COUNT(*) as duplicate_count
FROM content_items
GROUP BY title
HAVING COUNT(*) > 1
ORDER BY duplicate_count DESC;

-- Check interaction value ranges
\echo 'Checking interaction value ranges...'
SELECT 
    'Invalid rating values (outside 1-5 range)' as check_name,
    COUNT(*) as count
FROM user_interactions
WHERE interaction_type = 'rating' 
    AND (value < 1 OR value > 5);

-- Check session consistency
\echo 'Checking session data consistency...'
SELECT 
    'Sessions with inconsistent user_ids' as check_name,
    session_id,
    COUNT(DISTINCT user_id) as unique_users
FROM user_interactions
GROUP BY session_id
HAVING COUNT(DISTINCT user_id) > 1
ORDER BY unique_users DESC;

-- Check timestamp consistency
\echo 'Checking timestamp consistency...'
SELECT 
    'Future timestamps' as check_name,
    COUNT(*) as count
FROM (
    SELECT timestamp FROM user_interactions WHERE timestamp > NOW()
    UNION ALL
    SELECT timestamp FROM recommendation_metrics WHERE timestamp > NOW()
    UNION ALL
    SELECT created_at FROM content_items WHERE created_at > NOW()
    UNION ALL
    SELECT created_at FROM user_profiles WHERE created_at > NOW()
) future_timestamps;

-- Check content job consistency
\echo 'Checking content job data consistency...'
SELECT 
    'Content jobs with inconsistent progress' as check_name,
    id,
    status,
    progress,
    processed_items,
    total_items,
    CASE 
        WHEN total_items > 0 THEN ROUND(processed_items::numeric / total_items * 100, 2)
        ELSE 0
    END as calculated_progress
FROM content_jobs
WHERE total_items > 0 
    AND progress != ROUND(processed_items::numeric / total_items * 100, 2)
ORDER BY ABS(progress - ROUND(processed_items::numeric / total_items * 100, 2)) DESC;

-- Check for negative values where they shouldn't exist
\echo 'Checking for invalid negative values...'
SELECT 
    'Negative interaction counts' as check_name,
    COUNT(*) as count
FROM user_profiles
WHERE interaction_count < 0;

SELECT 
    'Negative durations' as check_name,
    COUNT(*) as count
FROM user_interactions
WHERE duration < 0;

SELECT 
    'Negative processed items' as check_name,
    COUNT(*) as count
FROM content_jobs
WHERE processed_items < 0 OR failed_items < 0 OR total_items < 0;

-- Check category consistency
\echo 'Checking category data...'
SELECT 
    'Content items with empty categories' as check_name,
    COUNT(*) as count
FROM content_items
WHERE categories IS NULL OR array_length(categories, 1) IS NULL;

-- Check metadata structure
\echo 'Checking metadata structure...'
SELECT 
    'Content items with invalid metadata JSON' as check_name,
    COUNT(*) as count
FROM content_items
WHERE metadata IS NOT NULL 
    AND NOT (metadata ? 'brand' OR metadata ? 'price' OR metadata ? 'author' OR metadata ? 'duration');

-- Summary statistics
\echo 'Data summary statistics...'
SELECT 
    'Total content items' as metric,
    COUNT(*) as value
FROM content_items
UNION ALL
SELECT 
    'Active content items',
    COUNT(*)
FROM content_items
WHERE active = true
UNION ALL
SELECT 
    'Total user profiles',
    COUNT(*)
FROM user_profiles
UNION ALL
SELECT 
    'Total user interactions',
    COUNT(*)
FROM user_interactions
UNION ALL
SELECT 
    'Total recommendation metrics',
    COUNT(*)
FROM recommendation_metrics
UNION ALL
SELECT 
    'Completed content jobs',
    COUNT(*)
FROM content_jobs
WHERE status = 'completed';

-- Performance indicators
\echo 'Performance indicators...'
SELECT 
    'Average interactions per user' as metric,
    ROUND(AVG(interaction_count), 2) as value
FROM user_profiles
UNION ALL
SELECT 
    'Average content quality score',
    ROUND(AVG(quality_score), 3)
FROM content_items
UNION ALL
SELECT 
    'Average confidence score',
    ROUND(AVG(confidence_score), 3)
FROM recommendation_metrics
WHERE confidence_score IS NOT NULL;

\echo 'Data integrity validation complete!'
\echo 'Review any non-zero counts above as they may indicate data quality issues.'