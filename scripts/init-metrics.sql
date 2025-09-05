-- Business metrics table for detailed event tracking
CREATE TABLE IF NOT EXISTS recommendation_metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    item_id UUID NOT NULL,
    recommendation_id UUID NOT NULL,
    event_type VARCHAR(50) NOT NULL CHECK (event_type IN ('impression', 'click', 'conversion', 'view', 'like', 'dislike', 'share')),
    algorithm_used VARCHAR(100) NOT NULL,
    position_in_list INTEGER,
    confidence_score FLOAT CHECK (confidence_score >= 0 AND confidence_score <= 1),
    timestamp TIMESTAMP DEFAULT NOW(),
    session_id UUID,
    context JSONB,
    user_tier VARCHAR(50) DEFAULT 'free',
    content_category VARCHAR(100),
    created_at TIMESTAMP DEFAULT NOW()
);

-- Indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_recommendation_metrics_timestamp ON recommendation_metrics(timestamp);
CREATE INDEX IF NOT EXISTS idx_recommendation_metrics_user_id ON recommendation_metrics(user_id);
CREATE INDEX IF NOT EXISTS idx_recommendation_metrics_event_type ON recommendation_metrics(event_type);
CREATE INDEX IF NOT EXISTS idx_recommendation_metrics_algorithm ON recommendation_metrics(algorithm_used);
CREATE INDEX IF NOT EXISTS idx_recommendation_metrics_session ON recommendation_metrics(session_id);
CREATE INDEX IF NOT EXISTS idx_recommendation_metrics_composite ON recommendation_metrics(timestamp, event_type, algorithm_used);

-- Aggregated metrics table for fast dashboard queries
CREATE TABLE IF NOT EXISTS daily_metrics_summary (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    date DATE NOT NULL,
    algorithm_used VARCHAR(100) NOT NULL,
    user_tier VARCHAR(50) DEFAULT 'free',
    content_category VARCHAR(100) DEFAULT 'general',
    total_recommendations INTEGER DEFAULT 0,
    total_clicks INTEGER DEFAULT 0,
    total_conversions INTEGER DEFAULT 0,
    total_views INTEGER DEFAULT 0,
    click_through_rate FLOAT DEFAULT 0,
    conversion_rate FLOAT DEFAULT 0,
    avg_confidence_score FLOAT DEFAULT 0,
    avg_position FLOAT DEFAULT 0,
    unique_users INTEGER DEFAULT 0,
    unique_sessions INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(date, algorithm_used, user_tier, content_category)
);

-- Indexes for aggregated metrics
CREATE INDEX IF NOT EXISTS idx_daily_metrics_date ON daily_metrics_summary(date);
CREATE INDEX IF NOT EXISTS idx_daily_metrics_algorithm ON daily_metrics_summary(algorithm_used);
CREATE INDEX IF NOT EXISTS idx_daily_metrics_composite ON daily_metrics_summary(date, algorithm_used, user_tier);

-- User engagement metrics table
CREATE TABLE IF NOT EXISTS user_engagement_metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    date DATE NOT NULL,
    session_count INTEGER DEFAULT 0,
    total_session_duration INTEGER DEFAULT 0, -- in seconds
    avg_session_duration FLOAT DEFAULT 0,
    total_interactions INTEGER DEFAULT 0,
    interactions_per_session FLOAT DEFAULT 0,
    unique_items_viewed INTEGER DEFAULT 0,
    categories_explored INTEGER DEFAULT 0,
    return_visits INTEGER DEFAULT 0,
    user_tier VARCHAR(50) DEFAULT 'free',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, date)
);

-- Indexes for user engagement
CREATE INDEX IF NOT EXISTS idx_user_engagement_user_id ON user_engagement_metrics(user_id);
CREATE INDEX IF NOT EXISTS idx_user_engagement_date ON user_engagement_metrics(date);
CREATE INDEX IF NOT EXISTS idx_user_engagement_tier ON user_engagement_metrics(user_tier);

-- Cohort analysis table
CREATE TABLE IF NOT EXISTS user_cohorts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cohort_month DATE NOT NULL, -- First month user was active
    period_number INTEGER NOT NULL, -- Months since first activity
    users_count INTEGER DEFAULT 0,
    active_users INTEGER DEFAULT 0,
    retention_rate FLOAT DEFAULT 0,
    avg_recommendations_per_user FLOAT DEFAULT 0,
    avg_ctr FLOAT DEFAULT 0,
    avg_conversion_rate FLOAT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(cohort_month, period_number)
);

-- Indexes for cohort analysis
CREATE INDEX IF NOT EXISTS idx_user_cohorts_month ON user_cohorts(cohort_month);
CREATE INDEX IF NOT EXISTS idx_user_cohorts_period ON user_cohorts(period_number);

-- A/B test results table
CREATE TABLE IF NOT EXISTS ab_test_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    test_name VARCHAR(200) NOT NULL,
    variant VARCHAR(100) NOT NULL,
    date DATE NOT NULL,
    user_count INTEGER DEFAULT 0,
    impressions INTEGER DEFAULT 0,
    clicks INTEGER DEFAULT 0,
    conversions INTEGER DEFAULT 0,
    ctr FLOAT DEFAULT 0,
    conversion_rate FLOAT DEFAULT 0,
    statistical_significance FLOAT DEFAULT 0,
    confidence_interval_lower FLOAT DEFAULT 0,
    confidence_interval_upper FLOAT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(test_name, variant, date)
);

