package testing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/temcen/pirex/internal/docs"
	"github.com/temcen/pirex/internal/middleware"
	"github.com/temcen/pirex/internal/validation"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAPIContractCompliance tests that the API implementation matches the OpenAPI specification
func TestAPIContractCompliance(t *testing.T) {
	// Initialize validator
	validator := validation.NewSchemaValidator()
	err := validator.LoadSchemas("../../docs/api/schemas")
	require.NoError(t, err, "Failed to load validation schemas")

	// Setup test router with validation middleware
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add validation middleware
	validationMiddleware := middleware.NewValidationMiddleware(validator)
	router.Use(validationMiddleware.ValidateHeaders())
	router.Use(validationMiddleware.ValidateQueryParams())

	// Setup documentation routes
	swaggerConfig := docs.GetSwaggerConfig()
	swaggerHandler := docs.NewSwaggerHandler(swaggerConfig, "../../docs/api/openapi.yaml")
	swaggerHandler.RegisterRoutes(router)

	// Add mock API routes for testing
	setupMockAPIRoutes(router, validationMiddleware)

	// Create contract tester
	contractTester := NewContractTester(validator, router)

	// Run contract tests
	t.Run("ContentItemContracts", func(t *testing.T) {
		testCases := contractTester.ContentItemContractTests()
		contractTester.APIContractTest(t, testCases)
	})

	t.Run("UserInteractionContracts", func(t *testing.T) {
		testCases := contractTester.UserInteractionContractTests()
		contractTester.APIContractTest(t, testCases)
	})

	t.Run("RecommendationContracts", func(t *testing.T) {
		testCases := contractTester.RecommendationContractTests()
		contractTester.APIContractTest(t, testCases)
	})

	t.Run("OpenAPICompliance", func(t *testing.T) {
		contractTester.ValidateOpenAPICompliance(t, "../../docs/api/openapi.yaml")
	})
}

// TestSchemaValidation tests JSON schema validation functionality
func TestSchemaValidation(t *testing.T) {
	validator := validation.NewSchemaValidator()
	err := validator.LoadSchemas("../../docs/api/schemas")
	require.NoError(t, err, "Failed to load validation schemas")

	t.Run("ValidContentItem", func(t *testing.T) {
		validContent := map[string]interface{}{
			"id":          "test-product-123",
			"type":        "product",
			"title":       "Test Product",
			"description": "A test product for validation",
			"imageUrls":   []string{"https://example.com/image.jpg"},
			"categories":  []string{"electronics", "test"},
			"metadata": map[string]interface{}{
				"price": 99.99,
				"brand": "TestBrand",
			},
		}

		result := validator.ValidateContentItem(validContent)
		assert.True(t, result.Valid, "Valid content item should pass validation: %v", result.Errors)
	})

	t.Run("InvalidContentItem", func(t *testing.T) {
		invalidContent := map[string]interface{}{
			"id":   "test-invalid-123",
			"type": "invalid_type", // Invalid type
			// Missing required fields: title, categories
		}

		result := validator.ValidateContentItem(invalidContent)
		assert.False(t, result.Valid, "Invalid content item should fail validation")
		assert.NotEmpty(t, result.Errors, "Should have validation errors")

		// Check for specific errors
		hasTypeError := false
		hasTitleError := false
		hasCategoriesError := false

		for _, err := range result.Errors {
			if strings.Contains(err.Field, "type") {
				hasTypeError = true
			}
			if strings.Contains(err.Field, "title") {
				hasTitleError = true
			}
			if strings.Contains(err.Field, "categories") {
				hasCategoriesError = true
			}
		}

		assert.True(t, hasTypeError, "Should have type validation error")
		assert.True(t, hasTitleError, "Should have title validation error")
		assert.True(t, hasCategoriesError, "Should have categories validation error")
	})

	t.Run("ValidUserInteraction", func(t *testing.T) {
		validInteraction := map[string]interface{}{
			"userId":          "550e8400-e29b-41d4-a716-446655440000",
			"itemId":          "test-product-123",
			"interactionType": "rating",
			"value":           4.5,
			"timestamp":       "2024-01-15T10:30:00Z",
			"sessionId":       "550e8400-e29b-41d4-a716-446655440001",
			"context": map[string]interface{}{
				"source": "product_page",
				"device": "mobile",
			},
		}

		result := validator.ValidateUserInteraction(validInteraction)
		assert.True(t, result.Valid, "Valid user interaction should pass validation: %v", result.Errors)
	})

	t.Run("InvalidUserInteraction", func(t *testing.T) {
		invalidInteraction := map[string]interface{}{
			"userId":          "invalid-uuid",
			"itemId":          "test-product-123",
			"interactionType": "invalid_type",
			"value":           10.0, // Invalid rating value (should be 1-5)
			"timestamp":       "invalid-timestamp",
		}

		result := validator.ValidateUserInteraction(invalidInteraction)
		assert.False(t, result.Valid, "Invalid user interaction should fail validation")
		assert.NotEmpty(t, result.Errors, "Should have validation errors")
	})

	t.Run("ValidRecommendation", func(t *testing.T) {
		validRecommendation := map[string]interface{}{
			"itemId":     "prod-789",
			"score":      0.85,
			"algorithm":  "hybrid",
			"confidence": 0.82,
			"position":   1,
			"explanation": map[string]interface{}{
				"type":    "content_based",
				"message": "Because you liked similar items",
				"evidence": map[string]interface{}{
					"similarItems": []string{"prod-123", "prod-456"},
					"confidence":   0.8,
				},
			},
		}

		result := validator.ValidateRecommendation(validRecommendation)
		assert.True(t, result.Valid, "Valid recommendation should pass validation: %v", result.Errors)
	})

	t.Run("ValidErrorResponse", func(t *testing.T) {
		validError := map[string]interface{}{
			"error": map[string]interface{}{
				"code":      "VALIDATION_ERROR",
				"message":   "Request validation failed",
				"timestamp": "2024-01-15T10:30:00Z",
				"requestId": "550e8400-e29b-41d4-a716-446655440000",
				"details": map[string]interface{}{
					"field": "type",
				},
			},
		}

		result := validator.ValidateErrorResponse(validError)
		assert.True(t, result.Valid, "Valid error response should pass validation: %v", result.Errors)
	})
}

