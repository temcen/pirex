package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/temcen/pirex/internal/config"
	"github.com/temcen/pirex/pkg/models"
)

// Mock services for testing
type MockRecommendationAlgorithmsService struct {
	mock.Mock
}

func (m *MockRecommendationAlgorithmsService) SemanticSearchRecommendations(
	ctx context.Context, userID uuid.UUID, userEmbedding []float32, contentTypes []string, categories []string, limit int,
) ([]models.ScoredItem, error) {
	args := m.Called(ctx, userID, userEmbedding, contentTypes, categories, limit)
	return args.Get(0).([]models.ScoredItem), args.Error(1)
}

func (m *MockRecommendationAlgorithmsService) CollaborativeFilteringRecommendations(
	ctx context.Context, userID uuid.UUID, limit int,
) ([]models.ScoredItem, error) {
	args := m.Called(ctx, userID, limit)
	return args.Get(0).([]models.ScoredItem), args.Error(1)
}

func (m *MockRecommendationAlgorithmsService) PersonalizedPageRankRecommendations(
	ctx context.Context, userID uuid.UUID, limit int,
) ([]models.ScoredItem, error) {
	args := m.Called(ctx, userID, limit)
	return args.Get(0).([]models.ScoredItem), args.Error(1)
}

func (m *MockRecommendationAlgorithmsService) GraphSignalAnalysisRecommendations(
	ctx context.Context, userID uuid.UUID, limit int,
) ([]models.ScoredItem, error) {
	args := m.Called(ctx, userID, limit)
	return args.Get(0).([]models.ScoredItem), args.Error(1)
}

type MockUserInteractionService struct {
	mock.Mock
}

func (m *MockUserInteractionService) RecordExplicitInteraction(ctx context.Context, req *models.ExplicitInteractionRequest) (*models.UserInteraction, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*models.UserInteraction), args.Error(1)
}

func (m *MockUserInteractionService) RecordImplicitInteraction(ctx context.Context, req *models.ImplicitInteractionRequest) (*models.UserInteraction, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*models.UserInteraction), args.Error(1)
}

func (m *MockUserInteractionService) RecordBatchInteractions(ctx context.Context, req *models.InteractionBatchRequest) ([]models.UserInteraction, error) {
	args := m.Called(ctx, req)
	return args.Get(0).([]models.UserInteraction), args.Error(1)
}

func (m *MockUserInteractionService) GetUserInteractions(ctx context.Context, userID uuid.UUID, interactionType string, limit, offset int, startDate, endDate *time.Time) ([]models.UserInteraction, int, error) {
	args := m.Called(ctx, userID, interactionType, limit, offset, startDate, endDate)
	return args.Get(0).([]models.UserInteraction), args.Int(1), args.Error(2)
}

func (m *MockUserInteractionService) GetUserProfile(ctx context.Context, userID uuid.UUID) (*models.UserProfile, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.UserProfile), args.Error(1)
}

func (m *MockUserInteractionService) GetSimilarUsers(ctx context.Context, userID uuid.UUID, limit int) ([]models.SimilarUser, error) {
	args := m.Called(ctx, userID, limit)
	return args.Get(0).([]models.SimilarUser), args.Error(1)
}

func (m *MockUserInteractionService) Stop() {
	m.Called()
}