-- Indexes for A/B test results
CREATE INDEX IF NOT EXISTS idx_ab_test_name ON ab_test_results(test_name);
CREATE INDEX IF NOT EXISTS idx_ab_test_date ON ab_test_results(date);
CREATE INDEX IF NOT EXISTS idx_ab_test_variant ON ab_test_results(variant);

-- Function to update daily metrics summary
CREATE OR REPLACE FUNCTION update_daily_metrics_summary()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO daily_metrics_summary (
        date, algorithm_used, user_tier, content_category,
        total_recommendations, total_clicks, total_conversions, total_views
    )
    VALUES (
        DATE(NEW.timestamp), 
        NEW.algorithm_used, 
        COALESCE(NEW.user_tier, 'free'),
        COALESCE(NEW.content_category, 'general'),
        CASE WHEN NEW.event_type = 'impression' THEN 1 ELSE 0 END,
        CASE WHEN NEW.event_type = 'click' THEN 1 ELSE 0 END,
        CASE WHEN NEW.event_type = 'conversion' THEN 1 ELSE 0 END,
        CASE WHEN NEW.event_type = 'view' THEN 1 ELSE 0 END
    )
    ON CONFLICT (date, algorithm_used, user_tier, content_category)
    DO UPDATE SET
        total_recommendations = daily_metrics_summary.total_recommendations + 
            CASE WHEN NEW.event_type = 'impression' THEN 1 ELSE 0 END,
        total_clicks = daily_metrics_summary.total_clicks + 
            CASE WHEN NEW.event_type = 'click' THEN 1 ELSE 0 END,
        total_conversions = daily_metrics_summary.total_conversions + 
            CASE WHEN NEW.event_type = 'conversion' THEN 1 ELSE 0 END,
        total_views = daily_metrics_summary.total_views + 
            CASE WHEN NEW.event_type = 'view' THEN 1 ELSE 0 END,
        click_through_rate = CASE 
            WHEN daily_metrics_summary.total_recommendations + 
                 CASE WHEN NEW.event_type = 'impression' THEN 1 ELSE 0 END > 0 
            THEN (daily_metrics_summary.total_clicks + 
                  CASE WHEN NEW.event_type = 'click' THEN 1 ELSE 0 END)::FLOAT / 
                 (daily_metrics_summary.total_recommendations + 
                  CASE WHEN NEW.event_type = 'impression' THEN 1 ELSE 0 END) * 100
            ELSE 0 
        END,
        conversion_rate = CASE 
            WHEN daily_metrics_summary.total_clicks + 
                 CASE WHEN NEW.event_type = 'click' THEN 1 ELSE 0 END > 0 
            THEN (daily_metrics_summary.total_conversions + 
                  CASE WHEN NEW.event_type = 'conversion' THEN 1 ELSE 0 END)::FLOAT / 
                 (daily_metrics_summary.total_clicks + 
                  CASE WHEN NEW.event_type = 'click' THEN 1 ELSE 0 END) * 100
            ELSE 0 
        END,
        updated_at = NOW();
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to automatically update daily metrics
CREATE TRIGGER trigger_update_daily_metrics
    AFTER INSERT ON recommendation_metrics
    FOR EACH ROW
    EXECUTE FUNCTION update_daily_metrics_summary();

-- Function to calculate cohort retention
CREATE OR REPLACE FUNCTION calculate_cohort_retention()
RETURNS void AS $$
DECLARE
    cohort_record RECORD;
    period_record RECORD;
BEGIN
    -- Clear existing cohort data for recalculation
    DELETE FROM user_cohorts;
    
    -- Calculate cohorts based on first activity month
    FOR cohort_record IN
        SELECT 
            DATE_TRUNC('month', MIN(timestamp)) as cohort_month,
            COUNT(DISTINCT user_id) as cohort_size
        FROM recommendation_metrics
        GROUP BY DATE_TRUNC('month', MIN(timestamp))
    LOOP
        -- For each period (month) after cohort month
        FOR period_record IN
            SELECT 
                EXTRACT(YEAR FROM age(DATE_TRUNC('month', timestamp), cohort_record.cohort_month)) * 12 +
                EXTRACT(MONTH FROM age(DATE_TRUNC('month', timestamp), cohort_record.cohort_month)) as period_number,
                COUNT(DISTINCT rm.user_id) as active_users,
                AVG(CASE WHEN event_type = 'impression' THEN 1 ELSE 0 END) as avg_recommendations,
                AVG(CASE WHEN event_type = 'impression' THEN 
                    CASE WHEN event_type = 'click' THEN 1 ELSE 0 END ELSE NULL END) as avg_ctr
            FROM recommendation_metrics rm
            WHERE rm.user_id IN (
                SELECT DISTINCT user_id 
                FROM recommendation_metrics 
                WHERE DATE_TRUNC('month', timestamp) = cohort_record.cohort_month
            )
            AND DATE_TRUNC('month', timestamp) >= cohort_record.cohort_month
            GROUP BY DATE_TRUNC('month', timestamp)
        LOOP
            INSERT INTO user_cohorts (
                cohort_month, period_number, users_count, active_users, retention_rate,
                avg_recommendations_per_user, avg_ctr
            ) VALUES (
                cohort_record.cohort_month,
                period_record.period_number,
                cohort_record.cohort_size,
                period_record.active_users,
                period_record.active_users::FLOAT / cohort_record.cohort_size * 100,
                period_record.avg_recommendations,
                period_record.avg_ctr
            );
        END LOOP;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

-- Create a scheduled job to run cohort analysis daily (requires pg_cron extension)
-- SELECT cron.schedule('cohort-analysis', '0 2 * * *', 'SELECT calculate_cohort_retention();');