// TestValidationMiddleware tests the validation middleware functionality
func TestValidationMiddleware(t *testing.T) {
	validator := validation.NewSchemaValidator()
	err := validator.LoadSchemas("../../docs/api/schemas")
	require.NoError(t, err, "Failed to load validation schemas")

	gin.SetMode(gin.TestMode)
	router := gin.New()

	validationMiddleware := middleware.NewValidationMiddleware(validator)

	// Test route with content item validation
	router.POST("/api/v1/content",
		validationMiddleware.ValidateHeaders(),
		validationMiddleware.ValidateContentItem(),
		func(c *gin.Context) {
			c.JSON(http.StatusAccepted, gin.H{"status": "accepted"})
		})

	// Test route with user interaction validation
	router.POST("/api/v1/interactions",
		validationMiddleware.ValidateHeaders(),
		validationMiddleware.ValidateUserInteraction(),
		func(c *gin.Context) {
			c.JSON(http.StatusCreated, gin.H{"status": "created"})
		})

	// Test route with query parameter validation
	router.GET("/api/v1/recommendations/:userId",
		validationMiddleware.ValidateQueryParams(),
		func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"recommendations": []interface{}{}})
		})

	t.Run("ValidContentItemRequest", func(t *testing.T) {
		validJSON := `{
			"id": "test-product-123",
			"type": "product",
			"title": "Test Product",
			"categories": ["electronics"]
		}`

		req := httptest.NewRequest("POST", "/api/v1/content", strings.NewReader(validJSON))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusAccepted, w.Code)
	})

	t.Run("InvalidContentItemRequest", func(t *testing.T) {
		invalidJSON := `{
			"id": "test-invalid-123",
			"type": "invalid_type"
		}`

		req := httptest.NewRequest("POST", "/api/v1/content", strings.NewReader(invalidJSON))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		errorObj, exists := response["error"]
		assert.True(t, exists, "Response should contain error object")

		if errorMap, ok := errorObj.(map[string]interface{}); ok {
			assert.Equal(t, "VALIDATION_ERROR", errorMap["code"])
			assert.NotEmpty(t, errorMap["message"])
		}
	})

	t.Run("MissingContentType", func(t *testing.T) {
		validJSON := `{"id": "test", "type": "product", "title": "Test", "categories": ["test"]}`

		req := httptest.NewRequest("POST", "/api/v1/content", strings.NewReader(validJSON))
		// Missing Content-Type header
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("InvalidQueryParameters", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/recommendations/invalid-uuid?count=999", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("ValidQueryParameters", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/recommendations/550e8400-e29b-41d4-a716-446655440000?count=10&context=home", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestDocumentationEndpoints tests that documentation endpoints are accessible
func TestDocumentationEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	swaggerConfig := docs.GetSwaggerConfig()
	swaggerHandler := docs.NewSwaggerHandler(swaggerConfig, "../../docs/api/openapi.yaml")
	swaggerHandler.RegisterRoutes(router)

	testCases := []struct {
		name           string
		path           string
		expectedStatus int
		expectedType   string
	}{
		{
			name:           "Swagger UI",
			path:           "/docs/",
			expectedStatus: http.StatusOK,
			expectedType:   "text/html",
		},
		{
			name:           "OpenAPI Spec",
			path:           "/docs/openapi.yaml",
			expectedStatus: http.StatusOK,
			expectedType:   "application/x-yaml",
		},
		{
			name:           "Schemas Page",
			path:           "/docs/schemas",
			expectedStatus: http.StatusOK,
			expectedType:   "text/html",
		},
		{
			name:           "Examples Page",
			path:           "/docs/examples",
			expectedStatus: http.StatusOK,
			expectedType:   "text/html",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code)
			if tc.expectedType != "" {
				contentType := w.Header().Get("Content-Type")
				assert.Contains(t, contentType, tc.expectedType)
			}
		})
	}
}