func TestRecommendationOrchestrator_GenerateRecommendations(t *testing.T) {
	// Setup
	mockAlgorithmService := new(MockRecommendationAlgorithmsService)
	mockUserService := new(MockUserInteractionService)

	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Use test database
	})
	redisClient.FlushDB(context.Background()) // Clear test database

	cfg := &config.AlgorithmConfig{
		SemanticSearch: config.AlgorithmWeightConfig{
			Enabled: true,
			Weight:  0.4,
		},
		CollaborativeFilter: config.AlgorithmWeightConfig{
			Enabled: true,
			Weight:  0.3,
		},
		PageRank: config.AlgorithmWeightConfig{
			Enabled: true,
			Weight:  0.3,
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Create mock diversity filter and explanation service
	diversityFilter := NewDiversityFilter(nil, &cfg.Diversity, logger)
	explanationService := NewExplanationService(nil, logger)

	orchestrator := NewRecommendationOrchestrator(
		mockAlgorithmService, mockUserService, diversityFilter, explanationService, redisClient, cfg, logger,
	)

	t.Run("successful orchestration for active user", func(t *testing.T) {
		userID := uuid.New()

		// Mock user profile (active user)
		userProfile := &models.UserProfile{
			UserID:           userID,
			PreferenceVector: []float32{0.1, 0.2, 0.3, 0.4},
			InteractionCount: 25,
			LastInteraction:  timePtr(time.Now().AddDate(0, 0, -5)), // 5 days ago
		}

		mockUserService.On("GetUserProfile", mock.Anything, userID).Return(userProfile, nil)

		// Mock algorithm results
		semanticItems := []models.ScoredItem{
			{ItemID: uuid.New(), Score: 0.9, Algorithm: "semantic_search", Confidence: 0.8},
			{ItemID: uuid.New(), Score: 0.8, Algorithm: "semantic_search", Confidence: 0.7},
		}

		collaborativeItems := []models.ScoredItem{
			{ItemID: uuid.New(), Score: 4.5, Algorithm: "collaborative_filtering", Confidence: 0.9},
			{ItemID: uuid.New(), Score: 4.2, Algorithm: "collaborative_filtering", Confidence: 0.8},
		}

		pageRankItems := []models.ScoredItem{
			{ItemID: uuid.New(), Score: 0.15, Algorithm: "pagerank", Confidence: 0.7},
			{ItemID: uuid.New(), Score: 0.12, Algorithm: "pagerank", Confidence: 0.6},
		}

		mockAlgorithmService.On("SemanticSearchRecommendations",
			mock.Anything, userID, userProfile.PreferenceVector, []string(nil), []string(nil), 20).
			Return(semanticItems, nil)

		mockAlgorithmService.On("CollaborativeFilteringRecommendations",
			mock.Anything, userID, 20).
			Return(collaborativeItems, nil)

		mockAlgorithmService.On("PersonalizedPageRankRecommendations",
			mock.Anything, userID, 20).
			Return(pageRankItems, nil)

		// Create request context
		reqCtx := &RecommendationContext{
			UserID:              userID,
			Count:               10,
			Context:             "home",
			IncludeExplanations: true,
			TimeoutMs:           2000,
		}

		// Execute
		result, err := orchestrator.GenerateRecommendations(context.Background(), reqCtx)

		// Verify
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, userID, result.UserID)
		assert.Equal(t, "home", result.Context)
		assert.Equal(t, ActiveUser, result.UserTier)
		assert.Equal(t, "full_personalization", result.Strategy)
		assert.False(t, result.CacheHit)
		assert.Greater(t, len(result.Recommendations), 0)
		assert.Len(t, result.AlgorithmResults, 3) // semantic, collaborative, pagerank

		// Verify algorithm results
		assert.Contains(t, result.AlgorithmResults, "semantic_search")
		assert.Contains(t, result.AlgorithmResults, "collaborative_filtering")
		assert.Contains(t, result.AlgorithmResults, "pagerank")

		// Verify recommendations have proper structure
		for i, rec := range result.Recommendations {
			assert.NotEqual(t, uuid.Nil, rec.ItemID)
			assert.Greater(t, rec.Score, 0.0)
			assert.LessOrEqual(t, rec.Score, 1.0)
			assert.Equal(t, "orchestrated", rec.Algorithm)
			assert.Equal(t, i+1, rec.Position)
			assert.Greater(t, rec.Confidence, 0.0)
			assert.LessOrEqual(t, rec.Confidence, 1.0)
			if reqCtx.IncludeExplanations {
				assert.NotNil(t, rec.Explanation)
			}
		}

		mockUserService.AssertExpectations(t)
		mockAlgorithmService.AssertExpectations(t)
	})

	t.Run("new user strategy", func(t *testing.T) {
		userID := uuid.New()

		// Mock user profile (new user)
		userProfile := &models.UserProfile{
			UserID:           userID,
			PreferenceVector: []float32{0.1, 0.2, 0.3, 0.4},
			InteractionCount: 2, // New user
			LastInteraction:  timePtr(time.Now().AddDate(0, 0, -1)),
		}

		mockUserService.On("GetUserProfile", mock.Anything, userID).Return(userProfile, nil)

		// Mock semantic search only (new user strategy)
		semanticItems := []models.ScoredItem{
			{ItemID: uuid.New(), Score: 0.9, Algorithm: "semantic_search", Confidence: 0.8},
		}

		mockAlgorithmService.On("SemanticSearchRecommendations",
			mock.Anything, userID, userProfile.PreferenceVector, []string(nil), []string(nil), 10).
			Return(semanticItems, nil)

		reqCtx := &RecommendationContext{
			UserID:  userID,
			Count:   5,
			Context: "home",
		}

		result, err := orchestrator.GenerateRecommendations(context.Background(), reqCtx)

		require.NoError(t, err)
		assert.Equal(t, NewUser, result.UserTier)
		assert.Equal(t, "popularity_with_exploration", result.Strategy)
		assert.Len(t, result.AlgorithmResults, 1) // Only semantic search
		assert.Contains(t, result.AlgorithmResults, "semantic_search")

		mockUserService.AssertExpectations(t)
		mockAlgorithmService.AssertExpectations(t)
	})

	t.Run("power user strategy", func(t *testing.T) {
		userID := uuid.New()

		// Mock user profile (power user)
		userProfile := &models.UserProfile{
			UserID:           userID,
			PreferenceVector: []float32{0.1, 0.2, 0.3, 0.4},
			InteractionCount: 75, // Power user
			LastInteraction:  timePtr(time.Now().AddDate(0, 0, -2)),
		}

		mockUserService.On("GetUserProfile", mock.Anything, userID).Return(userProfile, nil)

		// Mock advanced algorithms for power user
		collaborativeItems := []models.ScoredItem{
			{ItemID: uuid.New(), Score: 4.8, Algorithm: "collaborative_filtering", Confidence: 0.9},
		}

		pageRankItems := []models.ScoredItem{
			{ItemID: uuid.New(), Score: 0.18, Algorithm: "pagerank", Confidence: 0.8},
		}

		graphItems := []models.ScoredItem{
			{ItemID: uuid.New(), Score: 0.75, Algorithm: "graph_signal_analysis", Confidence: 0.7},
		}

		mockAlgorithmService.On("CollaborativeFilteringRecommendations",
			mock.Anything, userID, 10).
			Return(collaborativeItems, nil)

		mockAlgorithmService.On("PersonalizedPageRankRecommendations",
			mock.Anything, userID, 10).
			Return(pageRankItems, nil)

		mockAlgorithmService.On("GraphSignalAnalysisRecommendations",
			mock.Anything, userID, 10).
			Return(graphItems, nil)

		reqCtx := &RecommendationContext{
			UserID:  userID,
			Count:   5,
			Context: "home",
		}

		result, err := orchestrator.GenerateRecommendations(context.Background(), reqCtx)

		require.NoError(t, err)
		assert.Equal(t, PowerUser, result.UserTier)
		assert.Equal(t, "advanced_personalization", result.Strategy)
		assert.Len(t, result.AlgorithmResults, 3) // collaborative, pagerank, graph
		assert.Contains(t, result.AlgorithmResults, "collaborative_filtering")
		assert.Contains(t, result.AlgorithmResults, "pagerank")
		assert.Contains(t, result.AlgorithmResults, "graph_signal_analysis")

		mockUserService.AssertExpectations(t)
		mockAlgorithmService.AssertExpectations(t)
	})

	t.Run("cache hit scenario", func(t *testing.T) {
		userID := uuid.New()

		// Prepare cached result
		cachedResult := &OrchestrationResult{
			UserID:      userID,
			Context:     "home",
			UserTier:    ActiveUser,
			Strategy:    "full_personalization",
			CacheHit:    false,
			GeneratedAt: time.Now(),
			Recommendations: []models.Recommendation{
				{
					ItemID:     uuid.New(),
					Score:      0.95,
					Algorithm:  "orchestrated",
					Confidence: 0.8,
					Position:   1,
				},
			},
		}

		// Cache the result
		reqCtx := &RecommendationContext{
			UserID:  userID,
			Count:   5,
			Context: "home",
		}

		cacheKey := orchestrator.buildCacheKey(reqCtx)
		data, _ := json.Marshal(cachedResult)
		redisClient.Set(context.Background(), cacheKey, data, time.Minute)

		// Execute
		result, err := orchestrator.GenerateRecommendations(context.Background(), reqCtx)

		// Verify cache hit
		require.NoError(t, err)
		assert.True(t, result.CacheHit)
		assert.Equal(t, userID, result.UserID)
		assert.Len(t, result.Recommendations, 1)
	})
}

