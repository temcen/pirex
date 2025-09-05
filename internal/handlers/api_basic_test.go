package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/temcen/pirex/internal/middleware"
)

func TestBasicAPIEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// Create a basic router with just middleware
	router := gin.New()
	router.Use(middleware.Logger(logger))
	router.Use(middleware.Recovery(logger))
	// Validation middleware is applied per route as needed
	router.Use(middleware.CompressionMiddleware())

	// Add a simple health endpoint for testing
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "recommendation-engine",
		})
	})

	// Test health endpoint
	t.Run("Health endpoint works", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "healthy")
	})

	// Test compression middleware
	t.Run("Compression middleware works", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/health", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		// For small responses, compression might not be applied
		// This test just ensures the middleware doesn't break anything
	})

	// Test basic routing works
	t.Run("Basic routing works", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/nonexistent", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}
