package testing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/temcen/pirex/internal/validation"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ContractTester provides utilities for API contract testing
type ContractTester struct {
	validator *validation.SchemaValidator
	router    *gin.Engine
}

// NewContractTester creates a new contract tester instance
func NewContractTester(validator *validation.SchemaValidator, router *gin.Engine) *ContractTester {
	return &ContractTester{
		validator: validator,
		router:    router,
	}
}

// TestCase represents a single API contract test case
type TestCase struct {
	Name           string
	Method         string
	Path           string
	Headers        map[string]string
	Body           interface{}
	ExpectedStatus int
	ExpectedSchema string
	ValidateBody   bool
}

// APIContractTest runs a comprehensive contract test suite
func (ct *ContractTester) APIContractTest(t *testing.T, testCases []TestCase) {
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			ct.runTestCase(t, tc)
		})
	}
}

// runTestCase executes a single test case
func (ct *ContractTester) runTestCase(t *testing.T, tc TestCase) {
	// Prepare request
	var bodyReader io.Reader
	if tc.Body != nil {
		bodyBytes, err := json.Marshal(tc.Body)
		require.NoError(t, err, "Failed to marshal request body")
		bodyReader = bytes.NewReader(bodyBytes)

		// Validate request body if schema validation is enabled
		if tc.ValidateBody && tc.ExpectedSchema != "" {
			result := ct.validator.ValidateStruct(tc.ExpectedSchema, tc.Body)
			assert.True(t, result.Valid, "Request body should be valid: %v", result.Errors)
		}
	}

	// Create HTTP request
	req := httptest.NewRequest(tc.Method, tc.Path, bodyReader)

	// Set headers
	if tc.Headers != nil {
		for key, value := range tc.Headers {
			req.Header.Set(key, value)
		}
	}

	// Set default Content-Type for requests with body
	if tc.Body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Execute request
	w := httptest.NewRecorder()
	ct.router.ServeHTTP(w, req)

	// Validate response status
	assert.Equal(t, tc.ExpectedStatus, w.Code,
		"Expected status %d, got %d. Response: %s",
		tc.ExpectedStatus, w.Code, w.Body.String())

	// Validate response headers
	ct.validateResponseHeaders(t, w)

	// Validate response body schema if specified
	if tc.ExpectedSchema != "" && w.Body.Len() > 0 {
		ct.validateResponseSchema(t, w.Body.String(), tc.ExpectedSchema)
	}

	// Validate error response format for error status codes
	if w.Code >= 400 {
		ct.validateErrorResponse(t, w.Body.String())
	}
}

// validateResponseHeaders validates common response headers
func (ct *ContractTester) validateResponseHeaders(t *testing.T, w *httptest.ResponseRecorder) {
	// Check Content-Type for JSON responses
	if w.Body.Len() > 0 {
		contentType := w.Header().Get("Content-Type")
		assert.True(t, strings.Contains(contentType, "application/json"),
			"Response Content-Type should be application/json, got: %s", contentType)
	}

	// Check for rate limiting headers if present
	if rateLimitRemaining := w.Header().Get("X-RateLimit-Remaining"); rateLimitRemaining != "" {
		assert.Regexp(t, `^\d+$`, rateLimitRemaining,
			"X-RateLimit-Remaining should be a number")
	}

	if rateLimitReset := w.Header().Get("X-RateLimit-Reset"); rateLimitReset != "" {
		assert.Regexp(t, `^\d+$`, rateLimitReset,
			"X-RateLimit-Reset should be a Unix timestamp")
	}
}

// validateResponseSchema validates response body against JSON schema
func (ct *ContractTester) validateResponseSchema(t *testing.T, responseBody, schemaName string) {
	result := ct.validator.ValidateJSONString(schemaName, responseBody)
	if !result.Valid {
		t.Errorf("Response validation failed for schema '%s':\n%s\nErrors: %v",
			schemaName, responseBody, result.Errors)
	}
}

