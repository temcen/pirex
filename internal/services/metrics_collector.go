package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// MetricEvent represents a business metric event
type MetricEvent struct {
	UserID           string                 `json:"user_id"`
	ItemID           string                 `json:"item_id"`
	RecommendationID string                 `json:"recommendation_id"`
	EventType        string                 `json:"event_type"` // 'impression', 'click', 'conversion'
	AlgorithmUsed    string                 `json:"algorithm_used"`
	PositionInList   int                    `json:"position_in_list"`
	ConfidenceScore  float64                `json:"confidence_score"`
	Timestamp        time.Time              `json:"timestamp"`
	SessionID        string                 `json:"session_id"`
	Context          map[string]interface{} `json:"context"`
	UserTier         string                 `json:"user_tier,omitempty"`
	ContentCategory  string                 `json:"content_category,omitempty"`
}

// BusinessMetrics holds aggregated business metrics
type BusinessMetrics struct {
	Date                 time.Time                   `json:"date"`
	TotalRecommendations int                         `json:"total_recommendations"`
	TotalClicks          int                         `json:"total_clicks"`
	TotalConversions     int                         `json:"total_conversions"`
	ClickThroughRate     float64                     `json:"click_through_rate"`
	ConversionRate       float64                     `json:"conversion_rate"`
	AvgConfidenceScore   float64                     `json:"avg_confidence_score"`
	AlgorithmPerformance map[string]AlgorithmMetrics `json:"algorithm_performance"`
}

// AlgorithmMetrics holds performance metrics for a specific algorithm
type AlgorithmMetrics struct {
	Impressions    int     `json:"impressions"`
	Clicks         int     `json:"clicks"`
	Conversions    int     `json:"conversions"`
	CTR            float64 `json:"ctr"`
	ConversionRate float64 `json:"conversion_rate"`
	AvgConfidence  float64 `json:"avg_confidence"`
	AvgPosition    float64 `json:"avg_position"`
}

// MetricsCollector handles business metrics collection and aggregation
type MetricsCollector struct {
	db           *pgxpool.Pool
	buffer       chan MetricEvent
	batchSize    int
	flushTimeout time.Duration
	mu           sync.RWMutex

	// Prometheus metrics
	recommendationRequests prometheus.Counter
	recommendationLatency  prometheus.Histogram
	algorithmPerformance   *prometheus.GaugeVec
	cacheHitRatio          *prometheus.GaugeVec
	dbConnectionPool       prometheus.Gauge

	// Business metrics
	clickThroughRate *prometheus.GaugeVec
	conversionRate   *prometheus.GaugeVec
	userEngagement   *prometheus.GaugeVec
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(db *pgxpool.Pool) *MetricsCollector {
	mc := &MetricsCollector{
		db:           db,
		buffer:       make(chan MetricEvent, 10000), // Large buffer for high throughput
		batchSize:    100,
		flushTimeout: 5 * time.Second,

		// Initialize Prometheus metrics
		recommendationRequests: promauto.NewCounter(prometheus.CounterOpts{
			Name: "recommendation_requests_total",
			Help: "Total number of recommendation requests",
		}),

		recommendationLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "recommendation_latency_seconds",
			Help:    "Recommendation request latency in seconds",
			Buckets: []float64{0.1, 0.5, 1.0, 2.0, 5.0},
		}),

		algorithmPerformance: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "algorithm_performance_score",
			Help: "Performance score by algorithm",
		}, []string{"algorithm"}),

		cacheHitRatio: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "cache_hit_ratio",
			Help: "Cache hit ratio by cache type",
		}, []string{"cache_type"}),

		dbConnectionPool: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "database_connection_pool_usage",
			Help: "Database connection pool usage percentage",
		}),

		clickThroughRate: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "click_through_rate",
			Help: "Click-through rate by algorithm and user segment",
		}, []string{"algorithm", "user_tier", "content_category"}),

		conversionRate: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "conversion_rate",
			Help: "Conversion rate by algorithm and user segment",
		}, []string{"algorithm", "user_tier", "content_category"}),

		userEngagement: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "user_engagement_score",
			Help: "User engagement metrics",
		}, []string{"metric_type", "user_tier"}),
	}

	// Start background processors
	go mc.processBatch()
	go mc.updateAggregatedMetrics()

	return mc
}

// RecordEvent records a business metric event
func (mc *MetricsCollector) RecordEvent(event MetricEvent) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	select {
	case mc.buffer <- event:
		// Event buffered successfully
	default:
		// Buffer full, log warning but don't block
		log.Printf("Warning: Metrics buffer full, dropping event: %+v", event)
	}
}

// RecordRecommendationRequest records a recommendation request for Prometheus
func (mc *MetricsCollector) RecordRecommendationRequest() {
	mc.recommendationRequests.Inc()
}

// RecordRecommendationLatency records recommendation latency
func (mc *MetricsCollector) RecordRecommendationLatency(duration time.Duration) {
	mc.recommendationLatency.Observe(duration.Seconds())
}

