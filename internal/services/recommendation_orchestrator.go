package services

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"github.com/temcen/pirex/internal/config"
	"github.com/temcen/pirex/pkg/models"
)

// UserTier represents different user engagement levels
type UserTier int

const (
	NewUser UserTier = iota
	ActiveUser
	PowerUser
	InactiveUser
)

// RecommendationContext contains context information for generating recommendations
type RecommendationContext struct {
	UserID              uuid.UUID   `json:"user_id"`
	Count               int         `json:"count"`
	Context             string      `json:"context"` // home, search, category, product, similar
	ContentTypes        []string    `json:"content_types,omitempty"`
	Categories          []string    `json:"categories,omitempty"`
	ExcludeItems        []uuid.UUID `json:"exclude_items,omitempty"`
	SeedItemID          *uuid.UUID  `json:"seed_item_id,omitempty"` // For item-based recommendations
	IncludeExplanations bool        `json:"include_explanations"`
	TimeoutMs           int         `json:"timeout_ms"`
}

// AlgorithmResult represents the result from a single algorithm
type AlgorithmResult struct {
	Algorithm string              `json:"algorithm"`
	Items     []models.ScoredItem `json:"items"`
	Latency   time.Duration       `json:"latency"`
	Error     error               `json:"error,omitempty"`
	Cached    bool                `json:"cached"`
}

// OrchestrationResult represents the final orchestrated result
type OrchestrationResult struct {
	UserID           uuid.UUID                   `json:"user_id"`
	Recommendations  []models.Recommendation     `json:"recommendations"`
	Context          string                      `json:"context"`
	AlgorithmResults map[string]*AlgorithmResult `json:"algorithm_results"`
	TotalLatency     time.Duration               `json:"total_latency"`
	CacheHit         bool                        `json:"cache_hit"`
	UserTier         UserTier                    `json:"user_tier"`
	Strategy         string                      `json:"strategy"`
	GeneratedAt      time.Time                   `json:"generated_at"`
}

// RecommendationOrchestrator coordinates multiple recommendation algorithms
type RecommendationOrchestrator struct {
	algorithmService   RecommendationAlgorithmsServiceInterface
	userService        UserInteractionServiceInterface
	diversityFilter    *DiversityFilter
	explanationService *ExplanationService
	redis              *redis.Client
	config             *config.AlgorithmConfig
	logger             *logrus.Logger

	// Algorithm weights by user tier
	algorithmWeights map[UserTier]map[string]float64
}

// NewRecommendationOrchestrator creates a new recommendation orchestrator
func NewRecommendationOrchestrator(
	algorithmService RecommendationAlgorithmsServiceInterface,
	userService UserInteractionServiceInterface,
	diversityFilter *DiversityFilter,
	explanationService *ExplanationService,
	redis *redis.Client,
	config *config.AlgorithmConfig,
	logger *logrus.Logger,
) *RecommendationOrchestrator {
	orchestrator := &RecommendationOrchestrator{
		algorithmService:   algorithmService,
		userService:        userService,
		diversityFilter:    diversityFilter,
		explanationService: explanationService,
		redis:              redis,
		config:             config,
		logger:             logger,
		algorithmWeights:   make(map[UserTier]map[string]float64),
	}

	// Initialize default algorithm weights by user tier
	orchestrator.initializeAlgorithmWeights()

	return orchestrator
}

