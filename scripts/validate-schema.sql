-- Database Schema Validation Script
-- This script validates that the database schema matches the expected entity-relationship diagram

\echo 'Starting database schema validation...'

-- Check if required extensions are installed
\echo 'Checking required extensions...'
SELECT 
    extname as extension_name,
    extversion as version
FROM pg_extension 
WHERE extname IN ('uuid-ossp', 'vector');

-- Validate content_items table structure
\echo 'Validating content_items table...'
SELECT 
    column_name,
    data_type,
    is_nullable,
    column_default,
    character_maximum_length
FROM information_schema.columns 
WHERE table_name = 'content_items' 
ORDER BY ordinal_position;

-- Check content_items constraints
SELECT 
    conname as constraint_name,
    contype as constraint_type,
    pg_get_constraintdef(oid) as constraint_definition
FROM pg_constraint 
WHERE conrelid = 'content_items'::regclass;

-- Validate user_profiles table structure
\echo 'Validating user_profiles table...'
SELECT 
    column_name,
    data_type,
    is_nullable,
    column_default
FROM information_schema.columns 
WHERE table_name = 'user_profiles' 
ORDER BY ordinal_position;

-- Validate user_interactions table structure
\echo 'Validating user_interactions table...'
SELECT 
    column_name,
    data_type,
    is_nullable,
    column_default
FROM information_schema.columns 
WHERE table_name = 'user_interactions' 
ORDER BY ordinal_position;

-- Check foreign key relationships
\echo 'Validating foreign key relationships...'
SELECT 
    tc.table_name,
    kcu.column_name,
    ccu.table_name AS foreign_table_name,
    ccu.column_name AS foreign_column_name,
    tc.constraint_name
FROM information_schema.table_constraints AS tc 
JOIN information_schema.key_column_usage AS kcu
    ON tc.constraint_name = kcu.constraint_name
    AND tc.table_schema = kcu.table_schema
JOIN information_schema.constraint_column_usage AS ccu
    ON ccu.constraint_name = tc.constraint_name
    AND ccu.table_schema = tc.table_schema
WHERE tc.constraint_type = 'FOREIGN KEY'
    AND tc.table_name IN ('user_interactions', 'recommendation_metrics');

-- Validate indexes exist
\echo 'Validating indexes...'
SELECT 
    schemaname,
    tablename,
    indexname,
    indexdef
FROM pg_indexes 
WHERE tablename IN ('content_items', 'user_profiles', 'user_interactions', 'recommendation_metrics')
ORDER BY tablename, indexname;

-- Check vector indexes specifically
\echo 'Validating vector indexes...'
SELECT 
    indexname,
    indexdef
FROM pg_indexes 
WHERE indexdef LIKE '%vector%' OR indexdef LIKE '%hnsw%' OR indexdef LIKE '%ivfflat%';

-- Validate recommendation_metrics table
\echo 'Validating recommendation_metrics table...'
SELECT 
    column_name,
    data_type,
    is_nullable,
    column_default
FROM information_schema.columns 
WHERE table_name = 'recommendation_metrics' 
ORDER BY ordinal_position;

-- Validate daily_metrics_summary table
\echo 'Validating daily_metrics_summary table...'
SELECT 
    column_name,
    data_type,
    is_nullable,
    column_default
FROM information_schema.columns 
WHERE table_name = 'daily_metrics_summary' 
ORDER BY ordinal_position;

-- Validate content_jobs table
\echo 'Validating content_jobs table...'
SELECT 
    column_name,
    data_type,
    is_nullable,
    column_default
FROM information_schema.columns 
WHERE table_name = 'content_jobs' 
ORDER BY ordinal_position;

-- Check triggers
\echo 'Validating triggers...'
SELECT 
    trigger_name,
    event_manipulation,
    event_object_table,
    action_statement
FROM information_schema.triggers
WHERE event_object_table IN ('content_items', 'user_profiles', 'content_jobs', 'recommendation_metrics');

-- Check functions
\echo 'Validating custom functions...'
SELECT 
    routine_name,
    routine_type,
    data_type as return_type
FROM information_schema.routines
WHERE routine_schema = 'public' 
    AND routine_name IN ('find_similar_content', 'find_similar_users', 'update_updated_at_column', 'update_daily_metrics_summary');

-- Validate views
\echo 'Validating views...'
SELECT 
    table_name,
    view_definition
FROM information_schema.views
WHERE table_schema = 'public'
    AND table_name IN ('active_content', 'recent_jobs', 'content_quality_stats');

-- Check table sizes and row counts
\echo 'Checking table statistics...'
SELECT 
    schemaname,
    tablename,
    attname as column_name,
    n_distinct,
    most_common_vals,
    most_common_freqs
FROM pg_stats 
WHERE tablename IN ('content_items', 'user_profiles', 'user_interactions', 'recommendation_metrics')
    AND schemaname = 'public'
ORDER BY tablename, attname;

-- Summary of validation
\echo 'Schema validation complete!'
\echo 'Expected tables: content_items, user_profiles, user_interactions, recommendation_metrics, daily_metrics_summary, content_jobs'
\echo 'Expected indexes: Vector indexes (HNSW/IVFFlat), performance indexes on common query columns'
\echo 'Expected functions: find_similar_content, find_similar_users, update_updated_at_column'
\echo 'Expected triggers: Auto-update timestamps, metrics aggregation'