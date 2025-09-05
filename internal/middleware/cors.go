package middleware

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/temcen/pirex/internal/config"
)

func CORS(cfg *config.Config) gin.HandlerFunc {
	config := cors.Config{
		AllowOrigins:     cfg.Security.CORS.AllowedOrigins,
		AllowMethods:     cfg.Security.CORS.AllowedMethods,
		AllowHeaders:     cfg.Security.CORS.AllowedHeaders,
		ExposeHeaders:    []string{"X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset"},
		AllowCredentials: true,
	}

	return cors.New(config)
}
