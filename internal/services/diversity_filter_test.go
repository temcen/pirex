package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/temcen/pirex/internal/config"
	"github.com/temcen/pirex/pkg/models"
)

func TestDiversityFilter_CalculateJaccardSimilarity(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	config := &config.DiversityConfig{
		IntraListDiversity:     0.3,
		CategoryMaxItems:       3,
		SerendipityRatio:       0.15,
		MaxSimilarityThreshold: 0.8,
		TemporalDecayFactor:    7.0,
		MaxRecentSimilarItems:  2,
	}

	df := NewDiversityFilter(nil, config, logger)

	tests := []struct {
		name     string
		set1     []string
		set2     []string
		expected float64
	}{
		{
			name:     "identical sets",
			set1:     []string{"electronics", "smartphones"},
			set2:     []string{"electronics", "smartphones"},
			expected: 1.0,
		},
		{
			name:     "no overlap",
			set1:     []string{"electronics", "smartphones"},
			set2:     []string{"books", "fiction"},
			expected: 0.0,
		},
		{
			name:     "partial overlap",
			set1:     []string{"electronics", "smartphones", "android"},
			set2:     []string{"electronics", "tablets"},
			expected: 0.25, // 1 intersection / 4 union
		},
		{
			name:     "empty sets",
			set1:     []string{},
			set2:     []string{},
			expected: 1.0,
		},
		{
			name:     "one empty set",
			set1:     []string{"electronics"},
			set2:     []string{},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := df.calculateJaccardSimilarity(tt.set1, tt.set2)
			assert.InDelta(t, tt.expected, result, 0.001)
		})
	}
}

func TestDiversityFilter_CalculateCosineSimilarity(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	config := &config.DiversityConfig{
		IntraListDiversity:     0.3,
		CategoryMaxItems:       3,
		SerendipityRatio:       0.15,
		MaxSimilarityThreshold: 0.8,
		TemporalDecayFactor:    7.0,
		MaxRecentSimilarItems:  2,
	}

	df := NewDiversityFilter(nil, config, logger)

	tests := []struct {
		name     string
		vec1     []float32
		vec2     []float32
		expected float64
	}{
		{
			name:     "identical vectors",
			vec1:     []float32{1.0, 0.0, 0.0},
			vec2:     []float32{1.0, 0.0, 0.0},
			expected: 1.0,
		},
		{
			name:     "orthogonal vectors",
			vec1:     []float32{1.0, 0.0, 0.0},
			vec2:     []float32{0.0, 1.0, 0.0},
			expected: 0.0,
		},
		{
			name:     "opposite vectors",
			vec1:     []float32{1.0, 0.0, 0.0},
			vec2:     []float32{-1.0, 0.0, 0.0},
			expected: -1.0,
		},
		{
			name:     "different lengths",
			vec1:     []float32{1.0, 0.0},
			vec2:     []float32{1.0, 0.0, 0.0},
			expected: 0.0,
		},
		{
			name:     "zero vectors",
			vec1:     []float32{0.0, 0.0, 0.0},
			vec2:     []float32{0.0, 0.0, 0.0},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := df.calculateCosineSimilarity(tt.vec1, tt.vec2)
			assert.InDelta(t, tt.expected, result, 0.001)
		})
	}
}

func TestDiversityFilter_ApplyIntraListDiversityFilter(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	config := &config.DiversityConfig{
		IntraListDiversity:     0.3,
		CategoryMaxItems:       3,
		SerendipityRatio:       0.15,
		MaxSimilarityThreshold: 0.5, // Lower threshold to actually filter items
		TemporalDecayFactor:    7.0,
		MaxRecentSimilarItems:  2,
	}

	df := NewDiversityFilter(nil, config, logger)

	// Create test content items - make items 1 and 2 very similar
	contentItems := map[uuid.UUID]*models.ContentItem{
		uuid.MustParse("00000000-0000-0000-0000-000000000001"): {
			ID:         uuid.MustParse("00000000-0000-0000-0000-000000000001"),
			Categories: []string{"electronics", "smartphones", "apple"},
			Embedding:  []float32{1.0, 0.0, 0.0},
		},
		uuid.MustParse("00000000-0000-0000-0000-000000000002"): {
			ID:         uuid.MustParse("00000000-0000-0000-0000-000000000002"),
			Categories: []string{"electronics", "smartphones", "apple"}, // Identical categories
			Embedding:  []float32{0.99, 0.01, 0.0},                      // Very similar embedding
		},
		uuid.MustParse("00000000-0000-0000-0000-000000000003"): {
			ID:         uuid.MustParse("00000000-0000-0000-0000-000000000003"),
			Categories: []string{"books", "fiction"},
			Embedding:  []float32{0.0, 1.0, 0.0},
		},
	}

	// Create test recommendations (similar items first)
	recommendations := []FilteredRecommendation{
		{
			Recommendation: models.Recommendation{
				ItemID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
				Score:  0.9,
			},
		},
		{
			Recommendation: models.Recommendation{
				ItemID: uuid.MustParse("00000000-0000-0000-0000-000000000002"),
				Score:  0.8,
			},
		},
		{
			Recommendation: models.Recommendation{
				ItemID: uuid.MustParse("00000000-0000-0000-0000-000000000003"),
				Score:  0.7,
			},
		},
	}

	result := df.applyIntraListDiversityFilter(recommendations, contentItems)

	// With lower threshold (0.5), should filter out the similar item 2
	require.Len(t, result, 2)
	assert.Equal(t, uuid.MustParse("00000000-0000-0000-0000-000000000001"), result[0].ItemID)
	assert.Equal(t, uuid.MustParse("00000000-0000-0000-0000-000000000003"), result[1].ItemID)
}

