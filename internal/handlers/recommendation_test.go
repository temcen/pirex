package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/temcen/pirex/internal/services"
	"github.com/temcen/pirex/pkg/models"
)

// MockRecommendationOrchestrator is a mock implementation
type MockRecommendationOrchestrator struct {
	mock.Mock
}

func (m *MockRecommendationOrchestrator) GenerateRecommendations(ctx context.Context, reqCtx *services.RecommendationContext) (*services.OrchestrationResult, error) {
	args := m.Called(ctx, reqCtx)
	return args.Get(0).(*services.OrchestrationResult), args.Error(1)
}

func (m *MockRecommendationOrchestrator) ProcessFeedback(ctx context.Context, feedback *models.RecommendationFeedback) error {
	args := m.Called(ctx, feedback)
	return args.Error(0)
}

func TestRecommendationHandler_Get(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise in tests

	mockOrchestrator := new(MockRecommendationOrchestrator)
	handler := NewRecommendationHandler(mockOrchestrator, logger)

	userID := uuid.New()

	// Mock response
	mockResult := &services.OrchestrationResult{
		UserID: userID,
		Recommendations: []models.Recommendation{
			{
				ItemID:     uuid.New(),
				Score:      0.95,
				Algorithm:  "semantic_search",
				Confidence: 0.9,
				Position:   1,
			},
			{
				ItemID:     uuid.New(),
				Score:      0.87,
				Algorithm:  "collaborative_filtering",
				Confidence: 0.8,
				Position:   2,
			},
		},
		Context:     "home",
		CacheHit:    false,
		GeneratedAt: time.Now(),
	}

	// Set up mock expectations for both test cases
	mockOrchestrator.On("GenerateRecommendations", mock.Anything, mock.MatchedBy(func(reqCtx *services.RecommendationContext) bool {
		return reqCtx.UserID == userID && (reqCtx.Count == 10 || reqCtx.Count == 5) && (reqCtx.Context == "home" || reqCtx.Context == "search")
	})).Return(mockResult, nil)

	// Test cases
	tests := []struct {
		name           string
		userID         string
		queryParams    string
		expectedStatus int
		expectedCount  int
	}{
		{
			name:           "Valid request with default parameters",
			userID:         userID.String(),
			queryParams:    "",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name:           "Valid request with custom count",
			userID:         userID.String(),
			queryParams:    "?count=5&context=search&explain=true",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name:           "Invalid user ID",
			userID:         "invalid-uuid",
			queryParams:    "",
			expectedStatus: http.StatusBadRequest,
			expectedCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup router
			router := gin.New()
			router.GET("/api/v1/recommendations/:userId", handler.Get)

			// Create request
			req, _ := http.NewRequest("GET", "/api/v1/recommendations/"+tt.userID+tt.queryParams, nil)
			w := httptest.NewRecorder()

			// Execute request
			router.ServeHTTP(w, req)

			// Assert status code
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				// Parse response
				var response models.RecommendationResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)

				// Assert response content
				assert.Equal(t, userID, response.UserID)
				assert.Equal(t, tt.expectedCount, len(response.Recommendations))
				assert.Equal(t, "home", response.Context)
				assert.False(t, response.CacheHit)
			}
		})
	}

	mockOrchestrator.AssertExpectations(t)
}

func TestRecommendationHandler_GetBatch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mockOrchestrator := new(MockRecommendationOrchestrator)
	handler := NewRecommendationHandler(mockOrchestrator, logger)

	userID1 := uuid.New()
	userID2 := uuid.New()

	// Mock responses
	mockResult1 := &services.OrchestrationResult{
		UserID:          userID1,
		Recommendations: []models.Recommendation{{ItemID: uuid.New(), Score: 0.9, Algorithm: "test", Confidence: 0.8, Position: 1}},
		Context:         "home",
		GeneratedAt:     time.Now(),
	}

	mockResult2 := &services.OrchestrationResult{
		UserID:          userID2,
		Recommendations: []models.Recommendation{{ItemID: uuid.New(), Score: 0.8, Algorithm: "test", Confidence: 0.7, Position: 1}},
		Context:         "search",
		GeneratedAt:     time.Now(),
	}

	mockOrchestrator.On("GenerateRecommendations", mock.Anything, mock.MatchedBy(func(reqCtx *services.RecommendationContext) bool {
		return reqCtx.UserID == userID1
	})).Return(mockResult1, nil)

	mockOrchestrator.On("GenerateRecommendations", mock.Anything, mock.MatchedBy(func(reqCtx *services.RecommendationContext) bool {
		return reqCtx.UserID == userID2
	})).Return(mockResult2, nil)

	// Test cases
	tests := []struct {
		name           string
		requestBody    models.BatchRecommendationRequest
		expectedStatus int
		expectedCount  int
	}{
		{
			name: "Valid batch request",
			requestBody: models.BatchRecommendationRequest{
				Requests: []models.RecommendationRequest{
					{UserID: userID1, Count: 10, Context: "home"},
					{UserID: userID2, Count: 5, Context: "search"},
				},
			},
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name: "Empty batch request",
			requestBody: models.BatchRecommendationRequest{
				Requests: []models.RecommendationRequest{},
			},
			expectedStatus: http.StatusBadRequest,
			expectedCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup router
			router := gin.New()
			router.POST("/api/v1/recommendations/batch", handler.GetBatch)

			// Create request body
			body, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest("POST", "/api/v1/recommendations/batch", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Execute request
			router.ServeHTTP(w, req)

			// Assert status code
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				// Parse response
				var response models.BatchRecommendationResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)

				// Assert response content
				assert.Equal(t, tt.expectedCount, len(response.Responses))
			}
		})
	}

	mockOrchestrator.AssertExpectations(t)
}

func TestRecommendationHandler_RecordFeedback(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mockOrchestrator := new(MockRecommendationOrchestrator)
	handler := NewRecommendationHandler(mockOrchestrator, logger)

	userID := uuid.New()
	itemID := uuid.New()
	recommendationID := uuid.New()

	mockOrchestrator.On("ProcessFeedback", mock.Anything, mock.MatchedBy(func(feedback *models.RecommendationFeedback) bool {
		return feedback.UserID == userID && feedback.ItemID == itemID && feedback.FeedbackType == "positive"
	})).Return(nil)

	// Test cases
	tests := []struct {
		name           string
		requestBody    models.RecommendationFeedback
		expectedStatus int
	}{
		{
			name: "Valid feedback",
			requestBody: models.RecommendationFeedback{
				UserID:           userID,
				RecommendationID: recommendationID,
				ItemID:           itemID,
				FeedbackType:     "positive",
				Timestamp:        time.Now(),
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "Invalid feedback type",
			requestBody: models.RecommendationFeedback{
				UserID:           userID,
				RecommendationID: recommendationID,
				ItemID:           itemID,
				FeedbackType:     "invalid_type",
				Timestamp:        time.Now(),
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup router
			router := gin.New()
			router.POST("/api/v1/feedback", handler.RecordFeedback)

			// Create request body
			body, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest("POST", "/api/v1/feedback", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Execute request
			router.ServeHTTP(w, req)

			// Assert status code
			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}

	mockOrchestrator.AssertExpectations(t)
}