// GenerateRecommendations orchestrates multiple algorithms to generate final recommendations
func (o *RecommendationOrchestrator) GenerateRecommendations(
	ctx context.Context,
	reqCtx *RecommendationContext,
) (*OrchestrationResult, error) {
	startTime := time.Now()

	// Check cache first
	if cached, err := o.getCachedRecommendations(ctx, reqCtx); err == nil && cached != nil {
		o.logger.Debug("Orchestration cache hit", "user_id", reqCtx.UserID)
		return cached, nil
	}

	// Determine user tier and strategy
	userTier, err := o.determineUserTier(ctx, reqCtx.UserID)
	if err != nil {
		o.logger.Warn("Failed to determine user tier, using default", "error", err)
		userTier = NewUser
	}

	strategy := o.selectStrategy(userTier, reqCtx)

	// Get user profile for personalization
	userProfile, err := o.userService.GetUserProfile(ctx, reqCtx.UserID)
	if err != nil {
		o.logger.Warn("Failed to get user profile", "user_id", reqCtx.UserID, "error", err)
	}

	// Execute algorithms in parallel
	algorithmResults := o.executeAlgorithmsParallel(ctx, reqCtx, userProfile, userTier, strategy)

	// Combine and rank results
	finalRecommendations, err := o.combineAndRankResults(ctx, reqCtx, algorithmResults, userTier)
	if err != nil {
		return nil, fmt.Errorf("failed to combine results: %w", err)
	}

	// Apply fallback if needed
	if len(finalRecommendations) < reqCtx.Count {
		fallbackItems, err := o.applyFallbackStrategy(ctx, reqCtx, userTier, len(finalRecommendations))
		if err != nil {
			o.logger.Warn("Fallback strategy failed", "error", err)
		} else {
			finalRecommendations = append(finalRecommendations, fallbackItems...)
		}
	}

	// Limit to requested count
	if len(finalRecommendations) > reqCtx.Count {
		finalRecommendations = finalRecommendations[:reqCtx.Count]
	}

	// Apply diversity filters
	if o.diversityFilter != nil {
		filteredRecommendations, err := o.diversityFilter.ApplyDiversityFilters(ctx, reqCtx.UserID, finalRecommendations)
		if err != nil {
			o.logger.Warn("Failed to apply diversity filters", "error", err)
		} else {
			finalRecommendations = filteredRecommendations
		}
	}

	// Generate explanations if requested
	if reqCtx.IncludeExplanations && o.explanationService != nil {
		explainedRecommendations, err := o.explanationService.GenerateExplanations(ctx, reqCtx.UserID, finalRecommendations)
		if err != nil {
			o.logger.Warn("Failed to generate explanations", "error", err)
		} else {
			finalRecommendations = explainedRecommendations
		}
	}

	// Create final result
	result := &OrchestrationResult{
		UserID:           reqCtx.UserID,
		Recommendations:  finalRecommendations,
		Context:          reqCtx.Context,
		AlgorithmResults: algorithmResults,
		TotalLatency:     time.Since(startTime),
		CacheHit:         false,
		UserTier:         userTier,
		Strategy:         strategy,
		GeneratedAt:      time.Now(),
	}

	// Cache the result
	if err := o.cacheRecommendations(ctx, reqCtx, result); err != nil {
		o.logger.Warn("Failed to cache recommendations", "error", err)
	}

	o.logger.Info("Recommendations generated",
		"user_id", reqCtx.UserID,
		"count", len(finalRecommendations),
		"strategy", strategy,
		"latency", result.TotalLatency,
	)

	return result, nil
}

// executeAlgorithmsParallel runs multiple algorithms concurrently
func (o *RecommendationOrchestrator) executeAlgorithmsParallel(
	ctx context.Context,
	reqCtx *RecommendationContext,
	userProfile *models.UserProfile,
	userTier UserTier,
	strategy string,
) map[string]*AlgorithmResult {

	// Determine which algorithms to run based on strategy
	algorithmsToRun := o.selectAlgorithms(userTier, strategy)

	// Set timeout for algorithm execution
	timeout := time.Duration(reqCtx.TimeoutMs) * time.Millisecond
	if timeout == 0 {
		timeout = 1500 * time.Millisecond // Default 1.5s timeout
	}

	algorithmCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute algorithms in parallel
	var wg sync.WaitGroup
	results := make(map[string]*AlgorithmResult)
	resultsMutex := sync.RWMutex{}

	for _, algorithm := range algorithmsToRun {
		wg.Add(1)
		go func(alg string) {
			defer wg.Done()

			startTime := time.Now()
			result := &AlgorithmResult{
				Algorithm: alg,
				Latency:   0,
				Error:     nil,
				Cached:    false,
			}

			// Execute specific algorithm
			switch alg {
			case "semantic_search":
				if userProfile != nil && len(userProfile.PreferenceVector) > 0 {
					items, err := o.algorithmService.SemanticSearchRecommendations(
						algorithmCtx, reqCtx.UserID, userProfile.PreferenceVector,
						reqCtx.ContentTypes, reqCtx.Categories, reqCtx.Count*2,
					)
					result.Items = items
					result.Error = err
				} else {
					result.Error = fmt.Errorf("no user preference vector available")
				}

			case "collaborative_filtering":
				items, err := o.algorithmService.CollaborativeFilteringRecommendations(
					algorithmCtx, reqCtx.UserID, reqCtx.Count*2,
				)
				result.Items = items
				result.Error = err

			case "pagerank":
				items, err := o.algorithmService.PersonalizedPageRankRecommendations(
					algorithmCtx, reqCtx.UserID, reqCtx.Count*2,
				)
				result.Items = items
				result.Error = err

			case "graph_signal_analysis":
				items, err := o.algorithmService.GraphSignalAnalysisRecommendations(
					algorithmCtx, reqCtx.UserID, reqCtx.Count*2,
				)
				result.Items = items
				result.Error = err

			default:
				result.Error = fmt.Errorf("unknown algorithm: %s", alg)
			}

			result.Latency = time.Since(startTime)

			if result.Error != nil {
				o.logger.Warn("Algorithm execution failed",
					"algorithm", alg,
					"user_id", reqCtx.UserID,
					"error", result.Error,
					"latency", result.Latency,
				)
			} else {
				o.logger.Debug("Algorithm execution completed",
					"algorithm", alg,
					"user_id", reqCtx.UserID,
					"items", len(result.Items),
					"latency", result.Latency,
				)
			}

			resultsMutex.Lock()
			results[alg] = result
			resultsMutex.Unlock()
		}(algorithm)
	}

	wg.Wait()
	return results
}

