package services

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/temcen/pirex/pkg/models"
)

// FeatureVector represents the feature vector for ML-based ranking
type FeatureVector struct {
	ContentSimilarity   float64 `json:"content_similarity"`
	UserItemAffinity    float64 `json:"user_item_affinity"`
	PopularityScore     float64 `json:"popularity_score"`
	RecencyScore        float64 `json:"recency_score"`
	DiversityScore      float64 `json:"diversity_score"`
	AlgorithmConfidence float64 `json:"algorithm_confidence"`
}

// RankingFeatures contains all features for ranking
type RankingFeatures struct {
	ItemID   uuid.UUID     `json:"item_id"`
	Features FeatureVector `json:"features"`
	Score    float64       `json:"score"`
}

// MLRankingService implements Learning-to-Rank functionality
type MLRankingService struct {
	logger *logrus.Logger

	// Model weights (in a real implementation, these would be learned)
	weights FeatureVector
}

// NewMLRankingService creates a new ML ranking service
func NewMLRankingService(logger *logrus.Logger) *MLRankingService {
	return &MLRankingService{
		logger: logger,
		// Initialize with default weights (would be learned in production)
		weights: FeatureVector{
			ContentSimilarity:   0.25,
			UserItemAffinity:    0.30,
			PopularityScore:     0.15,
			RecencyScore:        0.10,
			DiversityScore:      0.10,
			AlgorithmConfidence: 0.10,
		},
	}
}

// RankRecommendations applies ML-based ranking to recommendations
func (s *MLRankingService) RankRecommendations(
	ctx context.Context,
	recommendations []models.Recommendation,
	userProfile *models.UserProfile,
	contextFeatures map[string]interface{},
) ([]models.Recommendation, error) {

	if len(recommendations) == 0 {
		return recommendations, nil
	}

	// Extract features for each recommendation
	var rankingFeatures []RankingFeatures

	for _, rec := range recommendations {
		features := s.extractFeatures(rec, userProfile, contextFeatures)
		score := s.calculateMLScore(features)

		rankingFeatures = append(rankingFeatures, RankingFeatures{
			ItemID:   rec.ItemID,
			Features: features,
			Score:    score,
		})
	}

	// Sort by ML score descending
	sort.Slice(rankingFeatures, func(i, j int) bool {
		return rankingFeatures[i].Score > rankingFeatures[j].Score
	})

	// Create reranked recommendations
	var reranked []models.Recommendation
	for i, rf := range rankingFeatures {
		// Find original recommendation
		for _, rec := range recommendations {
			if rec.ItemID == rf.ItemID {
				// Update score and position
				rec.Score = rf.Score
				rec.Position = i + 1

				// Update confidence based on feature strength
				rec.Confidence = s.calculateFeatureBasedConfidence(rf.Features)

				reranked = append(reranked, rec)
				break
			}
		}
	}

	s.logger.Debug("ML ranking completed",
		"original_count", len(recommendations),
		"reranked_count", len(reranked),
	)

	return reranked, nil
}

// extractFeatures extracts feature vector for a recommendation
func (s *MLRankingService) extractFeatures(
	rec models.Recommendation,
	userProfile *models.UserProfile,
	contextFeatures map[string]interface{},
) FeatureVector {

	features := FeatureVector{}

	// Content Similarity (based on original algorithm score)
	features.ContentSimilarity = s.normalizeScore(rec.Score)

	// User-Item Affinity (simplified calculation)
	features.UserItemAffinity = s.calculateUserItemAffinity(rec, userProfile)

	// Popularity Score (mock calculation)
	features.PopularityScore = s.calculatePopularityScore(rec)

	// Recency Score (based on algorithm and current time)
	features.RecencyScore = s.calculateRecencyScore(rec)

	// Diversity Score (simplified)
	features.DiversityScore = s.calculateDiversityScore(rec, contextFeatures)

	// Algorithm Confidence
	features.AlgorithmConfidence = rec.Confidence

	return features
}

// calculateMLScore computes the final ML score using feature weights
func (s *MLRankingService) calculateMLScore(features FeatureVector) float64 {
	score := features.ContentSimilarity*s.weights.ContentSimilarity +
		features.UserItemAffinity*s.weights.UserItemAffinity +
		features.PopularityScore*s.weights.PopularityScore +
		features.RecencyScore*s.weights.RecencyScore +
		features.DiversityScore*s.weights.DiversityScore +
		features.AlgorithmConfidence*s.weights.AlgorithmConfidence

	// Apply sigmoid activation for better distribution
	return 1.0 / (1.0 + math.Exp(-5.0*(score-0.5)))
}

// calculateFeatureBasedConfidence calculates confidence based on feature strength
func (s *MLRankingService) calculateFeatureBasedConfidence(features FeatureVector) float64 {
	// Confidence based on feature consistency and strength
	featureStrength := (features.ContentSimilarity + features.UserItemAffinity +
		features.AlgorithmConfidence) / 3.0

	// Boost confidence if multiple features are strong
	consistencyBonus := 0.0
	strongFeatures := 0
	threshold := 0.7

	if features.ContentSimilarity > threshold {
		strongFeatures++
	}
	if features.UserItemAffinity > threshold {
		strongFeatures++
	}
	if features.AlgorithmConfidence > threshold {
		strongFeatures++
	}

	if strongFeatures >= 2 {
		consistencyBonus = 0.1 * float64(strongFeatures-1)
	}

	confidence := math.Min(featureStrength+consistencyBonus, 1.0)
	return math.Max(confidence, 0.1) // Minimum confidence of 0.1
}

