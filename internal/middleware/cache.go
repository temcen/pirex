package middleware

import (
	"context"
	"crypto/md5"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

// CacheConfig represents cache configuration
type CacheConfig struct {
	DefaultTTL time.Duration
	MaxSize    int64
	KeyPrefix  string
}

// CacheMiddleware provides HTTP response caching
func CacheMiddleware(redis *redis.Client, config *CacheConfig, logger *logrus.Logger) gin.HandlerFunc {
	if redis == nil {
		logger.Warn("Redis client not available, caching disabled")
		return func(c *gin.Context) { c.Next() }
	}

	return func(c *gin.Context) {
		// Only cache GET requests
		if c.Request.Method != "GET" {
			c.Next()
			return
		}

		// Skip caching for certain paths
		if shouldSkipCaching(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Generate cache key
		cacheKey := generateCacheKey(c, config.KeyPrefix)

		// Try to get cached response
		cached := redis.Get(c.Request.Context(), cacheKey).Val()
		if cached != "" {
			// Parse cached response
			if response, err := parseCachedResponse(cached); err == nil {
				// Set headers
				for key, value := range response.Headers {
					c.Header(key, value)
				}
				c.Header("X-Cache", "HIT")
				c.Header("X-Cache-Key", cacheKey)

				// Return cached data
				c.Data(response.StatusCode, response.ContentType, response.Body)
				c.Abort()
				return
			}
		}

		// Create response writer wrapper
		writer := &cacheWriter{
			ResponseWriter: c.Writer,
			redis:          redis,
			cacheKey:       cacheKey,
			config:         config,
			logger:         logger,
		}
		c.Writer = writer

		// Set cache headers
		c.Header("X-Cache", "MISS")
		c.Header("X-Cache-Key", cacheKey)

		c.Next()
	}
}

// cacheWriter wraps gin.ResponseWriter to cache responses
type cacheWriter struct {
	gin.ResponseWriter
	redis    *redis.Client
	cacheKey string
	config   *CacheConfig
	logger   *logrus.Logger
	body     []byte
}

// cachedResponse represents a cached HTTP response
type cachedResponse struct {
	StatusCode  int               `json:"status_code"`
	Headers     map[string]string `json:"headers"`
	ContentType string            `json:"content_type"`
	Body        []byte            `json:"body"`
}

// Write captures response data for caching
func (w *cacheWriter) Write(data []byte) (int, error) {
	w.body = append(w.body, data...)
	return w.ResponseWriter.Write(data)
}

// WriteHeader captures status code and caches the response
func (w *cacheWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)

	// Only cache successful responses
	if statusCode >= 200 && statusCode < 300 {
		go w.cacheResponse(statusCode)
	}
}

// cacheResponse stores the response in cache
func (w *cacheWriter) cacheResponse(statusCode int) {
	if len(w.body) == 0 {
		return
	}

	// Don't cache large responses
	if w.config.MaxSize > 0 && int64(len(w.body)) > w.config.MaxSize {
		w.logger.Debug("Response too large to cache", "size", len(w.body), "max_size", w.config.MaxSize)
		return
	}

	// Create cached response
	response := &cachedResponse{
		StatusCode:  statusCode,
		Headers:     make(map[string]string),
		ContentType: w.Header().Get("Content-Type"),
		Body:        w.body,
	}

	// Copy relevant headers
	headersToCache := []string{
		"Content-Type",
		"Content-Encoding",
		"Cache-Control",
		"ETag",
		"Last-Modified",
	}

	for _, header := range headersToCache {
		if value := w.Header().Get(header); value != "" {
			response.Headers[header] = value
		}
	}

	// Serialize and cache
	if data, err := serializeCachedResponse(response); err == nil {
		ttl := w.config.DefaultTTL
		if ttl == 0 {
			ttl = 5 * time.Minute // Default 5 minutes
		}

		if err := w.redis.Set(context.Background(), w.cacheKey, data, ttl).Err(); err != nil {
			w.logger.Warn("Failed to cache response", "error", err, "cache_key", w.cacheKey)
		} else {
			w.logger.Debug("Response cached", "cache_key", w.cacheKey, "ttl", ttl)
		}
	}
}

// generateCacheKey creates a cache key for the request
func generateCacheKey(c *gin.Context, prefix string) string {
	// Include user ID for personalized responses
	userID := ""
	if uid, exists := c.Get("user_id"); exists {
		if uidStr, ok := uid.(string); ok {
			userID = uidStr
		} else if uidObj, ok := uid.(interface{ String() string }); ok {
			userID = uidObj.String()
		}
	}

	// Create key components
	components := []string{
		prefix,
		c.Request.Method,
		c.Request.URL.Path,
		c.Request.URL.RawQuery,
		userID,
	}

	// Create hash of components
	keyStr := strings.Join(components, ":")
	hash := md5.Sum([]byte(keyStr))
	return fmt.Sprintf("%s:%x", prefix, hash)
}

// shouldSkipCaching determines if caching should be skipped for a path
func shouldSkipCaching(path string) bool {
	skipPaths := []string{
		"/health",
		"/metrics",
		"/api/v1/feedback",
		"/api/v1/interactions",
		"/graphql",
	}

	for _, skipPath := range skipPaths {
		if strings.HasPrefix(path, skipPath) {
			return true
		}
	}

	return false
}

// serializeCachedResponse serializes a cached response
func serializeCachedResponse(response *cachedResponse) (string, error) {
	// Simple serialization format: statusCode|contentType|headers|body
	headers := ""
	for key, value := range response.Headers {
		headers += fmt.Sprintf("%s:%s;", key, value)
	}

	return fmt.Sprintf("%d|%s|%s|%s",
		response.StatusCode,
		response.ContentType,
		headers,
		string(response.Body),
	), nil
}

// parseCachedResponse parses a cached response
func parseCachedResponse(data string) (*cachedResponse, error) {
	parts := strings.SplitN(data, "|", 4)
	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid cached response format")
	}

	statusCode, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid status code: %w", err)
	}

	response := &cachedResponse{
		StatusCode:  statusCode,
		ContentType: parts[1],
		Headers:     make(map[string]string),
		Body:        []byte(parts[3]),
	}

	// Parse headers
	if parts[2] != "" {
		headerPairs := strings.Split(parts[2], ";")
		for _, pair := range headerPairs {
			if pair != "" {
				keyValue := strings.SplitN(pair, ":", 2)
				if len(keyValue) == 2 {
					response.Headers[keyValue[0]] = keyValue[1]
				}
			}
		}
	}

	return response, nil
}

