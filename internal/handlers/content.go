package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/temcen/pirex/internal/messaging"
	"github.com/temcen/pirex/internal/services"
	"github.com/temcen/pirex/pkg/models"
)

type ContentHandler struct {
	messageBus *messaging.MessageBus
	jobManager *services.JobManager
	validator  *validator.Validate
	logger     *logrus.Logger
}

type ContentResponse struct {
	JobID         uuid.UUID `json:"job_id"`
	Status        string    `json:"status"`
	EstimatedTime *int      `json:"estimated_time,omitempty"`
	Message       string    `json:"message"`
}

func NewContentHandler(messageBus *messaging.MessageBus, jobManager *services.JobManager, logger *logrus.Logger) *ContentHandler {
	return &ContentHandler{
		messageBus: messageBus,
		jobManager: jobManager,
		validator:  validator.New(),
		logger:     logger,
	}
}

func (h *ContentHandler) Create(c *gin.Context) {
	var request models.ContentIngestionRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.WithError(err).Warn("Invalid JSON in content creation request")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_JSON",
				"message": "Invalid JSON format",
				"details": err.Error(),
			},
		})
		return
	}

	// Validate request
	if err := h.validator.Struct(&request); err != nil {
		h.logger.WithError(err).Warn("Content validation failed")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "VALIDATION_FAILED",
				"message": "Content validation failed",
				"details": err.Error(),
			},
		})
		return
	}

	// Create job for tracking
	job, err := h.jobManager.CreateJob(c.Request.Context(), 1, "single_content_ingestion")
	if err != nil {
		h.logger.WithError(err).Error("Failed to create job")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "JOB_CREATION_FAILED",
				"message": "Failed to create processing job",
			},
		})
		return
	}

	// Publish to Kafka for async processing
	processingHints := map[string]interface{}{
		"source":    "api",
		"user_id":   c.GetString("user_id"), // From JWT middleware
		"timestamp": time.Now(),
	}

	if err := h.messageBus.PublishContentIngestion(job.JobID, request, processingHints); err != nil {
		h.logger.WithError(err).WithField("job_id", job.JobID).Error("Failed to publish content to message bus")

		// Update job status to failed
		errorMsg := "Failed to queue content for processing"
		h.jobManager.UpdateJobProgress(c.Request.Context(), job.JobID, 0, 1, services.JobStatusFailed, &errorMsg)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "PROCESSING_QUEUE_FAILED",
				"message": "Failed to queue content for processing",
			},
		})
		return
	}

	h.logger.WithFields(logrus.Fields{
		"job_id":       job.JobID,
		"content_type": request.Type,
		"title":        request.Title,
	}).Info("Content queued for processing")

	response := ContentResponse{
		JobID:         job.JobID,
		Status:        job.Status,
		EstimatedTime: job.EstimatedTime,
		Message:       "Content queued for processing",
	}

	c.JSON(http.StatusAccepted, response)
}

func (h *ContentHandler) CreateBatch(c *gin.Context) {
	var request models.ContentBatchRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.WithError(err).Warn("Invalid JSON in batch content creation request")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_JSON",
				"message": "Invalid JSON format",
				"details": err.Error(),
			},
		})
		return
	}

	// Validate request
	if err := h.validator.Struct(&request); err != nil {
		h.logger.WithError(err).Warn("Batch content validation failed")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "VALIDATION_FAILED",
				"message": "Batch content validation failed",
				"details": err.Error(),
			},
		})
		return
	}

	if len(request.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "EMPTY_BATCH",
				"message": "Batch must contain at least one item",
			},
		})
		return
	}

	if len(request.Items) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "BATCH_TOO_LARGE",
				"message": "Batch cannot contain more than 100 items",
			},
		})
		return
	}

	// Validate each item in the batch
	for i, item := range request.Items {
		if err := h.validator.Struct(&item); err != nil {
			h.logger.WithError(err).WithField("item_index", i).Warn("Batch item validation failed")
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "ITEM_VALIDATION_FAILED",
					"message": fmt.Sprintf("Item at index %d failed validation", i),
					"details": err.Error(),
				},
			})
			return
		}
	}

	// Create job for tracking batch processing
	job, err := h.jobManager.CreateJob(c.Request.Context(), len(request.Items), "batch_content_ingestion")
	if err != nil {
		h.logger.WithError(err).Error("Failed to create batch job")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "JOB_CREATION_FAILED",
				"message": "Failed to create processing job",
			},
		})
		return
	}

	// Publish each item to Kafka for async processing
	successCount := 0
	processingHints := map[string]interface{}{
		"source":     "api_batch",
		"user_id":    c.GetString("user_id"),
		"batch_id":   job.JobID,
		"batch_size": len(request.Items),
		"timestamp":  time.Now(),
	}

	for i, item := range request.Items {
		itemHints := make(map[string]interface{})
		for k, v := range processingHints {
			itemHints[k] = v
		}
		itemHints["batch_index"] = i

		if err := h.messageBus.PublishContentIngestion(job.JobID, item, itemHints); err != nil {
			h.logger.WithError(err).WithFields(logrus.Fields{
				"job_id":     job.JobID,
				"item_index": i,
			}).Error("Failed to publish batch item to message bus")
			continue
		}
		successCount++
	}

	if successCount == 0 {
		// All items failed to queue
		errorMsg := "Failed to queue any items for processing"
		h.jobManager.UpdateJobProgress(c.Request.Context(), job.JobID, 0, len(request.Items), services.JobStatusFailed, &errorMsg)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "BATCH_PROCESSING_FAILED",
				"message": "Failed to queue batch items for processing",
			},
		})
		return
	}

	if successCount < len(request.Items) {
		h.logger.WithFields(logrus.Fields{
			"job_id":        job.JobID,
			"success_count": successCount,
			"total_items":   len(request.Items),
		}).Warn("Partial batch queuing success")
	}

	h.logger.WithFields(logrus.Fields{
		"job_id":       job.JobID,
		"items_queued": successCount,
		"total_items":  len(request.Items),
	}).Info("Batch content queued for processing")

	response := ContentResponse{
		JobID:         job.JobID,
		Status:        job.Status,
		EstimatedTime: job.EstimatedTime,
		Message:       fmt.Sprintf("Batch queued for processing (%d/%d items)", successCount, len(request.Items)),
	}

	c.JSON(http.StatusAccepted, response)
}

func (h *ContentHandler) GetJobStatus(c *gin.Context) {
	jobIDStr := c.Param("jobId")
	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_JOB_ID",
				"message": "Invalid job ID format",
			},
		})
		return
	}

	job, err := h.jobManager.GetJob(c.Request.Context(), jobID)
	if err != nil {
		h.logger.WithError(err).WithField("job_id", jobID).Warn("Job not found")
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"code":    "JOB_NOT_FOUND",
				"message": "Job not found",
			},
		})
		return
	}

	// Convert to API response format
	response := models.ContentJobStatus{
		JobID:          job.JobID,
		Status:         job.Status,
		Progress:       job.Progress,
		TotalItems:     job.TotalItems,
		ProcessedItems: job.ProcessedItems,
		FailedItems:    job.FailedItems,
		EstimatedTime:  job.EstimatedTime,
		ErrorMessage:   job.ErrorMessage,
		CreatedAt:      job.CreatedAt,
		UpdatedAt:      job.UpdatedAt,
	}

	c.JSON(http.StatusOK, response)
}
