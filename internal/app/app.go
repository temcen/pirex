package app

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"

	"github.com/temcen/pirex/internal/config"
	"github.com/temcen/pirex/internal/database"
	"github.com/temcen/pirex/internal/handlers"
	"github.com/temcen/pirex/internal/middleware"
	"github.com/temcen/pirex/internal/services"
)

type App struct {
	config           *config.Config
	logger           *logrus.Logger
	db               *database.Database
	services         *services.Services
	handlers         *handlers.Handlers
	router           *gin.Engine
	metricsCollector *services.MetricsCollector
}

func New(cfg *config.Config) (*App, error) {
	app := &App{
		config: cfg,
		logger: setupLogger(cfg),
	}

	// Initialize database connections
	db, err := database.New(cfg, app.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}
	app.db = db

	// Initialize metrics collector
	metricsCollector := services.NewMetricsCollector(db.PG)
	app.metricsCollector = metricsCollector

	// Initialize services
	services, err := services.New(cfg, app.logger, db)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize services: %w", err)
	}
	app.services = services

	// Initialize handlers
	app.handlers = handlers.New(app.logger, services)

	// Initialize additional handlers for monitoring
	app.handlers.Metrics = handlers.NewMetricsHandler(app.logger, metricsCollector, services.Health)
	app.handlers.Admin = handlers.NewAdminHandler(app.logger, cfg)

	// Setup router
	app.setupRouter()

	return app, nil
}

func (a *App) Router() *gin.Engine {
	return a.router
}

func (a *App) Shutdown(ctx context.Context) error {
	a.logger.Info("Shutting down application...")

	if err := a.db.Close(); err != nil {
		a.logger.WithError(err).Error("Error closing database connections")
		return err
	}

	return nil
}

func setupLogger(cfg *config.Config) *logrus.Logger {
	logger := logrus.New()

	level, err := logrus.ParseLevel(cfg.Logging.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	if cfg.Logging.Format == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}

	return logger
}

func (a *App) setupRouter() {
	if a.config.Server.Mode == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Global middleware
	router.Use(middleware.Logger(a.logger))
	router.Use(middleware.Recovery(a.logger))
	router.Use(middleware.CORS(a.config))
	router.Use(middleware.Security())
	router.Use(middleware.CompressionMiddleware())
	// Validation middleware is applied per route as needed

	// Health check endpoints (no auth required)
	router.GET("/health", a.handlers.Health.Check)
	router.GET("/health/detailed", a.handlers.Health.Check) // Enhanced health check

	// Prometheus metrics endpoint (no auth required)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Documentation endpoints (no auth required)
	docs := router.Group("/docs")
	{
		docs.GET("/", a.handlers.SwaggerUI)
		docs.GET("/swagger.json", a.handlers.SwaggerSpec)
	}

	// GraphQL endpoint (with auth)
	router.POST("/graphql", middleware.Auth(a.services.Auth, a.logger), a.handlers.GraphQL.Handle)
	router.GET("/graphql", a.handlers.GraphQL.HandlePlayground)

	// API routes
	api := router.Group("/api/v1")
	{
		// Authentication middleware for API routes
		api.Use(middleware.Auth(a.services.Auth, a.logger))
		api.Use(middleware.RateLimit(a.services.RateLimit, a.logger))

		// Content routes
		content := api.Group("/content")
		{
			content.POST("", a.handlers.Content.Create)
			content.POST("/batch", a.handlers.Content.CreateBatch)
			content.GET("/jobs/:jobId", a.handlers.Content.GetJobStatus)
		}

		// Interaction routes
		interactions := api.Group("/interactions")
		{
			interactions.POST("/explicit", a.handlers.Interaction.RecordExplicit)
			interactions.POST("/implicit", a.handlers.Interaction.RecordImplicit)
			interactions.POST("/batch", a.handlers.Interaction.RecordBatch)
		}

		// Recommendation routes
		recommendations := api.Group("/recommendations")
		{
			recommendations.GET("/:userId", a.handlers.Recommendation.Get)
			recommendations.POST("/batch", a.handlers.Recommendation.GetBatch)
			recommendations.GET("/:userId/similar/:itemId", a.handlers.Recommendation.GetSimilar)
		}

		// Feedback routes
		api.POST("/feedback", a.handlers.Recommendation.RecordFeedback)

		// User routes
		users := api.Group("/users")
		{
			users.GET("/:userId/interactions", a.handlers.User.GetInteractions)
		}

		// Metrics routes
		metrics := api.Group("/metrics")
		{
			metrics.GET("/business", a.handlers.Metrics.GetBusinessMetrics)
			metrics.GET("/performance", a.handlers.Metrics.GetPerformanceMetrics)
			metrics.POST("/interactions", a.handlers.Metrics.RecordInteraction)
		}

		// Admin routes (additional auth/role checking would be added in production)
		admin := api.Group("/admin")
		{
			admin.GET("/metrics/overview", a.handlers.Metrics.GetAdminOverviewMetrics)
			admin.GET("/analytics", a.handlers.Metrics.GetAdminAnalytics)
			admin.GET("/content/status", a.handlers.Metrics.GetContentStatus)
			admin.GET("/users/analytics", a.handlers.Metrics.GetUserAnalytics)
			admin.GET("/monitoring/metrics", a.handlers.Metrics.GetMonitoringMetrics)
			admin.GET("/alerts/recent", a.handlers.Metrics.GetRecentAlerts)
			admin.GET("/ab-tests", a.handlers.Metrics.GetABTests)

			// Algorithm configuration
			admin.GET("/algorithms/config", a.handlers.Admin.GetAlgorithmConfig)
			admin.PUT("/algorithms/config", a.handlers.Admin.UpdateAlgorithmConfig)
			admin.POST("/algorithms/test", a.handlers.Admin.TestAlgorithmConfig)

			// System configuration
			admin.GET("/system/config", a.handlers.Admin.GetSystemConfiguration)
			admin.PUT("/system/config", a.handlers.Admin.UpdateSystemConfiguration)
		}
	}

	a.router = router
}
