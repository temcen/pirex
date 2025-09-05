package services

import (
	"github.com/temcen/pirex/internal/config"
	"github.com/temcen/pirex/internal/database"
	"github.com/temcen/pirex/internal/messaging"

	"github.com/sirupsen/logrus"
)

type Services struct {
	Auth                       *AuthService
	Health                     *HealthService
	RateLimit                  *RateLimitService
	MessageBus                 *messaging.MessageBus
	JobManager                 *JobManager
	DataPreprocessor           *DataPreprocessor
	PipelineOrchestrator       *PipelineOrchestrator
	UserInteraction            *UserInteractionService
	RecommendationAlgorithms   *RecommendationAlgorithmsService
	DiversityFilter            *DiversityFilter
	ExplanationService         *ExplanationService
	RecommendationOrchestrator *RecommendationOrchestrator
}

func New(cfg *config.Config, logger *logrus.Logger, db *database.Database) (*Services, error) {
	authService := NewAuthService(cfg, logger, db.Redis.Hot)
	healthService := NewHealthService(cfg, logger, db)
	rateLimitService := NewRateLimitService(cfg, logger, db.Redis.Hot)

	// Initialize content ingestion services
	messageBus, err := messaging.NewMessageBus(cfg, logger)
	if err != nil {
		return nil, err
	}

	jobManager := NewJobManager(db, logger)
	dataPreprocessor := NewDataPreprocessor(logger)
	pipelineOrchestrator := NewPipelineOrchestrator(db, messageBus, dataPreprocessor, jobManager, logger)
	userInteractionService := NewUserInteractionService(db, cfg, logger)

	// Initialize recommendation services
	recommendationAlgorithms := NewRecommendationAlgorithmsService(
		db.PG, db.Neo4j, db.Redis.Warm, &cfg.Algorithms, logger,
	)

	// Initialize diversity filter and explanation service
	diversityFilter := NewDiversityFilter(db.PG, &cfg.Algorithms.Diversity, logger)
	explanationService := NewExplanationService(db.PG, logger)

	recommendationOrchestrator := NewRecommendationOrchestrator(
		recommendationAlgorithms, userInteractionService, diversityFilter, explanationService,
		db.Redis.Warm, &cfg.Algorithms, logger,
	)

	return &Services{
		Auth:                       authService,
		Health:                     healthService,
		RateLimit:                  rateLimitService,
		MessageBus:                 messageBus,
		JobManager:                 jobManager,
		DataPreprocessor:           dataPreprocessor,
		PipelineOrchestrator:       pipelineOrchestrator,
		UserInteraction:            userInteractionService,
		RecommendationAlgorithms:   recommendationAlgorithms,
		DiversityFilter:            diversityFilter,
		ExplanationService:         explanationService,
		RecommendationOrchestrator: recommendationOrchestrator,
	}, nil
}
