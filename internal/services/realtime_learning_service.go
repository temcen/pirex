package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

// RealtimeLearningService orchestrates all real-time learning components
type RealtimeLearningService struct {
	// Core components
	feedbackProcessor  *FeedbackProcessor
	algorithmOptimizer *AlgorithmOptimizer
	abTestingFramework *ABTestingFramework
	continuousLearning *ContinuousLearningPipeline

	// Dependencies
	db          *sql.DB
	redisClient *redis.Client
	kafkaWriter *kafka.Writer

	// Service state
	isRunning bool
	mutex     sync.RWMutex

	// Background processing
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// RealtimeLearningConfig holds configuration for the real-time learning service
type RealtimeLearningConfig struct {
	FeedbackProcessing struct {
		ExplicitWorkers int           `yaml:"explicit_workers"`
		ImplicitWorkers int           `yaml:"implicit_workers"`
		BatchSize       int           `yaml:"batch_size"`
		BatchInterval   time.Duration `yaml:"batch_interval"`
	} `yaml:"feedback_processing"`

	AlgorithmOptimization struct {
		UpdateInterval          time.Duration `yaml:"update_interval"`
		PerformanceWindow       time.Duration `yaml:"performance_window"`
		MinPerformanceThreshold float64       `yaml:"min_performance_threshold"`
	} `yaml:"algorithm_optimization"`

	ContinuousLearning struct {
		RetrainingInterval time.Duration `yaml:"retraining_interval"`
		MinTrainingData    int64         `yaml:"min_training_data"`
		ValidationSplit    float64       `yaml:"validation_split"`
		ExplorationRate    float64       `yaml:"exploration_rate"`
	} `yaml:"continuous_learning"`
}

// NewRealtimeLearningService creates a new real-time learning service
func NewRealtimeLearningService(db *sql.DB, redisClient *redis.Client, kafkaWriter *kafka.Writer) *RealtimeLearningService {
	ctx, cancel := context.WithCancel(context.Background())

	rls := &RealtimeLearningService{
		db:          db,
		redisClient: redisClient,
		kafkaWriter: kafkaWriter,
		ctx:         ctx,
		cancel:      cancel,
	}

	// Initialize components
	rls.feedbackProcessor = NewFeedbackProcessor(db, redisClient, kafkaWriter)
	rls.algorithmOptimizer = NewAlgorithmOptimizer(db, redisClient)
	rls.abTestingFramework = NewABTestingFramework(db, redisClient)
	rls.continuousLearning = NewContinuousLearningPipeline(db, redisClient)

	return rls
}

// Start initializes and starts all real-time learning components
func (rls *RealtimeLearningService) Start() error {
	rls.mutex.Lock()
	defer rls.mutex.Unlock()

	if rls.isRunning {
		return fmt.Errorf("real-time learning service is already running")
	}

	log.Println("Starting real-time learning service...")

	// Start all components
	components := []struct {
		name    string
		starter func() error
	}{
		{"feedback processor", rls.feedbackProcessor.Start},
		{"algorithm optimizer", rls.algorithmOptimizer.Start},
		{"A/B testing framework", rls.abTestingFramework.Start},
		{"continuous learning pipeline", rls.continuousLearning.Start},
	}

	for _, component := range components {
		if err := component.starter(); err != nil {
			// Stop already started components
			rls.stopComponents()
			return fmt.Errorf("failed to start %s: %w", component.name, err)
		}
		log.Printf("Started %s", component.name)
	}

	// Start coordination worker
	rls.wg.Add(1)
	go rls.coordinationWorker()

	rls.isRunning = true
	log.Println("Real-time learning service started successfully")

	return nil
}

// Stop gracefully shuts down all components
func (rls *RealtimeLearningService) Stop() {
	rls.mutex.Lock()
	defer rls.mutex.Unlock()

	if !rls.isRunning {
		return
	}

	log.Println("Stopping real-time learning service...")

	// Cancel context to signal shutdown
	rls.cancel()

	// Stop all components
	rls.stopComponents()

	// Wait for coordination worker to finish
	rls.wg.Wait()

	rls.isRunning = false
	log.Println("Real-time learning service stopped")
}

// stopComponents stops all learning components
func (rls *RealtimeLearningService) stopComponents() {
	components := []struct {
		name    string
		stopper func()
	}{
		{"continuous learning pipeline", rls.continuousLearning.Stop},
		{"A/B testing framework", rls.abTestingFramework.Stop},
		{"algorithm optimizer", rls.algorithmOptimizer.Stop},
		{"feedback processor", rls.feedbackProcessor.Stop},
	}

	for _, component := range components {
		component.stopper()
		log.Printf("Stopped %s", component.name)
	}
}

// ProcessFeedback processes user feedback through the feedback processor
func (rls *RealtimeLearningService) ProcessFeedback(event FeedbackEvent) error {
	if !rls.isRunning {
		return fmt.Errorf("real-time learning service is not running")
	}

	return rls.feedbackProcessor.ProcessFeedback(event)
}

// GetAlgorithmWeights returns current algorithm weights for a user segment
func (rls *RealtimeLearningService) GetAlgorithmWeights(userID string) map[string]float64 {
	if !rls.isRunning {
		return make(map[string]float64)
	}

	segment := rls.algorithmOptimizer.DetermineUserSegment(userID)
	return rls.algorithmOptimizer.GetWeights(segment)
}

// RecordAlgorithmPerformance records performance metrics for algorithm optimization
func (rls *RealtimeLearningService) RecordAlgorithmPerformance(algorithmName string, userID string,
	impressions, clicks, conversions int64, userSatisfaction float64) error {

	if !rls.isRunning {
		return fmt.Errorf("real-time learning service is not running")
	}

	segment := rls.algorithmOptimizer.DetermineUserSegment(userID)
	return rls.algorithmOptimizer.RecordPerformance(algorithmName, segment, impressions, clicks, conversions, userSatisfaction)
}

// AssignUserToExperiment assigns a user to an A/B test experiment
func (rls *RealtimeLearningService) AssignUserToExperiment(userID, experimentID string) (string, error) {
	if !rls.isRunning {
		return "", fmt.Errorf("real-time learning service is not running")
	}

	return rls.abTestingFramework.AssignUserToVariant(userID, experimentID)
}

// RecordExperimentEvent records an event for A/B testing
func (rls *RealtimeLearningService) RecordExperimentEvent(userID, experimentID, eventType string, value float64) error {
	if !rls.isRunning {
		return fmt.Errorf("real-time learning service is not running")
	}

	return rls.abTestingFramework.RecordEvent(userID, experimentID, eventType, value)
}

// GetRecommendationStrategy returns the recommendation strategy for a user
func (rls *RealtimeLearningService) GetRecommendationStrategy(userID string) map[string]interface{} {
	if !rls.isRunning {
		return make(map[string]interface{})
	}

	return rls.continuousLearning.GetRecommendationStrategy(userID)
}

// ShouldExplore determines if the system should explore vs exploit for a user
func (rls *RealtimeLearningService) ShouldExplore(userID string) bool {
	if !rls.isRunning {
		return false
	}

	return rls.continuousLearning.ShouldExplore(userID)
}

// coordinationWorker coordinates between different learning components
func (rls *RealtimeLearningService) coordinationWorker() {
	defer rls.wg.Done()

	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rls.coordinateComponents()

		case <-rls.ctx.Done():
			return
		}
	}
}

