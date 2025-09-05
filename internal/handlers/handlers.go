package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	graphqlHandler "github.com/temcen/pirex/internal/graphql"
	"github.com/temcen/pirex/internal/services"
)

type Handlers struct {
	Health         *HealthHandler
	Content        *ContentHandler
	Interaction    *InteractionHandler
	Recommendation *RecommendationHandler
	User           *UserHandler
	GraphQL        *GraphQLHandler
	Metrics        *MetricsHandler
	Admin          *AdminHandler
	SwaggerSpec    gin.HandlerFunc
	SwaggerUI      gin.HandlerFunc
}

func New(logger *logrus.Logger, services *services.Services) *Handlers {
	// Create GraphQL handler
	graphqlSvc, err := graphqlHandler.NewGraphQLHandler(
		services.RecommendationOrchestrator,
		services.UserInteraction,
		logger,
	)
	if err != nil {
		logger.Error("Failed to create GraphQL handler", "error", err)
		graphqlSvc = nil
	}

	var graphqlHTTPHandler *GraphQLHandler
	if graphqlSvc != nil {
		graphqlHTTPHandler = NewGraphQLHandler(graphqlSvc, logger)
	}

	return &Handlers{
		Health:         NewHealthHandler(logger, services.Health),
		Content:        NewContentHandler(services.MessageBus, services.JobManager, logger),
		Interaction:    NewInteractionHandler(logger, services.UserInteraction),
		Recommendation: NewRecommendationHandler(services.RecommendationOrchestrator, logger),
		User:           NewUserHandler(logger, services.UserInteraction),
		GraphQL:        graphqlHTTPHandler,
		SwaggerSpec:    nil, // TODO: Implement swagger spec handler
		SwaggerUI:      nil, // TODO: Implement swagger UI handler
	}
}
