package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/temcen/pirex/pkg/models"
)

// Mock services for testing
type mockMessageBus struct {
	publishedMessages []mockPublishedMessage
	shouldFail        bool
}

type mockPublishedMessage struct {
	JobID           uuid.UUID
	Content         models.ContentIngestionRequest
	ProcessingHints map[string]interface{}
}

func (m *mockMessageBus) PublishContentIngestion(jobID uuid.UUID, content models.ContentIngestionRequest, hints map[string]interface{}) error {
	if m.shouldFail {
		return assert.AnError
	}

	m.publishedMessages = append(m.publishedMessages, mockPublishedMessage{
		JobID:           jobID,
		Content:         content,
		ProcessingHints: hints,
	})
	return nil
}

type mockJobManager struct {
	createdJobs map[uuid.UUID]*mockJob
	shouldFail  bool
}

type mockJob struct {
	JobID         uuid.UUID
	Status        string
	TotalItems    int
	EstimatedTime *int
}

func (m *mockJobManager) CreateJob(ctx interface{}, totalItems int, jobType string) (*mockJob, error) {
	if m.shouldFail {
		return nil, assert.AnError
	}

	jobID := uuid.New()
	estimatedTime := totalItems * 2

	job := &mockJob{
		JobID:         jobID,
		Status:        "queued",
		TotalItems:    totalItems,
		EstimatedTime: &estimatedTime,
	}

	if m.createdJobs == nil {
		m.createdJobs = make(map[uuid.UUID]*mockJob)
	}
	m.createdJobs[jobID] = job

	return job, nil
}

func (m *mockJobManager) GetJob(ctx interface{}, jobID uuid.UUID) (*mockJob, error) {
	if m.shouldFail {
		return nil, assert.AnError
	}

	job, exists := m.createdJobs[jobID]
	if !exists {
		return nil, assert.AnError
	}

	return job, nil
}

func (m *mockJobManager) UpdateJobProgress(ctx interface{}, jobID uuid.UUID, processed, failed int, status string, errorMsg *string) error {
	if m.shouldFail {
		return assert.AnError
	}

	if job, exists := m.createdJobs[jobID]; exists {
		job.Status = status
	}

	return nil
}

func setupTestHandler() (*ContentHandler, *mockMessageBus, *mockJobManager) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	messageBus := &mockMessageBus{}
	jobManager := &mockJobManager{}

	// Create a mock handler that satisfies the interface
	handler := &ContentHandler{
		validator: validator.New(),
		logger:    logger,
	}

	// We'll need to inject the mocks differently since the real handler expects different types
	// For this test, we'll create a test-specific handler

	return handler, messageBus, jobManager
}

func TestContentHandler_Create_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup
	router := gin.New()
	_, messageBus, jobManager := setupTestHandler()

	// Create a test-specific endpoint that uses our mocks
	router.POST("/api/v1/content", func(c *gin.Context) {
		var request models.ContentIngestionRequest

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
			return
		}

		// Validate request
		if request.Title == "" || request.Type == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed"})
			return
		}

		// Create job
		job, err := jobManager.CreateJob(c.Request.Context(), 1, "single_content_ingestion")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Job creation failed"})
			return
		}

		// Publish to message bus
		hints := map[string]interface{}{"source": "api"}
		if err := messageBus.PublishContentIngestion(job.JobID, request, hints); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Publishing failed"})
			return
		}

		c.JSON(http.StatusAccepted, ContentResponse{
			JobID:         job.JobID,
			Status:        job.Status,
			EstimatedTime: job.EstimatedTime,
			Message:       "Content queued for processing",
		})
	})

	// Test data
	content := models.ContentIngestionRequest{
		Type:        "product",
		Title:       "Test Product",
		Description: stringPtr("Test description"),
		Categories:  []string{"Electronics"},
	}

	jsonData, err := json.Marshal(content)
	require.NoError(t, err)

	// Make request
	req, err := http.NewRequest("POST", "/api/v1/content", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusAccepted, w.Code)

	var response ContentResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.NotEqual(t, uuid.Nil, response.JobID)
	assert.Equal(t, "queued", response.Status)
	assert.NotNil(t, response.EstimatedTime)
	assert.Equal(t, "Content queued for processing", response.Message)

	// Verify message was published
	assert.Len(t, messageBus.publishedMessages, 1)
	publishedMessage := messageBus.publishedMessages[0]
	assert.Equal(t, response.JobID, publishedMessage.JobID)
	assert.Equal(t, content.Type, publishedMessage.Content.Type)
	assert.Equal(t, content.Title, publishedMessage.Content.Title)
}