// coordinateComponents coordinates between different learning components
func (rls *RealtimeLearningService) coordinateComponents() {
	// Sync algorithm performance data with A/B testing
	rls.syncAlgorithmPerformanceWithABTesting()

	// Update exploration rates based on A/B test results
	rls.updateExplorationRates()

	// Coordinate model retraining with algorithm optimization
	rls.coordinateModelRetraining()
}

// syncAlgorithmPerformanceWithABTesting syncs performance data between components
func (rls *RealtimeLearningService) syncAlgorithmPerformanceWithABTesting() {
	// Get algorithm performance metrics
	performanceMetrics := rls.algorithmOptimizer.GetPerformanceMetrics()

	// Create or update A/B tests for algorithm comparison
	for segment, algorithms := range performanceMetrics {
		for algorithmName, performance := range algorithms {
			// Check if there's an active experiment for this algorithm
			experimentID := fmt.Sprintf("algorithm_%s_%s", algorithmName, segment)

			// Record performance as experiment metrics
			if err := rls.abTestingFramework.RecordEvent("system", experimentID, "impression", float64(performance.Impressions)); err != nil {
				log.Printf("Warning: failed to record algorithm performance for A/B testing: %v", err)
			}
		}
	}
}

// updateExplorationRates updates exploration rates based on A/B test results
func (rls *RealtimeLearningService) updateExplorationRates() {
	// This would analyze A/B test results to determine optimal exploration rates
	// For now, this is a placeholder for the coordination logic
	log.Println("Updating exploration rates based on A/B test results...")
}

