package services

import (
	"context"
	"crypto/md5"
	"database/sql"
	"fmt"
	"hash/fnv"
	"log"
	"math"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// ExperimentStatus represents the status of an A/B test experiment
type ExperimentStatus string

const (
	ExperimentStatusDraft    ExperimentStatus = "draft"
	ExperimentStatusActive   ExperimentStatus = "active"
	ExperimentStatusPaused   ExperimentStatus = "paused"
	ExperimentStatusComplete ExperimentStatus = "complete"
)

// ExperimentType represents the type of experiment
type ExperimentType string

const (
	ExperimentTypeAlgorithm ExperimentType = "algorithm"
	ExperimentTypeUI        ExperimentType = "ui"
	ExperimentTypeRanking   ExperimentType = "ranking"
)

// ExperimentVariant represents a variant in an A/B test
type ExperimentVariant struct {
	ID                string                 `json:"id"`
	Name              string                 `json:"name"`
	TrafficAllocation float64                `json:"traffic_allocation"` // 0.0 to 1.0
	Configuration     map[string]interface{} `json:"configuration"`
	IsControl         bool                   `json:"is_control"`
}

// ExperimentMetrics represents metrics for an experiment variant
type ExperimentMetrics struct {
	VariantID      string    `json:"variant_id"`
	Impressions    int64     `json:"impressions"`
	Clicks         int64     `json:"clicks"`
	Conversions    int64     `json:"conversions"`
	Revenue        float64   `json:"revenue"`
	CTR            float64   `json:"ctr"`
	ConversionRate float64   `json:"conversion_rate"`
	RevenuePerUser float64   `json:"revenue_per_user"`
	LastUpdated    time.Time `json:"last_updated"`
}

// StatisticalResult represents the result of statistical significance testing
type StatisticalResult struct {
	VariantA        string  `json:"variant_a"`
	VariantB        string  `json:"variant_b"`
	Metric          string  `json:"metric"`
	PValue          float64 `json:"p_value"`
	ConfidenceLevel float64 `json:"confidence_level"`
	IsSignificant   bool    `json:"is_significant"`
	Effect          float64 `json:"effect"` // Relative difference
	SampleSizeA     int64   `json:"sample_size_a"`
	SampleSizeB     int64   `json:"sample_size_b"`
}

// Experiment represents an A/B test experiment
type Experiment struct {
	ID                string              `json:"id"`
	Name              string              `json:"name"`
	Description       string              `json:"description"`
	Type              ExperimentType      `json:"type"`
	Status            ExperimentStatus    `json:"status"`
	Variants          []ExperimentVariant `json:"variants"`
	SuccessMetrics    []string            `json:"success_metrics"`
	StartDate         time.Time           `json:"start_date"`
	EndDate           time.Time           `json:"end_date"`
	MinSampleSize     int64               `json:"min_sample_size"`
	TargetPower       float64             `json:"target_power"`       // Statistical power (0.8 = 80%)
	SignificanceLevel float64             `json:"significance_level"` // Alpha (0.05 = 5%)
	CreatedAt         time.Time           `json:"created_at"`
	UpdatedAt         time.Time           `json:"updated_at"`

	// Runtime data
	Metrics     map[string]*ExperimentMetrics `json:"metrics,omitempty"`
	StatResults []StatisticalResult           `json:"statistical_results,omitempty"`
}

// ABTestingFramework manages A/B testing experiments
type ABTestingFramework struct {
	db          *sql.DB
	redisClient *redis.Client

	// Active experiments cache
	experiments map[string]*Experiment
	mutex       sync.RWMutex

	// Background processing
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewABTestingFramework creates a new A/B testing framework
func NewABTestingFramework(db *sql.DB, redisClient *redis.Client) *ABTestingFramework {
	ctx, cancel := context.WithCancel(context.Background())

	return &ABTestingFramework{
		db:          db,
		redisClient: redisClient,
		experiments: make(map[string]*Experiment),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start initializes the A/B testing framework
func (ab *ABTestingFramework) Start() error {
	log.Println("Starting A/B testing framework...")

	// Load active experiments
	if err := ab.loadActiveExperiments(); err != nil {
		return fmt.Errorf("failed to load active experiments: %w", err)
	}

	// Start background workers
	ab.wg.Add(1)
	go ab.metricsCollector()

	ab.wg.Add(1)
	go ab.statisticalAnalyzer()

	log.Printf("A/B testing framework started with %d active experiments", len(ab.experiments))
	return nil
}

// Stop gracefully shuts down the framework
func (ab *ABTestingFramework) Stop() {
	log.Println("Stopping A/B testing framework...")
	ab.cancel()
	ab.wg.Wait()
	log.Println("A/B testing framework stopped")
}

// CreateExperiment creates a new A/B test experiment
func (ab *ABTestingFramework) CreateExperiment(experiment *Experiment) error {
	// Validate experiment configuration
	if err := ab.validateExperiment(experiment); err != nil {
		return fmt.Errorf("experiment validation failed: %w", err)
	}

	// Generate ID if not provided
	if experiment.ID == "" {
		experiment.ID = ab.generateExperimentID(experiment.Name)
	}

	experiment.CreatedAt = time.Now()
	experiment.UpdatedAt = time.Now()
	experiment.Status = ExperimentStatusDraft

	// Store in database
	if err := ab.storeExperiment(experiment); err != nil {
		return fmt.Errorf("failed to store experiment: %w", err)
	}

	log.Printf("Created experiment: %s (%s)", experiment.Name, experiment.ID)
	return nil
}

// StartExperiment starts an A/B test experiment
func (ab *ABTestingFramework) StartExperiment(experimentID string) error {
	ab.mutex.Lock()
	defer ab.mutex.Unlock()

	experiment, err := ab.getExperiment(experimentID)
	if err != nil {
		return err
	}

	if experiment.Status != ExperimentStatusDraft {
		return fmt.Errorf("experiment must be in draft status to start")
	}

	experiment.Status = ExperimentStatusActive
	experiment.StartDate = time.Now()
	experiment.UpdatedAt = time.Now()

	// Initialize metrics
	experiment.Metrics = make(map[string]*ExperimentMetrics)
	for _, variant := range experiment.Variants {
		experiment.Metrics[variant.ID] = &ExperimentMetrics{
			VariantID:   variant.ID,
			LastUpdated: time.Now(),
		}
	}

	// Cache active experiment
	ab.experiments[experimentID] = experiment

	// Update in database
	if err := ab.updateExperiment(experiment); err != nil {
		return err
	}

	log.Printf("Started experiment: %s", experiment.Name)
	return nil
}

// AssignUserToVariant assigns a user to an experiment variant
func (ab *ABTestingFramework) AssignUserToVariant(userID, experimentID string) (string, error) {
	ab.mutex.RLock()
	experiment, exists := ab.experiments[experimentID]
	ab.mutex.RUnlock()

	if !exists {
		return "", fmt.Errorf("experiment not found or not active: %s", experimentID)
	}

	if experiment.Status != ExperimentStatusActive {
		return "", fmt.Errorf("experiment is not active: %s", experimentID)
	}

	// Check cache first
	cacheKey := fmt.Sprintf("ab_assignment:%s:%s", experimentID, userID)
	if cached, err := ab.redisClient.Get(ab.ctx, cacheKey).Result(); err == nil {
		return cached, nil
	}

	// Hash-based consistent assignment
	variantID := ab.assignVariantByHash(userID, experiment)

	// Cache assignment
	ab.redisClient.Set(ab.ctx, cacheKey, variantID, 24*time.Hour)

	return variantID, nil
}

// RecordEvent records an event for A/B testing metrics
func (ab *ABTestingFramework) RecordEvent(userID, experimentID, eventType string, value float64) error {
	// Get user's variant assignment
	variantID, err := ab.AssignUserToVariant(userID, experimentID)
	if err != nil {
		return err // User not in experiment
	}

	ab.mutex.Lock()
	experiment, exists := ab.experiments[experimentID]
	if !exists {
		ab.mutex.Unlock()
		return fmt.Errorf("experiment not found: %s", experimentID)
	}

	metrics, exists := experiment.Metrics[variantID]
	if !exists {
		ab.mutex.Unlock()
		return fmt.Errorf("variant metrics not found: %s", variantID)
	}

	// Update metrics based on event type
	switch eventType {
	case "impression":
		metrics.Impressions++
	case "click":
		metrics.Clicks++
	case "conversion":
		metrics.Conversions++
	case "revenue":
		metrics.Revenue += value
	}

	// Recalculate rates
	if metrics.Impressions > 0 {
		metrics.CTR = float64(metrics.Clicks) / float64(metrics.Impressions)
		metrics.ConversionRate = float64(metrics.Conversions) / float64(metrics.Impressions)
		metrics.RevenuePerUser = metrics.Revenue / float64(metrics.Impressions)
	}

	metrics.LastUpdated = time.Now()
	ab.mutex.Unlock()

	// Store event for detailed analysis
	return ab.storeExperimentEvent(experimentID, variantID, userID, eventType, value)
}

// GetExperimentResults returns the current results of an experiment
func (ab *ABTestingFramework) GetExperimentResults(experimentID string) (*Experiment, error) {
	ab.mutex.RLock()
	experiment, exists := ab.experiments[experimentID]
	ab.mutex.RUnlock()

	if !exists {
		return ab.getExperiment(experimentID)
	}

	// Create a copy to avoid race conditions
	result := *experiment
	result.Metrics = make(map[string]*ExperimentMetrics)
	for k, v := range experiment.Metrics {
		metricsCopy := *v
		result.Metrics[k] = &metricsCopy
	}

	return &result, nil
}

// assignVariantByHash assigns a variant using consistent hashing
func (ab *ABTestingFramework) assignVariantByHash(userID string, experiment *Experiment) string {
	// Create hash of user ID + experiment ID for consistency
	hasher := fnv.New32a()
	hasher.Write([]byte(userID + experiment.ID))
	hash := float64(hasher.Sum32()) / float64(^uint32(0))

	// Assign based on traffic allocation
	cumulative := 0.0
	for _, variant := range experiment.Variants {
		cumulative += variant.TrafficAllocation
		if hash <= cumulative {
			return variant.ID
		}
	}

	// Fallback to control variant
	for _, variant := range experiment.Variants {
		if variant.IsControl {
			return variant.ID
		}
	}

	// Fallback to first variant
	if len(experiment.Variants) > 0 {
		return experiment.Variants[0].ID
	}

	return ""
}

// metricsCollector runs background metrics collection
func (ab *ABTestingFramework) metricsCollector() {
	defer ab.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ab.collectMetrics()

		case <-ab.ctx.Done():
			return
		}
	}
}

// statisticalAnalyzer runs background statistical analysis
func (ab *ABTestingFramework) statisticalAnalyzer() {
	defer ab.wg.Done()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ab.runStatisticalAnalysis()

		case <-ab.ctx.Done():
			return
		}
	}
}

