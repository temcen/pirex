-- Content Ingestion and Processing Pipeline Database Schema

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "vector";

-- Content items table with vector embeddings
CREATE TABLE IF NOT EXISTS content_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    type VARCHAR(50) NOT NULL CHECK (type IN ('product', 'video', 'article')),
    title VARCHAR(255) NOT NULL,
    description TEXT,
    image_urls TEXT[] DEFAULT '{}',
    metadata JSONB DEFAULT '{}',
    categories TEXT[] DEFAULT '{}',
    embedding vector(768), -- 768 dimensions as specified in design
    quality_score FLOAT NOT NULL DEFAULT 0.0 CHECK (quality_score >= 0.0 AND quality_score <= 1.0),
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Content processing jobs table
CREATE TABLE IF NOT EXISTS content_jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    status VARCHAR(50) NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'processing', 'completed', 'failed', 'cancelled')),
    progress INTEGER NOT NULL DEFAULT 0 CHECK (progress >= 0 AND progress <= 100),
    total_items INTEGER NOT NULL DEFAULT 0,
    processed_items INTEGER NOT NULL DEFAULT 0,
    failed_items INTEGER NOT NULL DEFAULT 0,
    estimated_time INTEGER, -- seconds
    error_message TEXT,
    details JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Indexes for performance

-- Content items indexes
CREATE INDEX IF NOT EXISTS idx_content_items_type ON content_items(type);
CREATE INDEX IF NOT EXISTS idx_content_items_active ON content_items(active);
CREATE INDEX IF NOT EXISTS idx_content_items_quality_score ON content_items(quality_score DESC);
CREATE INDEX IF NOT EXISTS idx_content_items_created_at ON content_items(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_content_items_categories ON content_items USING GIN(categories);
CREATE INDEX IF NOT EXISTS idx_content_items_metadata ON content_items USING GIN(metadata);

-- Vector similarity search index (HNSW for fast approximate nearest neighbor search)
CREATE INDEX IF NOT EXISTS idx_content_items_embedding_hnsw ON content_items 
USING hnsw (embedding vector_cosine_ops) 
WITH (m = 16, ef_construction = 64);

-- Alternative IVFFlat index for exact search (commented out, use HNSW for better performance)
-- CREATE INDEX IF NOT EXISTS idx_content_items_embedding_ivfflat ON content_items 
-- USING ivfflat (embedding vector_cosine_ops) 
-- WITH (lists = 100);

-- Content jobs indexes
CREATE INDEX IF NOT EXISTS idx_content_jobs_status ON content_jobs(status);
CREATE INDEX IF NOT EXISTS idx_content_jobs_created_at ON content_jobs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_content_jobs_updated_at ON content_jobs(updated_at DESC);

-- Composite indexes for common queries
CREATE INDEX IF NOT EXISTS idx_content_items_type_active ON content_items(type, active);
CREATE INDEX IF NOT EXISTS idx_content_items_active_quality ON content_items(active, quality_score DESC);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers to automatically update updated_at
CREATE TRIGGER update_content_items_updated_at 
    BEFORE UPDATE ON content_items 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_content_jobs_updated_at 
    BEFORE UPDATE ON content_jobs 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Sample data for testing (optional)
-- INSERT INTO content_items (type, title, description, categories, quality_score) VALUES
-- ('product', 'Sample Product', 'This is a sample product for testing', ARRAY['Electronics', 'Gadgets'], 0.8),
-- ('video', 'Sample Video', 'This is a sample video for testing', ARRAY['Entertainment', 'Education'], 0.9),
-- ('article', 'Sample Article', 'This is a sample article for testing', ARRAY['Technology', 'News'], 0.7);

-- Views for common queries

-- Active content with quality scores
CREATE OR REPLACE VIEW active_content AS
SELECT 
    id, type, title, description, image_urls, categories, 
    quality_score, created_at, updated_at
FROM content_items 
WHERE active = true
ORDER BY quality_score DESC, created_at DESC;

-- Recent jobs with status
CREATE OR REPLACE VIEW recent_jobs AS
SELECT 
    id, status, progress, total_items, processed_items, failed_items,
    estimated_time, error_message, created_at, updated_at,
    CASE 
        WHEN status = 'processing' AND estimated_time IS NOT NULL 
        THEN created_at + (estimated_time || ' seconds')::INTERVAL
        ELSE NULL 
    END as estimated_completion
FROM content_jobs 
WHERE created_at > NOW() - INTERVAL '7 days'
ORDER BY created_at DESC;

-- Content quality statistics
CREATE OR REPLACE VIEW content_quality_stats AS
SELECT 
    type,
    COUNT(*) as total_items,
    COUNT(*) FILTER (WHERE active = true) as active_items,
    AVG(quality_score) as avg_quality_score,
    MIN(quality_score) as min_quality_score,
    MAX(quality_score) as max_quality_score,
    COUNT(*) FILTER (WHERE quality_score >= 0.8) as high_quality_items,
    COUNT(*) FILTER (WHERE quality_score < 0.5) as low_quality_items
FROM content_items 
GROUP BY type;

-- Grant permissions (adjust as needed for your setup)
-- GRANT SELECT, INSERT, UPDATE, DELETE ON content_items TO recommendation_engine;
-- GRANT SELECT, INSERT, UPDATE, DELETE ON content_jobs TO recommendation_engine;
-- GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO recommendation_engine;