// coordinateModelRetraining coordinates model retraining with algorithm optimization
func (rls *RealtimeLearningService) coordinateModelRetraining() {
	// This would coordinate between continuous learning and algorithm optimization
	// to ensure models are retrained when algorithm weights change significantly
	log.Println("Coordinating model retraining with algorithm optimization...")
}

// GetSystemMetrics returns comprehensive metrics from all learning components
func (rls *RealtimeLearningService) GetSystemMetrics() map[string]interface{} {
	if !rls.isRunning {
		return map[string]interface{}{"status": "not_running"}
	}

	metrics := make(map[string]interface{})

	// Feedback processing metrics
	metrics["feedback_processing"] = rls.feedbackProcessor.GetMetrics()

	// Algorithm optimization metrics
	metrics["algorithm_performance"] = rls.algorithmOptimizer.GetPerformanceMetrics()

	// A/B testing metrics would be added here
	// metrics["ab_testing"] = rls.abTestingFramework.GetMetrics()

	// System status
	metrics["status"] = "running"
	metrics["uptime"] = time.Since(time.Now()) // This would be tracked properly

	return metrics
}

// HealthCheck returns the health status of all learning components
func (rls *RealtimeLearningService) HealthCheck() map[string]string {
	health := make(map[string]string)

	if !rls.isRunning {
		health["overall"] = "not_running"
		return health
	}

	// Check each component's health
	components := []string{
		"feedback_processor",
		"algorithm_optimizer",
		"ab_testing_framework",
		"continuous_learning",
	}

	allHealthy := true
	for _, component := range components {
		// In a real implementation, each component would have a health check method
		health[component] = "healthy"
	}

	if allHealthy {
		health["overall"] = "healthy"
	} else {
		health["overall"] = "degraded"
	}

	return health
}

// CreateExperiment creates a new A/B test experiment
func (rls *RealtimeLearningService) CreateExperiment(experiment *Experiment) error {
	if !rls.isRunning {
		return fmt.Errorf("real-time learning service is not running")
	}

	return rls.abTestingFramework.CreateExperiment(experiment)
}

// StartExperiment starts an A/B test experiment
func (rls *RealtimeLearningService) StartExperiment(experimentID string) error {
	if !rls.isRunning {
		return fmt.Errorf("real-time learning service is not running")
	}

	return rls.abTestingFramework.StartExperiment(experimentID)
}

// GetExperimentResults returns the results of an A/B test experiment
func (rls *RealtimeLearningService) GetExperimentResults(experimentID string) (*Experiment, error) {
	if !rls.isRunning {
		return nil, fmt.Errorf("real-time learning service is not running")
	}

	return rls.abTestingFramework.GetExperimentResults(experimentID)
}

// UpdateUserReliability updates a user's reliability score for spam detection
func (rls *RealtimeLearningService) UpdateUserReliability(userID string, delta int) error {
	if !rls.isRunning {
		return fmt.Errorf("real-time learning service is not running")
	}

	return rls.feedbackProcessor.spamDetector.UpdateUserReliability(userID, delta)
}

// GetUserReliability gets a user's current reliability score
func (rls *RealtimeLearningService) GetUserReliability(userID string) int {
	if !rls.isRunning {
		return 50 // Default neutral score
	}

	return rls.feedbackProcessor.spamDetector.GetUserReliability(userID)
}