// combineAndRankResults combines results from multiple algorithms using weighted scoring
func (o *RecommendationOrchestrator) combineAndRankResults(
	ctx context.Context,
	reqCtx *RecommendationContext,
	algorithmResults map[string]*AlgorithmResult,
	userTier UserTier,
) ([]models.Recommendation, error) {

	// Collect all items with their algorithm scores
	itemScores := make(map[uuid.UUID]*CombinedScore)

	weights := o.algorithmWeights[userTier]

	// Process each algorithm's results
	for algorithm, result := range algorithmResults {
		if result.Error != nil || len(result.Items) == 0 {
			continue
		}

		weight, exists := weights[algorithm]
		if !exists {
			continue
		}

		// Normalize scores for this algorithm
		normalizedItems := o.normalizeScores(result.Items)

		// Add to combined scores
		for _, item := range normalizedItems {
			if _, exists := itemScores[item.ItemID]; !exists {
				itemScores[item.ItemID] = &CombinedScore{
					ItemID:      item.ItemID,
					TotalScore:  0,
					WeightSum:   0,
					Algorithms:  make([]string, 0),
					Confidences: make([]float64, 0),
				}
			}

			// Weighted combination with confidence
			contributionScore := weight * item.Score * item.Confidence
			itemScores[item.ItemID].TotalScore += contributionScore
			itemScores[item.ItemID].WeightSum += weight * item.Confidence
			itemScores[item.ItemID].Algorithms = append(itemScores[item.ItemID].Algorithms, algorithm)
			itemScores[item.ItemID].Confidences = append(itemScores[item.ItemID].Confidences, item.Confidence)
		}
	}

	// Calculate final scores and create recommendations
	var recommendations []models.Recommendation
	for itemID, combined := range itemScores {
		if combined.WeightSum == 0 {
			continue
		}

		// Final score calculation
		finalScore := combined.TotalScore / combined.WeightSum

		// Apply sigmoid calibration for better distribution
		calibratedScore := o.applySigmoidCalibration(finalScore)

		// Calculate overall confidence
		avgConfidence := o.calculateAverageConfidence(combined.Confidences)

		// Create explanation
		explanation := o.generateExplanation(combined.Algorithms, reqCtx.IncludeExplanations)

		recommendation := models.Recommendation{
			ItemID:      itemID,
			Score:       calibratedScore,
			Algorithm:   "orchestrated",
			Explanation: explanation,
			Confidence:  avgConfidence,
			Position:    0, // Will be set after sorting
		}

		recommendations = append(recommendations, recommendation)
	}

	// Sort by final score descending
	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].Score > recommendations[j].Score
	})

	// Set positions
	for i := range recommendations {
		recommendations[i].Position = i + 1
	}

	// Remove excluded items
	if len(reqCtx.ExcludeItems) > 0 {
		recommendations = o.filterExcludedItems(recommendations, reqCtx.ExcludeItems)
	}

	return recommendations, nil
}

