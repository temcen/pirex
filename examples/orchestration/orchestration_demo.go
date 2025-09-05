package main

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/temcen/pirex/internal/config"
	"github.com/temcen/pirex/internal/services"
	"github.com/temcen/pirex/pkg/models"
)

// Mock services for demonstration
type MockAlgorithmService struct{}

func (m *MockAlgorithmService) SemanticSearchRecommendations(
	ctx context.Context, userID uuid.UUID, userEmbedding []float32, contentTypes []string, categories []string, limit int,
) ([]models.ScoredItem, error) {
	return []models.ScoredItem{
		{ItemID: uuid.New(), Score: 0.92, Algorithm: "semantic_search", Confidence: 0.85},
		{ItemID: uuid.New(), Score: 0.87, Algorithm: "semantic_search", Confidence: 0.80},
		{ItemID: uuid.New(), Score: 0.81, Algorithm: "semantic_search", Confidence: 0.75},
	}, nil
}

func (m *MockAlgorithmService) CollaborativeFilteringRecommendations(
	ctx context.Context, userID uuid.UUID, limit int,
) ([]models.ScoredItem, error) {
	return []models.ScoredItem{
		{ItemID: uuid.New(), Score: 4.8, Algorithm: "collaborative_filtering", Confidence: 0.90},
		{ItemID: uuid.New(), Score: 4.5, Algorithm: "collaborative_filtering", Confidence: 0.85},
		{ItemID: uuid.New(), Score: 4.2, Algorithm: "collaborative_filtering", Confidence: 0.80},
	}, nil
}

func (m *MockAlgorithmService) PersonalizedPageRankRecommendations(
	ctx context.Context, userID uuid.UUID, limit int,
) ([]models.ScoredItem, error) {
	return []models.ScoredItem{
		{ItemID: uuid.New(), Score: 0.15, Algorithm: "pagerank", Confidence: 0.75},
		{ItemID: uuid.New(), Score: 0.12, Algorithm: "pagerank", Confidence: 0.70},
		{ItemID: uuid.New(), Score: 0.09, Algorithm: "pagerank", Confidence: 0.65},
	}, nil
}

func (m *MockAlgorithmService) GraphSignalAnalysisRecommendations(
	ctx context.Context, userID uuid.UUID, limit int,
) ([]models.ScoredItem, error) {
	return []models.ScoredItem{
		{ItemID: uuid.New(), Score: 0.78, Algorithm: "graph_signal_analysis", Confidence: 0.70},
		{ItemID: uuid.New(), Score: 0.65, Algorithm: "graph_signal_analysis", Confidence: 0.65},
	}, nil
}

type MockUserService struct{}

func (m *MockUserService) GetUserProfile(ctx context.Context, userID uuid.UUID) (*models.UserProfile, error) {
	// Simulate different user profiles based on UUID
	userIDStr := userID.String()

	switch {
	case userIDStr[0] == '1': // New user
		return &models.UserProfile{
			UserID:           userID,
			PreferenceVector: []float32{0.1, 0.2, 0.3, 0.4},
			InteractionCount: 3,
			LastInteraction:  timePtr(time.Now().AddDate(0, 0, -1)),
		}, nil
	case userIDStr[0] == '2': // Power user
		return &models.UserProfile{
			UserID:           userID,
			PreferenceVector: []float32{0.8, 0.7, 0.9, 0.6},
			InteractionCount: 150,
			LastInteraction:  timePtr(time.Now().AddDate(0, 0, -1)),
		}, nil
	case userIDStr[0] == '3': // Inactive user
		return &models.UserProfile{
			UserID:           userID,
			PreferenceVector: []float32{0.4, 0.5, 0.3, 0.6},
			InteractionCount: 25,
			LastInteraction:  timePtr(time.Now().AddDate(0, 0, -45)), // 45 days ago
		}, nil
	default: // Active user
		return &models.UserProfile{
			UserID:           userID,
			PreferenceVector: []float32{0.6, 0.7, 0.5, 0.8},
			InteractionCount: 35,
			LastInteraction:  timePtr(time.Now().AddDate(0, 0, -3)),
		}, nil
	}
}

