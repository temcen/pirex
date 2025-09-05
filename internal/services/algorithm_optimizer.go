package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// UserSegment represents different user segments for algorithm optimization
type UserSegment string

const (
	SegmentNewUser      UserSegment = "new_user"      // < 5 interactions
	SegmentPowerUser    UserSegment = "power_user"    // > 100 interactions
	SegmentInactiveUser UserSegment = "inactive_user" // no interactions in 30 days
	SegmentRegularUser  UserSegment = "regular_user"  // default segment
)

// AlgorithmPerformance tracks performance metrics for an algorithm
type AlgorithmPerformance struct {
	AlgorithmName    string      `json:"algorithm_name"`
	UserSegment      UserSegment `json:"user_segment"`
	Impressions      int64       `json:"impressions"`
	Clicks           int64       `json:"clicks"`
	Conversions      int64       `json:"conversions"`
	CTR              float64     `json:"ctr"`
	ConversionRate   float64     `json:"conversion_rate"`
	UserSatisfaction float64     `json:"user_satisfaction"`
	LastUpdated      time.Time   `json:"last_updated"`

	// Thompson Sampling parameters
	Alpha float64 `json:"alpha"` // Success count + 1
	Beta  float64 `json:"beta"`  // Failure count + 1
}

// AlgorithmWeights represents the current weights for different algorithms
type AlgorithmWeights struct {
	UserSegment     UserSegment                      `json:"user_segment"`
	Weights         map[string]float64               `json:"weights"`
	LastUpdated     time.Time                        `json:"last_updated"`
	PerformanceData map[string]*AlgorithmPerformance `json:"performance_data"`
}

// AlgorithmOptimizer manages dynamic algorithm weight adjustment
type AlgorithmOptimizer struct {
	db          *sql.DB
	redisClient *redis.Client

	// Configuration
	performanceWindow       time.Duration
	updateInterval          time.Duration
	minPerformanceThreshold float64

	// Current weights by segment
	weights map[UserSegment]*AlgorithmWeights
	mutex   sync.RWMutex

	// Background processing
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewAlgorithmOptimizer creates a new algorithm optimizer
func NewAlgorithmOptimizer(db *sql.DB, redisClient *redis.Client) *AlgorithmOptimizer {
	ctx, cancel := context.WithCancel(context.Background())

	ao := &AlgorithmOptimizer{
		db:                      db,
		redisClient:             redisClient,
		performanceWindow:       7 * 24 * time.Hour, // 7 days
		updateInterval:          1 * time.Hour,      // Update hourly
		minPerformanceThreshold: 0.005,              // 0.5% CTR minimum
		weights:                 make(map[UserSegment]*AlgorithmWeights),
		ctx:                     ctx,
		cancel:                  cancel,
	}

	// Initialize default weights
	ao.initializeDefaultWeights()

	return ao
}

// Start begins the algorithm optimization process
func (ao *AlgorithmOptimizer) Start() error {
	log.Println("Starting algorithm optimizer...")

	// Load existing weights from Redis
	if err := ao.loadWeightsFromCache(); err != nil {
		log.Printf("Warning: failed to load weights from cache: %v", err)
	}

	// Start background optimizer
	ao.wg.Add(1)
	go ao.optimizationWorker()

	log.Println("Algorithm optimizer started")
	return nil
}

// Stop gracefully shuts down the optimizer
func (ao *AlgorithmOptimizer) Stop() {
	log.Println("Stopping algorithm optimizer...")
	ao.cancel()
	ao.wg.Wait()
	log.Println("Algorithm optimizer stopped")
}

// GetWeights returns the current algorithm weights for a user segment
func (ao *AlgorithmOptimizer) GetWeights(segment UserSegment) map[string]float64 {
	ao.mutex.RLock()
	defer ao.mutex.RUnlock()

	if weights, exists := ao.weights[segment]; exists {
		return weights.Weights
	}

	// Return default weights if segment not found
	return ao.getDefaultWeights()
}

// RecordPerformance records performance metrics for an algorithm
func (ao *AlgorithmOptimizer) RecordPerformance(algorithmName string, segment UserSegment,
	impressions, clicks, conversions int64, userSatisfaction float64) error {

	ao.mutex.Lock()
	defer ao.mutex.Unlock()

	// Get or create weights for segment
	if _, exists := ao.weights[segment]; !exists {
		ao.weights[segment] = ao.createDefaultWeightsForSegment(segment)
	}

	// Get or create performance data
	if ao.weights[segment].PerformanceData == nil {
		ao.weights[segment].PerformanceData = make(map[string]*AlgorithmPerformance)
	}

	perf, exists := ao.weights[segment].PerformanceData[algorithmName]
	if !exists {
		perf = &AlgorithmPerformance{
			AlgorithmName: algorithmName,
			UserSegment:   segment,
			Alpha:         1.0, // Prior
			Beta:          1.0, // Prior
		}
		ao.weights[segment].PerformanceData[algorithmName] = perf
	}

	// Update performance metrics
	perf.Impressions += impressions
	perf.Clicks += clicks
	perf.Conversions += conversions
	perf.UserSatisfaction = (perf.UserSatisfaction + userSatisfaction) / 2.0
	perf.LastUpdated = time.Now()

	// Calculate rates
	if perf.Impressions > 0 {
		perf.CTR = float64(perf.Clicks) / float64(perf.Impressions)
		perf.ConversionRate = float64(perf.Conversions) / float64(perf.Impressions)
	}

	// Update Thompson Sampling parameters
	perf.Alpha += float64(clicks)
	perf.Beta += float64(impressions - clicks)

	return nil
}

// optimizationWorker runs the background optimization process
func (ao *AlgorithmOptimizer) optimizationWorker() {
	defer ao.wg.Done()

	ticker := time.NewTicker(ao.updateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := ao.optimizeWeights(); err != nil {
				log.Printf("Error optimizing weights: %v", err)
			}

		case <-ao.ctx.Done():
			return
		}
	}
}

