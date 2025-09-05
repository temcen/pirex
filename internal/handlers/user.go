package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/temcen/pirex/internal/services"
)

type UserHandler struct {
	logger             *logrus.Logger
	userInteractionSvc services.UserInteractionServiceInterface
}

func NewUserHandler(logger *logrus.Logger, userInteractionSvc services.UserInteractionServiceInterface) *UserHandler {
	return &UserHandler{
		logger:             logger,
		userInteractionSvc: userInteractionSvc,
	}
}

func (h *UserHandler) GetInteractions(c *gin.Context) {
	// Parse user ID from URL parameter
	userIDStr := c.Param("userId")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.logger.WithError(err).Error("Invalid user ID format")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_USER_ID",
				"message": "Invalid user ID format",
			},
		})
		return
	}

	// Parse query parameters
	interactionType := c.Query("type")

	// Parse pagination parameters
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 1000 {
		limit = 50
	}

	offsetStr := c.DefaultQuery("offset", "0")
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	// Parse date filters
	var startDate, endDate *time.Time
	if startDateStr := c.Query("start_date"); startDateStr != "" {
		if parsed, err := time.Parse("2006-01-02", startDateStr); err == nil {
			startDate = &parsed
		}
	}
	if endDateStr := c.Query("end_date"); endDateStr != "" {
		if parsed, err := time.Parse("2006-01-02", endDateStr); err == nil {
			endDate = &parsed
		}
	}

	// Get interactions
	interactions, totalCount, err := h.userInteractionSvc.GetUserInteractions(
		c.Request.Context(),
		userID,
		interactionType,
		limit,
		offset,
		startDate,
		endDate,
	)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get user interactions")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "QUERY_FAILED",
				"message": "Failed to retrieve user interactions",
			},
		})
		return
	}

	// Calculate pagination info
	hasMore := offset+len(interactions) < totalCount
	nextOffset := offset + len(interactions)

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"interactions": interactions,
			"pagination": gin.H{
				"total":       totalCount,
				"limit":       limit,
				"offset":      offset,
				"has_more":    hasMore,
				"next_offset": nextOffset,
			},
		},
	})
}

func (h *UserHandler) GetProfile(c *gin.Context) {
	// Parse user ID from URL parameter
	userIDStr := c.Param("userId")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.logger.WithError(err).Error("Invalid user ID format")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_USER_ID",
				"message": "Invalid user ID format",
			},
		})
		return
	}

	// Get user profile
	profile, err := h.userInteractionSvc.GetUserProfile(c.Request.Context(), userID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get user profile")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "PROFILE_FAILED",
				"message": "Failed to retrieve user profile",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": profile,
	})
}

func (h *UserHandler) GetSimilarUsers(c *gin.Context) {
	// Parse user ID from URL parameter
	userIDStr := c.Param("userId")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.logger.WithError(err).Error("Invalid user ID format")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_USER_ID",
				"message": "Invalid user ID format",
			},
		})
		return
	}

	// Parse limit parameter
	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 10
	}

	// Get similar users
	similarUsers, err := h.userInteractionSvc.GetSimilarUsers(c.Request.Context(), userID, limit)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get similar users")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "SIMILARITY_FAILED",
				"message": "Failed to retrieve similar users",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"similar_users": similarUsers,
			"count":         len(similarUsers),
		},
	})
}