// validateErrorResponse validates error response format
func (ct *ContractTester) validateErrorResponse(t *testing.T, responseBody string) {
	result := ct.validator.ValidateJSONString("error-response", responseBody)
	assert.True(t, result.Valid, "Error response should match error schema: %v", result.Errors)

	// Parse and validate error structure
	var errorResp map[string]interface{}
	err := json.Unmarshal([]byte(responseBody), &errorResp)
	require.NoError(t, err, "Error response should be valid JSON")

	// Validate required error fields
	errorObj, exists := errorResp["error"]
	assert.True(t, exists, "Error response should contain 'error' field")

	if errorMap, ok := errorObj.(map[string]interface{}); ok {
		assert.NotEmpty(t, errorMap["code"], "Error should have a code")
		assert.NotEmpty(t, errorMap["message"], "Error should have a message")
		assert.NotEmpty(t, errorMap["timestamp"], "Error should have a timestamp")
	}
}

// ContentItemContractTests returns test cases for content item endpoints
func (ct *ContractTester) ContentItemContractTests() []TestCase {
	return []TestCase{
		{
			Name:   "Valid content item creation",
			Method: "POST",
			Path:   "/api/v1/content",
			Body: map[string]interface{}{
				"id":          "test-product-123",
				"type":        "product",
				"title":       "Test Product",
				"description": "A test product for contract testing",
				"imageUrls":   []string{"https://example.com/image.jpg"},
				"categories":  []string{"electronics", "test"},
				"metadata": map[string]interface{}{
					"price": 99.99,
					"brand": "TestBrand",
				},
			},
			ExpectedStatus: 202,
			ExpectedSchema: "content-item",
			ValidateBody:   true,
		},
		{
			Name:   "Invalid content type",
			Method: "POST",
			Path:   "/api/v1/content",
			Body: map[string]interface{}{
				"id":         "test-invalid-123",
				"type":       "invalid_type",
				"title":      "Test Product",
				"categories": []string{"test"},
			},
			ExpectedStatus: 400,
			ExpectedSchema: "error-response",
		},
		{
			Name:   "Missing required fields",
			Method: "POST",
			Path:   "/api/v1/content",
			Body: map[string]interface{}{
				"id":   "test-missing-123",
				"type": "product",
				// Missing title and categories
			},
			ExpectedStatus: 400,
			ExpectedSchema: "error-response",
		},
	}
}

// UserInteractionContractTests returns test cases for user interaction endpoints
func (ct *ContractTester) UserInteractionContractTests() []TestCase {
	return []TestCase{
		{
			Name:   "Valid explicit interaction",
			Method: "POST",
			Path:   "/api/v1/interactions",
			Body: map[string]interface{}{
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
			},
			ExpectedStatus: 201,
			ValidateBody:   true,
		},
		{
			Name:   "Valid implicit interaction",
			Method: "POST",
			Path:   "/api/v1/interactions",
			Body: map[string]interface{}{
				"userId":          "550e8400-e29b-41d4-a716-446655440000",
				"itemId":          "test-product-123",
				"interactionType": "view",
				"value":           45.5,
				"timestamp":       "2024-01-15T10:30:00Z",
				"sessionId":       "550e8400-e29b-41d4-a716-446655440001",
				"context": map[string]interface{}{
					"source":   "search_results",
					"duration": 45.5,
					"position": 3,
				},
			},
			ExpectedStatus: 201,
			ValidateBody:   true,
		},
		{
			Name:   "Invalid user ID format",
			Method: "POST",
			Path:   "/api/v1/interactions",
			Body: map[string]interface{}{
				"userId":          "invalid-uuid",
				"itemId":          "test-product-123",
				"interactionType": "rating",
				"value":           4.5,
				"timestamp":       "2024-01-15T10:30:00Z",
			},
			ExpectedStatus: 400,
			ExpectedSchema: "error-response",
		},
	}
}

