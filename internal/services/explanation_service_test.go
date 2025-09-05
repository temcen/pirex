package services

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/temcen/pirex/pkg/models"
)

func TestExplanationService_SelectBestExplanation(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	es := NewExplanationService(nil, logger)

	explanations := []ExplanationData{
		{Type: ContentBasedExplanation, Confidence: 0.7},
		{Type: CollaborativeExplanation, Confidence: 0.9},
		{Type: PopularityBasedExplanation, Confidence: 0.5},
		{Type: GenericExplanation, Confidence: 0.1},
	}

	best := es.selectBestExplanation(explanations)
	assert.Equal(t, CollaborativeExplanation, best.Type)
	assert.Equal(t, 0.9, best.Confidence)
}

func TestExplanationService_SelectBestExplanation_Empty(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	es := NewExplanationService(nil, logger)

	best := es.selectBestExplanation([]ExplanationData{})
	assert.Equal(t, GenericExplanation, best.Type)
	assert.Equal(t, 0.1, best.Confidence)
}

func TestExplanationService_GenerateContentBasedText(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	es := NewExplanationService(nil, logger)

	tests := []struct {
		name     string
		data     ExplanationData
		expected string
	}{
		{
			name: "single similar item",
			data: ExplanationData{
				Type: ContentBasedExplanation,
				SimilarItems: []SimilarItemInfo{
					{
						ItemID:           uuid.New(),
						Title:            "iPhone 13",
						SharedCategories: []string{"electronics", "smartphones"},
					},
				},
			},
			expected: "Because you liked \"iPhone 13\" and both are electronics, smartphones",
		},
		{
			name: "multiple similar items",
			data: ExplanationData{
				Type: ContentBasedExplanation,
				SimilarItems: []SimilarItemInfo{
					{
						ItemID:           uuid.New(),
						Title:            "iPhone 13",
						SharedCategories: []string{"electronics", "smartphones"},
					},
					{
						ItemID:           uuid.New(),
						Title:            "Samsung Galaxy",
						SharedCategories: []string{"electronics"},
					},
				},
			},
			expected: "Because you liked \"iPhone 13\" and 1 other similar items in electronics, smartphones",
		},
		{
			name: "no similar items",
			data: ExplanationData{
				Type:         ContentBasedExplanation,
				SimilarItems: []SimilarItemInfo{},
			},
			expected: "Based on your preferences and similar content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := es.generateContentBasedText(tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExplanationService_GenerateCollaborativeText(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	es := NewExplanationService(nil, logger)

	tests := []struct {
		name     string
		data     ExplanationData
		expected string
	}{
		{
			name: "with shared items",
			data: ExplanationData{
				Type: CollaborativeExplanation,
				SharedUsers: []SharedUserInfo{
					{
						UserCount:     25,
						SharedItems:   []string{"iPhone 13", "MacBook Pro", "AirPods"},
						AverageRating: 4.5,
					},
				},
			},
			expected: "Users who liked \"iPhone 13\", \"MacBook Pro\" also rated this 4.5/5 (25 users)",
		},
		{
			name: "without shared items",
			data: ExplanationData{
				Type: CollaborativeExplanation,
				SharedUsers: []SharedUserInfo{
					{
						UserCount:     15,
						SharedItems:   []string{},
						AverageRating: 4.2,
					},
				},
			},
			expected: "Highly rated by 15 users with similar preferences (4.2/5 stars)",
		},
		{
			name: "no shared users",
			data: ExplanationData{
				Type:        CollaborativeExplanation,
				SharedUsers: []SharedUserInfo{},
			},
			expected: "Recommended by users with similar tastes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := es.generateCollaborativeText(tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExplanationService_GenerateGraphBasedText(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	es := NewExplanationService(nil, logger)

	tests := []struct {
		name     string
		data     ExplanationData
		expected string
	}{
		{
			name: "with connections",
			data: ExplanationData{
				Type: GraphBasedExplanation,
				GraphConnections: []GraphConnectionInfo{
					{
						ConnectedItem:   "iPhone 13",
						ConnectionType:  "similar_category",
						PathDescription: "shared categories with iPhone 13",
						Strength:        0.8,
					},
				},
			},
			expected: "This connects to \"iPhone 13\" through shared categories with iPhone 13",
		},
		{
			name: "no connections",
			data: ExplanationData{
				Type:             GraphBasedExplanation,
				GraphConnections: []GraphConnectionInfo{},
			},
			expected: "Connected to items in your network",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := es.generateGraphBasedText(tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExplanationService_GeneratePopularityBasedText(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	es := NewExplanationService(nil, logger)

	tests := []struct {
		name     string
		data     ExplanationData
		expected string
	}{
		{
			name: "trending item",
			data: ExplanationData{
				Type: PopularityBasedExplanation,
				PopularityStats: &PopularityStats{
					UserCount:     150,
					AverageRating: 4.3,
					IsTrending:    true,
					TimeFrame:     "recently",
				},
			},
			expected: "Trending recently - highly rated by 150 users (4.3/5 stars)",
		},
		{
			name: "popular item",
			data: ExplanationData{
				Type: PopularityBasedExplanation,
				PopularityStats: &PopularityStats{
					UserCount:     200,
					AverageRating: 4.1,
					IsTrending:    false,
					TimeFrame:     "overall",
				},
			},
			expected: "Highly rated by 200 users (4.1/5 stars)",
		},
		{
			name: "no stats",
			data: ExplanationData{
				Type:            PopularityBasedExplanation,
				PopularityStats: nil,
			},
			expected: "Popular recommendation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := es.generatePopularityBasedText(tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExplanationService_GenerateSerendipityText(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	es := NewExplanationService(nil, logger)

	tests := []struct {
		name     string
		data     ExplanationData
		expected string
	}{
		{
			name: "with unexplored categories and similar users",
			data: ExplanationData{
				Type: SerendipityExplanation,
				SerendipityReason: &SerendipityReason{
					UnexploredCategories: []string{"art", "photography", "design"},
					SimilarUserLikes:     12,
					CategoryNovelty:      0.8,
				},
			},
			expected: "Explore art, photography - liked by 12 users with similar tastes",
		},
		{
			name: "with unexplored categories only",
			data: ExplanationData{
				Type: SerendipityExplanation,
				SerendipityReason: &SerendipityReason{
					UnexploredCategories: []string{"cooking", "recipes"},
					SimilarUserLikes:     0,
					CategoryNovelty:      0.6,
				},
			},
			expected: "Discover something new in cooking, recipes",
		},
		{
			name: "no specific reason",
			data: ExplanationData{
				Type: SerendipityExplanation,
				SerendipityReason: &SerendipityReason{
					UnexploredCategories: []string{},
					SimilarUserLikes:     0,
					CategoryNovelty:      0.0,
				},
			},
			expected: "Something different you might enjoy",
		},
		{
			name: "no serendipity reason",
			data: ExplanationData{
				Type:              SerendipityExplanation,
				SerendipityReason: nil,
			},
			expected: "Something new you might enjoy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := es.generateSerendipityText(tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExplanationService_GenerateExplanationText(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	es := NewExplanationService(nil, logger)

	tests := []struct {
		name     string
		data     ExplanationData
		expected string
	}{
		{
			name: "content based",
			data: ExplanationData{
				Type:         ContentBasedExplanation,
				SimilarItems: []SimilarItemInfo{},
			},
			expected: "Based on your preferences and similar content",
		},
		{
			name: "collaborative",
			data: ExplanationData{
				Type:        CollaborativeExplanation,
				SharedUsers: []SharedUserInfo{},
			},
			expected: "Recommended by users with similar tastes",
		},
		{
			name: "graph based",
			data: ExplanationData{
				Type:             GraphBasedExplanation,
				GraphConnections: []GraphConnectionInfo{},
			},
			expected: "Connected to items in your network",
		},
		{
			name: "popularity based",
			data: ExplanationData{
				Type:            PopularityBasedExplanation,
				PopularityStats: nil,
			},
			expected: "Popular recommendation",
		},
		{
			name: "serendipity",
			data: ExplanationData{
				Type:              SerendipityExplanation,
				SerendipityReason: nil,
			},
			expected: "Something new you might enjoy",
		},
		{
			name: "generic",
			data: ExplanationData{
				Type: GenericExplanation,
			},
			expected: "Personalized recommendation based on your preferences",
		},
		{
			name: "unknown type",
			data: ExplanationData{
				Type: ExplanationType("unknown"),
			},
			expected: "Personalized recommendation based on your preferences",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := es.generateExplanationText(tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExplanationService_FindSharedCategories(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	es := NewExplanationService(nil, logger)

	tests := []struct {
		name        string
		categories1 []string
		categories2 []string
		expected    []string
	}{
		{
			name:        "some overlap",
			categories1: []string{"electronics", "smartphones", "apple"},
			categories2: []string{"electronics", "tablets", "apple"},
			expected:    []string{"electronics", "apple"},
		},
		{
			name:        "no overlap",
			categories1: []string{"electronics", "smartphones"},
			categories2: []string{"books", "fiction"},
			expected:    []string{},
		},
		{
			name:        "complete overlap",
			categories1: []string{"electronics", "smartphones"},
			categories2: []string{"electronics", "smartphones"},
			expected:    []string{"electronics", "smartphones"},
		},
		{
			name:        "empty categories",
			categories1: []string{},
			categories2: []string{"electronics"},
			expected:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := es.findSharedCategories(tt.categories1, tt.categories2)
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

// Integration test for the complete explanation generation flow
func TestExplanationService_GenerateExplanations_Integration(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	es := NewExplanationService(nil, logger)

	userID := uuid.New()
	recommendations := []models.Recommendation{
		{
			ItemID:      uuid.New(),
			Score:       0.9,
			Algorithm:   "semantic_search",
			Confidence:  0.8,
			Position:    1,
			Explanation: nil,
		},
		{
			ItemID:      uuid.New(),
			Score:       0.8,
			Algorithm:   "collaborative_filtering",
			Confidence:  0.7,
			Position:    2,
			Explanation: nil,
		},
		{
			ItemID:      uuid.New(),
			Score:       0.7,
			Algorithm:   "serendipity",
			Confidence:  0.6,
			Position:    3,
			Explanation: nil,
		},
	}

	// Since we don't have a real database connection, this will use fallback explanations
	result, err := es.GenerateExplanations(context.Background(), userID, recommendations)

	assert.NoError(t, err)
	assert.Len(t, result, 3)

	// All recommendations should have explanations (even if generic)
	for i, rec := range result {
		assert.NotNil(t, rec.Explanation, "Recommendation %d should have an explanation", i)
		assert.NotEmpty(t, *rec.Explanation, "Explanation should not be empty")
	}
}

// Benchmark tests for performance validation

func BenchmarkExplanationService_GenerateExplanationText(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	es := NewExplanationService(nil, logger)

	data := ExplanationData{
		Type: ContentBasedExplanation,
		SimilarItems: []SimilarItemInfo{
			{
				ItemID:           uuid.New(),
				Title:            "Test Item",
				SharedCategories: []string{"category1", "category2"},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		es.generateExplanationText(data)
	}
}

func BenchmarkExplanationService_SelectBestExplanation(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	es := NewExplanationService(nil, logger)

	explanations := []ExplanationData{
		{Type: ContentBasedExplanation, Confidence: 0.7},
		{Type: CollaborativeExplanation, Confidence: 0.9},
		{Type: GraphBasedExplanation, Confidence: 0.6},
		{Type: PopularityBasedExplanation, Confidence: 0.5},
		{Type: SerendipityExplanation, Confidence: 0.4},
		{Type: GenericExplanation, Confidence: 0.1},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		es.selectBestExplanation(explanations)
	}
}
