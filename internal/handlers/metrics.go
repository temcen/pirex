package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/temcen/pirex/internal/services"
)

// MetricsHandler handles metrics-related requests
type MetricsHandler struct {
	logger           *logrus.Logger
	metricsCollector *services.MetricsCollector
	healthService    *services.HealthService
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(logger *logrus.Logger, metricsCollector *services.MetricsCollector, healthService *services.HealthService) *MetricsHandler {
	return &MetricsHandler{
		logger:           logger,
		metricsCollector: metricsCollector,
		healthService:    healthService,
	}
}

// GetBusinessMetrics returns business metrics for a date range
func (h *MetricsHandler) GetBusinessMetrics(c *gin.Context) {
	// Parse date range parameters
	startDateStr := c.DefaultQuery("start_date", time.Now().AddDate(0, 0, -7).Format("2006-01-02"))
	endDateStr := c.DefaultQuery("end_date", time.Now().Format("2006-01-02"))

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid start_date format. Use YYYY-MM-DD",
		})
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid end_date format. Use YYYY-MM-DD",
		})
		return
	}

	// Add time to end date to include the full day
	endDate = endDate.Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	if h.metricsCollector == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Metrics service not available",
		})
		return
	}

	metrics, err := h.metricsCollector.GetBusinessMetrics(startDate, endDate)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get business metrics")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve business metrics",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"metrics":    metrics,
		"start_date": startDate.Format("2006-01-02"),
		"end_date":   endDate.Format("2006-01-02"),
		"timestamp":  time.Now().UTC(),
	})
}

// GetPerformanceMetrics returns performance metrics for charts
func (h *MetricsHandler) GetPerformanceMetrics(c *gin.Context) {
	// This would typically query a time-series database or aggregated metrics
	// For now, return mock data structure

	performanceData := gin.H{
		"timeline": gin.H{
			"labels": []string{
				"00:00", "04:00", "08:00", "12:00", "16:00", "20:00",
			},
			"ctr":             []float64{2.1, 1.8, 3.2, 4.1, 3.8, 2.9},
			"conversion_rate": []float64{0.8, 0.6, 1.2, 1.8, 1.5, 1.1},
		},
		"algorithm_performance": gin.H{
			"semantic_search":         85.2,
			"collaborative_filtering": 78.9,
			"pagerank":                72.4,
		},
		"timestamp": time.Now().UTC(),
	}

	c.JSON(http.StatusOK, performanceData)
}

// RecordInteraction records a user interaction for metrics
func (h *MetricsHandler) RecordInteraction(c *gin.Context) {
	var interaction struct {
		UserID           string                 `json:"user_id" binding:"required"`
		ItemID           string                 `json:"item_id" binding:"required"`
		InteractionType  string                 `json:"interaction_type" binding:"required"`
		RecommendationID string                 `json:"recommendation_id,omitempty"`
		Timestamp        time.Time              `json:"timestamp"`
		SessionID        string                 `json:"session_id,omitempty"`
		Context          map[string]interface{} `json:"context,omitempty"`
	}

	if err := c.ShouldBindJSON(&interaction); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid interaction data",
			"details": err.Error(),
		})
		return
	}

	// Set timestamp if not provided
	if interaction.Timestamp.IsZero() {
		interaction.Timestamp = time.Now()
	}

	// Create metric event
	event := services.MetricEvent{
		UserID:           interaction.UserID,
		ItemID:           interaction.ItemID,
		RecommendationID: interaction.RecommendationID,
		EventType:        interaction.InteractionType,
		AlgorithmUsed:    "unknown", // This would be determined from the recommendation context
		PositionInList:   0,         // This would come from the recommendation context
		ConfidenceScore:  0.0,       // This would come from the recommendation context
		Timestamp:        interaction.Timestamp,
		SessionID:        interaction.SessionID,
		Context:          interaction.Context,
	}

	// Extract additional context if available
	if interaction.Context != nil {
		if algorithm, ok := interaction.Context["algorithm"].(string); ok {
			event.AlgorithmUsed = algorithm
		}
		if position, ok := interaction.Context["position"].(float64); ok {
			event.PositionInList = int(position)
		}
		if confidence, ok := interaction.Context["confidence"].(float64); ok {
			event.ConfidenceScore = confidence
		}
		if userTier, ok := interaction.Context["user_tier"].(string); ok {
			event.UserTier = userTier
		}
		if category, ok := interaction.Context["content_category"].(string); ok {
			event.ContentCategory = category
		}
	}

	if h.metricsCollector != nil {
		h.metricsCollector.RecordEvent(event)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "recorded",
		"timestamp": event.Timestamp,
	})
}

// GetAdminOverviewMetrics returns overview metrics for admin dashboard
func (h *MetricsHandler) GetAdminOverviewMetrics(c *gin.Context) {
	// This would typically aggregate data from various sources
	overviewData := gin.H{
		"active_users":          1250,
		"total_recommendations": 45600,
		"avg_response_time":     125.5,
		"system_health":         "healthy",
		"timestamp":             time.Now().UTC(),
	}

	c.JSON(http.StatusOK, overviewData)
}