// RecommendationContractTests returns test cases for recommendation endpoints
func (ct *ContractTester) RecommendationContractTests() []TestCase {
	return []TestCase{
		{
			Name:           "Get recommendations with valid user ID",
			Method:         "GET",
			Path:           "/api/v1/recommendations/550e8400-e29b-41d4-a716-446655440000",
			ExpectedStatus: 200,
			Headers: map[string]string{
				"Authorization": "Bearer valid-jwt-token",
			},
		},
		{
			Name:           "Get recommendations with query parameters",
			Method:         "GET",
			Path:           "/api/v1/recommendations/550e8400-e29b-41d4-a716-446655440000?count=5&context=home&explain=true",
			ExpectedStatus: 200,
			Headers: map[string]string{
				"Authorization": "Bearer valid-jwt-token",
			},
		},
		{
			Name:           "Invalid user ID format in path",
			Method:         "GET",
			Path:           "/api/v1/recommendations/invalid-uuid",
			ExpectedStatus: 400,
			ExpectedSchema: "error-response",
		},
		{
			Name:           "Invalid count parameter",
			Method:         "GET",
			Path:           "/api/v1/recommendations/550e8400-e29b-41d4-a716-446655440000?count=999",
			ExpectedStatus: 400,
			ExpectedSchema: "error-response",
		},
	}
}

// RunFullContractTestSuite runs all contract tests
func (ct *ContractTester) RunFullContractTestSuite(t *testing.T) {
	t.Run("ContentItemContracts", func(t *testing.T) {
		ct.APIContractTest(t, ct.ContentItemContractTests())
	})

	t.Run("UserInteractionContracts", func(t *testing.T) {
		ct.APIContractTest(t, ct.UserInteractionContractTests())
	})

	t.Run("RecommendationContracts", func(t *testing.T) {
		ct.APIContractTest(t, ct.RecommendationContractTests())
	})
}

// ValidateOpenAPICompliance validates that the API matches OpenAPI specification
func (ct *ContractTester) ValidateOpenAPICompliance(t *testing.T, openAPISpecPath string) {
	// This would integrate with tools like openapi-generator or swagger-codegen
	// to validate that the actual API matches the OpenAPI specification

	// For now, we'll implement basic validation
	t.Run("OpenAPICompliance", func(t *testing.T) {
		// Test that all documented endpoints are accessible
		endpoints := []struct {
			method string
			path   string
		}{
			{"POST", "/api/v1/content"},
			{"POST", "/api/v1/content/batch"},
			{"GET", "/api/v1/content/jobs/550e8400-e29b-41d4-a716-446655440000"},
			{"POST", "/api/v1/interactions"},
			{"POST", "/api/v1/interactions/batch"},
			{"GET", "/api/v1/recommendations/550e8400-e29b-41d4-a716-446655440000"},
			{"POST", "/api/v1/recommendations/batch"},
			{"GET", "/api/v1/users/550e8400-e29b-41d4-a716-446655440000/interactions"},
			{"POST", "/api/v1/feedback"},
		}

		for _, endpoint := range endpoints {
			t.Run(fmt.Sprintf("%s %s", endpoint.method, endpoint.path), func(t *testing.T) {
				req := httptest.NewRequest(endpoint.method, endpoint.path, nil)
				w := httptest.NewRecorder()
				ct.router.ServeHTTP(w, req)

				// Should not return 404 (endpoint exists)
				assert.NotEqual(t, 404, w.Code,
					"Endpoint %s %s should exist", endpoint.method, endpoint.path)
			})
		}
	})
}

// BenchmarkContractValidation benchmarks schema validation performance
func (ct *ContractTester) BenchmarkContractValidation(b *testing.B) {
	testData := map[string]interface{}{
		"id":          "benchmark-product-123",
		"type":        "product",
		"title":       "Benchmark Product",
		"description": "A product for benchmarking validation performance",
		"imageUrls":   []string{"https://example.com/image.jpg"},
		"categories":  []string{"electronics", "benchmark"},
		"metadata": map[string]interface{}{
			"price": 99.99,
			"brand": "BenchmarkBrand",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := ct.validator.ValidateStruct("content-item", testData)
		if !result.Valid {
			b.Fatalf("Validation failed: %v", result.Errors)
		}
	}
}
