package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

// FeedbackType represents the type of user feedback
type FeedbackType string

const (
	FeedbackExplicit FeedbackType = "explicit" // ratings, likes, dislikes
	FeedbackImplicit FeedbackType = "implicit" // clicks, views, time spent
)

// FeedbackEvent represents a user feedback event
type FeedbackEvent struct {
	UserID           string                 `json:"user_id"`
	ItemID           string                 `json:"item_id"`
	RecommendationID string                 `json:"recommendation_id,omitempty"`
	Type             FeedbackType           `json:"type"`
	Action           string                 `json:"action"` // rating, like, click, view, etc.
	Value            float64                `json:"value"`  // rating value, duration, etc.
	Timestamp        time.Time              `json:"timestamp"`
	SessionID        string                 `json:"session_id"`
	Context          map[string]interface{} `json:"context"`
	Algorithm        string                 `json:"algorithm,omitempty"`
	Position         int                    `json:"position,omitempty"`
}

// FeedbackProcessor handles real-time feedback processing
type FeedbackProcessor struct {
	db          *sql.DB
	redisClient *redis.Client
	kafkaWriter *kafka.Writer

	// Worker pools
	explicitWorkers chan FeedbackEvent
	implicitWorkers chan FeedbackEvent
	batchBuffer     []FeedbackEvent
	batchMutex      sync.Mutex

	// Configuration
	explicitWorkerCount int
	implicitWorkerCount int
	batchSize           int
	batchInterval       time.Duration

	// Feedback validation
	rateLimiter  *RateLimiter
	spamDetector *SpamDetector

	// Metrics
	processedCount    int64
	errorCount        int64
	avgProcessingTime time.Duration

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewFeedbackProcessor creates a new feedback processor
func NewFeedbackProcessor(db *sql.DB, redisClient *redis.Client, kafkaWriter *kafka.Writer) *FeedbackProcessor {
	ctx, cancel := context.WithCancel(context.Background())

	fp := &FeedbackProcessor{
		db:                  db,
		redisClient:         redisClient,
		kafkaWriter:         kafkaWriter,
		explicitWorkers:     make(chan FeedbackEvent, 100),
		implicitWorkers:     make(chan FeedbackEvent, 200),
		batchBuffer:         make([]FeedbackEvent, 0, 1000),
		explicitWorkerCount: 10,
		implicitWorkerCount: 5,
		batchSize:           100,
		batchInterval:       5 * time.Minute,
		rateLimiter:         NewRateLimiter(redisClient),
		spamDetector:        NewSpamDetector(redisClient),
		ctx:                 ctx,
		cancel:              cancel,
	}

	return fp
}

// Start initializes the feedback processor workers
func (fp *FeedbackProcessor) Start() error {
	log.Println("Starting feedback processor...")

	// Start explicit feedback workers
	for i := 0; i < fp.explicitWorkerCount; i++ {
		fp.wg.Add(1)
		go fp.explicitWorker(i)
	}

	// Start implicit feedback workers
	for i := 0; i < fp.implicitWorkerCount; i++ {
		fp.wg.Add(1)
		go fp.implicitWorker(i)
	}

	// Start batch processor
	fp.wg.Add(1)
	go fp.batchProcessor()

	log.Printf("Started %d explicit and %d implicit feedback workers",
		fp.explicitWorkerCount, fp.implicitWorkerCount)

	return nil
}

// Stop gracefully shuts down the feedback processor
func (fp *FeedbackProcessor) Stop() {
	log.Println("Stopping feedback processor...")
	fp.cancel()
	fp.wg.Wait()
	log.Println("Feedback processor stopped")
}

// ProcessFeedback processes a feedback event
func (fp *FeedbackProcessor) ProcessFeedback(event FeedbackEvent) error {
	// Validate feedback authenticity
	if err := fp.validateFeedback(event); err != nil {
		return fmt.Errorf("feedback validation failed: %w", err)
	}

	// Route to appropriate worker pool
	switch event.Type {
	case FeedbackExplicit:
		select {
		case fp.explicitWorkers <- event:
			return nil
		default:
			return fmt.Errorf("explicit feedback worker pool full")
		}
	case FeedbackImplicit:
		select {
		case fp.implicitWorkers <- event:
			return nil
		default:
			return fmt.Errorf("implicit feedback worker pool full")
		}
	default:
		return fmt.Errorf("unknown feedback type: %s", event.Type)
	}
}

// validateFeedback validates feedback authenticity
func (fp *FeedbackProcessor) validateFeedback(event FeedbackEvent) error {
	// Rate limiting check
	if !fp.rateLimiter.Allow(event.UserID, "feedback", 100, time.Minute) {
		return fmt.Errorf("rate limit exceeded for user %s", event.UserID)
	}

	// Spam detection
	if fp.spamDetector.IsSpam(event) {
		return fmt.Errorf("spam detected for user %s", event.UserID)
	}

	// Basic validation
	if event.UserID == "" || event.ItemID == "" {
		return fmt.Errorf("missing required fields")
	}

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	return nil
}

// explicitWorker processes explicit feedback events
func (fp *FeedbackProcessor) explicitWorker(workerID int) {
	defer fp.wg.Done()

	log.Printf("Starting explicit feedback worker %d", workerID)

	for {
		select {
		case event := <-fp.explicitWorkers:
			start := time.Now()

			if err := fp.processExplicitFeedback(event); err != nil {
				log.Printf("Error processing explicit feedback: %v", err)
				fp.errorCount++
			} else {
				fp.processedCount++
			}

			// Update processing time metrics
			duration := time.Since(start)
			fp.avgProcessingTime = (fp.avgProcessingTime + duration) / 2

		case <-fp.ctx.Done():
			log.Printf("Stopping explicit feedback worker %d", workerID)
			return
		}
	}
}

// implicitWorker processes implicit feedback events
func (fp *FeedbackProcessor) implicitWorker(workerID int) {
	defer fp.wg.Done()

	log.Printf("Starting implicit feedback worker %d", workerID)

	for {
		select {
		case event := <-fp.implicitWorkers:
			// Add to batch buffer for processing
			fp.addToBatch(event)

		case <-fp.ctx.Done():
			log.Printf("Stopping implicit feedback worker %d", workerID)
			return
		}
	}
}

// processExplicitFeedback processes explicit feedback immediately
func (fp *FeedbackProcessor) processExplicitFeedback(event FeedbackEvent) error {
	// Update user preference vector immediately
	if err := fp.updateUserPreferenceVector(event); err != nil {
		return fmt.Errorf("failed to update user preference vector: %w", err)
	}

	// Invalidate user-specific recommendation caches
	if err := fp.invalidateUserCaches(event.UserID); err != nil {
		log.Printf("Warning: failed to invalidate caches for user %s: %v", event.UserID, err)
	}

	// Store feedback in database
	if err := fp.storeFeedback(event); err != nil {
		return fmt.Errorf("failed to store feedback: %w", err)
	}

	// Publish to Kafka for downstream processing
	if err := fp.publishFeedbackEvent(event); err != nil {
		log.Printf("Warning: failed to publish feedback event: %v", err)
	}

	return nil
}

// addToBatch adds implicit feedback to batch buffer
func (fp *FeedbackProcessor) addToBatch(event FeedbackEvent) {
	fp.batchMutex.Lock()
	defer fp.batchMutex.Unlock()

	fp.batchBuffer = append(fp.batchBuffer, event)

	// Process batch if it reaches the size limit
	if len(fp.batchBuffer) >= fp.batchSize {
		go fp.processBatch()
	}
}

// batchProcessor processes batched implicit feedback
func (fp *FeedbackProcessor) batchProcessor() {
	defer fp.wg.Done()

	ticker := time.NewTicker(fp.batchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fp.processBatch()

		case <-fp.ctx.Done():
			// Process remaining batch before shutdown
			fp.processBatch()
			return
		}
	}
}