func TestRecommendationOrchestrator_UserTierDetermination(t *testing.T) {
	mockUserService := new(MockUserInteractionService)
	orchestrator := &RecommendationOrchestrator{
		userService: mockUserService,
		logger:      logrus.New(),
	}

	t.Run("new user tier", func(t *testing.T) {
		userID := uuid.New()
		profile := &models.UserProfile{
			UserID:           userID,
			InteractionCount: 3,
			LastInteraction:  timePtr(time.Now().AddDate(0, 0, -1)),
		}

		mockUserService.On("GetUserProfile", mock.Anything, userID).Return(profile, nil)

		tier, err := orchestrator.determineUserTier(context.Background(), userID)

		require.NoError(t, err)
		assert.Equal(t, NewUser, tier)
		mockUserService.AssertExpectations(t)
	})

	t.Run("active user tier", func(t *testing.T) {
		userID := uuid.New()
		profile := &models.UserProfile{
			UserID:           userID,
			InteractionCount: 25,
			LastInteraction:  timePtr(time.Now().AddDate(0, 0, -5)),
		}

		mockUserService.On("GetUserProfile", mock.Anything, userID).Return(profile, nil)

		tier, err := orchestrator.determineUserTier(context.Background(), userID)

		require.NoError(t, err)
		assert.Equal(t, ActiveUser, tier)
		mockUserService.AssertExpectations(t)
	})

	t.Run("power user tier", func(t *testing.T) {
		userID := uuid.New()
		profile := &models.UserProfile{
			UserID:           userID,
			InteractionCount: 75,
			LastInteraction:  timePtr(time.Now().AddDate(0, 0, -2)),
		}

		mockUserService.On("GetUserProfile", mock.Anything, userID).Return(profile, nil)

		tier, err := orchestrator.determineUserTier(context.Background(), userID)

		require.NoError(t, err)
		assert.Equal(t, PowerUser, tier)
		mockUserService.AssertExpectations(t)
	})

	t.Run("inactive user tier", func(t *testing.T) {
		userID := uuid.New()
		profile := &models.UserProfile{
			UserID:           userID,
			InteractionCount: 30,
			LastInteraction:  timePtr(time.Now().AddDate(0, 0, -45)), // 45 days ago
		}

		mockUserService.On("GetUserProfile", mock.Anything, userID).Return(profile, nil)

		tier, err := orchestrator.determineUserTier(context.Background(), userID)

		require.NoError(t, err)
		assert.Equal(t, InactiveUser, tier)
		mockUserService.AssertExpectations(t)
	})
}

