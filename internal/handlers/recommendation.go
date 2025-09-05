package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/temcen/pirex/internal/services"
	"github.com/temcen/pirex/pkg/models"
)

type RecommendationHandler struct {
	orchestrator services.RecommendationOrchestratorInterface
	logger       *logrus.Logger
}

func NewRecommendationHandler(
	orchestrator services.RecommendationOrchestratorInterface,
	logger *logrus.Logger,
) *RecommendationHandler {
	return &RecommendationHandler{
		orchestrator: orchestrator,
		logger:       logger,
	}
}

func (h *RecommendationHandler) Get(c *gin.Context) {
	// Parse user ID from path
	userIDStr := c.Param("userId")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_USER_ID",
				"message": "Invalid user ID format",
			},
		})
		return
	}

	// Parse query parameters
	count := 10 // default
	if countStr := c.Query("count"); countStr != "" {
		if parsedCount, err := strconv.Atoi(countStr); err == nil && parsedCount > 0 && parsedCount <= 100 {
			count = parsedCount
		}
	}

	context := c.DefaultQuery("context", "home")
	explain := c.Query("explain") == "true"

	// Parse content types and categories
	var contentTypes []string
	if typesStr := c.Query("content_types"); typesStr != "" {
		contentTypes = strings.Split(typesStr, ",")
	}

	var categories []string
	if categoriesStr := c.Query("categories"); categoriesStr != "" {
		categories = strings.Split(categoriesStr, ",")
	}

	// Parse excluded items
	var excludeItems []uuid.UUID
	if excludeStr := c.Query("exclude"); excludeStr != "" {
		excludeItemStrs := strings.Split(excludeStr, ",")
		for _, itemStr := range excludeItemStrs {
			if itemID, err := uuid.Parse(strings.TrimSpace(itemStr)); err == nil {
				excludeItems = append(excludeItems, itemID)
			}
		}
	}

	// Create recommendation context
	reqCtx := &services.RecommendationContext{
		UserID:              userID,
		Count:               count,
		Context:             context,
		ContentTypes:        contentTypes,
		Categories:          categories,
		ExcludeItems:        excludeItems,
		IncludeExplanations: explain,
		TimeoutMs:           2000, // 2 second timeout
	}

	// Generate recommendations
	result, err := h.orchestrator.GenerateRecommendations(c.Request.Context(), reqCtx)
	if err != nil {
		h.logger.Error("Failed to generate recommendations", "error", err, "user_id", userID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "RECOMMENDATION_GENERATION_FAILED",
				"message": "Failed to generate recommendations",
			},
		})
		return
	}

	// Convert to response format
	response := models.RecommendationResponse{
		UserID:          result.UserID,
		Recommendations: result.Recommendations,
		Context:         result.Context,
		GeneratedAt:     result.GeneratedAt,
		CacheHit:        result.CacheHit,
	}

	c.JSON(http.StatusOK, response)
}

func (h *RecommendationHandler) GetBatch(c *gin.Context) {
	var batchRequest models.BatchRecommendationRequest
	if err := c.ShouldBindJSON(&batchRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_REQUEST_BODY",
				"message": "Invalid request body format",
			},
		})
		return
	}

	// Validate batch size
	if len(batchRequest.Requests) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "EMPTY_BATCH_REQUEST",
				"message": "Batch request cannot be empty",
			},
		})
		return
	}

	if len(batchRequest.Requests) > 50 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "BATCH_SIZE_EXCEEDED",
				"message": "Batch size cannot exceed 50 requests",
			},
		})
		return
	}

	var responses []models.RecommendationResponse

	// Process each request in the batch
	for _, req := range batchRequest.Requests {
		// Create recommendation context
		reqCtx := &services.RecommendationContext{
			UserID:              req.UserID,
			Count:               req.Count,
			Context:             req.Context,
			IncludeExplanations: req.Explain,
			TimeoutMs:           1500, // Shorter timeout for batch requests
		}

		// Generate recommendations
		result, err := h.orchestrator.GenerateRecommendations(c.Request.Context(), reqCtx)
		if err != nil {
			h.logger.Warn("Failed to generate recommendations for batch user",
				"error", err, "user_id", req.UserID)

			// Add empty response for failed requests
			responses = append(responses, models.RecommendationResponse{
				UserID:          req.UserID,
				Recommendations: []models.Recommendation{},
				Context:         req.Context,
				GeneratedAt:     result.GeneratedAt,
				CacheHit:        false,
			})
			continue
		}

		// Convert to response format
		response := models.RecommendationResponse{
			UserID:          result.UserID,
			Recommendations: result.Recommendations,
			Context:         result.Context,
			GeneratedAt:     result.GeneratedAt,
			CacheHit:        result.CacheHit,
		}

		responses = append(responses, response)
	}

	batchResponse := models.BatchRecommendationResponse{
		Responses: responses,
	}

	c.JSON(http.StatusOK, batchResponse)
}