// processBatch processes a batch of implicit feedback
func (fp *FeedbackProcessor) processBatch() {
	fp.batchMutex.Lock()
	if len(fp.batchBuffer) == 0 {
		fp.batchMutex.Unlock()
		return
	}

	batch := make([]FeedbackEvent, len(fp.batchBuffer))
	copy(batch, fp.batchBuffer)
	fp.batchBuffer = fp.batchBuffer[:0] // Clear buffer
	fp.batchMutex.Unlock()

	log.Printf("Processing batch of %d implicit feedback events", len(batch))

	// Group by user for efficient processing
	userEvents := make(map[string][]FeedbackEvent)
	for _, event := range batch {
		userEvents[event.UserID] = append(userEvents[event.UserID], event)
	}

	// Process each user's events
	for userID, events := range userEvents {
		if err := fp.processUserBatch(userID, events); err != nil {
			log.Printf("Error processing batch for user %s: %v", userID, err)
		}
	}
}

// processUserBatch processes a batch of events for a single user
func (fp *FeedbackProcessor) processUserBatch(userID string, events []FeedbackEvent) error {
	// Aggregate implicit feedback for preference vector update
	aggregatedFeedback := fp.aggregateImplicitFeedback(events)

	// Update user preference vector with aggregated data
	if err := fp.updateUserPreferenceVectorBatch(userID, aggregatedFeedback); err != nil {
		return fmt.Errorf("failed to update user preference vector: %w", err)
	}

	// Store all events in database
	if err := fp.storeFeedbackBatch(events); err != nil {
		return fmt.Errorf("failed to store feedback batch: %w", err)
	}

	return nil
}