func TestDiversityFilter_ApplyCategoryDiversityFilter(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	config := &config.DiversityConfig{
		IntraListDiversity:     0.3,
		CategoryMaxItems:       2, // Max 2 items per category
		SerendipityRatio:       0.15,
		MaxSimilarityThreshold: 0.8,
		TemporalDecayFactor:    7.0,
		MaxRecentSimilarItems:  2,
	}

	df := NewDiversityFilter(nil, config, logger)

	// Create test content items
	contentItems := map[uuid.UUID]*models.ContentItem{
		uuid.MustParse("00000000-0000-0000-0000-000000000001"): {
			ID:         uuid.MustParse("00000000-0000-0000-0000-000000000001"),
			Categories: []string{"electronics"},
		},
		uuid.MustParse("00000000-0000-0000-0000-000000000002"): {
			ID:         uuid.MustParse("00000000-0000-0000-0000-000000000002"),
			Categories: []string{"electronics"},
		},
		uuid.MustParse("00000000-0000-0000-0000-000000000003"): {
			ID:         uuid.MustParse("00000000-0000-0000-0000-000000000003"),
			Categories: []string{"electronics"}, // Third electronics item - should be filtered
		},
		uuid.MustParse("00000000-0000-0000-0000-000000000004"): {
			ID:         uuid.MustParse("00000000-0000-0000-0000-000000000004"),
			Categories: []string{"books"},
		},
	}

	// Create test recommendations
	recommendations := []FilteredRecommendation{
		{Recommendation: models.Recommendation{ItemID: uuid.MustParse("00000000-0000-0000-0000-000000000001")}},
		{Recommendation: models.Recommendation{ItemID: uuid.MustParse("00000000-0000-0000-0000-000000000002")}},
		{Recommendation: models.Recommendation{ItemID: uuid.MustParse("00000000-0000-0000-0000-000000000003")}},
		{Recommendation: models.Recommendation{ItemID: uuid.MustParse("00000000-0000-0000-0000-000000000004")}},
	}

	result := df.applyCategoryDiversityFilter(recommendations, contentItems)

	// Should keep first 2 electronics items and the books item
	require.Len(t, result, 3)
	assert.Equal(t, uuid.MustParse("00000000-0000-0000-0000-000000000001"), result[0].ItemID)
	assert.Equal(t, uuid.MustParse("00000000-0000-0000-0000-000000000002"), result[1].ItemID)
	assert.Equal(t, uuid.MustParse("00000000-0000-0000-0000-000000000004"), result[2].ItemID)
}

