# Data Contracts and API Specifications

This document describes the implementation of Task 11: Data Contracts and API Specifications for the Recommendation Engine.

## Overview

The implementation provides comprehensive API documentation, validation schemas, and contract testing to ensure API reliability and consistency. All components work together to create a robust API specification system.

## Components Implemented

### 1. OpenAPI/Swagger Specification

**File:** `docs/api/openapi.yaml`

- Complete OpenAPI 3.0.3 specification
- All REST endpoints documented with examples
- Request/response schemas with validation rules
- Authentication requirements (JWT Bearer tokens)
- Rate limiting headers specification
- Interactive documentation at `/docs`

**Key Features:**
- Comprehensive endpoint documentation
- Detailed error response specifications
- Security scheme definitions
- Example requests and responses
- Parameter validation rules

### 2. GraphQL Schema Documentation

**File:** `docs/api/schema.graphql`

- Complete GraphQL schema with all types
- Queries, mutations, and subscriptions
- Proper type relationships and connections
- Directive-based authentication and rate limiting
- Comprehensive field documentation

**Key Types:**
- `User`, `Content`, `Recommendation`, `Interaction`
- `UserProfile`, `Explanation`, `ContentIngestionJob`
- Input types for mutations
- Connection types for pagination

### 3. JSON Schema Validation

**Files:** `docs/api/schemas/*.json`

Individual JSON schemas for data validation:

- **content-item.json**: Content ingestion validation
- **user-interaction.json**: User interaction validation  
- **recommendation.json**: Recommendation response validation
- **user-profile.json**: User profile data validation
- **error-response.json**: Standardized error format validation

**Features:**
- Detailed validation rules with custom error messages
- Type constraints and format validation
- Enum validation for controlled vocabularies
- Conditional validation based on interaction types
- Comprehensive examples for each schema

### 4. Validation Implementation

**Files:** 
- `internal/validation/schemas.go`: Core validation logic
- `internal/middleware/validation.go`: Request/response validation middleware

**Capabilities:**
- Runtime JSON schema validation
- Request body validation middleware
- Query parameter validation
- Header validation
- Structured error responses
- Performance-optimized validation

### 5. Contract Testing Framework

**Files:**
- `internal/testing/contract_test.go`: Contract testing utilities
- `internal/testing/api_contract_test.go`: Comprehensive test suite

**Features:**
- Automated API contract testing
- Schema compliance validation
- Request/response validation testing
- OpenAPI specification compliance checks
- Performance benchmarking
- Mock API endpoints for testing

### 6. Interactive Documentation

**Files:**
- `internal/docs/swagger.go`: Documentation server
- `internal/docs/templates/*.html`: Documentation templates

**Pages:**
- `/docs/`: Interactive Swagger UI
- `/docs/schemas`: JSON schema documentation
- `/docs/examples`: API usage examples
- `/docs/authentication`: Authentication guide
- `/docs/rate-limiting`: Rate limiting documentation
- `/docs/errors`: Error handling guide

## Usage

### Starting the Documentation Server

```go
package main

import (
    "github.com/gin-gonic/gin"
    "recommendation-engine/internal/docs"
    "recommendation-engine/internal/validation"
    "recommendation-engine/internal/middleware"
)

func main() {
    router := gin.Default()
    
    // Load validation schemas
    validator := validation.NewSchemaValidator()
    err := validator.LoadSchemas("docs/api/schemas")
    if err != nil {
        panic(err)
    }
    
    // Setup validation middleware
    validationMiddleware := middleware.NewValidationMiddleware(validator)
    router.Use(validationMiddleware.ValidateHeaders())
    router.Use(validationMiddleware.ValidateQueryParams())
    
    // Setup documentation
    swaggerConfig := docs.GetSwaggerConfig()
    swaggerHandler := docs.NewSwaggerHandler(swaggerConfig, "docs/api/openapi.yaml")
    swaggerHandler.RegisterRoutes(router)
    
    // Add API routes with validation
    api := router.Group("/api/v1")
    api.POST("/content", 
        validationMiddleware.ValidateContentItem(),
        handleContentIngestion)
    
    router.Run(":8080")
}
```

### Validating Data