// collectMetrics collects and persists experiment metrics
func (ab *ABTestingFramework) collectMetrics() {
	ab.mutex.RLock()
	experiments := make([]*Experiment, 0, len(ab.experiments))
	for _, exp := range ab.experiments {
		experiments = append(experiments, exp)
	}
	ab.mutex.RUnlock()

	for _, experiment := range experiments {
		if err := ab.persistExperimentMetrics(experiment); err != nil {
			log.Printf("Error persisting metrics for experiment %s: %v", experiment.ID, err)
		}
	}
}

// runStatisticalAnalysis performs statistical significance testing
func (ab *ABTestingFramework) runStatisticalAnalysis() {
	ab.mutex.Lock()
	defer ab.mutex.Unlock()

	for _, experiment := range ab.experiments {
		if len(experiment.Variants) < 2 {
			continue
		}

		// Find control variant
		var controlVariant *ExperimentVariant
		for _, variant := range experiment.Variants {
			if variant.IsControl {
				controlVariant = &variant
				break
			}
		}

		if controlVariant == nil {
			continue
		}

		// Compare each variant against control
		controlMetrics := experiment.Metrics[controlVariant.ID]
		if controlMetrics == nil {
			continue
		}

		experiment.StatResults = []StatisticalResult{}

		for _, variant := range experiment.Variants {
			if variant.IsControl {
				continue
			}

			variantMetrics := experiment.Metrics[variant.ID]
			if variantMetrics == nil {
				continue
			}

			// Perform statistical tests for each success metric
			for _, metric := range experiment.SuccessMetrics {
				result := ab.performStatisticalTest(controlMetrics, variantMetrics, metric, experiment.SignificanceLevel)
				result.VariantA = controlVariant.ID
				result.VariantB = variant.ID
				experiment.StatResults = append(experiment.StatResults, result)
			}
		}
	}
}

