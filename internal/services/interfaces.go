package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/temcen/pirex/pkg/models"
)

// UserInteractionServiceInterface defines the interface for user interaction operations
type UserInteractionServiceInterface interface {
	RecordExplicitInteraction(ctx context.Context, req *models.ExplicitInteractionRequest) (*models.UserInteraction, error)
	RecordImplicitInteraction(ctx context.Context, req *models.ImplicitInteractionRequest) (*models.UserInteraction, error)
	RecordBatchInteractions(ctx context.Context, req *models.InteractionBatchRequest) ([]models.UserInteraction, error)
	GetUserInteractions(ctx context.Context, userID uuid.UUID, interactionType string, limit, offset int, startDate, endDate *time.Time) ([]models.UserInteraction, int, error)
	GetUserProfile(ctx context.Context, userID uuid.UUID) (*models.UserProfile, error)
	GetSimilarUsers(ctx context.Context, userID uuid.UUID, limit int) ([]models.SimilarUser, error)
	Stop()
}

// RecommendationAlgorithmsServiceInterface defines the interface for recommendation algorithms
type RecommendationAlgorithmsServiceInterface interface {
	SemanticSearchRecommendations(ctx context.Context, userID uuid.UUID, userEmbedding []float32, contentTypes []string, categories []string, limit int) ([]models.ScoredItem, error)
	CollaborativeFilteringRecommendations(ctx context.Context, userID uuid.UUID, limit int) ([]models.ScoredItem, error)
	PersonalizedPageRankRecommendations(ctx context.Context, userID uuid.UUID, limit int) ([]models.ScoredItem, error)
	GraphSignalAnalysisRecommendations(ctx context.Context, userID uuid.UUID, limit int) ([]models.ScoredItem, error)
}

// RecommendationOrchestratorInterface defines the interface for recommendation orchestration
type RecommendationOrchestratorInterface interface {
	GenerateRecommendations(ctx context.Context, reqCtx *RecommendationContext) (*OrchestrationResult, error)
	ProcessFeedback(ctx context.Context, feedback *models.RecommendationFeedback) error
}

// DiversityFilterInterface defines the interface for diversity filtering
type DiversityFilterInterface interface {
	ApplyDiversityFilters(ctx context.Context, userID uuid.UUID, recommendations []models.Recommendation) ([]models.Recommendation, error)
}

// ExplanationServiceInterface defines the interface for explanation generation
type ExplanationServiceInterface interface {
	GenerateExplanations(ctx context.Context, userID uuid.UUID, recommendations []models.Recommendation) ([]models.Recommendation, error)
}