// UpdateAlgorithmPerformance updates algorithm performance metrics
func (mc *MetricsCollector) UpdateAlgorithmPerformance(algorithm string, score float64) {
	mc.algorithmPerformance.WithLabelValues(algorithm).Set(score)
}

// UpdateCacheHitRatio updates cache hit ratio metrics
func (mc *MetricsCollector) UpdateCacheHitRatio(cacheType string, ratio float64) {
	mc.cacheHitRatio.WithLabelValues(cacheType).Set(ratio)
}

// UpdateDBConnectionPoolUsage updates database connection pool usage
func (mc *MetricsCollector) UpdateDBConnectionPoolUsage(usage float64) {
	mc.dbConnectionPool.Set(usage)
}

// processBatch processes events in batches
func (mc *MetricsCollector) processBatch() {
	ticker := time.NewTicker(mc.flushTimeout)
	defer ticker.Stop()

	events := make([]MetricEvent, 0, mc.batchSize)

	for {
		select {
		case event := <-mc.buffer:
			events = append(events, event)

			if len(events) >= mc.batchSize {
				mc.insertBatch(events)
				events = events[:0] // Reset slice
			}

		case <-ticker.C:
			if len(events) > 0 {
				mc.insertBatch(events)
				events = events[:0] // Reset slice
			}
		}
	}
}

// insertBatch inserts a batch of events into the database
func (mc *MetricsCollector) insertBatch(events []MetricEvent) {
	if len(events) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tx, err := mc.db.Begin(ctx)
	if err != nil {
		log.Printf("Error starting transaction for metrics batch: %v", err)
		return
	}
	defer tx.Rollback(ctx)

	for _, event := range events {
		contextJSON, _ := json.Marshal(event.Context)

		_, err := tx.Exec(ctx, `
			INSERT INTO recommendation_metrics (
				user_id, item_id, recommendation_id, event_type, algorithm_used,
				position_in_list, confidence_score, timestamp, session_id, context,
				user_tier, content_category
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		`,
			event.UserID,
			event.ItemID,
			event.RecommendationID,
			event.EventType,
			event.AlgorithmUsed,
			event.PositionInList,
			event.ConfidenceScore,
			event.Timestamp,
			event.SessionID,
			string(contextJSON),
			event.UserTier,
			event.ContentCategory,
		)
		if err != nil {
			log.Printf("Error inserting metric event: %v", err)
			continue
		}
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("Error committing metrics batch: %v", err)
		return
	}

	log.Printf("Successfully inserted %d metric events", len(events))
}

// updateAggregatedMetrics updates aggregated metrics every 5 minutes
func (mc *MetricsCollector) updateAggregatedMetrics() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		mc.calculateAndUpdateMetrics()
	}
}

// calculateAndUpdateMetrics calculates and updates business metrics
func (mc *MetricsCollector) calculateAndUpdateMetrics() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Calculate CTR by algorithm, user tier, and content category
	ctrQuery := `
		SELECT 
			algorithm_used,
			COALESCE(user_tier, 'unknown') as user_tier,
			COALESCE(content_category, 'unknown') as content_category,
			COUNT(CASE WHEN event_type = 'impression' THEN 1 END) as impressions,
			COUNT(CASE WHEN event_type = 'click' THEN 1 END) as clicks,
			COUNT(CASE WHEN event_type = 'conversion' THEN 1 END) as conversions
		FROM recommendation_metrics 
		WHERE timestamp >= NOW() - INTERVAL '1 hour'
		GROUP BY algorithm_used, user_tier, content_category
		HAVING COUNT(CASE WHEN event_type = 'impression' THEN 1 END) > 0
	`

	rows, err := mc.db.Query(ctx, ctrQuery)
	if err != nil {
		log.Printf("Error querying CTR metrics: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var algorithm, userTier, contentCategory string
		var impressions, clicks, conversions int

		err := rows.Scan(&algorithm, &userTier, &contentCategory, &impressions, &clicks, &conversions)
		if err != nil {
			log.Printf("Error scanning CTR row: %v", err)
			continue
		}

		// Calculate and update CTR
		ctr := float64(clicks) / float64(impressions) * 100
		mc.clickThroughRate.WithLabelValues(algorithm, userTier, contentCategory).Set(ctr)

		// Calculate and update conversion rate
		if clicks > 0 {
			convRate := float64(conversions) / float64(clicks) * 100
			mc.conversionRate.WithLabelValues(algorithm, userTier, contentCategory).Set(convRate)
		}

		// Update algorithm performance score (composite metric)
		performanceScore := (ctr * 0.7) + (float64(conversions) / float64(impressions) * 100 * 0.3)
		mc.algorithmPerformance.WithLabelValues(algorithm).Set(performanceScore)
	}

	// Calculate user engagement metrics
	mc.calculateUserEngagement(ctx)
}

