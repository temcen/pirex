-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Update content_items table to set proper vector dimension
ALTER TABLE content_items ALTER COLUMN embedding TYPE VECTOR(768);

-- Update user_profiles table to set proper vector dimension  
ALTER TABLE user_profiles ALTER COLUMN preference_vector TYPE VECTOR(768);

-- Create vector indexes for similarity search
-- Using HNSW index for fast approximate nearest neighbor search
CREATE INDEX idx_content_items_embedding_hnsw ON content_items 
USING hnsw (embedding vector_cosine_ops) 
WITH (m = 16, ef_construction = 64);

CREATE INDEX idx_user_profiles_preference_vector_hnsw ON user_profiles 
USING hnsw (preference_vector vector_cosine_ops) 
WITH (m = 16, ef_construction = 64);

-- Create IVFFlat indexes as alternative for exact search
CREATE INDEX idx_content_items_embedding_ivfflat ON content_items 
USING ivfflat (embedding vector_cosine_ops) 
WITH (lists = 100);

CREATE INDEX idx_user_profiles_preference_vector_ivfflat ON user_profiles 
USING ivfflat (preference_vector vector_cosine_ops) 
WITH (lists = 100);

-- Create function for cosine similarity search
CREATE OR REPLACE FUNCTION find_similar_content(
    query_embedding VECTOR(768),
    similarity_threshold FLOAT DEFAULT 0.7,
    result_limit INTEGER DEFAULT 100
)
RETURNS TABLE (
    content_id UUID,
    similarity_score FLOAT
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        ci.id as content_id,
        (1 - (ci.embedding <=> query_embedding)) as similarity_score
    FROM content_items ci
    WHERE ci.active = true
        AND ci.embedding IS NOT NULL
        AND (1 - (ci.embedding <=> query_embedding)) >= similarity_threshold
    ORDER BY ci.embedding <=> query_embedding
    LIMIT result_limit;
END;
$$ LANGUAGE plpgsql;

-- Create function for user similarity search
CREATE OR REPLACE FUNCTION find_similar_users(
    query_vector VECTOR(768),
    similarity_threshold FLOAT DEFAULT 0.5,
    result_limit INTEGER DEFAULT 50
)
RETURNS TABLE (
    user_id UUID,
    similarity_score FLOAT
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        up.user_id,
        (1 - (up.preference_vector <=> query_vector)) as similarity_score
    FROM user_profiles up
    WHERE up.preference_vector IS NOT NULL
        AND (1 - (up.preference_vector <=> query_vector)) >= similarity_threshold
    ORDER BY up.preference_vector <=> query_vector
    LIMIT result_limit;
END;
$$ LANGUAGE plpgsql;