// performStatisticalTest performs a statistical significance test
func (ab *ABTestingFramework) performStatisticalTest(controlMetrics, variantMetrics *ExperimentMetrics,
	metric string, alpha float64) StatisticalResult {

	result := StatisticalResult{
		Metric:          metric,
		ConfidenceLevel: 1.0 - alpha,
		SampleSizeA:     controlMetrics.Impressions,
		SampleSizeB:     variantMetrics.Impressions,
	}

	switch metric {
	case "ctr":
		result.PValue, result.Effect = ab.proportionZTest(
			controlMetrics.Clicks, controlMetrics.Impressions,
			variantMetrics.Clicks, variantMetrics.Impressions)

	case "conversion_rate":
		result.PValue, result.Effect = ab.proportionZTest(
			controlMetrics.Conversions, controlMetrics.Impressions,
			variantMetrics.Conversions, variantMetrics.Impressions)

	case "revenue_per_user":
		// For continuous metrics, use t-test (simplified implementation)
		result.PValue = 0.5 // Placeholder - implement proper t-test
		result.Effect = (variantMetrics.RevenuePerUser - controlMetrics.RevenuePerUser) / controlMetrics.RevenuePerUser
	}

	result.IsSignificant = result.PValue < alpha

	return result
}

// proportionZTest performs a two-proportion z-test
func (ab *ABTestingFramework) proportionZTest(successes1, trials1, successes2, trials2 int64) (pValue, effect float64) {
	if trials1 == 0 || trials2 == 0 {
		return 1.0, 0.0
	}

	p1 := float64(successes1) / float64(trials1)
	p2 := float64(successes2) / float64(trials2)

	// Pooled proportion
	pPool := float64(successes1+successes2) / float64(trials1+trials2)

	// Standard error
	se := math.Sqrt(pPool * (1 - pPool) * (1.0/float64(trials1) + 1.0/float64(trials2)))

	if se == 0 {
		return 1.0, 0.0
	}

	// Z-score
	z := (p1 - p2) / se

	// Two-tailed p-value (simplified - use proper normal CDF in production)
	pValue = 2.0 * (1.0 - ab.normalCDF(math.Abs(z)))

	// Clamp p-value to [0, 1] range
	if pValue > 1.0 {
		pValue = 1.0
	} else if pValue < 0.0 {
		pValue = 0.0
	}

	// Effect size (relative difference)
	if p1 != 0 {
		effect = (p2 - p1) / p1
	}

	return pValue, effect
}

