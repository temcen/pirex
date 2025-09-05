package handlers

import (
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

	"github.com/temcen/pirex/pkg/models"
)

// Extend the mock service for user-specific methods
func (m *MockUserInteractionService) GetUserInteractions(ctx context.Context, userID uuid.UUID, interactionType string, limit, offset int, startDate, endDate *time.Time) ([]models.UserInteraction, int, error) {
	args := m.Called(ctx, userID, interactionType, limit, offset, startDate, endDate)
	return args.Get(0).([]models.UserInteraction), args.Int(1), args.Error(2)
}

func (m *MockUserInteractionService) GetUserProfile(ctx context.Context, userID uuid.UUID) (*models.UserProfile, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(*models.UserProfile), args.Error(1)
}

func (m *MockUserInteractionService) GetSimilarUsers(ctx context.Context, userID uuid.UUID, limit int) ([]models.SimilarUser, error) {
	args := m.Called(ctx, userID, limit)
	return args.Get(0).([]models.SimilarUser), args.Error(1)
}

func TestUserHandler_GetInteractions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tests := []struct {
		name           string
		userID         string
		queryParams    map[string]string
		mockSetup      func(*MockUserInteractionService)
		expectedStatus int
		expectedError  string
	}{
		{
			name:   "valid request with pagination",
			userID: uuid.New().String(),
			queryParams: map[string]string{
				"limit":  "10",
				"offset": "0",
				"type":   "click",
			},
			mockSetup: func(m *MockUserInteractionService) {
				interactions := []models.UserInteraction{
					{
						ID:              uuid.New(),
						UserID:          uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
						InteractionType: "click",
						Timestamp:       time.Now(),
					},
				}
				m.On("GetUserInteractions", mock.Anything, mock.AnythingOfType("uuid.UUID"), "click", 10, 0, (*time.Time)(nil), (*time.Time)(nil)).
					Return(interactions, 1, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "invalid user ID",
			userID: "invalid-uuid",
			mockSetup: func(m *MockUserInteractionService) {
				// No mock setup needed as validation fails before service call
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "INVALID_USER_ID",
		},
		{
			name:   "valid request with date filters",
			userID: uuid.New().String(),
			queryParams: map[string]string{
				"start_date": "2023-01-01",
				"end_date":   "2023-12-31",
			},
			mockSetup: func(m *MockUserInteractionService) {
				interactions := []models.UserInteraction{}
				m.On("GetUserInteractions", mock.Anything, mock.AnythingOfType("uuid.UUID"), "", 50, 0, mock.AnythingOfType("*time.Time"), mock.AnythingOfType("*time.Time")).
					Return(interactions, 0, nil)
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockUserInteractionService)
			tt.mockSetup(mockService)

			handler := NewUserHandler(logger, mockService)

			// Create request
			req, _ := http.NewRequest("GET", "/api/v1/users/"+tt.userID+"/interactions", nil)

			// Add query parameters
			q := req.URL.Query()
			for key, value := range tt.queryParams {
				q.Add(key, value)
			}
			req.URL.RawQuery = q.Encode()

			// Create response recorder
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req
			c.Params = []gin.Param{{Key: "userId", Value: tt.userID}}

			// Call handler
			handler.GetInteractions(c)

			// Assert response
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedError != "" {
				var response map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &response)
				errorObj := response["error"].(map[string]interface{})
				assert.Equal(t, tt.expectedError, errorObj["code"])
			} else if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &response)
				assert.Contains(t, response, "data")
				dataObj := response["data"].(map[string]interface{})
				assert.Contains(t, dataObj, "interactions")
				assert.Contains(t, dataObj, "pagination")
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestUserHandler_GetProfile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tests := []struct {
		name           string
		userID         string
		mockSetup      func(*MockUserInteractionService)
		expectedStatus int
		expectedError  string
	}{
		{
			name:   "valid user profile request",
			userID: uuid.New().String(),
			mockSetup: func(m *MockUserInteractionService) {
				profile := &models.UserProfile{
					UserID:           uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
					PreferenceVector: make([]float32, 768),
					ExplicitPrefs:    make(map[string]interface{}),
					BehaviorPatterns: make(map[string]interface{}),
					InteractionCount: 10,
					CreatedAt:        time.Now(),
					UpdatedAt:        time.Now(),
				}
				m.On("GetUserProfile", mock.Anything, mock.AnythingOfType("uuid.UUID")).
					Return(profile, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "invalid user ID",
			userID: "invalid-uuid",
			mockSetup: func(m *MockUserInteractionService) {
				// No mock setup needed as validation fails before service call
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "INVALID_USER_ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockUserInteractionService)
			tt.mockSetup(mockService)

			handler := NewUserHandler(logger, mockService)

			// Create request
			req, _ := http.NewRequest("GET", "/api/v1/users/"+tt.userID+"/profile", nil)

			// Create response recorder
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req
			c.Params = []gin.Param{{Key: "userId", Value: tt.userID}}

			// Call handler
			handler.GetProfile(c)

			// Assert response
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedError != "" {
				var response map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &response)
				errorObj := response["error"].(map[string]interface{})
				assert.Equal(t, tt.expectedError, errorObj["code"])
			} else if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &response)
				assert.Contains(t, response, "data")
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestUserHandler_GetSimilarUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tests := []struct {
		name           string
		userID         string
		queryParams    map[string]string
		mockSetup      func(*MockUserInteractionService)
		expectedStatus int
		expectedError  string
	}{
		{
			name:   "valid similar users request",
			userID: uuid.New().String(),
			queryParams: map[string]string{
				"limit": "5",
			},
			mockSetup: func(m *MockUserInteractionService) {
				similarUsers := []models.SimilarUser{
					{
						UserID:          uuid.New(),
						SimilarityScore: 0.85,
						Basis:           "collaborative_filtering",
						SharedItems:     5,
					},
				}
				m.On("GetSimilarUsers", mock.Anything, mock.AnythingOfType("uuid.UUID"), 5).
					Return(similarUsers, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "invalid user ID",
			userID: "invalid-uuid",
			mockSetup: func(m *MockUserInteractionService) {
				// No mock setup needed as validation fails before service call
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "INVALID_USER_ID",
		},
		{
			name:   "default limit",
			userID: uuid.New().String(),
			mockSetup: func(m *MockUserInteractionService) {
				similarUsers := []models.SimilarUser{}
				m.On("GetSimilarUsers", mock.Anything, mock.AnythingOfType("uuid.UUID"), 10).
					Return(similarUsers, nil)
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockUserInteractionService)
			tt.mockSetup(mockService)

			handler := NewUserHandler(logger, mockService)

			// Create request
			req, _ := http.NewRequest("GET", "/api/v1/users/"+tt.userID+"/similar", nil)

			// Add query parameters
			q := req.URL.Query()
			for key, value := range tt.queryParams {
				q.Add(key, value)
			}
			req.URL.RawQuery = q.Encode()

			// Create response recorder
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req
			c.Params = []gin.Param{{Key: "userId", Value: tt.userID}}

			// Call handler
			handler.GetSimilarUsers(c)

			// Assert response
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedError != "" {
				var response map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &response)
				errorObj := response["error"].(map[string]interface{})
				assert.Equal(t, tt.expectedError, errorObj["code"])
			} else if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &response)
				assert.Contains(t, response, "data")
				dataObj := response["data"].(map[string]interface{})
				assert.Contains(t, dataObj, "similar_users")
				assert.Contains(t, dataObj, "count")
			}

			mockService.AssertExpectations(t)
		})
	}
}