func TestRecommendationOrchestrator_ScoreNormalization(t *testing.T) {
	orchestrator := &RecommendationOrchestrator{}

	t.Run("normalize scores with different ranges", func(t *testing.T) {
		items := []models.ScoredItem{
			{ItemID: uuid.New(), Score: 10.0},
			{ItemID: uuid.New(), Score: 5.0},
			{ItemID: uuid.New(), Score: 1.0},
		}

		normalized := orchestrator.normalizeScores(items)

		assert.Equal(t, 1.0, normalized[0].Score)           // Max score -> 1.0
		assert.InDelta(t, 0.444, normalized[1].Score, 0.01) // Mid score
		assert.Equal(t, 0.0, normalized[2].Score)           // Min score -> 0.0
	})

	t.Run("normalize scores with same values", func(t *testing.T) {
		items := []models.ScoredItem{
			{ItemID: uuid.New(), Score: 5.0},
			{ItemID: uuid.New(), Score: 5.0},
			{ItemID: uuid.New(), Score: 5.0},
		}

		normalized := orchestrator.normalizeScores(items)

		// All scores should be 1.0 when range is 0
		for _, item := range normalized {
			assert.Equal(t, 1.0, item.Score)
		}
	})

	t.Run("normalize empty slice", func(t *testing.T) {
		items := []models.ScoredItem{}

		normalized := orchestrator.normalizeScores(items)

		assert.Empty(t, normalized)
	})
}

