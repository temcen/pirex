package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/temcen/pirex/internal/validation"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ValidationMiddleware provides request/response validation middleware
type ValidationMiddleware struct {
	validator *validation.SchemaValidator
}

// NewValidationMiddleware creates a new validation middleware instance
func NewValidationMiddleware(validator *validation.SchemaValidator) *ValidationMiddleware {
	return &ValidationMiddleware{
		validator: validator,
	}
}

// ValidateContentItem validates content item requests
func (vm *ValidationMiddleware) ValidateContentItem() gin.HandlerFunc {
	return vm.validateRequestBody("content-item")
}

// ValidateUserInteraction validates user interaction requests
func (vm *ValidationMiddleware) ValidateUserInteraction() gin.HandlerFunc {
	return vm.validateRequestBody("user-interaction")
}

// ValidateUserProfile validates user profile requests
func (vm *ValidationMiddleware) ValidateUserProfile() gin.HandlerFunc {
	return vm.validateRequestBody("user-profile")
}

// validateRequestBody creates a middleware that validates request body against a schema
func (vm *ValidationMiddleware) validateRequestBody(schemaName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only validate for methods that have request bodies
		if c.Request.Method == "GET" || c.Request.Method == "DELETE" {
			c.Next()
			return
		}

		// Read request body
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			vm.sendValidationError(c, "BODY_READ_ERROR", "Failed to read request body", map[string]interface{}{
				"error": err.Error(),
			})
			return
		}

		// Restore request body for downstream handlers
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// Skip validation for empty bodies
		if len(bodyBytes) == 0 {
			vm.sendValidationError(c, "EMPTY_BODY", "Request body is required", nil)
			return
		}

		// Validate JSON format first
		var jsonData interface{}
		if err := json.Unmarshal(bodyBytes, &jsonData); err != nil {
			vm.sendValidationError(c, "INVALID_JSON", "Request body must be valid JSON", map[string]interface{}{
				"parseError": err.Error(),
			})
			return
		}

		// Validate against schema
		result := vm.validator.ValidateJSONString(schemaName, string(bodyBytes))
		if !result.Valid {
			apiError := result.ToAPIError()
			if errorObj, ok := apiError["error"].(map[string]interface{}); ok {
				errorObj["timestamp"] = time.Now().UTC().Format(time.RFC3339)
				errorObj["requestId"] = uuid.New().String()
				errorObj["path"] = c.Request.URL.Path
				errorObj["method"] = c.Request.Method
			}

			c.JSON(http.StatusBadRequest, apiError)
			c.Abort()
			return
		}

		// Store validated data in context for downstream handlers
		c.Set("validatedBody", jsonData)
		c.Next()
	}
}

// ValidateQueryParams validates query parameters
func (vm *ValidationMiddleware) ValidateQueryParams() gin.HandlerFunc {
	return func(c *gin.Context) {
		errors := make([]validation.ValidationError, 0)

		// Validate common query parameters
		if count := c.Query("count"); count != "" {
			if !vm.isValidPositiveInt(count, 1, 100) {
				errors = append(errors, validation.ValidationError{
					Field:   "count",
					Message: "Count must be an integer between 1 and 100",
					Code:    "INVALID_QUERY_PARAM",
					Value:   count,
				})
			}
		}

		if limit := c.Query("limit"); limit != "" {
			if !vm.isValidPositiveInt(limit, 1, 1000) {
				errors = append(errors, validation.ValidationError{
					Field:   "limit",
					Message: "Limit must be an integer between 1 and 1000",
					Code:    "INVALID_QUERY_PARAM",
					Value:   limit,
				})
			}
		}

		if offset := c.Query("offset"); offset != "" {
			if !vm.isValidNonNegativeInt(offset) {
				errors = append(errors, validation.ValidationError{
					Field:   "offset",
					Message: "Offset must be a non-negative integer",
					Code:    "INVALID_QUERY_PARAM",
					Value:   offset,
				})
			}
		}

		// Validate UUID parameters in path
		if userID := c.Param("userId"); userID != "" {
			if !vm.isValidUUID(userID) {
				errors = append(errors, validation.ValidationError{
					Field:   "userId",
					Message: "User ID must be a valid UUID",
					Code:    "INVALID_PATH_PARAM",
					Value:   userID,
				})
			}
		}

		if jobID := c.Param("jobId"); jobID != "" {
			if !vm.isValidUUID(jobID) {
				errors = append(errors, validation.ValidationError{
					Field:   "jobId",
					Message: "Job ID must be a valid UUID",
					Code:    "INVALID_PATH_PARAM",
					Value:   jobID,
				})
			}
		}

		// Validate item ID format
		if itemID := c.Param("itemId"); itemID != "" {
			if !vm.isValidItemID(itemID) {
				errors = append(errors, validation.ValidationError{
					Field:   "itemId",
					Message: "Item ID must contain only alphanumeric characters, hyphens, and underscores",
					Code:    "INVALID_PATH_PARAM",
					Value:   itemID,
				})
			}
		}

		// Validate context parameter
		if context := c.Query("context"); context != "" {
			validContexts := []string{"home", "search", "product_page", "category", "checkout"}
			if !vm.isValidEnum(context, validContexts) {
				errors = append(errors, validation.ValidationError{
					Field:   "context",
					Message: fmt.Sprintf("Context must be one of: %s", strings.Join(validContexts, ", ")),
					Code:    "INVALID_QUERY_PARAM",
					Value:   context,
				})
			}
		}

		// Validate interaction type parameter
		if interactionType := c.Query("type"); interactionType != "" {
			validTypes := []string{"rating", "like", "dislike", "share", "click", "view", "search", "browse"}
			if !vm.isValidEnum(interactionType, validTypes) {
				errors = append(errors, validation.ValidationError{
					Field:   "type",
					Message: fmt.Sprintf("Interaction type must be one of: %s", strings.Join(validTypes, ", ")),
					Code:    "INVALID_QUERY_PARAM",
					Value:   interactionType,
				})
			}
		}

		// If there are validation errors, return them
		if len(errors) > 0 {
			vm.sendValidationErrors(c, errors)
			return
		}

		c.Next()
	}
}