// updateUserPreferenceVector updates user preference vector using exponential moving average
func (fp *FeedbackProcessor) updateUserPreferenceVector(event FeedbackEvent) error {
	// Get current user preference vector
	currentVector, err := fp.getUserPreferenceVector(event.UserID)
	if err != nil {
		return err
	}

	// Get item embedding
	itemEmbedding, err := fp.getItemEmbedding(event.ItemID)
	if err != nil {
		return err
	}

	// Calculate feedback vector based on event
	feedbackVector := fp.calculateFeedbackVector(event, itemEmbedding)

	// Apply exponential moving average: new_vector = α * feedback_vector + (1-α) * old_vector
	alpha := fp.calculateAlpha(event)
	newVector := make([]float64, len(currentVector))

	for i := range newVector {
		newVector[i] = alpha*feedbackVector[i] + (1-alpha)*currentVector[i]
	}

	// Store updated preference vector
	return fp.storeUserPreferenceVector(event.UserID, newVector)
}

// calculateAlpha determines the learning rate based on feedback type and user history
func (fp *FeedbackProcessor) calculateAlpha(event FeedbackEvent) float64 {
	baseAlpha := 0.1

	switch event.Type {
	case FeedbackExplicit:
		// Higher learning rate for explicit feedback
		switch event.Action {
		case "rating":
			return baseAlpha * 2.0 * (event.Value / 5.0) // Scale by rating value
		case "like":
			return baseAlpha * 1.5
		case "dislike":
			return baseAlpha * 1.8 // Slightly higher for negative feedback
		}
	case FeedbackImplicit:
		// Lower learning rate for implicit feedback
		switch event.Action {
		case "click":
			return baseAlpha * 0.5
		case "view":
			return baseAlpha * 0.3
		case "purchase":
			return baseAlpha * 3.0 // Very high for conversions
		}
	}

	return baseAlpha
}