// Feature calculation methods

func (s *MLRankingService) normalizeScore(score float64) float64 {
	// Ensure score is in [0,1] range
	return math.Max(0.0, math.Min(1.0, score))
}

func (s *MLRankingService) calculateUserItemAffinity(
	rec models.Recommendation,
	userProfile *models.UserProfile,
) float64 {
	if userProfile == nil || len(userProfile.PreferenceVector) == 0 {
		return 0.5 // Default affinity for unknown users
	}

	// Simplified affinity calculation
	// In a real implementation, this would use item embeddings
	baseAffinity := rec.Score * rec.Confidence

	// Adjust based on user interaction history
	interactionBoost := math.Min(float64(userProfile.InteractionCount)/100.0, 0.3)

	return math.Min(baseAffinity+interactionBoost, 1.0)
}

func (s *MLRankingService) calculatePopularityScore(rec models.Recommendation) float64 {
	// Mock popularity calculation
	// In a real implementation, this would query actual popularity metrics

	// Simulate popularity based on algorithm type
	switch rec.Algorithm {
	case "collaborative_filtering":
		return 0.8 // Collaborative items tend to be popular
	case "pagerank":
		return 0.7 // PageRank captures some popularity
	case "semantic_search":
		return 0.6 // Content-based may be less popular
	case "graph_signal_analysis":
		return 0.5 // Graph signals may be more niche
	default:
		return 0.5
	}
}

func (s *MLRankingService) calculateRecencyScore(rec models.Recommendation) float64 {
	// Mock recency calculation
	// In a real implementation, this would use actual item creation/update times

	// Simulate recency decay
	// Assume items are relatively recent for now
	baseRecency := 0.8

	// Slight penalty for lower confidence (might be older items)
	recencyPenalty := (1.0 - rec.Confidence) * 0.2

	return math.Max(baseRecency-recencyPenalty, 0.1)
}

func (s *MLRankingService) calculateDiversityScore(
	rec models.Recommendation,
	contextFeatures map[string]interface{},
) float64 {
	// Mock diversity calculation
	// In a real implementation, this would consider:
	// - Category diversity within the recommendation set
	// - User's historical category preferences
	// - Temporal diversity patterns

	baseDiversity := 0.6

	// Check if we have context about recommendation position
	if pos, exists := contextFeatures["position"]; exists {
		if position, ok := pos.(int); ok {
			// Higher diversity bonus for later positions
			diversityBonus := math.Min(float64(position)*0.05, 0.3)
			baseDiversity += diversityBonus
		}
	}

	return math.Min(baseDiversity, 1.0)
}

// UpdateModelWeights updates the model weights (for online learning)
func (s *MLRankingService) UpdateModelWeights(
	ctx context.Context,
	feedbackData []RankingFeedback,
) error {

	if len(feedbackData) == 0 {
		return nil
	}

	s.logger.Info("Updating ML ranking model weights",
		"feedback_samples", len(feedbackData),
	)

	// Simplified weight update using gradient descent
	// In a real implementation, this would use proper ML algorithms

	learningRate := 0.01

	for _, feedback := range feedbackData {
		// Calculate prediction error
		predicted := s.calculateMLScore(feedback.Features)
		error := feedback.ActualScore - predicted

		// Update weights using gradient descent
		s.weights.ContentSimilarity += learningRate * error * feedback.Features.ContentSimilarity
		s.weights.UserItemAffinity += learningRate * error * feedback.Features.UserItemAffinity
		s.weights.PopularityScore += learningRate * error * feedback.Features.PopularityScore
		s.weights.RecencyScore += learningRate * error * feedback.Features.RecencyScore
		s.weights.DiversityScore += learningRate * error * feedback.Features.DiversityScore
		s.weights.AlgorithmConfidence += learningRate * error * feedback.Features.AlgorithmConfidence
	}

	// Normalize weights to sum to 1.0
	s.normalizeWeights()

	s.logger.Debug("Model weights updated",
		"content_similarity", s.weights.ContentSimilarity,
		"user_item_affinity", s.weights.UserItemAffinity,
		"popularity_score", s.weights.PopularityScore,
		"recency_score", s.weights.RecencyScore,
		"diversity_score", s.weights.DiversityScore,
		"algorithm_confidence", s.weights.AlgorithmConfidence,
	)

	return nil
}

// normalizeWeights ensures weights sum to 1.0
func (s *MLRankingService) normalizeWeights() {
	sum := s.weights.ContentSimilarity + s.weights.UserItemAffinity +
		s.weights.PopularityScore + s.weights.RecencyScore +
		s.weights.DiversityScore + s.weights.AlgorithmConfidence

	if sum > 0 {
		s.weights.ContentSimilarity /= sum
		s.weights.UserItemAffinity /= sum
		s.weights.PopularityScore /= sum
		s.weights.RecencyScore /= sum
		s.weights.DiversityScore /= sum
		s.weights.AlgorithmConfidence /= sum
	}
}

// RankingFeedback represents feedback for model training
type RankingFeedback struct {
	ItemID      uuid.UUID     `json:"item_id"`
	Features    FeatureVector `json:"features"`
	ActualScore float64       `json:"actual_score"` // Based on user interactions
	Timestamp   time.Time     `json:"timestamp"`
}

// GetModelWeights returns current model weights (for monitoring)
func (s *MLRankingService) GetModelWeights() FeatureVector {
	return s.weights
}

// SetModelWeights sets model weights (for A/B testing)
func (s *MLRankingService) SetModelWeights(weights FeatureVector) {
	s.weights = weights
	s.normalizeWeights()
}