func TestRecommendationOrchestrator_SigmoidCalibration(t *testing.T) {
	orchestrator := &RecommendationOrchestrator{}

	testCases := []struct {
		input       float64
		description string
	}{
		{0.0, "Low score"},
		{0.25, "Quarter score"},
		{0.5, "Mid score (inflection point)"},
		{0.75, "Three-quarter score"},
		{1.0, "High score"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			result := orchestrator.applySigmoidCalibration(tc.input)

			// Verify result is in valid range
			assert.Greater(t, result, 0.0, "Sigmoid output should be > 0")
			assert.Less(t, result, 1.0, "Sigmoid output should be < 1")

			// Verify sigmoid properties
			if tc.input == 0.5 {
				assert.InDelta(t, 0.5, result, 0.01, "Sigmoid(0.5) should be ~0.5")
			}

			// Verify monotonicity (higher input should give higher output)
			if tc.input > 0.5 {
				assert.Greater(t, result, 0.5, "Sigmoid should be > 0.5 for input > 0.5")
			} else if tc.input < 0.5 {
				assert.Less(t, result, 0.5, "Sigmoid should be < 0.5 for input < 0.5")
			}
		})
	}
}

func TestRecommendationOrchestrator_ConfidenceCalculation(t *testing.T) {
	orchestrator := &RecommendationOrchestrator{}

	t.Run("average confidence calculation", func(t *testing.T) {
		confidences := []float64{0.8, 0.9, 0.7, 0.6}

		avg := orchestrator.calculateAverageConfidence(confidences)

		assert.InDelta(t, 0.75, avg, 0.01)
	})

	t.Run("empty confidence slice", func(t *testing.T) {
		confidences := []float64{}

		avg := orchestrator.calculateAverageConfidence(confidences)

		assert.Equal(t, 0.0, avg)
	})

	t.Run("single confidence value", func(t *testing.T) {
		confidences := []float64{0.85}

		avg := orchestrator.calculateAverageConfidence(confidences)

		assert.Equal(t, 0.85, avg)
	})
}

func TestRecommendationOrchestrator_ExplanationGeneration(t *testing.T) {
	orchestrator := &RecommendationOrchestrator{}

	t.Run("single algorithm explanation", func(t *testing.T) {
		algorithms := []string{"semantic_search"}

		explanation := orchestrator.generateExplanation(algorithms, true)

		require.NotNil(t, explanation)
		assert.Equal(t, "Based on your preferences and similar content", *explanation)
	})

	t.Run("multiple algorithms explanation", func(t *testing.T) {
		algorithms := []string{"semantic_search", "collaborative_filtering"}

		explanation := orchestrator.generateExplanation(algorithms, true)

		require.NotNil(t, explanation)
		assert.Contains(t, *explanation, "2 algorithms")
		assert.Contains(t, *explanation, "semantic_search")
	})

	t.Run("explanations disabled", func(t *testing.T) {
		algorithms := []string{"semantic_search"}

		explanation := orchestrator.generateExplanation(algorithms, false)

		assert.Nil(t, explanation)
	})

	t.Run("empty algorithms", func(t *testing.T) {
		algorithms := []string{}

		explanation := orchestrator.generateExplanation(algorithms, true)

		assert.Nil(t, explanation)
	})
}

func TestRecommendationOrchestrator_FilterExcludedItems(t *testing.T) {
	orchestrator := &RecommendationOrchestrator{}

	t.Run("filter excluded items", func(t *testing.T) {
		item1 := uuid.New()
		item2 := uuid.New()
		item3 := uuid.New()

		recommendations := []models.Recommendation{
			{ItemID: item1, Score: 0.9},
			{ItemID: item2, Score: 0.8},
			{ItemID: item3, Score: 0.7},
		}

		excludeItems := []uuid.UUID{item2}

		filtered := orchestrator.filterExcludedItems(recommendations, excludeItems)

		assert.Len(t, filtered, 2)
		assert.Equal(t, item1, filtered[0].ItemID)
		assert.Equal(t, item3, filtered[1].ItemID)
	})

	t.Run("no excluded items", func(t *testing.T) {
		recommendations := []models.Recommendation{
			{ItemID: uuid.New(), Score: 0.9},
			{ItemID: uuid.New(), Score: 0.8},
		}

		excludeItems := []uuid.UUID{}

		filtered := orchestrator.filterExcludedItems(recommendations, excludeItems)

		assert.Len(t, filtered, 2)
		assert.Equal(t, recommendations, filtered)
	})
}

// Helper function
func timePtr(t time.Time) *time.Time {
	return &t
}
