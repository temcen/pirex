package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/temcen/pirex/internal/services"
)

func Auth(authService *services.AuthService, logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "MISSING_AUTHORIZATION",
					"message": "Authorization header is required",
				},
			})
			c.Abort()
			return
		}

		// Check for Bearer token format
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "INVALID_AUTHORIZATION_FORMAT",
					"message": "Authorization header must be in format 'Bearer <token>'",
				},
			})
			c.Abort()
			return
		}

		tokenString := tokenParts[1]

		// Check if it's an API key (simple heuristic: no dots means API key)
		if !strings.Contains(tokenString, ".") {
			// Handle API key authentication
			userTier, err := authService.ValidateAPIKey(tokenString)
			if err != nil {
				logger.WithError(err).Warn("Invalid API key")
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": gin.H{
						"code":    "INVALID_API_KEY",
						"message": "Invalid API key",
					},
				})
				c.Abort()
				return
			}

			// For API key auth, generate a temporary user ID or use from request
			userIDStr := c.GetHeader("X-User-ID")
			var userID uuid.UUID
			if userIDStr != "" {
				var err error
				userID, err = uuid.Parse(userIDStr)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"error": gin.H{
							"code":    "INVALID_USER_ID",
							"message": "Invalid user ID format",
						},
					})
					c.Abort()
					return
				}
			} else {
				userID = uuid.New() // Generate temporary ID for API key requests
			}

			// Set user context
			c.Set("user_id", userID)
			c.Set("user_tier", userTier)
			c.Set("api_key", tokenString)
			c.Next()
			return
		}

		// Handle JWT token authentication
		claims, err := authService.ValidateToken(tokenString)
		if err != nil {
			logger.WithError(err).Warn("Invalid JWT token")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "INVALID_TOKEN",
					"message": "Invalid or expired token",
				},
			})
			c.Abort()
			return
		}

		// Set user context
		c.Set("user_id", claims.UserID)
		c.Set("user_tier", claims.UserTier)
		c.Set("api_key", claims.APIKey)
		c.Next()
	}
}

func GetUserFromContext(c *gin.Context) (uuid.UUID, string, string) {
	userID, _ := c.Get("user_id")
	userTier, _ := c.Get("user_tier")
	apiKey, _ := c.Get("api_key")

	return userID.(uuid.UUID), userTier.(string), apiKey.(string)
}
