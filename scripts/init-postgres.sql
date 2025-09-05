-- Create database and user
CREATE DATABASE recommendations;

-- Connect to the recommendations database
\c recommendations;

-- Create UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create content_items table
CREATE TABLE content_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    type VARCHAR(50) NOT NULL CHECK (type IN ('product', 'video', 'article')),
    title VARCHAR(255) NOT NULL,
    description TEXT,
    image_urls TEXT[],
    metadata JSONB DEFAULT '{}',
    categories TEXT[],
    embedding VECTOR(768), -- Will be set after pgvector extension
    quality_score FLOAT DEFAULT 0.0 CHECK (quality_score >= 0.0 AND quality_score <= 1.0),
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Create user_profiles table
CREATE TABLE user_profiles (
    user_id UUID PRIMARY KEY,
    preference_vector VECTOR(768), -- Will be set after pgvector extension
    explicit_preferences JSONB DEFAULT '{}',
    behavior_patterns JSONB DEFAULT '{}',
    demographics JSONB DEFAULT '{}',
    interaction_count INTEGER DEFAULT 0,
    last_interaction TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Create user_interactions table
CREATE TABLE user_interactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL,
    item_id UUID,
    interaction_type VARCHAR(50) NOT NULL,
    value FLOAT,
    duration INTEGER, -- seconds
    query TEXT,
    session_id UUID NOT NULL,
    context JSONB DEFAULT '{}',
    timestamp TIMESTAMP DEFAULT NOW(),
    
    FOREIGN KEY (item_id) REFERENCES content_items(id) ON DELETE SET NULL
);

-- Create recommendation_metrics table for business analytics
CREATE TABLE recommendation_metrics (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL,
    item_id UUID,
    recommendation_id UUID NOT NULL,
    event_type VARCHAR(50) NOT NULL CHECK (event_type IN ('impression', 'click', 'conversion')),
    algorithm_used VARCHAR(100) NOT NULL,
    position_in_list INTEGER,
    confidence_score FLOAT,
    timestamp TIMESTAMP DEFAULT NOW(),
    session_id UUID,
    context JSONB DEFAULT '{}',
    
    FOREIGN KEY (item_id) REFERENCES content_items(id) ON DELETE SET NULL
);

-- Create daily_metrics_summary table for aggregated analytics
CREATE TABLE daily_metrics_summary (
    date DATE PRIMARY KEY,
    total_recommendations INTEGER DEFAULT 0,
    total_clicks INTEGER DEFAULT 0,
    total_conversions INTEGER DEFAULT 0,
    click_through_rate FLOAT DEFAULT 0.0,
    conversion_rate FLOAT DEFAULT 0.0,
    avg_confidence_score FLOAT DEFAULT 0.0,
    algorithm_performance JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW()
);

-- Create content_jobs table for tracking ingestion jobs
CREATE TABLE content_jobs (
    job_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    status VARCHAR(20) NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'processing', 'completed', 'failed')),
    progress INTEGER DEFAULT 0 CHECK (progress >= 0 AND progress <= 100),
    total_items INTEGER DEFAULT 0,
    processed_items INTEGER DEFAULT 0,
    failed_items INTEGER DEFAULT 0,
    estimated_time INTEGER, -- seconds
    error_message TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Create indexes for performance
CREATE INDEX idx_content_items_type ON content_items(type);
CREATE INDEX idx_content_items_categories ON content_items USING GIN(categories);
CREATE INDEX idx_content_items_active ON content_items(active);
CREATE INDEX idx_content_items_created_at ON content_items(created_at);

CREATE INDEX idx_user_interactions_user_id ON user_interactions(user_id);
CREATE INDEX idx_user_interactions_item_id ON user_interactions(item_id);
CREATE INDEX idx_user_interactions_type ON user_interactions(interaction_type);
CREATE INDEX idx_user_interactions_timestamp ON user_interactions(timestamp);
CREATE INDEX idx_user_interactions_session_id ON user_interactions(session_id);

CREATE INDEX idx_recommendation_metrics_user_id ON recommendation_metrics(user_id);
CREATE INDEX idx_recommendation_metrics_item_id ON recommendation_metrics(item_id);
CREATE INDEX idx_recommendation_metrics_event_type ON recommendation_metrics(event_type);
CREATE INDEX idx_recommendation_metrics_timestamp ON recommendation_metrics(timestamp);
CREATE INDEX idx_recommendation_metrics_algorithm ON recommendation_metrics(algorithm_used);

-- Create triggers for updated_at timestamps
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_content_items_updated_at BEFORE UPDATE ON content_items
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_user_profiles_updated_at BEFORE UPDATE ON user_profiles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_content_jobs_updated_at BEFORE UPDATE ON content_jobs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();