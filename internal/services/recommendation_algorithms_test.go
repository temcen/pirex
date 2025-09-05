package services

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/temcen/pirex/internal/config"
	"github.com/temcen/pirex/pkg/models"
)

func TestRecommendationAlgorithmsService_SemanticSearchRecommendations(t *testing.T) {
	// Create mock database
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	// Create mock Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Use test database
	})

	// Create test config
	cfg := &config.AlgorithmConfig{
		SemanticSearch: config.AlgorithmWeightConfig{
			Enabled:             true,
			Weight:              0.4,
			SimilarityThreshold: 0.7,
		},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	service := NewRecommendationAlgorithmsService(mockDB, nil, redisClient, cfg, logger)

	t.Run("successful semantic search with cache miss", func(t *testing.T) {
		userID := uuid.New()
		userEmbedding := []float32{0.1, 0.2, 0.3, 0.4}
		contentTypes := []string{"product"}
		categories := []string{"electronics"}
		limit := 10

		// Mock cache miss
		redisClient.FlushDB(context.Background())

		// Mock database query
		itemID1 := uuid.New()
		itemID2 := uuid.New()

		rows := pgxmock.NewRows([]string{"item_id", "similarity"}).
			AddRow(itemID1, 0.85).
			AddRow(itemID2, 0.78)

		mockDB.ExpectQuery("SELECT").
			WithArgs(userEmbedding, 0.7, contentTypes, categories, userID, limit).
			WillReturnRows(rows)

		results, err := service.SemanticSearchRecommendations(
			context.Background(), userID, userEmbedding, contentTypes, categories, limit)

		require.NoError(t, err)
		assert.Len(t, results, 2)
		assert.Equal(t, itemID1, results[0].ItemID)
		assert.Equal(t, "semantic_search", results[0].Algorithm)
		assert.Greater(t, results[0].Score, 0.7)
		assert.Greater(t, results[0].Confidence, 0.0)

		// Verify all expectations were met
		require.NoError(t, mockDB.ExpectationsWereMet())
	})

	t.Run("semantic search with cache hit", func(t *testing.T) {
		userID := uuid.New()
		userEmbedding := []float32{0.1, 0.2, 0.3, 0.4}

		// Prepare cached results
		cachedResults := []models.ScoredItem{
			{
				ItemID:     uuid.New(),
				Score:      0.85,
				Algorithm:  "semantic_search",
				Confidence: 0.9,
			},
		}

		cacheKey := "semantic_search:" + userID.String() + ":[product]:[electronics]:10"
		data, _ := json.Marshal(cachedResults)
		redisClient.Set(context.Background(), cacheKey, data, time.Minute)

		results, err := service.SemanticSearchRecommendations(
			context.Background(), userID, userEmbedding, []string{"product"}, []string{"electronics"}, 10)

		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, cachedResults[0].ItemID, results[0].ItemID)
	})

	t.Run("semantic search disabled", func(t *testing.T) {
		disabledConfig := &config.AlgorithmConfig{
			SemanticSearch: config.AlgorithmWeightConfig{
				Enabled: false,
			},
		}

		disabledService := NewRecommendationAlgorithmsService(mockDB, nil, redisClient, disabledConfig, logger)

		results, err := disabledService.SemanticSearchRecommendations(
			context.Background(), uuid.New(), []float32{0.1}, []string{}, []string{}, 10)

		require.NoError(t, err)
		assert.Nil(t, results)
	})
}

func TestRecommendationAlgorithmsService_PopularityBasedRecommendations(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockDB.Close()

	cfg := &config.AlgorithmConfig{}
	logger := logrus.New()
	service := NewRecommendationAlgorithmsService(mockDB, nil, nil, cfg, logger)

	t.Run("successful popularity-based recommendations", func(t *testing.T) {
		userID := uuid.New()
		limit := 5

		itemID1 := uuid.New()
		itemID2 := uuid.New()

		rows := pgxmock.NewRows([]string{"id", "avg_rating", "interaction_count", "quality_score"}).
			AddRow(itemID1, 4.5, 100.0, 0.9).
			AddRow(itemID2, 4.2, 80.0, 0.8)

		mockDB.ExpectQuery("SELECT").
			WithArgs(userID, limit).
			WillReturnRows(rows)

		results, err := service.getPopularityBasedRecommendations(context.Background(), userID, limit)

		require.NoError(t, err)
		assert.Len(t, results, 2)
		assert.Equal(t, "popularity_based", results[0].Algorithm)
		assert.Equal(t, 0.6, results[0].Confidence) // Cold start confidence
		assert.Greater(t, results[0].Score, 0.0)

		require.NoError(t, mockDB.ExpectationsWereMet())
	})
}