// GetSimilar handles item-based recommendations
func (h *RecommendationHandler) GetSimilar(c *gin.Context) {
	// Parse user ID from path
	userIDStr := c.Param("userId")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_USER_ID",
				"message": "Invalid user ID format",
			},
		})
		return
	}

	// Parse item ID from path
	itemIDStr := c.Param("itemId")
	itemID, err := uuid.Parse(itemIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_ITEM_ID",
				"message": "Invalid item ID format",
			},
		})
		return
	}

	// Parse query parameters
	count := 10 // default
	if countStr := c.Query("count"); countStr != "" {
		if parsedCount, err := strconv.Atoi(countStr); err == nil && parsedCount > 0 && parsedCount <= 100 {
			count = parsedCount
		}
	}

	explain := c.Query("explain") == "true"

	// Create recommendation context for item-based recommendations
	reqCtx := &services.RecommendationContext{
		UserID:              userID,
		Count:               count,
		Context:             "similar",
		SeedItemID:          &itemID,
		IncludeExplanations: explain,
		TimeoutMs:           2000,
	}

	// Generate similar item recommendations
	result, err := h.orchestrator.GenerateRecommendations(c.Request.Context(), reqCtx)
	if err != nil {
		h.logger.Error("Failed to generate similar item recommendations",
			"error", err, "user_id", userID, "item_id", itemID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "SIMILAR_RECOMMENDATIONS_FAILED",
				"message": "Failed to generate similar item recommendations",
			},
		})
		return
	}

	// Convert to response format
	response := models.SimilarItemResponse{
		UserID:          result.UserID,
		SeedItemID:      itemID,
		Recommendations: result.Recommendations,
		GeneratedAt:     result.GeneratedAt,
		CacheHit:        result.CacheHit,
	}

	c.JSON(http.StatusOK, response)
}

// RecordFeedback handles user feedback on recommendations
func (h *RecommendationHandler) RecordFeedback(c *gin.Context) {
	var feedback models.RecommendationFeedback
	if err := c.ShouldBindJSON(&feedback); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_REQUEST_BODY",
				"message": "Invalid feedback format",
			},
		})
		return
	}

	// Validate feedback type
	validTypes := map[string]bool{
		"positive":       true,
		"negative":       true,
		"not_interested": true,
		"not_relevant":   true,
		"inappropriate":  true,
	}

	if !validTypes[feedback.FeedbackType] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_FEEDBACK_TYPE",
				"message": "Invalid feedback type",
			},
		})
		return
	}

	// Process feedback (this would integrate with the learning system)
	err := h.orchestrator.ProcessFeedback(c.Request.Context(), &feedback)
	if err != nil {
		h.logger.Error("Failed to process recommendation feedback",
			"error", err, "user_id", feedback.UserID, "recommendation_id", feedback.RecommendationID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "FEEDBACK_PROCESSING_FAILED",
				"message": "Failed to process feedback",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Feedback recorded successfully",
		"feedback_id": feedback.RecommendationID, // In a real system, this would be a unique feedback ID
	})
}