// CombinedScore represents the combined score from multiple algorithms
type CombinedScore struct {
	ItemID      uuid.UUID
	TotalScore  float64
	WeightSum   float64
	Algorithms  []string
	Confidences []float64
}

// normalizeScores normalizes algorithm scores to [0,1] range using min-max scaling
func (o *RecommendationOrchestrator) normalizeScores(items []models.ScoredItem) []models.ScoredItem {
	if len(items) == 0 {
		return items
	}

	// Find min and max scores
	minScore := items[0].Score
	maxScore := items[0].Score

	for _, item := range items {
		if item.Score < minScore {
			minScore = item.Score
		}
		if item.Score > maxScore {
			maxScore = item.Score
		}
	}

	// Avoid division by zero
	scoreRange := maxScore - minScore
	if scoreRange == 0 {
		for i := range items {
			items[i].Score = 1.0
		}
		return items
	}

	// Normalize to [0,1]
	for i := range items {
		items[i].Score = (items[i].Score - minScore) / scoreRange
	}

	return items
}

// applySigmoidCalibration applies sigmoid function for better score distribution
func (o *RecommendationOrchestrator) applySigmoidCalibration(score float64) float64 {
	// Sigmoid function: 1 / (1 + e^(-k*(x-0.5)))
	// k=6 provides good calibration for scores in [0,1]
	k := 6.0
	return 1.0 / (1.0 + math.Exp(-k*(score-0.5)))
}

// calculateAverageConfidence calculates the average confidence across algorithms
func (o *RecommendationOrchestrator) calculateAverageConfidence(confidences []float64) float64 {
	if len(confidences) == 0 {
		return 0.0
	}

	sum := 0.0
	for _, conf := range confidences {
		sum += conf
	}

	return sum / float64(len(confidences))
}

// generateExplanation creates an explanation based on contributing algorithms
func (o *RecommendationOrchestrator) generateExplanation(algorithms []string, includeExplanations bool) *string {
	if !includeExplanations || len(algorithms) == 0 {
		return nil
	}

	var explanation string
	if len(algorithms) == 1 {
		switch algorithms[0] {
		case "semantic_search":
			explanation = "Based on your preferences and similar content"
		case "collaborative_filtering":
			explanation = "Recommended by users with similar tastes"
		case "pagerank":
			explanation = "Popular in your network"
		case "graph_signal_analysis":
			explanation = "Trending in your community"
		default:
			explanation = "Personalized recommendation"
		}
	} else {
		explanation = fmt.Sprintf("Recommended by %d algorithms including %s",
			len(algorithms), algorithms[0])
	}

	return &explanation
}

// filterExcludedItems removes excluded items from recommendations
func (o *RecommendationOrchestrator) filterExcludedItems(
	recommendations []models.Recommendation,
	excludeItems []uuid.UUID,
) []models.Recommendation {
	excludeSet := make(map[uuid.UUID]bool)
	for _, itemID := range excludeItems {
		excludeSet[itemID] = true
	}

	var filtered []models.Recommendation
	for _, rec := range recommendations {
		if !excludeSet[rec.ItemID] {
			filtered = append(filtered, rec)
		}
	}

	return filtered
}

// determineUserTier analyzes user profile to determine engagement tier
func (o *RecommendationOrchestrator) determineUserTier(ctx context.Context, userID uuid.UUID) (UserTier, error) {
	profile, err := o.userService.GetUserProfile(ctx, userID)
	if err != nil {
		return NewUser, err
	}

	// Determine tier based on interaction count and recency
	interactionCount := profile.InteractionCount

	// Check for recent activity (last 30 days)
	recentThreshold := time.Now().AddDate(0, 0, -30)
	isRecentlyActive := profile.LastInteraction != nil && profile.LastInteraction.After(recentThreshold)

	switch {
	case interactionCount < 5:
		return NewUser, nil
	case interactionCount >= 50 && isRecentlyActive:
		return PowerUser, nil
	case interactionCount >= 5 && isRecentlyActive:
		return ActiveUser, nil
	default:
		return InactiveUser, nil
	}
}