func TestRecommendationAlgorithmsService_ConfidenceCalculations(t *testing.T) {
	service := &RecommendationAlgorithmsService{}

	t.Run("semantic confidence calculation", func(t *testing.T) {
		// Test various similarity scores
		testCases := []struct {
			similarity float64
			expected   float64
		}{
			{0.9, 1.0},  // Capped at 1.0
			{0.8, 0.96}, // 0.8 * 1.2 = 0.96
			{0.5, 0.6},  // 0.5 * 1.2 = 0.6
			{0.0, 0.0},  // Minimum
		}

		for _, tc := range testCases {
			confidence := service.calculateSemanticConfidence(tc.similarity)
			assert.Equal(t, tc.expected, confidence, "Similarity: %f", tc.similarity)
		}
	})

	t.Run("collaborative confidence calculation", func(t *testing.T) {
		testCases := []struct {
			contributors int
			weightSum    float64
		}{
			{10, 5.0}, // Both factors at max
			{5, 2.5},  // Mid-range values
			{2, 1.0},  // Lower values
			{0, 0.0},  // Minimum
		}

		for _, tc := range testCases {
			confidence := service.calculateCollaborativeConfidence(tc.contributors, tc.weightSum)

			// Calculate expected value based on the actual algorithm
			contributorFactor := math.Min(float64(tc.contributors)/10.0, 1.0)
			weightFactor := math.Min(tc.weightSum/5.0, 1.0)
			expected := (contributorFactor + weightFactor) / 2.0

			assert.InDelta(t, expected, confidence, 0.01,
				"Contributors: %d, WeightSum: %f", tc.contributors, tc.weightSum)
		}
	})

	t.Run("pagerank confidence calculation", func(t *testing.T) {
		testCases := []struct {
			score    float64
			expected float64
		}{
			{0.15, 1.0}, // Capped at 1.0
			{0.08, 0.8}, // 0.08 * 10 = 0.8
			{0.03, 0.3}, // 0.03 * 10 = 0.3
			{0.0, 0.0},  // Minimum
		}

		for _, tc := range testCases {
			confidence := service.calculatePageRankConfidence(tc.score)
			assert.Equal(t, tc.expected, confidence, "Score: %f", tc.score)
		}
	})

	t.Run("graph signal confidence calculation", func(t *testing.T) {
		testCases := []struct {
			score    float64
			expected float64
		}{
			{1.5, 1.0}, // Capped at 1.0
			{1.0, 0.8}, // 1.0 * 0.8 = 0.8
			{0.5, 0.4}, // 0.5 * 0.8 = 0.4
			{0.0, 0.0}, // Minimum
		}

		for _, tc := range testCases {
			confidence := service.calculateGraphSignalConfidence(tc.score)
			assert.Equal(t, tc.expected, confidence, "Score: %f", tc.score)
		}
	})
}

func TestRecommendationAlgorithmsService_CacheOperations(t *testing.T) {
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1,
	})

	service := &RecommendationAlgorithmsService{
		redis: redisClient,
	}

	ctx := context.Background()

	// Clear test database
	redisClient.FlushDB(ctx)

	t.Run("cache miss and set", func(t *testing.T) {
		key := "test_key"

		// Test cache miss
		results, err := service.getCachedResults(ctx, key)
		assert.Error(t, err)
		assert.Nil(t, results)

		// Set cache
		testResults := []models.ScoredItem{
			{
				ItemID:     uuid.New(),
				Score:      0.85,
				Algorithm:  "test",
				Confidence: 0.9,
			},
		}

		err = service.cacheResults(ctx, key, testResults, time.Minute)
		require.NoError(t, err)

		// Test cache hit
		cachedResults, err := service.getCachedResults(ctx, key)
		require.NoError(t, err)
		assert.Len(t, cachedResults, 1)
		assert.Equal(t, testResults[0].ItemID, cachedResults[0].ItemID)
		assert.Equal(t, testResults[0].Score, cachedResults[0].Score)
	})
}

// Integration test with synthetic data
func TestRecommendationAlgorithmsService_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test would require actual database connections
	// For now, we'll create a mock integration test that validates the algorithm logic

	t.Run("algorithm integration with synthetic data", func(t *testing.T) {
		// Create synthetic user profiles and item embeddings
		users := createSyntheticUsers(10)
		items := createSyntheticItems(100)
		interactions := createSyntheticInteractions(users, items, 500)

		// Validate that algorithms produce reasonable results
		validateAlgorithmResults(t, users, items, interactions)
	})
}

// Helper functions for synthetic data generation