// ValidateHeaders validates required headers
func (vm *ValidationMiddleware) ValidateHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		errors := make([]validation.ValidationError, 0)

		// Validate Content-Type for POST/PUT requests
		if c.Request.Method == "POST" || c.Request.Method == "PUT" || c.Request.Method == "PATCH" {
			contentType := c.GetHeader("Content-Type")
			if contentType == "" {
				errors = append(errors, validation.ValidationError{
					Field:   "Content-Type",
					Message: "Content-Type header is required",
					Code:    "MISSING_HEADER",
				})
			} else if !strings.Contains(contentType, "application/json") {
				errors = append(errors, validation.ValidationError{
					Field:   "Content-Type",
					Message: "Content-Type must be application/json",
					Code:    "INVALID_HEADER",
					Value:   contentType,
				})
			}
		}

		// Validate Accept header if present
		if accept := c.GetHeader("Accept"); accept != "" {
			if !strings.Contains(accept, "application/json") && !strings.Contains(accept, "*/*") {
				errors = append(errors, validation.ValidationError{
					Field:   "Accept",
					Message: "Accept header must include application/json",
					Code:    "INVALID_HEADER",
					Value:   accept,
				})
			}
		}

		if len(errors) > 0 {
			vm.sendValidationErrors(c, errors)
			return
		}

		c.Next()
	}
}

// Helper validation functions
func (vm *ValidationMiddleware) isValidPositiveInt(value string, min, max int) bool {
	var num int
	if _, err := fmt.Sscanf(value, "%d", &num); err != nil {
		return false
	}
	return num >= min && num <= max
}

func (vm *ValidationMiddleware) isValidNonNegativeInt(value string) bool {
	var num int
	if _, err := fmt.Sscanf(value, "%d", &num); err != nil {
		return false
	}
	return num >= 0
}

func (vm *ValidationMiddleware) isValidUUID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}

func (vm *ValidationMiddleware) isValidItemID(value string) bool {
	if len(value) == 0 || len(value) > 100 {
		return false
	}
	for _, char := range value {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_') {
			return false
		}
	}
	return true
}

func (vm *ValidationMiddleware) isValidEnum(value string, validValues []string) bool {
	for _, valid := range validValues {
		if value == valid {
			return true
		}
	}
	return false
}

// Error response helpers
func (vm *ValidationMiddleware) sendValidationError(c *gin.Context, code, message string, details map[string]interface{}) {
	errorResponse := map[string]interface{}{
		"error": map[string]interface{}{
			"code":      code,
			"message":   message,
			"details":   details,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"requestId": uuid.New().String(),
			"path":      c.Request.URL.Path,
			"method":    c.Request.Method,
		},
	}

	c.JSON(http.StatusBadRequest, errorResponse)
	c.Abort()
}

func (vm *ValidationMiddleware) sendValidationErrors(c *gin.Context, errors []validation.ValidationError) {
	errorDetails := make(map[string]interface{})
	errorDetails["validationErrors"] = errors

	// Group errors by field for easier client handling
	fieldErrors := make(map[string][]string)
	for _, err := range errors {
		if err.Field != "" {
			fieldErrors[err.Field] = append(fieldErrors[err.Field], err.Message)
		}
	}

	if len(fieldErrors) > 0 {
		errorDetails["fieldErrors"] = fieldErrors
	}

	errorResponse := map[string]interface{}{
		"error": map[string]interface{}{
			"code":      "VALIDATION_ERROR",
			"message":   "Request validation failed",
			"details":   errorDetails,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"requestId": uuid.New().String(),
			"path":      c.Request.URL.Path,
			"method":    c.Request.Method,
		},
	}

	c.JSON(http.StatusBadRequest, errorResponse)
	c.Abort()
}

// ResponseValidator validates outgoing responses (for development/testing)
type ResponseValidator struct {
	validator *validation.SchemaValidator
	enabled   bool
}

// NewResponseValidator creates a new response validator
func NewResponseValidator(validator *validation.SchemaValidator, enabled bool) *ResponseValidator {
	return &ResponseValidator{
		validator: validator,
		enabled:   enabled,
	}
}

// ValidateResponse validates response data against schema
func (rv *ResponseValidator) ValidateResponse(schemaName string, data interface{}) error {
	if !rv.enabled {
		return nil
	}

	result := rv.validator.ValidateStruct(schemaName, data)
	if !result.Valid {
		return fmt.Errorf("response validation failed: %v", result.Errors)
	}

	return nil
}
