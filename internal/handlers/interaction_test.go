package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/temcen/pirex/pkg/models"
)

// MockUserInteractionService is a mock implementation for testing
type MockUserInteractionService struct {
	mock.Mock
}

func (m *MockUserInteractionService) RecordExplicitInteraction(ctx context.Context, req *models.ExplicitInteractionRequest) (*models.UserInteraction, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*models.UserInteraction), args.Error(1)
}

func (m *MockUserInteractionService) RecordImplicitInteraction(ctx context.Context, req *models.ImplicitInteractionRequest) (*models.UserInteraction, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*models.UserInteraction), args.Error(1)
}

func (m *MockUserInteractionService) RecordBatchInteractions(ctx context.Context, req *models.InteractionBatchRequest) ([]models.UserInteraction, error) {
	args := m.Called(ctx, req)
	return args.Get(0).([]models.UserInteraction), args.Error(1)
}

func TestInteractionHandler_RecordExplicit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tests := []struct {
		name           string
		requestBody    interface{}
		mockSetup      func(*MockUserInteractionService)
		expectedStatus int
		expectedError  string
	}{
		{
			name: "valid rating interaction",
			requestBody: models.ExplicitInteractionRequest{
				UserID:    uuid.New(),
				ItemID:    uuid.New(),
				Type:      "rating",
				Value:     func() *float64 { v := 4.5; return &v }(),
				SessionID: uuid.New(),
			},
			mockSetup: func(m *MockUserInteractionService) {
				m.On("RecordExplicitInteraction", mock.Anything, mock.AnythingOfType("*models.ExplicitInteractionRequest")).
					Return(&models.UserInteraction{
						ID:              uuid.New(),
						InteractionType: "rating",
					}, nil)
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "invalid request body",
			requestBody: map[string]interface{}{
				"invalid": "data",
			},
			mockSetup:      func(m *MockUserInteractionService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "VALIDATION_FAILED",
		},
		{
			name: "rating without value",
			requestBody: models.ExplicitInteractionRequest{
				UserID:    uuid.New(),
				ItemID:    uuid.New(),
				Type:      "rating",
				SessionID: uuid.New(),
			},
			mockSetup:      func(m *MockUserInteractionService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "INVALID_RATING",
		},
		{
			name: "valid like interaction",
			requestBody: models.ExplicitInteractionRequest{
				UserID:    uuid.New(),
				ItemID:    uuid.New(),
				Type:      "like",
				SessionID: uuid.New(),
			},
			mockSetup: func(m *MockUserInteractionService) {
				m.On("RecordExplicitInteraction", mock.Anything, mock.AnythingOfType("*models.ExplicitInteractionRequest")).
					Return(&models.UserInteraction{
						ID:              uuid.New(),
						InteractionType: "like",
					}, nil)
			},
			expectedStatus: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockUserInteractionService)
			tt.mockSetup(mockService)

			handler := NewInteractionHandler(logger, mockService)

			// Create request
			body, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest("POST", "/api/v1/interactions/explicit", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			// Call handler
			handler.RecordExplicit(c)

			// Assert response
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedError != "" {
				var response map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &response)
				errorObj := response["error"].(map[string]interface{})
				assert.Equal(t, tt.expectedError, errorObj["code"])
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestInteractionHandler_RecordImplicit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tests := []struct {
		name           string
		requestBody    interface{}
		mockSetup      func(*MockUserInteractionService)
		expectedStatus int
		expectedError  string
	}{
		{
			name: "valid click interaction",
			requestBody: models.ImplicitInteractionRequest{
				UserID:    uuid.New(),
				ItemID:    func() *uuid.UUID { id := uuid.New(); return &id }(),
				Type:      "click",
				SessionID: uuid.New(),
			},
			mockSetup: func(m *MockUserInteractionService) {
				m.On("RecordImplicitInteraction", mock.Anything, mock.AnythingOfType("*models.ImplicitInteractionRequest")).
					Return(&models.UserInteraction{
						ID:              uuid.New(),
						InteractionType: "click",
					}, nil)
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "valid search interaction",
			requestBody: models.ImplicitInteractionRequest{
				UserID:    uuid.New(),
				Type:      "search",
				Query:     func() *string { q := "test query"; return &q }(),
				SessionID: uuid.New(),
			},
			mockSetup: func(m *MockUserInteractionService) {
				m.On("RecordImplicitInteraction", mock.Anything, mock.AnythingOfType("*models.ImplicitInteractionRequest")).
					Return(&models.UserInteraction{
						ID:              uuid.New(),
						InteractionType: "search",
					}, nil)
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "search without query",
			requestBody: models.ImplicitInteractionRequest{
				UserID:    uuid.New(),
				Type:      "search",
				SessionID: uuid.New(),
			},
			mockSetup:      func(m *MockUserInteractionService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "MISSING_QUERY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockUserInteractionService)
			tt.mockSetup(mockService)

			handler := NewInteractionHandler(logger, mockService)

			// Create request
			body, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest("POST", "/api/v1/interactions/implicit", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			// Call handler
			handler.RecordImplicit(c)

			// Assert response
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedError != "" {
				var response map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &response)
				errorObj := response["error"].(map[string]interface{})
				assert.Equal(t, tt.expectedError, errorObj["code"])
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestInteractionHandler_RecordBatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tests := []struct {
		name           string
		requestBody    interface{}
		mockSetup      func(*MockUserInteractionService)
		expectedStatus int
		expectedError  string
	}{
		{
			name: "valid batch with mixed interactions",
			requestBody: models.InteractionBatchRequest{
				ExplicitInteractions: []models.ExplicitInteractionRequest{
					{
						UserID:    uuid.New(),
						ItemID:    uuid.New(),
						Type:      "like",
						SessionID: uuid.New(),
					},
				},
				ImplicitInteractions: []models.ImplicitInteractionRequest{
					{
						UserID:    uuid.New(),
						ItemID:    func() *uuid.UUID { id := uuid.New(); return &id }(),
						Type:      "click",
						SessionID: uuid.New(),
					},
				},
			},
			mockSetup: func(m *MockUserInteractionService) {
				m.On("RecordBatchInteractions", mock.Anything, mock.AnythingOfType("*models.InteractionBatchRequest")).
					Return([]models.UserInteraction{
						{ID: uuid.New(), InteractionType: "like"},
						{ID: uuid.New(), InteractionType: "click"},
					}, nil)
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "empty batch",
			requestBody: models.InteractionBatchRequest{
				ExplicitInteractions: []models.ExplicitInteractionRequest{},
				ImplicitInteractions: []models.ImplicitInteractionRequest{},
			},
			mockSetup:      func(m *MockUserInteractionService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "EMPTY_BATCH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockUserInteractionService)
			tt.mockSetup(mockService)

			handler := NewInteractionHandler(logger, mockService)

			// Create request
			body, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest("POST", "/api/v1/interactions/batch", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			// Call handler
			handler.RecordBatch(c)

			// Assert response
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedError != "" {
				var response map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &response)
				errorObj := response["error"].(map[string]interface{})
				assert.Equal(t, tt.expectedError, errorObj["code"])
			}

			mockService.AssertExpectations(t)
		})
	}
}
func (m *MockUserInteractionService) Stop() {
	m.Called()
}