// CacheInvalidationMiddleware provides cache invalidation on data changes
func CacheInvalidationMiddleware(redis *redis.Client, logger *logrus.Logger) gin.HandlerFunc {
	if redis == nil {
		return func(c *gin.Context) { c.Next() }
	}

	return func(c *gin.Context) {
		// Store original method for later use
		originalMethod := c.Request.Method

		c.Next()

		// Invalidate cache on data modification
		if originalMethod == "POST" || originalMethod == "PUT" || originalMethod == "DELETE" {
			go invalidateRelatedCache(redis, c, logger)
		}
	}
}

// invalidateRelatedCache invalidates cache entries related to the request
func invalidateRelatedCache(redis *redis.Client, c *gin.Context, logger *logrus.Logger) {
	// Get user ID for targeted invalidation
	userID := ""
	if uid, exists := c.Get("user_id"); exists {
		if uidStr, ok := uid.(string); ok {
			userID = uidStr
		} else if uidObj, ok := uid.(interface{ String() string }); ok {
			userID = uidObj.String()
		}
	}

	// Invalidate user-specific caches
	if userID != "" {
		patterns := []string{
			fmt.Sprintf("cache:*:%s:*", userID),
			fmt.Sprintf("orchestration:%s:*", userID),
		}

		for _, pattern := range patterns {
			keys, err := redis.Keys(context.Background(), pattern).Result()
			if err != nil {
				logger.Warn("Failed to get cache keys for invalidation", "error", err, "pattern", pattern)
				continue
			}

			if len(keys) > 0 {
				if err := redis.Del(context.Background(), keys...).Err(); err != nil {
					logger.Warn("Failed to invalidate cache keys", "error", err, "count", len(keys))
				} else {
					logger.Debug("Invalidated cache keys", "count", len(keys), "user_id", userID)
				}
			}
		}
	}
}