// Implement other required methods (not used in demo)
func (m *MockUserService) RecordExplicitInteraction(ctx context.Context, req *models.ExplicitInteractionRequest) (*models.UserInteraction, error) {
	return nil, nil
}
func (m *MockUserService) RecordImplicitInteraction(ctx context.Context, req *models.ImplicitInteractionRequest) (*models.UserInteraction, error) {
	return nil, nil
}
func (m *MockUserService) RecordBatchInteractions(ctx context.Context, req *models.InteractionBatchRequest) ([]models.UserInteraction, error) {
	return nil, nil
}
func (m *MockUserService) GetUserInteractions(ctx context.Context, userID uuid.UUID, interactionType string, limit, offset int, startDate, endDate *time.Time) ([]models.UserInteraction, int, error) {
	return nil, 0, nil
}
func (m *MockUserService) GetSimilarUsers(ctx context.Context, userID uuid.UUID, limit int) ([]models.SimilarUser, error) {
	return nil, nil
}
func (m *MockUserService) Stop() {}

func main() {
	fmt.Println("=== Recommendation Orchestration Demo ===")

	// Setup logger
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// Create configuration
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

	// Create mock services
	algorithmService := &MockAlgorithmService{}
	userService := &MockUserService{}

	// Create orchestrator (without Redis for demo)
	orchestrator := services.NewRecommendationOrchestrator(
		algorithmService, userService, nil, nil, nil, cfg, logger,
	)

	// Demo different user tiers
	fmt.Println("\n1. Testing Different User Tiers:")

	userTiers := []struct {
		name   string
		userID uuid.UUID
	}{
		{"New User", uuid.MustParse("11111111-1111-1111-1111-111111111111")},
		{"Power User", uuid.MustParse("22222222-2222-2222-2222-222222222222")},
		{"Inactive User", uuid.MustParse("33333333-3333-3333-3333-333333333333")},
		{"Active User", uuid.MustParse("44444444-4444-4444-4444-444444444444")},
	}

	for _, tier := range userTiers {
		fmt.Printf("\n   %s (%s):\n", tier.name, tier.userID.String()[:8]+"...")

		reqCtx := &services.RecommendationContext{
			UserID:              tier.userID,
			Count:               5,
			Context:             "home",
			IncludeExplanations: true,
			TimeoutMs:           2000,
		}

		result, err := orchestrator.GenerateRecommendations(context.Background(), reqCtx)
		if err != nil {
			fmt.Printf("     Error: %v\n", err)
			continue
		}

		fmt.Printf("     Strategy: %s\n", result.Strategy)
		fmt.Printf("     User Tier: %v\n", result.UserTier)
		fmt.Printf("     Algorithms Used: %d\n", len(result.AlgorithmResults))
		fmt.Printf("     Recommendations: %d\n", len(result.Recommendations))
		fmt.Printf("     Total Latency: %v\n", result.TotalLatency)

		// Show algorithm performance
		for algorithm, algResult := range result.AlgorithmResults {
			status := "✓"
			if algResult.Error != nil {
				status = "✗"
			}
			fmt.Printf("       %s %s: %d items, %v latency\n",
				status, algorithm, len(algResult.Items), algResult.Latency)
		}

		// Show top recommendations
		fmt.Printf("     Top Recommendations:\n")
		for i, rec := range result.Recommendations {
			if i >= 3 { // Show only top 3
				break
			}
			explanation := "No explanation"
			if rec.Explanation != nil {
				explanation = *rec.Explanation
			}
			fmt.Printf("       %d. Score: %.3f, Confidence: %.3f\n",
				rec.Position, rec.Score, rec.Confidence)
			fmt.Printf("          %s\n", explanation)
		}
	}

	// Demo ML Ranking Service
	fmt.Println("\n2. ML Ranking Service Demo:")

	mlService := services.NewMLRankingService(logger)

	// Create sample recommendations
	sampleRecs := []models.Recommendation{
		{
			ItemID:     uuid.New(),
			Score:      0.85,
			Algorithm:  "semantic_search",
			Confidence: 0.9,
			Position:   1,
		},
		{
			ItemID:     uuid.New(),
			Score:      0.75,
			Algorithm:  "collaborative_filtering",
			Confidence: 0.8,
			Position:   2,
		},
		{
			ItemID:     uuid.New(),
			Score:      0.65,
			Algorithm:  "pagerank",
			Confidence: 0.7,
			Position:   3,
		},
	}

	// Create user profile for ML ranking
	userProfile := &models.UserProfile{
		UserID:           uuid.New(),
		PreferenceVector: []float32{0.6, 0.7, 0.8, 0.5},
		InteractionCount: 50,
	}

	contextFeatures := map[string]interface{}{
		"time_of_day":    "evening",
		"session_length": 300,
	}

	fmt.Printf("   Original Recommendations:\n")
	for _, rec := range sampleRecs {
		fmt.Printf("     %d. %s: Score %.3f, Confidence %.3f\n",
			rec.Position, rec.Algorithm, rec.Score, rec.Confidence)
	}

	// Apply ML ranking
	rankedRecs, err := mlService.RankRecommendations(
		context.Background(), sampleRecs, userProfile, contextFeatures,
	)
	if err != nil {
		fmt.Printf("   ML Ranking Error: %v\n", err)
	} else {
		fmt.Printf("\n   ML Re-ranked Recommendations:\n")
		for _, rec := range rankedRecs {
			fmt.Printf("     %d. %s: Score %.3f, Confidence %.3f\n",
				rec.Position, rec.Algorithm, rec.Score, rec.Confidence)
		}
	}

	// Show ML model weights
	weights := mlService.GetModelWeights()
	fmt.Printf("\n   Current ML Model Weights:\n")
	fmt.Printf("     Content Similarity: %.3f\n", weights.ContentSimilarity)
	fmt.Printf("     User-Item Affinity: %.3f\n", weights.UserItemAffinity)
	fmt.Printf("     Popularity Score: %.3f\n", weights.PopularityScore)
	fmt.Printf("     Recency Score: %.3f\n", weights.RecencyScore)
	fmt.Printf("     Diversity Score: %.3f\n", weights.DiversityScore)
	fmt.Printf("     Algorithm Confidence: %.3f\n", weights.AlgorithmConfidence)

	// Demo performance characteristics
	fmt.Println("\n3. Performance Characteristics:")

	// Simulate multiple requests to show performance
	start := time.Now()
	for i := 0; i < 10; i++ {
		userID := uuid.New()
		reqCtx := &services.RecommendationContext{
			UserID:    userID,
			Count:     10,
			Context:   "home",
			TimeoutMs: 1000,
		}

		_, err := orchestrator.GenerateRecommendations(context.Background(), reqCtx)
		if err != nil {
			fmt.Printf("   Request %d failed: %v\n", i+1, err)
		}
	}
	totalTime := time.Since(start)

	fmt.Printf("   10 Requests Completed:\n")
	fmt.Printf("     Total Time: %v\n", totalTime)
	fmt.Printf("     Average per Request: %v\n", totalTime/10)
	fmt.Printf("     Requests per Second: %.1f\n", float64(10)/totalTime.Seconds())

	// Demo algorithm selection logic
	fmt.Println("\n4. Algorithm Selection Logic:")

	fmt.Printf("   User Tier -> Algorithms Used:\n")
	fmt.Printf("     New User: semantic_search only\n")
	fmt.Printf("     Active User: semantic_search + collaborative_filtering + pagerank\n")
	fmt.Printf("     Power User: collaborative_filtering + pagerank + graph_signal_analysis\n")
	fmt.Printf("     Inactive User: semantic_search + collaborative_filtering\n")

	fmt.Printf("\n   Algorithm Weights by User Tier:\n")
	fmt.Printf("     New User: semantic(1.0)\n")
	fmt.Printf("     Active User: semantic(0.4) + collaborative(0.3) + pagerank(0.3)\n")
	fmt.Printf("     Power User: semantic(0.2) + collaborative(0.4) + pagerank(0.2) + graph(0.2)\n")
	fmt.Printf("     Inactive User: semantic(0.6) + collaborative(0.4)\n")

	// Demo fallback mechanisms
	fmt.Println("\n5. Fallback Mechanisms:")

	fmt.Printf("   Fallback Chain:\n")
	fmt.Printf("     1. Primary algorithms (based on user tier)\n")
	fmt.Printf("     2. Cached recommendations (15min TTL)\n")
	fmt.Printf("     3. Popularity-based recommendations\n")
	fmt.Printf("     4. Random high-quality items (quality_score > 0.7)\n")

	fmt.Printf("\n   Timeout Handling:\n")
	fmt.Printf("     - Algorithm timeout: 1.5s per algorithm\n")
	fmt.Printf("     - Partial results used if some algorithms timeout\n")
	fmt.Printf("     - Graceful degradation with lower confidence scores\n")

	fmt.Printf("\n   Error Handling:\n")
	fmt.Printf("     - Individual algorithm failures don't stop orchestration\n")
	fmt.Printf("     - Failed algorithms excluded from score combination\n")
	fmt.Printf("     - Fallback strategies ensure recommendations are always returned\n")

	fmt.Println("\n=== Demo Complete ===")
}

func timePtr(t time.Time) *time.Time {
	return &t
}