func TestDiversityFilter_ApplyTemporalDiversityFilter(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	config := &config.DiversityConfig{
		IntraListDiversity:     0.3,
		CategoryMaxItems:       3,
		SerendipityRatio:       0.15,
		MaxSimilarityThreshold: 0.8,
		TemporalDecayFactor:    7.0,
		MaxRecentSimilarItems:  1, // Max 1 similar to recent
	}

	df := NewDiversityFilter(nil, config, logger)

	// Create test content items
	contentItems := map[uuid.UUID]*models.ContentItem{
		uuid.MustParse("00000000-0000-0000-0000-000000000001"): {
			ID:         uuid.MustParse("00000000-0000-0000-0000-000000000001"),
			Categories: []string{"electronics", "smartphones"},
		},
		uuid.MustParse("00000000-0000-0000-0000-000000000002"): {
			ID:         uuid.MustParse("00000000-0000-0000-0000-000000000002"),
			Categories: []string{"electronics", "smartphones"}, // Similar to recent interaction
		},
		uuid.MustParse("00000000-0000-0000-0000-000000000003"): {
			ID:         uuid.MustParse("00000000-0000-0000-0000-000000000003"),
			Categories: []string{"electronics", "smartphones"}, // Also similar - should be filtered
		},
		uuid.MustParse("00000000-0000-0000-0000-000000000004"): {
			ID:         uuid.MustParse("00000000-0000-0000-0000-000000000004"),
			Categories: []string{"books"},
		},
	}

	// Recent interactions (similar to items 2 and 3)
	itemID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	recentInteractions := []models.UserInteraction{
		{
			ItemID:    &itemID,
			Timestamp: time.Now().AddDate(0, 0, -1), // 1 day ago
		},
	}

	// Create test recommendations
	recommendations := []FilteredRecommendation{
		{Recommendation: models.Recommendation{ItemID: uuid.MustParse("00000000-0000-0000-0000-000000000002"), Score: 0.9}},
		{Recommendation: models.Recommendation{ItemID: uuid.MustParse("00000000-0000-0000-0000-000000000003"), Score: 0.8}},
		{Recommendation: models.Recommendation{ItemID: uuid.MustParse("00000000-0000-0000-0000-000000000004"), Score: 0.7}},
	}

	result := df.applyTemporalDiversityFilter(recommendations, contentItems, recentInteractions)

	// Should keep only 1 similar item and the diverse item
	require.Len(t, result, 2)

	// Scores should be penalized for similar items
	for _, rec := range result {
		if rec.ItemID == uuid.MustParse("00000000-0000-0000-0000-000000000004") {
			assert.Equal(t, 0.7, rec.Score) // Books item should keep original score
		} else {
			assert.Less(t, rec.Score, 0.9) // Similar items should have reduced scores
		}
	}
}

func TestDiversityFilter_CalculateItemSimilarity(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	config := &config.DiversityConfig{}
	df := NewDiversityFilter(nil, config, logger)

	item1 := &models.ContentItem{
		Categories: []string{"electronics", "smartphones"},
		Embedding:  []float32{1.0, 0.0, 0.0},
	}

	item2 := &models.ContentItem{
		Categories: []string{"electronics", "tablets"},
		Embedding:  []float32{0.8, 0.6, 0.0},
	}

	item3 := &models.ContentItem{
		Categories: []string{"books", "fiction"},
		Embedding:  []float32{0.0, 1.0, 0.0},
	}

	// Test similarity between similar items
	similarity12 := df.calculateItemSimilarity(item1, item2)
	assert.Greater(t, similarity12, 0.3) // Should have some similarity due to "electronics"

	// Test similarity between different items
	similarity13 := df.calculateItemSimilarity(item1, item3)
	assert.Less(t, similarity13, 0.3) // Should have low similarity

	// Test with items without embeddings
	item4 := &models.ContentItem{
		Categories: []string{"electronics", "smartphones"},
		Embedding:  nil,
	}

	item5 := &models.ContentItem{
		Categories: []string{"electronics", "tablets"},
		Embedding:  nil,
	}

	similarity45 := df.calculateItemSimilarity(item4, item5)
	assert.Greater(t, similarity45, 0.0) // Should still have category-based similarity
	assert.Less(t, similarity45, 1.0)
}

// Benchmark tests for performance validation

func BenchmarkDiversityFilter_ApplyIntraListDiversityFilter(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	config := &config.DiversityConfig{
		IntraListDiversity:     0.3,
		CategoryMaxItems:       3,
		SerendipityRatio:       0.15,
		MaxSimilarityThreshold: 0.8,
		TemporalDecayFactor:    7.0,
		MaxRecentSimilarItems:  2,
	}

	df := NewDiversityFilter(nil, config, logger)

	// Create test data
	contentItems := make(map[uuid.UUID]*models.ContentItem)
	recommendations := make([]FilteredRecommendation, 100)

	for i := 0; i < 100; i++ {
		itemID := uuid.New()
		contentItems[itemID] = &models.ContentItem{
			ID:         itemID,
			Categories: []string{"category1", "category2"},
			Embedding:  []float32{float32(i) / 100.0, float32(100-i) / 100.0, 0.0},
		}
		recommendations[i] = FilteredRecommendation{
			Recommendation: models.Recommendation{
				ItemID: itemID,
				Score:  float64(100-i) / 100.0,
			},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		df.applyIntraListDiversityFilter(recommendations, contentItems)
	}
}

func BenchmarkDiversityFilter_CalculateCosineSimilarity(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	config := &config.DiversityConfig{}
	df := NewDiversityFilter(nil, config, logger)

	vec1 := make([]float32, 768) // Typical embedding size
	vec2 := make([]float32, 768)

	for i := 0; i < 768; i++ {
		vec1[i] = float32(i) / 768.0
		vec2[i] = float32(768-i) / 768.0
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		df.calculateCosineSimilarity(vec1, vec2)
	}
}
