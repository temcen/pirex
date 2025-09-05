package services

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/temcen/pirex/pkg/models"
)

func TestMLRankingService_RankRecommendations(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	service := NewMLRankingService(logger)

	t.Run("successful ranking with multiple recommendations", func(t *testing.T) {
		// Create test recommendations
		recommendations := []models.Recommendation{
			{
				ItemID:     uuid.New(),
				Score:      0.7,
				Algorithm:  "semantic_search",
				Confidence: 0.8,
				Position:   1,
			},
			{
				ItemID:     uuid.New(),
				Score:      0.9,
				Algorithm:  "collaborative_filtering",
				Confidence: 0.9,
				Position:   2,
			},
			{
				ItemID:     uuid.New(),
				Score:      0.6,
				Algorithm:  "pagerank",
				Confidence: 0.7,
				Position:   3,
			},
		}

		// Create test user profile
		userProfile := &models.UserProfile{
			UserID:           uuid.New(),
			PreferenceVector: []float32{0.1, 0.2, 0.3, 0.4},
			InteractionCount: 50,
			LastInteraction:  timePtr(time.Now().AddDate(0, 0, -1)),
		}

		contextFeatures := map[string]interface{}{
			"time_of_day":    "evening",
			"session_length": 300,
		}

		// Execute ranking
		ranked, err := service.RankRecommendations(
			context.Background(), recommendations, userProfile, contextFeatures,
		)

		// Verify results
		require.NoError(t, err)
		assert.Len(t, ranked, 3)

		// Verify positions are updated
		for i, rec := range ranked {
			assert.Equal(t, i+1, rec.Position)
			assert.Greater(t, rec.Score, 0.0)
			assert.LessOrEqual(t, rec.Score, 1.0)
			assert.Greater(t, rec.Confidence, 0.0)
			assert.LessOrEqual(t, rec.Confidence, 1.0)
		}

		// Verify sorting (scores should be descending)
		for i := 1; i < len(ranked); i++ {
			assert.GreaterOrEqual(t, ranked[i-1].Score, ranked[i].Score,
				"Recommendations should be sorted by score descending")
		}
	})

	t.Run("ranking with empty recommendations", func(t *testing.T) {
		recommendations := []models.Recommendation{}

		ranked, err := service.RankRecommendations(
			context.Background(), recommendations, nil, nil,
		)

		require.NoError(t, err)
		assert.Empty(t, ranked)
	})

	t.Run("ranking with nil user profile", func(t *testing.T) {
		recommendations := []models.Recommendation{
			{
				ItemID:     uuid.New(),
				Score:      0.8,
				Algorithm:  "semantic_search",
				Confidence: 0.7,
				Position:   1,
			},
		}

		ranked, err := service.RankRecommendations(
			context.Background(), recommendations, nil, nil,
		)

		require.NoError(t, err)
		assert.Len(t, ranked, 1)
		assert.Greater(t, ranked[0].Score, 0.0)
	})
}