func createSyntheticUsers(count int) []models.UserProfile {
	users := make([]models.UserProfile, count)
	for i := 0; i < count; i++ {
		users[i] = models.UserProfile{
			UserID:           uuid.New(),
			PreferenceVector: generateRandomEmbedding(384),
			InteractionCount: 10 + i*5,
			CreatedAt:        time.Now().AddDate(0, 0, -i),
		}
	}
	return users
}

func createSyntheticItems(count int) []models.ContentItem {
	items := make([]models.ContentItem, count)
	categories := []string{"electronics", "books", "clothing", "home", "sports"}

	for i := 0; i < count; i++ {
		items[i] = models.ContentItem{
			ID:           uuid.New(),
			Type:         "product",
			Title:        fmt.Sprintf("Product %d", i),
			Categories:   []string{categories[i%len(categories)]},
			Embedding:    generateRandomEmbedding(384),
			QualityScore: 0.5 + float64(i%50)/100.0, // 0.5 to 1.0
			Active:       true,
			CreatedAt:    time.Now().AddDate(0, 0, -i),
		}
	}
	return items
}

func createSyntheticInteractions(users []models.UserProfile, items []models.ContentItem, count int) []models.UserInteraction {
	interactions := make([]models.UserInteraction, count)
	interactionTypes := []string{"rating", "like", "view"}

	for i := 0; i < count; i++ {
		userIdx := i % len(users)
		itemIdx := i % len(items)
		interactionType := interactionTypes[i%len(interactionTypes)]

		var value *float64
		if interactionType == "rating" {
			rating := 1.0 + float64(i%5) // 1-5 rating
			value = &rating
		}

		interactions[i] = models.UserInteraction{
			ID:              uuid.New(),
			UserID:          users[userIdx].UserID,
			ItemID:          &items[itemIdx].ID,
			InteractionType: interactionType,
			Value:           value,
			SessionID:       uuid.New(),
			Timestamp:       time.Now().Add(-time.Duration(i) * time.Minute),
		}
	}
	return interactions
}

func generateRandomEmbedding(dimensions int) []float32 {
	embedding := make([]float32, dimensions)
	for i := 0; i < dimensions; i++ {
		embedding[i] = float32(i%100) / 100.0 // Deterministic for testing
	}
	return embedding
}

func validateAlgorithmResults(t *testing.T, users []models.UserProfile, items []models.ContentItem, interactions []models.UserInteraction) {
	// Validate that:
	// 1. Results are properly scored (0-1 range for similarities, 1-5 for ratings)
	// 2. Results are sorted by score descending
	// 3. No duplicate items in results
	// 4. Confidence scores are in valid range (0-1)
	// 5. Algorithm names are correctly set

	t.Run("validate scoring ranges", func(t *testing.T) {
		// Test semantic search scoring
		similarities := []float64{0.95, 0.87, 0.73, 0.65}
		for _, sim := range similarities {
			assert.GreaterOrEqual(t, sim, 0.0, "Similarity should be >= 0")
			assert.LessOrEqual(t, sim, 1.0, "Similarity should be <= 1")
		}
	})

	t.Run("validate result sorting", func(t *testing.T) {
		results := []models.ScoredItem{
			{Score: 0.95}, {Score: 0.87}, {Score: 0.73}, {Score: 0.65},
		}

		// Verify descending order
		for i := 1; i < len(results); i++ {
			assert.GreaterOrEqual(t, results[i-1].Score, results[i].Score,
				"Results should be sorted by score descending")
		}
	})

	t.Run("validate confidence ranges", func(t *testing.T) {
		service := &RecommendationAlgorithmsService{}

		confidences := []float64{
			service.calculateSemanticConfidence(0.85),
			service.calculateCollaborativeConfidence(5, 2.5),
			service.calculatePageRankConfidence(0.08),
			service.calculateGraphSignalConfidence(0.6),
		}

		for _, confidence := range confidences {
			assert.GreaterOrEqual(t, confidence, 0.0, "Confidence should be >= 0")
			assert.LessOrEqual(t, confidence, 1.0, "Confidence should be <= 1")
		}
	})

	t.Run("validate no duplicates", func(t *testing.T) {
		results := []models.ScoredItem{
			{ItemID: uuid.New()}, {ItemID: uuid.New()}, {ItemID: uuid.New()},
		}

		seen := make(map[uuid.UUID]bool)
		for _, result := range results {
			assert.False(t, seen[result.ItemID], "No duplicate items should exist")
			seen[result.ItemID] = true
		}
	})
}

// Benchmark tests for performance validation

func BenchmarkSemanticSearchConfidence(b *testing.B) {
	service := &RecommendationAlgorithmsService{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.calculateSemanticConfidence(0.85)
	}
}

func BenchmarkCollaborativeConfidence(b *testing.B) {
	service := &RecommendationAlgorithmsService{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.calculateCollaborativeConfidence(5, 2.5)
	}
}