// setupMockAPIRoutes sets up mock API routes for contract testing
func setupMockAPIRoutes(router *gin.Engine, validationMiddleware *middleware.ValidationMiddleware) {
	api := router.Group("/api/v1")

	// Content management routes
	api.POST("/content",
		validationMiddleware.ValidateContentItem(),
		func(c *gin.Context) {
			c.JSON(http.StatusAccepted, gin.H{
				"jobId":         "550e8400-e29b-41d4-a716-446655440000",
				"status":        "queued",
				"estimatedTime": "30s",
				"message":       "Content queued for processing",
			})
		})

	api.POST("/content/batch", func(c *gin.Context) {
		c.JSON(http.StatusAccepted, gin.H{
			"batchId":    "550e8400-e29b-41d4-a716-446655440001",
			"status":     "queued",
			"totalItems": 1,
		})
	})

	api.GET("/content/jobs/:jobId", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"jobId":       c.Param("jobId"),
			"status":      "completed",
			"progress":    100,
			"completedAt": "2024-01-15T10:35:00Z",
		})
	})

	// User interaction routes
	api.POST("/interactions",
		validationMiddleware.ValidateUserInteraction(),
		func(c *gin.Context) {
			c.JSON(http.StatusCreated, gin.H{
				"interactionId": "550e8400-e29b-41d4-a716-446655440002",
				"status":        "recorded",
				"message":       "Interaction recorded successfully",
			})
		})

	api.POST("/interactions/batch", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{
			"batchId":       "550e8400-e29b-41d4-a716-446655440003",
			"totalRecorded": 1,
		})
	})

	// Recommendation routes
	api.GET("/recommendations/:userId", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"userId": c.Param("userId"),
			"recommendations": []gin.H{
				{
					"itemId":     "prod-789",
					"score":      0.85,
					"algorithm":  "hybrid",
					"confidence": 0.82,
					"position":   1,
				},
			},
			"metadata": gin.H{
				"totalAvailable": 150,
				"algorithmsUsed": []string{"semantic_search", "collaborative_filtering"},
				"generatedAt":    "2024-01-15T10:30:00Z",
				"cacheHit":       false,
			},
		})
	})

	api.POST("/recommendations/batch", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"results": []gin.H{
				{
					"userId":          "550e8400-e29b-41d4-a716-446655440000",
					"recommendations": []gin.H{},
				},
			},
		})
	})

	api.GET("/recommendations/:userId/similar/:itemId", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"userId":          c.Param("userId"),
			"recommendations": []gin.H{},
		})
	})

	// User routes
	api.GET("/users/:userId/interactions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"interactions": []gin.H{},
			"pagination": gin.H{
				"limit":   100,
				"offset":  0,
				"total":   0,
				"hasMore": false,
			},
		})
	})

	// Feedback routes
	api.POST("/feedback", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{
			"feedbackId": "550e8400-e29b-41d4-a716-446655440004",
			"status":     "recorded",
			"message":    "Feedback recorded successfully",
		})
	})
}

// BenchmarkValidation benchmarks the validation performance
func BenchmarkValidation(b *testing.B) {
	validator := validation.NewSchemaValidator()
	err := validator.LoadSchemas("../../docs/api/schemas")
	if err != nil {
		b.Fatalf("Failed to load schemas: %v", err)
	}

	contractTester := NewContractTester(validator, nil)
	contractTester.BenchmarkContractValidation(b)
}