func TestMLRankingService_FeatureExtraction(t *testing.T) {
	service := NewMLRankingService(logrus.New())

	t.Run("extract features with user profile", func(t *testing.T) {
		rec := models.Recommendation{
			ItemID:     uuid.New(),
			Score:      0.85,
			Algorithm:  "collaborative_filtering",
			Confidence: 0.9,
		}

		userProfile := &models.UserProfile{
			UserID:           uuid.New(),
			PreferenceVector: []float32{0.1, 0.2, 0.3},
			InteractionCount: 75,
		}

		contextFeatures := map[string]interface{}{
			"position": 3,
		}

		features := service.extractFeatures(rec, userProfile, contextFeatures)

		// Verify all features are in valid range [0,1]
		assert.GreaterOrEqual(t, features.ContentSimilarity, 0.0)
		assert.LessOrEqual(t, features.ContentSimilarity, 1.0)
		assert.GreaterOrEqual(t, features.UserItemAffinity, 0.0)
		assert.LessOrEqual(t, features.UserItemAffinity, 1.0)
		assert.GreaterOrEqual(t, features.PopularityScore, 0.0)
		assert.LessOrEqual(t, features.PopularityScore, 1.0)
		assert.GreaterOrEqual(t, features.RecencyScore, 0.0)
		assert.LessOrEqual(t, features.RecencyScore, 1.0)
		assert.GreaterOrEqual(t, features.DiversityScore, 0.0)
		assert.LessOrEqual(t, features.DiversityScore, 1.0)
		assert.Equal(t, rec.Confidence, features.AlgorithmConfidence)
	})

	t.Run("extract features without user profile", func(t *testing.T) {
		rec := models.Recommendation{
			ItemID:     uuid.New(),
			Score:      0.7,
			Algorithm:  "semantic_search",
			Confidence: 0.8,
		}

		features := service.extractFeatures(rec, nil, nil)

		// Should still produce valid features
		assert.GreaterOrEqual(t, features.ContentSimilarity, 0.0)
		assert.LessOrEqual(t, features.ContentSimilarity, 1.0)
		assert.Equal(t, 0.5, features.UserItemAffinity) // Default for unknown users
		assert.Equal(t, rec.Confidence, features.AlgorithmConfidence)
	})
}

func TestMLRankingService_ScoreCalculation(t *testing.T) {
	service := NewMLRankingService(logrus.New())

	t.Run("calculate ML score", func(t *testing.T) {
		features := FeatureVector{
			ContentSimilarity:   0.8,
			UserItemAffinity:    0.9,
			PopularityScore:     0.7,
			RecencyScore:        0.6,
			DiversityScore:      0.5,
			AlgorithmConfidence: 0.85,
		}

		score := service.calculateMLScore(features)

		// Score should be in valid range after sigmoid activation
		assert.Greater(t, score, 0.0)
		assert.Less(t, score, 1.0)
	})

	t.Run("calculate score with zero features", func(t *testing.T) {
		features := FeatureVector{
			ContentSimilarity:   0.0,
			UserItemAffinity:    0.0,
			PopularityScore:     0.0,
			RecencyScore:        0.0,
			DiversityScore:      0.0,
			AlgorithmConfidence: 0.0,
		}

		score := service.calculateMLScore(features)

		// Should still produce valid score (sigmoid of 0 around 0.5)
		assert.Greater(t, score, 0.0)
		assert.Less(t, score, 1.0)
	})

	t.Run("calculate score with maximum features", func(t *testing.T) {
		features := FeatureVector{
			ContentSimilarity:   1.0,
			UserItemAffinity:    1.0,
			PopularityScore:     1.0,
			RecencyScore:        1.0,
			DiversityScore:      1.0,
			AlgorithmConfidence: 1.0,
		}

		score := service.calculateMLScore(features)

		// Should produce high score
		assert.Greater(t, score, 0.5)
		assert.Less(t, score, 1.0)
	})
}

func TestMLRankingService_ConfidenceCalculation(t *testing.T) {
	service := NewMLRankingService(logrus.New())

	t.Run("high confidence with strong features", func(t *testing.T) {
		features := FeatureVector{
			ContentSimilarity:   0.9,
			UserItemAffinity:    0.8,
			AlgorithmConfidence: 0.85,
			PopularityScore:     0.7,
			RecencyScore:        0.6,
			DiversityScore:      0.5,
		}

		confidence := service.calculateFeatureBasedConfidence(features)

		assert.Greater(t, confidence, 0.7) // Should be high confidence
		assert.LessOrEqual(t, confidence, 1.0)
	})

	t.Run("low confidence with weak features", func(t *testing.T) {
		features := FeatureVector{
			ContentSimilarity:   0.3,
			UserItemAffinity:    0.2,
			AlgorithmConfidence: 0.4,
			PopularityScore:     0.3,
			RecencyScore:        0.2,
			DiversityScore:      0.1,
		}

		confidence := service.calculateFeatureBasedConfidence(features)

		assert.GreaterOrEqual(t, confidence, 0.1) // Minimum confidence
		assert.Less(t, confidence, 0.5)
	})

	t.Run("consistency bonus with multiple strong features", func(t *testing.T) {
		features := FeatureVector{
			ContentSimilarity:   0.8,  // Strong
			UserItemAffinity:    0.75, // Strong
			AlgorithmConfidence: 0.9,  // Strong
			PopularityScore:     0.3,
			RecencyScore:        0.2,
			DiversityScore:      0.1,
		}

		confidence := service.calculateFeatureBasedConfidence(features)

		// Should get consistency bonus for 3 strong features
		assert.Greater(t, confidence, 0.8)
		assert.LessOrEqual(t, confidence, 1.0)
	})
}