// normalCDF approximates the normal cumulative distribution function
func (ab *ABTestingFramework) normalCDF(x float64) float64 {
	// Abramowitz and Stegun approximation
	if x < 0 {
		return 1 - ab.normalCDF(-x)
	}

	// Constants
	a1 := 0.254829592
	a2 := -0.284496736
	a3 := 1.421413741
	a4 := -1.453152027
	a5 := 1.061405429
	p := 0.3275911

	t := 1.0 / (1.0 + p*x)
	y := 1.0 - (((((a5*t+a4)*t)+a3)*t+a2)*t+a1)*t*math.Exp(-x*x)

	return y
}

// Helper methods for database operations
func (ab *ABTestingFramework) validateExperiment(experiment *Experiment) error {
	if experiment.Name == "" {
		return fmt.Errorf("experiment name is required")
	}

	if len(experiment.Variants) < 2 {
		return fmt.Errorf("experiment must have at least 2 variants")
	}

	totalAllocation := 0.0
	hasControl := false

	for _, variant := range experiment.Variants {
		totalAllocation += variant.TrafficAllocation
		if variant.IsControl {
			hasControl = true
		}
	}

	if math.Abs(totalAllocation-1.0) > 0.001 {
		return fmt.Errorf("traffic allocation must sum to 1.0, got %.3f", totalAllocation)
	}

	if !hasControl {
		return fmt.Errorf("experiment must have a control variant")
	}

	return nil
}

func (ab *ABTestingFramework) generateExperimentID(name string) string {
	hash := md5.Sum([]byte(name + time.Now().String()))
	return fmt.Sprintf("exp_%x", hash)[:16]
}

func (ab *ABTestingFramework) loadActiveExperiments() error {
	// Implementation to load active experiments from database
	return nil // Placeholder
}

func (ab *ABTestingFramework) getExperiment(experimentID string) (*Experiment, error) {
	// Implementation to get experiment from database
	return nil, fmt.Errorf("not implemented") // Placeholder
}

func (ab *ABTestingFramework) storeExperiment(experiment *Experiment) error {
	// Implementation to store experiment in database
	return nil // Placeholder
}

func (ab *ABTestingFramework) updateExperiment(experiment *Experiment) error {
	// Implementation to update experiment in database
	return nil // Placeholder
}

func (ab *ABTestingFramework) storeExperimentEvent(experimentID, variantID, userID, eventType string, value float64) error {
	// Implementation to store experiment event
	return nil // Placeholder
}

func (ab *ABTestingFramework) persistExperimentMetrics(experiment *Experiment) error {
	// Implementation to persist experiment metrics
	return nil // Placeholder
}
