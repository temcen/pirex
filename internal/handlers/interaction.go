package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/sirupsen/logrus"

	"github.com/temcen/pirex/internal/services"
	"github.com/temcen/pirex/pkg/models"
)

type InteractionHandler struct {
	logger             *logrus.Logger
	userInteractionSvc services.UserInteractionServiceInterface
	validator          *validator.Validate
}

func NewInteractionHandler(logger *logrus.Logger, userInteractionSvc services.UserInteractionServiceInterface) *InteractionHandler {
	return &InteractionHandler{
		logger:             logger,
		userInteractionSvc: userInteractionSvc,
		validator:          validator.New(),
	}
}

func (h *InteractionHandler) RecordExplicit(c *gin.Context) {
	var req models.ExplicitInteractionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.WithError(err).Error("Failed to bind explicit interaction request")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": "Invalid request format",
				"details": err.Error(),
			},
		})
		return
	}

	// Validate request
	if err := h.validator.Struct(&req); err != nil {
		h.logger.WithError(err).Error("Validation failed for explicit interaction")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "VALIDATION_FAILED",
				"message": "Request validation failed",
				"details": err.Error(),
			},
		})
		return
	}

	// Validate rating value for rating type
	if req.Type == "rating" && (req.Value == nil || *req.Value < 1 || *req.Value > 5) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_RATING",
				"message": "Rating value must be between 1 and 5",
			},
		})
		return
	}

	// Record interaction
	interaction, err := h.userInteractionSvc.RecordExplicitInteraction(c.Request.Context(), &req)
	if err != nil {
		h.logger.WithError(err).Error("Failed to record explicit interaction")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERACTION_FAILED",
				"message": "Failed to record interaction",
			},
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"data":    interaction,
		"message": "Explicit interaction recorded successfully",
	})
}

func (h *InteractionHandler) RecordImplicit(c *gin.Context) {
	var req models.ImplicitInteractionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.WithError(err).Error("Failed to bind implicit interaction request")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": "Invalid request format",
				"details": err.Error(),
			},
		})
		return
	}

	// Validate request
	if err := h.validator.Struct(&req); err != nil {
		h.logger.WithError(err).Error("Validation failed for implicit interaction")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "VALIDATION_FAILED",
				"message": "Request validation failed",
				"details": err.Error(),
			},
		})
		return
	}

	// Validate search queries have query field
	if req.Type == "search" && (req.Query == nil || *req.Query == "") {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "MISSING_QUERY",
				"message": "Search interactions must include a query",
			},
		})
		return
	}

	// Record interaction
	interaction, err := h.userInteractionSvc.RecordImplicitInteraction(c.Request.Context(), &req)
	if err != nil {
		h.logger.WithError(err).Error("Failed to record implicit interaction")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERACTION_FAILED",
				"message": "Failed to record interaction",
			},
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"data":    interaction,
		"message": "Implicit interaction recorded successfully",
	})
}

func (h *InteractionHandler) RecordBatch(c *gin.Context) {
	var req models.InteractionBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.WithError(err).Error("Failed to bind batch interaction request")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": "Invalid request format",
				"details": err.Error(),
			},
		})
		return
	}

	// Validate that at least one interaction type is provided
	if len(req.ExplicitInteractions) == 0 && len(req.ImplicitInteractions) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "EMPTY_BATCH",
				"message": "Batch request must contain at least one interaction",
			},
		})
		return
	}

	// Validate individual interactions
	for i, explicit := range req.ExplicitInteractions {
		if err := h.validator.Struct(&explicit); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_FAILED",
					"message": fmt.Sprintf("Explicit interaction %d validation failed: %s", i, err.Error()),
				},
			})
			return
		}
	}

	for i, implicit := range req.ImplicitInteractions {
		if err := h.validator.Struct(&implicit); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_FAILED",
					"message": fmt.Sprintf("Implicit interaction %d validation failed: %s", i, err.Error()),
				},
			})
			return
		}
	}

	// Process batch
	interactions, err := h.userInteractionSvc.RecordBatchInteractions(c.Request.Context(), &req)
	if err != nil {
		h.logger.WithError(err).Error("Failed to process batch interactions")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "BATCH_FAILED",
				"message": "Failed to process batch interactions",
			},
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"data": gin.H{
			"interactions":    interactions,
			"total_processed": len(interactions),
		},
		"message": "Batch interactions processed successfully",
	})
}