// selectStrategy determines the recommendation strategy based on user tier and context
func (o *RecommendationOrchestrator) selectStrategy(userTier UserTier, reqCtx *RecommendationContext) string {
	switch userTier {
	case NewUser:
		return "popularity_with_exploration"
	case ActiveUser:
		return "full_personalization"
	case PowerUser:
		return "advanced_personalization"
	case InactiveUser:
		return "reengagement"
	default:
		return "default"
	}
}

// selectAlgorithms determines which algorithms to run based on user tier and strategy
func (o *RecommendationOrchestrator) selectAlgorithms(userTier UserTier, strategy string) []string {
	switch userTier {
	case NewUser:
		return []string{"semantic_search"} // Simple content-based for new users
	case ActiveUser:
		return []string{"semantic_search", "collaborative_filtering", "pagerank"}
	case PowerUser:
		return []string{"collaborative_filtering", "pagerank", "graph_signal_analysis"}
	case InactiveUser:
		return []string{"semantic_search", "collaborative_filtering"} // Re-engagement focus
	default:
		return []string{"semantic_search"}
	}
}

// applyFallbackStrategy provides fallback recommendations when primary algorithms fail
func (o *RecommendationOrchestrator) applyFallbackStrategy(
	ctx context.Context,
	reqCtx *RecommendationContext,
	userTier UserTier,
	currentCount int,
) ([]models.Recommendation, error) {

	needed := reqCtx.Count - currentCount
	if needed <= 0 {
		return nil, nil
	}

	o.logger.Info("Applying fallback strategy",
		"user_id", reqCtx.UserID,
		"user_tier", userTier,
		"needed", needed,
	)

	// For now, return empty recommendations
	// In a real implementation, this would fetch popular/trending items
	// from the database based on the user tier and context

	return []models.Recommendation{}, nil
}

// initializeAlgorithmWeights sets up default algorithm weights by user tier
func (o *RecommendationOrchestrator) initializeAlgorithmWeights() {
	// New users: Focus on content-based recommendations
	o.algorithmWeights[NewUser] = map[string]float64{
		"semantic_search":         1.0,
		"collaborative_filtering": 0.0,
		"pagerank":                0.0,
		"graph_signal_analysis":   0.0,
	}

	// Active users: Balanced approach
	o.algorithmWeights[ActiveUser] = map[string]float64{
		"semantic_search":         0.4,
		"collaborative_filtering": 0.3,
		"pagerank":                0.3,
		"graph_signal_analysis":   0.0,
	}

	// Power users: Advanced algorithms
	o.algorithmWeights[PowerUser] = map[string]float64{
		"semantic_search":         0.2,
		"collaborative_filtering": 0.4,
		"pagerank":                0.2,
		"graph_signal_analysis":   0.2,
	}

	// Inactive users: Re-engagement focus
	o.algorithmWeights[InactiveUser] = map[string]float64{
		"semantic_search":         0.6,
		"collaborative_filtering": 0.4,
		"pagerank":                0.0,
		"graph_signal_analysis":   0.0,
	}
}

// Cache operations

func (o *RecommendationOrchestrator) getCachedRecommendations(
	ctx context.Context,
	reqCtx *RecommendationContext,
) (*OrchestrationResult, error) {

	if o.redis == nil {
		return nil, fmt.Errorf("cache not available")
	}

	cacheKey := o.buildCacheKey(reqCtx)
	cached := o.redis.Get(ctx, cacheKey).Val()

	if cached == "" {
		return nil, fmt.Errorf("cache miss")
	}

	var result OrchestrationResult
	if err := json.Unmarshal([]byte(cached), &result); err != nil {
		return nil, err
	}

	result.CacheHit = true
	return &result, nil
}

func (o *RecommendationOrchestrator) cacheRecommendations(
	ctx context.Context,
	reqCtx *RecommendationContext,
	result *OrchestrationResult,
) error {

	if o.redis == nil {
		return nil // No caching available, but not an error
	}

	cacheKey := o.buildCacheKey(reqCtx)
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}

	// Cache for 15 minutes
	return o.redis.Set(ctx, cacheKey, data, 15*time.Minute).Err()
}

func (o *RecommendationOrchestrator) buildCacheKey(reqCtx *RecommendationContext) string {
	return fmt.Sprintf("orchestration:%s:%s:%d:%v:%v",
		reqCtx.UserID.String(),
		reqCtx.Context,
		reqCtx.Count,
		reqCtx.ContentTypes,
		reqCtx.Categories,
	)
}

