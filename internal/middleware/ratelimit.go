package middleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/temcen/pirex/internal/services"
)

func RateLimit(rateLimitService *services.RateLimitService, logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			// This should not happen if auth middleware is properly configured
			logger.Error("Rate limit middleware called without user context")
			c.Next()
			return
		}

		userTier, exists := c.Get("user_tier")
		if !exists {
			userTier = "free" // Default tier
		}

		var userIDStr string
		switch v := userID.(type) {
		case string:
			userIDStr = v
		case interface{ String() string }:
			userIDStr = v.String()
		default:
			userIDStr = "unknown"
		}

		allowed, info, err := rateLimitService.IsAllowed(userIDStr, userTier.(string))
		if err != nil {
			logger.WithError(err).Error("Failed to check rate limit")
			// Continue on error to avoid blocking requests when Redis is down
			c.Next()
			return
		}

		// Set rate limit headers
		c.Header("X-RateLimit-Limit", strconv.Itoa(info.Limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(info.Remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(info.ResetTime, 10))

		if !allowed {
			logger.WithFields(logrus.Fields{
				"user_id":   userIDStr,
				"user_tier": userTier,
				"limit":     info.Limit,
			}).Warn("Rate limit exceeded")

			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{
					"code":    "RATE_LIMIT_EXCEEDED",
					"message": "Rate limit exceeded. Please try again later.",
				},
				"rate_limit": info,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