// GetAdminAnalytics returns detailed analytics for admin dashboard
func (h *MetricsHandler) GetAdminAnalytics(c *gin.Context) {
	analyticsData := gin.H{
		"revenue_impact": gin.H{
			"timeline": gin.H{
				"labels": []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"},
				"data":   []float64{12500, 13200, 11800, 14100, 15600, 16200, 14800},
			},
		},
		"user_engagement": gin.H{
			"timeline": gin.H{
				"labels":            []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"},
				"session_duration":  []float64{8.2, 7.9, 8.5, 9.1, 8.8, 9.5, 8.7},
				"pages_per_session": []float64{4.1, 3.8, 4.3, 4.7, 4.5, 4.9, 4.2},
			},
		},
		"algorithm_performance": gin.H{
			"semantic_search": gin.H{
				"ctr":               3.2,
				"conversion_rate":   1.8,
				"avg_confidence":    0.85,
				"performance_score": 85.2,
				"status":            "active",
			},
			"collaborative_filtering": gin.H{
				"ctr":               2.8,
				"conversion_rate":   1.5,
				"avg_confidence":    0.78,
				"performance_score": 78.9,
				"status":            "active",
			},
			"pagerank": gin.H{
				"ctr":               2.4,
				"conversion_rate":   1.2,
				"avg_confidence":    0.72,
				"performance_score": 72.4,
				"status":            "active",
			},
		},
		"timestamp": time.Now().UTC(),
	}

	c.JSON(http.StatusOK, analyticsData)
}

// GetContentStatus returns content management status
func (h *MetricsHandler) GetContentStatus(c *gin.Context) {
	contentData := gin.H{
		"total_items":  15420,
		"queue_size":   23,
		"failed_items": 5,
		"jobs": []gin.H{
			{
				"job_id":       "job-001",
				"content_type": "product",
				"status":       "completed",
				"progress":     100,
				"created_at":   time.Now().Add(-2 * time.Hour),
			},
			{
				"job_id":       "job-002",
				"content_type": "article",
				"status":       "processing",
				"progress":     65,
				"created_at":   time.Now().Add(-30 * time.Minute),
			},
			{
				"job_id":       "job-003",
				"content_type": "video",
				"status":       "queued",
				"progress":     0,
				"created_at":   time.Now().Add(-5 * time.Minute),
			},
		},
		"timestamp": time.Now().UTC(),
	}

	c.JSON(http.StatusOK, contentData)
}

// GetUserAnalytics returns user analytics for admin dashboard
func (h *MetricsHandler) GetUserAnalytics(c *gin.Context) {
	userAnalytics := gin.H{
		"segmentation": gin.H{
			"labels": []string{"New Users", "Active Users", "Inactive Users", "Power Users"},
			"data":   []int{1250, 3200, 800, 450},
		},
		"tier_stats": gin.H{
			"free":       gin.H{"count": 4200, "percentage": 75.0},
			"premium":    gin.H{"count": 1200, "percentage": 21.4},
			"enterprise": gin.H{"count": 200, "percentage": 3.6},
		},
		"timestamp": time.Now().UTC(),
	}

	c.JSON(http.StatusOK, userAnalytics)
}

// GetMonitoringMetrics returns system monitoring metrics
func (h *MetricsHandler) GetMonitoringMetrics(c *gin.Context) {
	monitoringData := gin.H{
		"cpu_usage":          45.2,
		"memory_usage":       68.7,
		"active_connections": 156,
		"cache_hit_rate":     87.3,
		"performance": gin.H{
			"timeline": gin.H{
				"labels":       []string{"00:00", "04:00", "08:00", "12:00", "16:00", "20:00"},
				"cpu_usage":    []float64{35.2, 28.1, 42.5, 55.8, 48.3, 39.7},
				"memory_usage": []float64{62.1, 58.9, 65.4, 72.3, 69.8, 66.2},
			},
		},
		"db_connections": gin.H{
			"labels": []string{"Active", "Idle", "Available"},
			"data":   []int{12, 8, 20},
		},
		"timestamp": time.Now().UTC(),
	}

	c.JSON(http.StatusOK, monitoringData)
}

// GetRecentAlerts returns recent system alerts
func (h *MetricsHandler) GetRecentAlerts(c *gin.Context) {
	limit := 10
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Mock alerts data - in production this would come from a monitoring system
	alerts := []gin.H{
		{
			"id":        "alert-001",
			"level":     "warning",
			"message":   "High memory usage detected (>80%)",
			"timestamp": time.Now().Add(-15 * time.Minute),
			"resolved":  false,
		},
		{
			"id":        "alert-002",
			"level":     "info",
			"message":   "Cache hit rate improved to 87%",
			"timestamp": time.Now().Add(-1 * time.Hour),
			"resolved":  true,
		},
		{
			"id":        "alert-003",
			"level":     "error",
			"message":   "Redis connection timeout",
			"timestamp": time.Now().Add(-2 * time.Hour),
			"resolved":  true,
		},
	}

	// Limit results
	if len(alerts) > limit {
		alerts = alerts[:limit]
	}

	c.JSON(http.StatusOK, gin.H{
		"alerts":    alerts,
		"count":     len(alerts),
		"timestamp": time.Now().UTC(),
	})
}

// GetABTests returns A/B test data
func (h *MetricsHandler) GetABTests(c *gin.Context) {
	abTests := gin.H{
		"tests": []gin.H{
			{
				"id":                       "test-001",
				"name":                     "Algorithm Weight Optimization",
				"status":                   "active",
				"traffic_split":            "50/50",
				"conversion_rate_a":        2.1,
				"conversion_rate_b":        2.4,
				"statistical_significance": 0.95,
				"created_at":               time.Now().Add(-7 * 24 * time.Hour),
			},
			{
				"id":                       "test-002",
				"name":                     "Diversity Filter Enhancement",
				"status":                   "completed",
				"traffic_split":            "70/30",
				"conversion_rate_a":        1.8,
				"conversion_rate_b":        2.0,
				"statistical_significance": 0.92,
				"created_at":               time.Now().Add(-14 * 24 * time.Hour),
			},
		},
		"timestamp": time.Now().UTC(),
	}

	c.JSON(http.StatusOK, abTests)
}
