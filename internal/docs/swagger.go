package docs

import (
	"embed"
	"html/template"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed templates/*
var templateFS embed.FS

// SwaggerConfig holds configuration for Swagger documentation
type SwaggerConfig struct {
	Title       string
	Description string
	Version     string
	Host        string
	BasePath    string
	Schemes     []string
}

// SwaggerHandler provides HTTP handlers for API documentation
type SwaggerHandler struct {
	config   SwaggerConfig
	specPath string
}

// NewSwaggerHandler creates a new Swagger documentation handler
func NewSwaggerHandler(config SwaggerConfig, specPath string) *SwaggerHandler {
	return &SwaggerHandler{
		config:   config,
		specPath: specPath,
	}
}

// RegisterRoutes registers Swagger documentation routes
func (sh *SwaggerHandler) RegisterRoutes(router *gin.Engine) {
	docs := router.Group("/docs")
	{
		// Serve Swagger UI
		docs.GET("/", sh.SwaggerUI)
		docs.GET("/index.html", sh.SwaggerUI)

		// Serve OpenAPI specification
		docs.GET("/openapi.yaml", sh.OpenAPISpec)
		docs.GET("/openapi.json", sh.OpenAPISpecJSON)

		// Serve static assets
		docs.GET("/static/*filepath", sh.StaticAssets)

		// API documentation pages
		docs.GET("/schemas", sh.SchemasPage)
		docs.GET("/examples", sh.ExamplesPage)
		docs.GET("/authentication", sh.AuthenticationPage)
		docs.GET("/rate-limiting", sh.RateLimitingPage)
		docs.GET("/errors", sh.ErrorsPage)
	}
}

// SwaggerUI serves the Swagger UI interface
func (sh *SwaggerHandler) SwaggerUI(c *gin.Context) {
	tmpl, err := template.ParseFS(templateFS, "templates/swagger-ui.html")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load template"})
		return
	}

	data := struct {
		Config  SwaggerConfig
		SpecURL string
		BaseURL string
	}{
		Config:  sh.config,
		SpecURL: "/docs/openapi.yaml",
		BaseURL: "/docs",
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(c.Writer, data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render template"})
		return
	}
}

// OpenAPISpec serves the OpenAPI specification in YAML format
func (sh *SwaggerHandler) OpenAPISpec(c *gin.Context) {
	c.Header("Content-Type", "application/x-yaml")
	c.File(sh.specPath)
}

// OpenAPISpecJSON serves the OpenAPI specification in JSON format
func (sh *SwaggerHandler) OpenAPISpecJSON(c *gin.Context) {
	// This would convert YAML to JSON
	// For now, we'll serve a placeholder
	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, gin.H{
		"openapi": "3.0.3",
		"info": gin.H{
			"title":       sh.config.Title,
			"description": sh.config.Description,
			"version":     sh.config.Version,
		},
		"message": "JSON format not yet implemented. Please use /docs/openapi.yaml",
	})
}

// StaticAssets serves static files for documentation
func (sh *SwaggerHandler) StaticAssets(c *gin.Context) {
	// Static files not implemented yet
	c.JSON(http.StatusNotFound, gin.H{"error": "Static files not available"})
}

// SchemasPage serves the schemas documentation page
func (sh *SwaggerHandler) SchemasPage(c *gin.Context) {
	tmpl, err := template.ParseFS(templateFS, "templates/schemas.html")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load template"})
		return
	}

	schemas := []SchemaInfo{
		{
			Name:        "ContentItem",
			Description: "Schema for content items (products, videos, articles, etc.)",
			FilePath:    "/docs/api/schemas/content-item.json",
		},
		{
			Name:        "UserInteraction",
			Description: "Schema for user interactions with content",
			FilePath:    "/docs/api/schemas/user-interaction.json",
		},
		{
			Name:        "Recommendation",
			Description: "Schema for recommendation responses",
			FilePath:    "/docs/api/schemas/recommendation.json",
		},
		{
			Name:        "UserProfile",
			Description: "Schema for user profile data",
			FilePath:    "/docs/api/schemas/user-profile.json",
		},
		{
			Name:        "ErrorResponse",
			Description: "Standardized error response format",
			FilePath:    "/docs/api/schemas/error-response.json",
		},
	}

	data := struct {
		Config  SwaggerConfig
		Schemas []SchemaInfo
	}{
		Config:  sh.config,
		Schemas: schemas,
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(c.Writer, data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render template"})
		return
	}
}

// ExamplesPage serves the API examples page
func (sh *SwaggerHandler) ExamplesPage(c *gin.Context) {
	tmpl, err := template.ParseFS(templateFS, "templates/examples.html")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load template"})
		return
	}

	examples := []APIExample{
		{
			Title:       "Create Content Item",
			Method:      "POST",
			Endpoint:    "/api/v1/content",
			Description: "Add a new product to the recommendation system",
			RequestBody: `{
  "id": "prod-123",
  "type": "product",
  "title": "Wireless Bluetooth Headphones",
  "description": "High-quality wireless headphones with noise cancellation",
  "imageUrls": ["https://example.com/images/headphones.jpg"],
  "metadata": {
    "price": 199.99,
    "brand": "AudioTech",
    "color": "black"
  },
  "categories": ["electronics", "audio", "headphones"]
}`,
			ResponseBody: `{
  "jobId": "550e8400-e29b-41d4-a716-446655440000",
  "status": "queued",
  "estimatedTime": "30s",
  "message": "Content queued for processing"
}`,
		},
		{
			Title:       "Record User Interaction",
			Method:      "POST",
			Endpoint:    "/api/v1/interactions",
			Description: "Record a user rating for a product",
			RequestBody: `{
  "userId": "550e8400-e29b-41d4-a716-446655440000",
  "itemId": "prod-123",
  "interactionType": "rating",
  "value": 4.5,
  "timestamp": "2024-01-15T10:30:00Z",
  "sessionId": "550e8400-e29b-41d4-a716-446655440001",
  "context": {
    "source": "product_page",
    "device": "mobile"
  }
}`,
			ResponseBody: `{
  "interactionId": "550e8400-e29b-41d4-a716-446655440002",
  "status": "recorded",
  "message": "Interaction recorded successfully"
}`,
		},
		{
			Title:       "Get Recommendations",
			Method:      "GET",
			Endpoint:    "/api/v1/recommendations/550e8400-e29b-41d4-a716-446655440000?count=5&explain=true",
			Description: "Get personalized recommendations for a user",
			Headers: map[string]string{
				"Authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
			},
			ResponseBody: `{
  "userId": "550e8400-e29b-41d4-a716-446655440000",
  "recommendations": [
    {
      "itemId": "prod-789",
      "score": 0.85,
      "algorithm": "hybrid",
      "explanation": {
        "type": "content_based",
        "message": "Because you liked 'Wireless Mouse' and both are in electronics category",
        "evidence": {
          "similarItems": ["prod-123", "prod-456"],
          "categories": ["electronics", "accessories"],
          "confidence": 0.8
        }
      },
      "confidence": 0.82,
      "position": 1
    }
  ],
  "metadata": {
    "totalAvailable": 150,
    "algorithmsUsed": ["semantic_search", "collaborative_filtering"],
    "generatedAt": "2024-01-15T10:30:00Z",
    "cacheHit": false
  }
}`,
		},
	}

	data := struct {
		Config   SwaggerConfig
		Examples []APIExample
	}{
		Config:   sh.config,
		Examples: examples,
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(c.Writer, data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render template"})
		return
	}
}

// AuthenticationPage serves the authentication documentation
func (sh *SwaggerHandler) AuthenticationPage(c *gin.Context) {
	tmpl, err := template.ParseFS(templateFS, "templates/authentication.html")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load template"})
		return
	}

	data := struct {
		Config SwaggerConfig
	}{
		Config: sh.config,
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(c.Writer, data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render template"})
		return
	}
}

// RateLimitingPage serves the rate limiting documentation
func (sh *SwaggerHandler) RateLimitingPage(c *gin.Context) {
	tmpl, err := template.ParseFS(templateFS, "templates/rate-limiting.html")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load template"})
		return
	}

	data := struct {
		Config SwaggerConfig
	}{
		Config: sh.config,
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(c.Writer, data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render template"})
		return
	}
}

// ErrorsPage serves the error handling documentation
func (sh *SwaggerHandler) ErrorsPage(c *gin.Context) {
	tmpl, err := template.ParseFS(templateFS, "templates/errors.html")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load template"})
		return
	}

	errorCodes := []ErrorCodeInfo{
		{
			Code:        "VALIDATION_ERROR",
			HTTPStatus:  400,
			Description: "Request validation failed",
			Example: `{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Request validation failed",
    "details": {
      "fieldErrors": {
        "type": ["Content type must be one of: product, video, article, course, book"]
      }
    },
    "timestamp": "2024-01-15T10:30:00Z",
    "requestId": "550e8400-e29b-41d4-a716-446655440000"
  }
}`,
		},
		{
			Code:        "USER_NOT_FOUND",
			HTTPStatus:  404,
			Description: "Requested user does not exist",
			Example: `{
  "error": {
    "code": "USER_NOT_FOUND",
    "message": "User with ID 'user-123' not found",
    "details": {
      "userId": "550e8400-e29b-41d4-a716-446655440000"
    },
    "timestamp": "2024-01-15T10:30:00Z",
    "requestId": "550e8400-e29b-41d4-a716-446655440001"
  },
  "fallback": {
    "used": true,
    "strategy": "popular_recommendations",
    "confidence": 0.3
  }
}`,
		},
		{
			Code:        "RATE_LIMIT_EXCEEDED",
			HTTPStatus:  429,
			Description: "API rate limit exceeded",
			Example: `{
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "Rate limit of 1000 requests per hour exceeded",
    "details": {
      "limit": 1000,
      "window": "1h",
      "retryAfter": 3600
    },
    "timestamp": "2024-01-15T10:30:00Z",
    "requestId": "550e8400-e29b-41d4-a716-446655440002"
  }
}`,
		},
	}

	data := struct {
		Config     SwaggerConfig
		ErrorCodes []ErrorCodeInfo
	}{
		Config:     sh.config,
		ErrorCodes: errorCodes,
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(c.Writer, data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to render template"})
		return
	}
}

// Supporting types for documentation
type SchemaInfo struct {
	Name        string
	Description string
	FilePath    string
}

type APIExample struct {
	Title        string
	Method       string
	Endpoint     string
	Description  string
	Headers      map[string]string
	RequestBody  string
	ResponseBody string
}

type ErrorCodeInfo struct {
	Code        string
	HTTPStatus  int
	Description string
	Example     string
}

// GetSwaggerConfig returns default Swagger configuration
func GetSwaggerConfig() SwaggerConfig {
	return SwaggerConfig{
		Title:       "Recommendation Engine API",
		Description: "A comprehensive recommendation engine API with multi-modal content support",
		Version:     "1.0.0",
		Host:        "localhost:8080",
		BasePath:    "/api/v1",
		Schemes:     []string{"http", "https"},
	}
}