// optimizeWeights optimizes algorithm weights using Thompson Sampling
func (ao *AlgorithmOptimizer) optimizeWeights() error {
	ao.mutex.Lock()
	defer ao.mutex.Unlock()

	log.Println("Optimizing algorithm weights...")

	for segment, weights := range ao.weights {
		if err := ao.optimizeSegmentWeights(segment, weights); err != nil {
			log.Printf("Error optimizing weights for segment %s: %v", segment, err)
			continue
		}
	}

	// Cache updated weights
	if err := ao.cacheWeights(); err != nil {
		log.Printf("Warning: failed to cache weights: %v", err)
	}

	log.Println("Algorithm weights optimization completed")
	return nil
}

// optimizeSegmentWeights optimizes weights for a specific user segment
func (ao *AlgorithmOptimizer) optimizeSegmentWeights(segment UserSegment, weights *AlgorithmWeights) error {
	if weights.PerformanceData == nil || len(weights.PerformanceData) == 0 {
		return nil // No performance data to optimize
	}

	// Sample from Thompson Sampling distributions
	sampledRewards := make(map[string]float64)
	totalSampled := 0.0

	for algorithmName, perf := range weights.PerformanceData {
		// Check minimum performance threshold
		if perf.CTR < ao.minPerformanceThreshold && perf.Impressions > 100 {
			// Disable underperforming algorithms
			sampledRewards[algorithmName] = 0.0
			log.Printf("Disabling underperforming algorithm %s for segment %s (CTR: %.4f)",
				algorithmName, segment, perf.CTR)
			continue
		}

		// Sample from Beta distribution (Thompson Sampling)
		reward := ao.sampleBeta(perf.Alpha, perf.Beta)
		sampledRewards[algorithmName] = reward
		totalSampled += reward
	}

	// Normalize to get new weights
	if totalSampled > 0 {
		for algorithmName := range weights.Weights {
			if reward, exists := sampledRewards[algorithmName]; exists {
				weights.Weights[algorithmName] = reward / totalSampled
			} else {
				weights.Weights[algorithmName] = 0.0
			}
		}
	}

	weights.LastUpdated = time.Now()

	log.Printf("Updated weights for segment %s: %v", segment, weights.Weights)
	return nil
}

// sampleBeta samples from a Beta distribution using rejection sampling
func (ao *AlgorithmOptimizer) sampleBeta(alpha, beta float64) float64 {
	// Simple Beta sampling using Gamma distributions
	// In production, you might want to use a more sophisticated method

	if alpha <= 0 || beta <= 0 {
		return 0.5 // Default value
	}

	// Use the fact that if X ~ Gamma(α) and Y ~ Gamma(β), then X/(X+Y) ~ Beta(α,β)
	x := ao.sampleGamma(alpha)
	y := ao.sampleGamma(beta)

	if x+y == 0 {
		return 0.5
	}

	return x / (x + y)
}

// sampleGamma samples from a Gamma distribution (simplified implementation)
func (ao *AlgorithmOptimizer) sampleGamma(shape float64) float64 {
	if shape < 1 {
		// Use rejection method for shape < 1
		return ao.sampleGamma(shape+1) * math.Pow(rand.Float64(), 1.0/shape)
	}

	// Marsaglia and Tsang's method for shape >= 1
	d := shape - 1.0/3.0
	c := 1.0 / math.Sqrt(9.0*d)

	for {
		x := rand.NormFloat64()
		v := 1.0 + c*x
		if v <= 0 {
			continue
		}

		v = v * v * v
		u := rand.Float64()

		if u < 1.0-0.0331*(x*x)*(x*x) {
			return d * v
		}

		if math.Log(u) < 0.5*x*x+d*(1.0-v+math.Log(v)) {
			return d * v
		}
	}
}