func TestMLRankingService_PopularityScoring(t *testing.T) {
	service := NewMLRankingService(logrus.New())

	testCases := []struct {
		algorithm     string
		expectedScore float64
		description   string
	}{
		{"collaborative_filtering", 0.8, "Collaborative filtering should have high popularity"},
		{"pagerank", 0.7, "PageRank should have medium-high popularity"},
		{"semantic_search", 0.6, "Semantic search should have medium popularity"},
		{"graph_signal_analysis", 0.5, "Graph signal should have medium popularity"},
		{"unknown_algorithm", 0.5, "Unknown algorithms should have default popularity"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			rec := models.Recommendation{
				Algorithm: tc.algorithm,
			}

			score := service.calculatePopularityScore(rec)

			assert.Equal(t, tc.expectedScore, score)
		})
	}
}

func TestMLRankingService_ModelWeightUpdates(t *testing.T) {
	service := NewMLRankingService(logrus.New())

	t.Run("update weights with feedback", func(t *testing.T) {
		// Get initial weights
		initialWeights := service.GetModelWeights()

		// Create feedback data
		feedback := []RankingFeedback{
			{
				ItemID: uuid.New(),
				Features: FeatureVector{
					ContentSimilarity:   0.8,
					UserItemAffinity:    0.7,
					PopularityScore:     0.6,
					RecencyScore:        0.5,
					DiversityScore:      0.4,
					AlgorithmConfidence: 0.9,
				},
				ActualScore: 0.9, // High actual score
				Timestamp:   time.Now(),
			},
			{
				ItemID: uuid.New(),
				Features: FeatureVector{
					ContentSimilarity:   0.3,
					UserItemAffinity:    0.2,
					PopularityScore:     0.4,
					RecencyScore:        0.3,
					DiversityScore:      0.2,
					AlgorithmConfidence: 0.5,
				},
				ActualScore: 0.2, // Low actual score
				Timestamp:   time.Now(),
			},
		}

		// Update weights
		err := service.UpdateModelWeights(context.Background(), feedback)
		require.NoError(t, err)

		// Get updated weights
		updatedWeights := service.GetModelWeights()

		// Weights should have changed
		assert.NotEqual(t, initialWeights, updatedWeights)

		// Weights should still sum to approximately 1.0
		sum := updatedWeights.ContentSimilarity + updatedWeights.UserItemAffinity +
			updatedWeights.PopularityScore + updatedWeights.RecencyScore +
			updatedWeights.DiversityScore + updatedWeights.AlgorithmConfidence

		assert.InDelta(t, 1.0, sum, 0.01)
	})

	t.Run("update weights with empty feedback", func(t *testing.T) {
		initialWeights := service.GetModelWeights()

		err := service.UpdateModelWeights(context.Background(), []RankingFeedback{})
		require.NoError(t, err)

		// Weights should remain unchanged
		updatedWeights := service.GetModelWeights()
		assert.Equal(t, initialWeights, updatedWeights)
	})

	t.Run("set and get model weights", func(t *testing.T) {
		newWeights := FeatureVector{
			ContentSimilarity:   0.3,
			UserItemAffinity:    0.3,
			PopularityScore:     0.2,
			RecencyScore:        0.1,
			DiversityScore:      0.05,
			AlgorithmConfidence: 0.05,
		}

		service.SetModelWeights(newWeights)
		retrievedWeights := service.GetModelWeights()

		// Weights should be normalized to sum to 1.0
		sum := retrievedWeights.ContentSimilarity + retrievedWeights.UserItemAffinity +
			retrievedWeights.PopularityScore + retrievedWeights.RecencyScore +
			retrievedWeights.DiversityScore + retrievedWeights.AlgorithmConfidence

		assert.InDelta(t, 1.0, sum, 0.01)
	})
}

