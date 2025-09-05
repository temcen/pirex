package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/temcen/pirex/internal/services"
)

// RealtimeLearningHandler handles real-time learning API endpoints
type RealtimeLearningHandler struct {
	learningService *services.RealtimeLearningService
}

// NewRealtimeLearningHandler creates a new real-time learning handler
func NewRealtimeLearningHandler(learningService *services.RealtimeLearningService) *RealtimeLearningHandler {
	return &RealtimeLearningHandler{
		learningService: learningService,
	}
}

// FeedbackRequest represents a feedback submission request
type FeedbackRequest struct {
	UserID           string                 `json:"user_id" binding:"required"`
	ItemID           string                 `json:"item_id" binding:"required"`
	RecommendationID string                 `json:"recommendation_id,omitempty"`
	Type             string                 `json:"type" binding:"required,oneof=explicit implicit"`
	Action           string                 `json:"action" binding:"required"`
	Value            float64                `json:"value"`
	SessionID        string                 `json:"session_id,omitempty"`
	Context          map[string]interface{} `json:"context,omitempty"`
	Algorithm        string                 `json:"algorithm,omitempty"`
	Position         int                    `json:"position,omitempty"`
}

// AlgorithmPerformanceRequest represents an algorithm performance recording request
type AlgorithmPerformanceRequest struct {
	AlgorithmName    string  `json:"algorithm_name" binding:"required"`
	UserID           string  `json:"user_id" binding:"required"`
	Impressions      int64   `json:"impressions"`
	Clicks           int64   `json:"clicks"`
	Conversions      int64   `json:"conversions"`
	UserSatisfaction float64 `json:"user_satisfaction"`
}

// ExperimentEventRequest represents an A/B test event recording request
type ExperimentEventRequest struct {
	UserID       string  `json:"user_id" binding:"required"`
	ExperimentID string  `json:"experiment_id" binding:"required"`
	EventType    string  `json:"event_type" binding:"required"`
	Value        float64 `json:"value"`
}

// SubmitFeedback handles feedback submission
func (rlh *RealtimeLearningHandler) SubmitFeedback(c *gin.Context) {
	var req FeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Convert to feedback event
	event := services.FeedbackEvent{
		UserID:           req.UserID,
		ItemID:           req.ItemID,
		RecommendationID: req.RecommendationID,
		Type:             services.FeedbackType(req.Type),
		Action:           req.Action,
		Value:            req.Value,
		Timestamp:        time.Now(),
		SessionID:        req.SessionID,
		Context:          req.Context,
		Algorithm:        req.Algorithm,
		Position:         req.Position,
	}

	// Process feedback
	if err := rlh.learningService.ProcessFeedback(event); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to process feedback",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Feedback processed successfully",
		"timestamp": event.Timestamp,
	})
}

// GetAlgorithmWeights returns current algorithm weights for a user
func (rlh *RealtimeLearningHandler) GetAlgorithmWeights(c *gin.Context) {
	userID := c.Param("userId")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "User ID is required",
		})
		return
	}

	weights := rlh.learningService.GetAlgorithmWeights(userID)

	c.JSON(http.StatusOK, gin.H{
		"user_id":   userID,
		"weights":   weights,
		"timestamp": time.Now(),
	})
}

// RecordAlgorithmPerformance records algorithm performance metrics
func (rlh *RealtimeLearningHandler) RecordAlgorithmPerformance(c *gin.Context) {
	var req AlgorithmPerformanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	err := rlh.learningService.RecordAlgorithmPerformance(
		req.AlgorithmName,
		req.UserID,
		req.Impressions,
		req.Clicks,
		req.Conversions,
		req.UserSatisfaction,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to record algorithm performance",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Algorithm performance recorded successfully",
	})
}

// GetRecommendationStrategy returns the recommendation strategy for a user
func (rlh *RealtimeLearningHandler) GetRecommendationStrategy(c *gin.Context) {
	userID := c.Param("userId")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "User ID is required",
		})
		return
	}

	strategy := rlh.learningService.GetRecommendationStrategy(userID)

	c.JSON(http.StatusOK, gin.H{
		"user_id":   userID,
		"strategy":  strategy,
		"timestamp": time.Now(),
	})
}

// CreateExperiment creates a new A/B test experiment
func (rlh *RealtimeLearningHandler) CreateExperiment(c *gin.Context) {
	var experiment services.Experiment
	if err := c.ShouldBindJSON(&experiment); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid experiment format",
			"details": err.Error(),
		})
		return
	}

	if err := rlh.learningService.CreateExperiment(&experiment); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create experiment",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":       "Experiment created successfully",
		"experiment_id": experiment.ID,
	})
}

// StartExperiment starts an A/B test experiment
func (rlh *RealtimeLearningHandler) StartExperiment(c *gin.Context) {
	experimentID := c.Param("experimentId")
	if experimentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Experiment ID is required",
		})
		return
	}

	if err := rlh.learningService.StartExperiment(experimentID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to start experiment",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Experiment started successfully",
		"experiment_id": experimentID,
	})
}