// initializeDefaultWeights sets up default algorithm weights
func (ao *AlgorithmOptimizer) initializeDefaultWeights() {
	segments := []UserSegment{SegmentNewUser, SegmentPowerUser, SegmentInactiveUser, SegmentRegularUser}

	for _, segment := range segments {
		ao.weights[segment] = ao.createDefaultWeightsForSegment(segment)
	}
}

// createDefaultWeightsForSegment creates default weights for a user segment
func (ao *AlgorithmOptimizer) createDefaultWeightsForSegment(segment UserSegment) *AlgorithmWeights {
	weights := &AlgorithmWeights{
		UserSegment:     segment,
		Weights:         make(map[string]float64),
		LastUpdated:     time.Now(),
		PerformanceData: make(map[string]*AlgorithmPerformance),
	}

	// Set segment-specific default weights
	switch segment {
	case SegmentNewUser:
		weights.Weights = map[string]float64{
			"semantic_search":         0.2,
			"collaborative_filtering": 0.1, // Lower for new users
			"pagerank":                0.2,
			"popularity_based":        0.5, // Higher for new users
		}
	case SegmentPowerUser:
		weights.Weights = map[string]float64{
			"semantic_search":         0.3,
			"collaborative_filtering": 0.4, // Higher for power users
			"pagerank":                0.3,
			"popularity_based":        0.0, // Lower for power users
		}
	case SegmentInactiveUser:
		weights.Weights = map[string]float64{
			"semantic_search":         0.3,
			"collaborative_filtering": 0.2,
			"pagerank":                0.2,
			"popularity_based":        0.3, // Re-engagement focus
		}
	default: // SegmentRegularUser
		weights.Weights = ao.getDefaultWeights()
	}

	return weights
}

// getDefaultWeights returns the default algorithm weights
func (ao *AlgorithmOptimizer) getDefaultWeights() map[string]float64 {
	return map[string]float64{
		"semantic_search":         0.4,
		"collaborative_filtering": 0.3,
		"pagerank":                0.3,
		"popularity_based":        0.0,
	}
}

// DetermineUserSegment determines the user segment based on interaction history
func (ao *AlgorithmOptimizer) DetermineUserSegment(userID string) UserSegment {
	// Query user interaction count and recency
	var interactionCount int
	var lastInteraction sql.NullTime

	query := `
		SELECT COUNT(*), MAX(timestamp)
		FROM user_interactions 
		WHERE user_id = $1 AND timestamp > NOW() - INTERVAL '90 days'
	`

	err := ao.db.QueryRow(query, userID).Scan(&interactionCount, &lastInteraction)
	if err != nil {
		log.Printf("Error determining user segment: %v", err)
		return SegmentRegularUser
	}

	// Determine segment based on interaction patterns
	if interactionCount < 5 {
		return SegmentNewUser
	}

	if interactionCount > 100 {
		return SegmentPowerUser
	}

	if lastInteraction.Valid && time.Since(lastInteraction.Time) > 30*24*time.Hour {
		return SegmentInactiveUser
	}

	return SegmentRegularUser
}

// cacheWeights caches the current weights in Redis
func (ao *AlgorithmOptimizer) cacheWeights() error {
	for segment, weights := range ao.weights {
		key := fmt.Sprintf("algorithm_weights:%s", segment)

		data, err := json.Marshal(weights)
		if err != nil {
			return err
		}

		if err := ao.redisClient.Set(ao.ctx, key, data, 24*time.Hour).Err(); err != nil {
			return err
		}
	}

	return nil
}

// loadWeightsFromCache loads weights from Redis cache
func (ao *AlgorithmOptimizer) loadWeightsFromCache() error {
	segments := []UserSegment{SegmentNewUser, SegmentPowerUser, SegmentInactiveUser, SegmentRegularUser}

	for _, segment := range segments {
		key := fmt.Sprintf("algorithm_weights:%s", segment)

		data, err := ao.redisClient.Get(ao.ctx, key).Result()
		if err != nil {
			continue // Skip if not found
		}

		var weights AlgorithmWeights
		if err := json.Unmarshal([]byte(data), &weights); err != nil {
			continue // Skip if invalid
		}

		ao.weights[segment] = &weights
	}

	return nil
}

// GetPerformanceMetrics returns performance metrics for all algorithms
func (ao *AlgorithmOptimizer) GetPerformanceMetrics() map[UserSegment]map[string]*AlgorithmPerformance {
	ao.mutex.RLock()
	defer ao.mutex.RUnlock()

	result := make(map[UserSegment]map[string]*AlgorithmPerformance)

	for segment, weights := range ao.weights {
		if weights.PerformanceData != nil {
			result[segment] = make(map[string]*AlgorithmPerformance)
			for name, perf := range weights.PerformanceData {
				// Create a copy to avoid race conditions
				perfCopy := *perf
				result[segment][name] = &perfCopy
			}
		}
	}

	return result
}