// ProcessFeedback processes user feedback on recommendations for learning
func (o *RecommendationOrchestrator) ProcessFeedback(ctx context.Context, feedback *models.RecommendationFeedback) error {
	o.logger.Info("Processing recommendation feedback",
		"user_id", feedback.UserID,
		"item_id", feedback.ItemID,
		"feedback_type", feedback.FeedbackType,
		"recommendation_id", feedback.RecommendationID,
	)

	// Invalidate user-specific caches on feedback
	if err := o.invalidateUserCaches(ctx, feedback.UserID); err != nil {
		o.logger.Warn("Failed to invalidate user caches", "error", err)
	}

	// Store feedback for future model training
	if err := o.storeFeedback(ctx, feedback); err != nil {
		o.logger.Error("Failed to store feedback", "error", err)
		return fmt.Errorf("failed to store feedback: %w", err)
	}

	// Update algorithm weights based on feedback (simple implementation)
	if err := o.updateAlgorithmWeights(ctx, feedback); err != nil {
		o.logger.Warn("Failed to update algorithm weights", "error", err)
	}

	return nil
}

// invalidateUserCaches removes cached recommendations for a user
func (o *RecommendationOrchestrator) invalidateUserCaches(ctx context.Context, userID uuid.UUID) error {
	if o.redis == nil {
		return nil
	}

	// Pattern to match all cache keys for this user
	pattern := fmt.Sprintf("orchestration:%s:*", userID.String())

	// Get all matching keys
	keys, err := o.redis.Keys(ctx, pattern).Result()
	if err != nil {
		return err
	}

	// Delete all matching keys
	if len(keys) > 0 {
		return o.redis.Del(ctx, keys...).Err()
	}

	return nil
}

// storeFeedback stores feedback for future analysis and model training
func (o *RecommendationOrchestrator) storeFeedback(ctx context.Context, feedback *models.RecommendationFeedback) error {
	// In a real implementation, this would store feedback in a database
	// For now, we'll just log it and store in Redis for temporary tracking

	if o.redis == nil {
		return nil
	}

	feedbackKey := fmt.Sprintf("feedback:%s:%s", feedback.UserID.String(), feedback.RecommendationID.String())
	feedbackData, err := json.Marshal(feedback)
	if err != nil {
		return err
	}

	// Store feedback with 30-day expiration
	return o.redis.Set(ctx, feedbackKey, feedbackData, 30*24*time.Hour).Err()
}

// updateAlgorithmWeights adjusts algorithm weights based on feedback
func (o *RecommendationOrchestrator) updateAlgorithmWeights(ctx context.Context, feedback *models.RecommendationFeedback) error {
	// Simple implementation: adjust weights based on feedback type
	// In a real system, this would be more sophisticated with A/B testing and statistical analysis

	userTier, err := o.determineUserTier(ctx, feedback.UserID)
	if err != nil {
		return err
	}

	weights := o.algorithmWeights[userTier]

	// Adjust weights based on feedback type
	switch feedback.FeedbackType {
	case "positive":
		// Slightly increase weights for all algorithms (positive reinforcement)
		for algorithm := range weights {
			weights[algorithm] *= 1.01 // 1% increase
		}
	case "negative", "not_relevant":
		// Slightly decrease weights for all algorithms
		for algorithm := range weights {
			weights[algorithm] *= 0.99 // 1% decrease
		}
	case "not_interested":
		// More significant decrease for collaborative filtering
		if weights["collaborative_filtering"] > 0 {
			weights["collaborative_filtering"] *= 0.95
			weights["semantic_search"] *= 1.02 // Compensate with content-based
		}
	}

	// Normalize weights to ensure they sum to 1.0
	o.normalizeWeights(weights)

	o.logger.Debug("Updated algorithm weights",
		"user_id", feedback.UserID,
		"user_tier", userTier,
		"feedback_type", feedback.FeedbackType,
		"weights", weights,
	)

	return nil
}

// normalizeWeights ensures algorithm weights sum to 1.0
func (o *RecommendationOrchestrator) normalizeWeights(weights map[string]float64) {
	sum := 0.0
	for _, weight := range weights {
		sum += weight
	}

	if sum > 0 {
		for algorithm := range weights {
			weights[algorithm] /= sum
		}
	}
}
