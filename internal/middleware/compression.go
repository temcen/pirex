package middleware

import (
	"compress/gzip"
	"io"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// CompressionMiddleware provides gzip compression for responses
func CompressionMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// Check if client accepts gzip encoding
		if !strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
			c.Next()
			return
		}

		// Skip compression for small responses or specific content types
		if shouldSkipCompression(c) {
			c.Next()
			return
		}

		// Create gzip writer
		gz := gzip.NewWriter(c.Writer)
		defer gz.Close()

		// Set compression headers
		c.Header("Content-Encoding", "gzip")
		c.Header("Vary", "Accept-Encoding")

		// Wrap the response writer
		c.Writer = &gzipWriter{
			ResponseWriter: c.Writer,
			Writer:         gz,
		}

		c.Next()
	})
}

// gzipWriter wraps gin.ResponseWriter with gzip compression
type gzipWriter struct {
	gin.ResponseWriter
	Writer io.Writer
}

// Write compresses and writes data
func (g *gzipWriter) Write(data []byte) (int, error) {
	// Only compress if response is large enough
	if len(data) < 1024 {
		// For small responses, write directly without compression
		g.Header().Del("Content-Encoding")
		return g.ResponseWriter.Write(data)
	}

	return g.Writer.Write(data)
}

// WriteString compresses and writes string data
func (g *gzipWriter) WriteString(s string) (int, error) {
	return g.Write([]byte(s))
}

// shouldSkipCompression determines if compression should be skipped
func shouldSkipCompression(c *gin.Context) bool {
	// Skip for certain content types
	contentType := c.GetHeader("Content-Type")
	skipTypes := []string{
		"image/",
		"video/",
		"audio/",
		"application/zip",
		"application/gzip",
		"application/x-gzip",
	}

	for _, skipType := range skipTypes {
		if strings.HasPrefix(contentType, skipType) {
			return true
		}
	}

	// Skip for small responses (if Content-Length is set)
	if contentLengthStr := c.GetHeader("Content-Length"); contentLengthStr != "" {
		if contentLength, err := strconv.Atoi(contentLengthStr); err == nil && contentLength < 1024 {
			return true
		}
	}

	return false
}