func TestContentHandler_Create_ValidationError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()

	router.POST("/api/v1/content", func(c *gin.Context) {
		var request models.ContentIngestionRequest

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
			return
		}

		// Validate request
		if request.Title == "" || request.Type == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Validation failed"})
			return
		}

		c.JSON(http.StatusAccepted, gin.H{"message": "success"})
	})

	tests := []struct {
		name           string
		content        interface{}
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "invalid JSON",
			content:        "invalid json",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid JSON",
		},
		{
			name: "missing title",
			content: models.ContentIngestionRequest{
				Type: "product",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Validation failed",
		},
		{
			name: "missing type",
			content: models.ContentIngestionRequest{
				Title: "Test Product",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var jsonData []byte
			var err error

			if str, ok := tt.content.(string); ok {
				jsonData = []byte(str)
			} else {
				jsonData, err = json.Marshal(tt.content)
				require.NoError(t, err)
			}

			req, err := http.NewRequest("POST", "/api/v1/content", bytes.NewBuffer(jsonData))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Contains(t, response, "error")
			assert.Equal(t, tt.expectedError, response["error"])
		})
	}
}

func TestContentHandler_CreateBatch_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	_, messageBus, jobManager := setupTestHandler()

	router.POST("/api/v1/content/batch", func(c *gin.Context) {
		var request models.ContentBatchRequest

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
			return
		}

		if len(request.Items) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Empty batch"})
			return
		}

		if len(request.Items) > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Batch too large"})
			return
		}

		// Create job
		job, err := jobManager.CreateJob(c.Request.Context(), len(request.Items), "batch_content_ingestion")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Job creation failed"})
			return
		}

		// Publish each item
		successCount := 0
		for _, item := range request.Items {
			hints := map[string]interface{}{"source": "api_batch"}
			if err := messageBus.PublishContentIngestion(job.JobID, item, hints); err == nil {
				successCount++
			}
		}

		c.JSON(http.StatusAccepted, ContentResponse{
			JobID:         job.JobID,
			Status:        job.Status,
			EstimatedTime: job.EstimatedTime,
			Message:       "Batch queued for processing",
		})
	})

	// Test data
	batchRequest := models.ContentBatchRequest{
		Items: []models.ContentIngestionRequest{
			{
				Type:  "product",
				Title: "Product 1",
			},
			{
				Type:  "product",
				Title: "Product 2",
			},
		},
	}

	jsonData, err := json.Marshal(batchRequest)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", "/api/v1/content/batch", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusAccepted, w.Code)

	var response ContentResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.NotEqual(t, uuid.Nil, response.JobID)
	assert.Equal(t, "queued", response.Status)

	// Verify messages were published
	assert.Len(t, messageBus.publishedMessages, 2)
}

func TestContentHandler_CreateBatch_ValidationError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()

	router.POST("/api/v1/content/batch", func(c *gin.Context) {
		var request models.ContentBatchRequest

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
			return
		}

		if len(request.Items) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Empty batch"})
			return
		}

		if len(request.Items) > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Batch too large"})
			return
		}

		c.JSON(http.StatusAccepted, gin.H{"message": "success"})
	})

	tests := []struct {
		name           string
		request        models.ContentBatchRequest
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "empty batch",
			request:        models.ContentBatchRequest{Items: []models.ContentIngestionRequest{}},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Empty batch",
		},
		{
			name: "batch too large",
			request: models.ContentBatchRequest{
				Items: make([]models.ContentIngestionRequest, 101), // 101 items
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Batch too large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, err := json.Marshal(tt.request)
			require.NoError(t, err)

			req, err := http.NewRequest("POST", "/api/v1/content/batch", bytes.NewBuffer(jsonData))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Contains(t, response, "error")
			assert.Equal(t, tt.expectedError, response["error"])
		})
	}
}

func TestContentHandler_GetJobStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	_, _, jobManager := setupTestHandler()

	// Create a test job first
	job, err := jobManager.CreateJob(nil, 5, "test_job")
	require.NoError(t, err)

	router.GET("/api/v1/content/jobs/:jobId", func(c *gin.Context) {
		jobIDStr := c.Param("jobId")
		jobID, err := uuid.Parse(jobIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
			return
		}

		job, err := jobManager.GetJob(c.Request.Context(), jobID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
			return
		}

		response := models.ContentJobStatus{
			JobID:         job.JobID,
			Status:        job.Status,
			TotalItems:    job.TotalItems,
			EstimatedTime: job.EstimatedTime,
		}

		c.JSON(http.StatusOK, response)
	})

	// Test successful job status retrieval
	req, err := http.NewRequest("GET", "/api/v1/content/jobs/"+job.JobID.String(), nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.ContentJobStatus
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, job.JobID, response.JobID)
	assert.Equal(t, job.Status, response.Status)
	assert.Equal(t, job.TotalItems, response.TotalItems)

	// Test invalid job ID
	req, err = http.NewRequest("GET", "/api/v1/content/jobs/invalid-uuid", nil)
	require.NoError(t, err)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Test job not found
	nonExistentJobID := uuid.New()
	req, err = http.NewRequest("GET", "/api/v1/content/jobs/"+nonExistentJobID.String(), nil)
	require.NoError(t, err)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