// invalidateUserCaches invalidates user-specific recommendation caches
func (fp *FeedbackProcessor) invalidateUserCaches(userID string) error {
	cacheKeys := []string{
		fmt.Sprintf("recommendations:user:%s", userID),
		fmt.Sprintf("user_profile:%s", userID),
		fmt.Sprintf("user_similarities:%s", userID),
	}

	for _, key := range cacheKeys {
		if err := fp.redisClient.Del(fp.ctx, key).Err(); err != nil {
			return err
		}
	}

	return nil
}

// Helper methods for database operations
func (fp *FeedbackProcessor) getUserPreferenceVector(userID string) ([]float64, error) {
	// Implementation to get user preference vector from database
	// This would query the user_profiles table
	return []float64{}, nil // Placeholder
}

func (fp *FeedbackProcessor) getItemEmbedding(itemID string) ([]float64, error) {
	// Implementation to get item embedding from database
	// This would query the content_items table
	return []float64{}, nil // Placeholder
}

func (fp *FeedbackProcessor) calculateFeedbackVector(event FeedbackEvent, itemEmbedding []float64) []float64 {
	// Calculate feedback vector based on event type and value
	feedbackVector := make([]float64, len(itemEmbedding))

	// Weight the item embedding by feedback strength
	weight := fp.getFeedbackWeight(event)
	for i, val := range itemEmbedding {
		feedbackVector[i] = val * weight
	}

	return feedbackVector
}

func (fp *FeedbackProcessor) getFeedbackWeight(event FeedbackEvent) float64 {
	switch event.Type {
	case FeedbackExplicit:
		switch event.Action {
		case "rating":
			return (event.Value - 2.5) / 2.5 // Normalize rating to [-1, 1]
		case "like":
			return 1.0
		case "dislike":
			return -1.0
		}
	case FeedbackImplicit:
		switch event.Action {
		case "click":
			return 0.3
		case "view":
			return 0.1 * (event.Value / 60.0) // Weight by view duration
		case "purchase":
			return 2.0
		}
	}

	return 0.1 // Default weight
}

func (fp *FeedbackProcessor) storeUserPreferenceVector(userID string, vector []float64) error {
	// Implementation to store updated preference vector
	// This would update the user_profiles table
	return nil // Placeholder
}

func (fp *FeedbackProcessor) storeFeedback(event FeedbackEvent) error {
	// Implementation to store feedback in database
	return nil // Placeholder
}

func (fp *FeedbackProcessor) storeFeedbackBatch(events []FeedbackEvent) error {
	// Implementation to store batch of feedback events
	return nil // Placeholder
}

func (fp *FeedbackProcessor) publishFeedbackEvent(event FeedbackEvent) error {
	// Publish to Kafka for downstream processing
	eventData, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return fp.kafkaWriter.WriteMessages(fp.ctx, kafka.Message{
		Topic: "feedback-events",
		Key:   []byte(event.UserID),
		Value: eventData,
	})
}

func (fp *FeedbackProcessor) aggregateImplicitFeedback(events []FeedbackEvent) map[string]interface{} {
	// Aggregate implicit feedback for batch processing
	aggregated := make(map[string]interface{})

	// Group by action type and aggregate
	actionCounts := make(map[string]int)
	actionValues := make(map[string]float64)

	for _, event := range events {
		actionCounts[event.Action]++
		actionValues[event.Action] += event.Value
	}

	aggregated["action_counts"] = actionCounts
	aggregated["action_values"] = actionValues
	aggregated["total_events"] = len(events)

	return aggregated
}

func (fp *FeedbackProcessor) updateUserPreferenceVectorBatch(userID string, aggregated map[string]interface{}) error {
	// Implementation for batch preference vector updates
	return nil // Placeholder
}

// GetMetrics returns processing metrics
func (fp *FeedbackProcessor) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"processed_count":     fp.processedCount,
		"error_count":         fp.errorCount,
		"avg_processing_time": fp.avgProcessingTime,
		"explicit_queue_size": len(fp.explicitWorkers),
		"implicit_queue_size": len(fp.implicitWorkers),
		"batch_buffer_size":   len(fp.batchBuffer),
	}
}