```go
// Validate content item
result := validator.ValidateContentItem(contentData)
if !result.Valid {
    // Handle validation errors
    for _, err := range result.Errors {
        fmt.Printf("Field: %s, Error: %s\n", err.Field, err.Message)
    }
}

// Convert to API error response
apiError := result.ToAPIError()
c.JSON(http.StatusBadRequest, apiError)
```

### Running Contract Tests

```bash
# Run all contract tests
go test ./internal/testing -v

# Run specific test suite
go test ./internal/testing -run TestAPIContractCompliance

# Run validation benchmarks
go test ./internal/testing -bench=BenchmarkValidation
```

## API Documentation Access

Once the server is running, access documentation at:

- **Swagger UI**: http://localhost:8080/docs/
- **OpenAPI Spec**: http://localhost:8080/docs/openapi.yaml
- **Schema Docs**: http://localhost:8080/docs/schemas
- **Examples**: http://localhost:8080/docs/examples

## Error Response Format

All API errors follow a standardized format:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Request validation failed",
    "details": {
      "fieldErrors": {
        "type": ["Content type must be one of: product, video, article, course, book"]
      }
    },
    "timestamp": "2024-01-15T10:30:00Z",
    "requestId": "550e8400-e29b-41d4-a716-446655440000",
    "path": "/api/v1/content",
    "method": "POST"
  },
  "fallback": {
    "used": true,
    "strategy": "cached_data",
    "confidence": 0.8
  }
}
```

## Validation Rules

### Content Items
- ID: Alphanumeric with hyphens/underscores (1-100 chars)
- Type: Must be product, video, article, course, or book
- Title: Required, 1-200 characters
- Categories: At least 1, maximum 10 categories
- Image URLs: Valid HTTP/HTTPS URLs, maximum 10

### User Interactions
- User ID: Valid UUID format
- Item ID: Alphanumeric with hyphens/underscores
- Interaction Type: Valid enum value
- Rating values: 1-5 for rating interactions
- Timestamps: ISO 8601 format

### Recommendations
- Score: 0-1 range
- Algorithm: Valid algorithm enum
- Confidence: 0-1 range
- Position: Positive integer

## Testing

The implementation includes comprehensive testing:

1. **Schema Validation Tests**: Verify all schemas work correctly
2. **Middleware Tests**: Test request/response validation
3. **Contract Tests**: Ensure API matches specification
4. **Documentation Tests**: Verify documentation endpoints work
5. **Performance Tests**: Benchmark validation performance

## Integration

### With Existing Handlers

```go
func handleContentIngestion(c *gin.Context) {
    // Validated data is available in context
    validatedData, exists := c.Get("validatedBody")
    if !exists {
        c.JSON(http.StatusBadRequest, gin.H{"error": "No validated data"})
        return
    }
    
    // Process the validated content item
    contentItem := validatedData.(map[string]interface{})
    // ... business logic
}
```

### With Response Validation (Development)

```go
responseValidator := middleware.NewResponseValidator(validator, true)

func handleGetRecommendations(c *gin.Context) {
    recommendations := generateRecommendations(userID)
    
    // Validate response in development
    if err := responseValidator.ValidateResponse("recommendation", recommendations); err != nil {
        log.Errorf("Response validation failed: %v", err)
    }
    
    c.JSON(http.StatusOK, recommendations)
}
```

## Benefits

1. **API Consistency**: Ensures all endpoints follow the same patterns
2. **Developer Experience**: Interactive documentation and clear examples
3. **Quality Assurance**: Automated validation prevents invalid data
4. **Contract Testing**: Catches API changes that break compatibility
5. **Client Generation**: Schemas can generate client SDKs
6. **Documentation**: Always up-to-date API documentation

## Future Enhancements

1. **OpenAPI JSON Generation**: Convert YAML to JSON automatically
2. **SDK Generation**: Auto-generate client libraries
3. **Postman Collection**: Generate Postman collections from OpenAPI
4. **API Versioning**: Support multiple API versions
5. **GraphQL Playground**: Interactive GraphQL explorer
6. **Performance Monitoring**: Track validation performance metrics

This implementation provides a solid foundation for API documentation, validation, and testing that will scale with the recommendation engine's growth.