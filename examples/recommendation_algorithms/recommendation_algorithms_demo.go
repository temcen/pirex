package main

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/sirupsen/logrus"

	"github.com/temcen/pirex/internal/config"
	"github.com/temcen/pirex/internal/services"
	"github.com/temcen/pirex/pkg/models"
)

// MockDatabaseQuerier implements the DatabaseQuerier interface for demo purposes
type MockDatabaseQuerier struct{}

func (m *MockDatabaseQuerier) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	// This is a mock implementation for demonstration
	// In a real scenario, this would execute the actual SQL query
	return nil, fmt.Errorf("mock implementation - not connected to real database")
}

func main() {
	fmt.Println("=== Recommendation Algorithms Demo ===")

	// Create logger
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// Create mock configuration
	cfg := &config.AlgorithmConfig{
		SemanticSearch: config.AlgorithmWeightConfig{
			Enabled:             true,
			Weight:              0.4,
			SimilarityThreshold: 0.7,
		},
		CollaborativeFilter: config.AlgorithmWeightConfig{
			Enabled:             true,
			Weight:              0.3,
			SimilarityThreshold: 0.5,
		},
		PageRank: config.AlgorithmWeightConfig{
			Enabled:             true,
			Weight:              0.3,
			SimilarityThreshold: 0.0,
		},
	}

	// Create service with mock dependencies
	mockDB := &MockDatabaseQuerier{}
	_ = services.NewRecommendationAlgorithmsService(
		mockDB, nil, nil, cfg, logger,
	)

	// Demo: Test confidence calculations
	fmt.Println("\n1. Testing Confidence Calculations:")

	// Semantic search confidence
	similarities := []float64{0.95, 0.85, 0.75, 0.65}
	fmt.Println("   Semantic Search Confidence:")
	for _, sim := range similarities {
		confidence := calculateSemanticConfidence(sim)
		fmt.Printf("     Similarity: %.2f -> Confidence: %.2f\n", sim, confidence)
	}

	// Collaborative filtering confidence
	fmt.Println("   Collaborative Filtering Confidence:")
	testCases := []struct {
		contributors int
		weightSum    float64
	}{
		{10, 5.0}, {5, 2.5}, {2, 1.0}, {1, 0.5},
	}

	for _, tc := range testCases {
		confidence := calculateCollaborativeConfidence(tc.contributors, tc.weightSum)
		fmt.Printf("     Contributors: %d, WeightSum: %.1f -> Confidence: %.2f\n",
			tc.contributors, tc.weightSum, confidence)
	}

	// Demo: Algorithm result structure
	fmt.Println("\n2. Algorithm Result Structure:")

	sampleResults := []models.ScoredItem{
		{
			ItemID:     uuid.New(),
			Score:      0.95,
			Algorithm:  "semantic_search",
			Confidence: 0.92,
		},
		{
			ItemID:     uuid.New(),
			Score:      4.2,
			Algorithm:  "collaborative_filtering",
			Confidence: 0.78,
		},
		{
			ItemID:     uuid.New(),
			Score:      0.08,
			Algorithm:  "pagerank",
			Confidence: 0.80,
		},
		{
			ItemID:     uuid.New(),
			Score:      0.65,
			Algorithm:  "graph_signal_analysis",
			Confidence: 0.52,
		},
	}

	for i, result := range sampleResults {
		fmt.Printf("   Result %d:\n", i+1)
		fmt.Printf("     ItemID: %s\n", result.ItemID.String()[:8]+"...")
		fmt.Printf("     Algorithm: %s\n", result.Algorithm)
		fmt.Printf("     Score: %.2f\n", result.Score)
		fmt.Printf("     Confidence: %.2f\n", result.Confidence)
		fmt.Println()
	}

	// Demo: Caching strategy
	fmt.Println("3. Caching Strategy:")
	fmt.Println("   - Semantic Search: 30min TTL (frequent queries)")
	fmt.Println("   - Collaborative Filtering: 1hour TTL (user similarities)")
	fmt.Println("   - PageRank: 30min TTL (graph computations)")
	fmt.Println("   - Graph Signal Analysis: 2hour TTL (community detection)")

	// Demo: Performance characteristics
	fmt.Println("\n4. Performance Characteristics:")
	fmt.Println("   Algorithm Performance (typical):")
	fmt.Println("   - Semantic Search: ~10ms (vector similarity)")
	fmt.Println("   - Collaborative Filtering: ~50ms (user correlation)")
	fmt.Println("   - PageRank: ~100ms (graph computation)")
	fmt.Println("   - Graph Signal Analysis: ~200ms (community detection)")

	// Demo: Algorithm selection strategy
	fmt.Println("\n5. Algorithm Selection Strategy:")
	fmt.Println("   User Profile Completeness:")
	fmt.Println("   - New users (< 5 interactions): Popularity-based + Semantic")
	fmt.Println("   - Active users (5-50 interactions): All algorithms")
	fmt.Println("   - Power users (> 50 interactions): Collaborative + Graph-based")

	fmt.Println("\n6. Integration with Recommendation Orchestrator:")
	fmt.Println("   - Each algorithm returns []ScoredItem")
	fmt.Println("   - Results combined with configurable weights")
	fmt.Println("   - Final ranking considers confidence scores")
	fmt.Println("   - Diversity filters applied post-ranking")

	fmt.Println("\n=== Demo Complete ===")
}

// Helper functions to demonstrate confidence calculations
func calculateSemanticConfidence(similarity float64) float64 {
	if similarity*1.2 > 1.0 {
		return 1.0
	}
	return similarity * 1.2
}

func calculateCollaborativeConfidence(contributorCount int, weightSum float64) float64 {
	contributorFactor := float64(contributorCount) / 10.0
	if contributorFactor > 1.0 {
		contributorFactor = 1.0
	}

	weightFactor := weightSum / 5.0
	if weightFactor > 1.0 {
		weightFactor = 1.0
	}

	return (contributorFactor + weightFactor) / 2.0
}