// GetExperimentResults returns A/B test experiment results
func (rlh *RealtimeLearningHandler) GetExperimentResults(c *gin.Context) {
	experimentID := c.Param("experimentId")
	if experimentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Experiment ID is required",
		})
		return
	}

	results, err := rlh.learningService.GetExperimentResults(experimentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get experiment results",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, results)
}

// AssignUserToExperiment assigns a user to an experiment variant
func (rlh *RealtimeLearningHandler) AssignUserToExperiment(c *gin.Context) {
	userID := c.Param("userId")
	experimentID := c.Param("experimentId")

	if userID == "" || experimentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "User ID and Experiment ID are required",
		})
		return
	}

	variantID, err := rlh.learningService.AssignUserToExperiment(userID, experimentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to assign user to experiment",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":       userID,
		"experiment_id": experimentID,
		"variant_id":    variantID,
	})
}

// RecordExperimentEvent records an event for A/B testing
func (rlh *RealtimeLearningHandler) RecordExperimentEvent(c *gin.Context) {
	var req ExperimentEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	err := rlh.learningService.RecordExperimentEvent(
		req.UserID,
		req.ExperimentID,
		req.EventType,
		req.Value,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to record experiment event",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Experiment event recorded successfully",
	})
}

// GetSystemMetrics returns comprehensive system metrics
func (rlh *RealtimeLearningHandler) GetSystemMetrics(c *gin.Context) {
	metrics := rlh.learningService.GetSystemMetrics()

	c.JSON(http.StatusOK, gin.H{
		"metrics":   metrics,
		"timestamp": time.Now(),
	})
}

// GetHealthStatus returns the health status of all learning components
func (rlh *RealtimeLearningHandler) GetHealthStatus(c *gin.Context) {
	health := rlh.learningService.HealthCheck()

	// Determine HTTP status based on overall health
	status := http.StatusOK
	if health["overall"] == "not_running" {
		status = http.StatusServiceUnavailable
	} else if health["overall"] == "degraded" {
		status = http.StatusPartialContent
	}

	c.JSON(status, gin.H{
		"health":    health,
		"timestamp": time.Now(),
	})
}

// UpdateUserReliability updates a user's reliability score
func (rlh *RealtimeLearningHandler) UpdateUserReliability(c *gin.Context) {
	userID := c.Param("userId")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "User ID is required",
		})
		return
	}

	deltaStr := c.Query("delta")
	if deltaStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Delta parameter is required",
		})
		return
	}

	delta, err := strconv.Atoi(deltaStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid delta value",
			"details": err.Error(),
		})
		return
	}

	if err := rlh.learningService.UpdateUserReliability(userID, delta); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update user reliability",
			"details": err.Error(),
		})
		return
	}

	newReliability := rlh.learningService.GetUserReliability(userID)

	c.JSON(http.StatusOK, gin.H{
		"user_id":           userID,
		"reliability_score": newReliability,
		"delta":             delta,
	})
}

// GetUserReliability gets a user's current reliability score
func (rlh *RealtimeLearningHandler) GetUserReliability(c *gin.Context) {
	userID := c.Param("userId")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "User ID is required",
		})
		return
	}

	reliability := rlh.learningService.GetUserReliability(userID)

	c.JSON(http.StatusOK, gin.H{
		"user_id":           userID,
		"reliability_score": reliability,
	})
}

// RegisterRoutes registers all real-time learning routes
func (rlh *RealtimeLearningHandler) RegisterRoutes(router *gin.RouterGroup) {
	// Feedback endpoints
	router.POST("/feedback", rlh.SubmitFeedback)

	// Algorithm optimization endpoints
	router.GET("/users/:userId/algorithm-weights", rlh.GetAlgorithmWeights)
	router.POST("/algorithm-performance", rlh.RecordAlgorithmPerformance)

	// Recommendation strategy endpoints
	router.GET("/users/:userId/strategy", rlh.GetRecommendationStrategy)

	// A/B testing endpoints
	router.POST("/experiments", rlh.CreateExperiment)
	router.POST("/experiments/:experimentId/start", rlh.StartExperiment)
	router.GET("/experiments/:experimentId/results", rlh.GetExperimentResults)
	router.GET("/users/:userId/experiments/:experimentId/assignment", rlh.AssignUserToExperiment)
	router.POST("/experiment-events", rlh.RecordExperimentEvent)

	// System monitoring endpoints
	router.GET("/metrics", rlh.GetSystemMetrics)
	router.GET("/health", rlh.GetHealthStatus)

	// User reliability endpoints
	router.PUT("/users/:userId/reliability", rlh.UpdateUserReliability)
	router.GET("/users/:userId/reliability", rlh.GetUserReliability)
}