// calculateUserEngagement calculates user engagement metrics
func (mc *MetricsCollector) calculateUserEngagement(ctx context.Context) {
	engagementQuery := `
		SELECT 
			COALESCE(user_tier, 'unknown') as user_tier,
			AVG(EXTRACT(EPOCH FROM (
				SELECT MAX(timestamp) - MIN(timestamp) 
				FROM recommendation_metrics rm2 
				WHERE rm2.session_id = rm1.session_id
			))) as avg_session_duration,
			COUNT(DISTINCT session_id) as total_sessions,
			COUNT(*) as total_events
		FROM recommendation_metrics rm1
		WHERE timestamp >= NOW() - INTERVAL '1 hour'
		GROUP BY user_tier
	`

	rows, err := mc.db.Query(ctx, engagementQuery)
	if err != nil {
		log.Printf("Error querying engagement metrics: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var userTier string
		var avgSessionDuration pgtype.Float8
		var totalSessions, totalEvents int

		err := rows.Scan(&userTier, &avgSessionDuration, &totalSessions, &totalEvents)
		if err != nil {
			log.Printf("Error scanning engagement row: %v", err)
			continue
		}

		// Update engagement metrics
		if avgSessionDuration.Valid {
			mc.userEngagement.WithLabelValues("avg_session_duration", userTier).Set(avgSessionDuration.Float64)
		}

		if totalSessions > 0 {
			eventsPerSession := float64(totalEvents) / float64(totalSessions)
			mc.userEngagement.WithLabelValues("events_per_session", userTier).Set(eventsPerSession)
		}
	}
}

// GetBusinessMetrics retrieves aggregated business metrics for a date range
func (mc *MetricsCollector) GetBusinessMetrics(startDate, endDate time.Time) (*BusinessMetrics, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	query := `
		SELECT 
			COUNT(CASE WHEN event_type = 'impression' THEN 1 END) as total_recommendations,
			COUNT(CASE WHEN event_type = 'click' THEN 1 END) as total_clicks,
			COUNT(CASE WHEN event_type = 'conversion' THEN 1 END) as total_conversions,
			AVG(confidence_score) as avg_confidence_score
		FROM recommendation_metrics 
		WHERE timestamp BETWEEN $1 AND $2
	`

	var metrics BusinessMetrics
	var avgConfidence pgtype.Float8

	err := mc.db.QueryRow(ctx, query, startDate, endDate).Scan(
		&metrics.TotalRecommendations,
		&metrics.TotalClicks,
		&metrics.TotalConversions,
		&avgConfidence,
	)
	if err != nil {
		return nil, fmt.Errorf("error querying business metrics: %w", err)
	}

	metrics.Date = startDate
	if avgConfidence.Valid {
		metrics.AvgConfidenceScore = avgConfidence.Float64
	}

	// Calculate rates
	if metrics.TotalRecommendations > 0 {
		metrics.ClickThroughRate = float64(metrics.TotalClicks) / float64(metrics.TotalRecommendations) * 100
	}
	if metrics.TotalClicks > 0 {
		metrics.ConversionRate = float64(metrics.TotalConversions) / float64(metrics.TotalClicks) * 100
	}

	// Get algorithm-specific metrics
	metrics.AlgorithmPerformance = make(map[string]AlgorithmMetrics)
	algorithmQuery := `
		SELECT 
			algorithm_used,
			COUNT(CASE WHEN event_type = 'impression' THEN 1 END) as impressions,
			COUNT(CASE WHEN event_type = 'click' THEN 1 END) as clicks,
			COUNT(CASE WHEN event_type = 'conversion' THEN 1 END) as conversions,
			AVG(confidence_score) as avg_confidence,
			AVG(position_in_list) as avg_position
		FROM recommendation_metrics 
		WHERE timestamp BETWEEN $1 AND $2
		GROUP BY algorithm_used
	`

	rows, err := mc.db.Query(ctx, algorithmQuery, startDate, endDate)
	if err != nil {
		return &metrics, nil // Return partial results
	}
	defer rows.Close()

	for rows.Next() {
		var algorithm string
		var impressions, clicks, conversions int
		var avgConfidence, avgPosition pgtype.Float8

		err := rows.Scan(&algorithm, &impressions, &clicks, &conversions, &avgConfidence, &avgPosition)
		if err != nil {
			continue
		}

		algMetrics := AlgorithmMetrics{
			Impressions: impressions,
			Clicks:      clicks,
			Conversions: conversions,
		}

		if impressions > 0 {
			algMetrics.CTR = float64(clicks) / float64(impressions) * 100
		}
		if clicks > 0 {
			algMetrics.ConversionRate = float64(conversions) / float64(clicks) * 100
		}
		if avgConfidence.Valid {
			algMetrics.AvgConfidence = avgConfidence.Float64
		}
		if avgPosition.Valid {
			algMetrics.AvgPosition = avgPosition.Float64
		}

		metrics.AlgorithmPerformance[algorithm] = algMetrics
	}

	return &metrics, nil
}

// Close gracefully shuts down the metrics collector
func (mc *MetricsCollector) Close() error {
	close(mc.buffer)
	return nil
}