func TestMLRankingService_UserItemAffinity(t *testing.T) {
	service := NewMLRankingService(logrus.New())

	t.Run("affinity with user profile", func(t *testing.T) {
		rec := models.Recommendation{
			Score:      0.8,
			Confidence: 0.9,
		}

		userProfile := &models.UserProfile{
			PreferenceVector: []float32{0.1, 0.2, 0.3},
			InteractionCount: 50,
		}

		affinity := service.calculateUserItemAffinity(rec, userProfile)

		assert.Greater(t, affinity, 0.0)
		assert.LessOrEqual(t, affinity, 1.0)
	})

	t.Run("affinity without user profile", func(t *testing.T) {
		rec := models.Recommendation{
			Score:      0.8,
			Confidence: 0.9,
		}

		affinity := service.calculateUserItemAffinity(rec, nil)

		assert.Equal(t, 0.5, affinity) // Default affinity
	})

	t.Run("affinity with high interaction count", func(t *testing.T) {
		rec := models.Recommendation{
			Score:      0.6,
			Confidence: 0.7,
		}

		userProfile := &models.UserProfile{
			PreferenceVector: []float32{0.1, 0.2, 0.3},
			InteractionCount: 200, // High interaction count
		}

		affinity := service.calculateUserItemAffinity(rec, userProfile)

		// Should get interaction boost
		baseAffinity := rec.Score * rec.Confidence
		assert.Greater(t, affinity, baseAffinity)
		assert.LessOrEqual(t, affinity, 1.0)
	})
}

func TestMLRankingService_RecencyScoring(t *testing.T) {
	service := NewMLRankingService(logrus.New())

	t.Run("recency with high confidence", func(t *testing.T) {
		rec := models.Recommendation{
			Confidence: 0.9,
		}

		recency := service.calculateRecencyScore(rec)

		assert.Greater(t, recency, 0.7) // Should be high recency
		assert.LessOrEqual(t, recency, 1.0)
	})

	t.Run("recency with low confidence", func(t *testing.T) {
		rec := models.Recommendation{
			Confidence: 0.3,
		}

		recency := service.calculateRecencyScore(rec)

		assert.GreaterOrEqual(t, recency, 0.1) // Minimum recency
		assert.Less(t, recency, 0.8)
	})
}

func TestMLRankingService_DiversityScoring(t *testing.T) {
	service := NewMLRankingService(logrus.New())

	t.Run("diversity with position context", func(t *testing.T) {
		rec := models.Recommendation{}

		contextFeatures := map[string]interface{}{
			"position": 5,
		}

		diversity := service.calculateDiversityScore(rec, contextFeatures)

		// Should get diversity bonus for later position
		assert.Greater(t, diversity, 0.6) // Base diversity
		assert.LessOrEqual(t, diversity, 1.0)
	})

	t.Run("diversity without context", func(t *testing.T) {
		rec := models.Recommendation{}

		diversity := service.calculateDiversityScore(rec, nil)

		assert.Equal(t, 0.6, diversity) // Base diversity
	})

	t.Run("diversity with high position", func(t *testing.T) {
		rec := models.Recommendation{}

		contextFeatures := map[string]interface{}{
			"position": 10,
		}

		diversity := service.calculateDiversityScore(rec, contextFeatures)

		// Should be capped at 1.0
		assert.InDelta(t, 0.9, diversity, 0.01) // 0.6 + min(10*0.05, 0.3)
	})
}
