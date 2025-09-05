package services

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/temcen/pirex/pkg/models"
)

// LoadTestUserInteractionService tests concurrent interaction processing
func TestUserInteractionService_ConcurrentInteractions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise

	// Test concurrent profile updates
	t.Run("concurrent profile updates", func(t *testing.T) {
		service := &UserInteractionService{
			logger:            logger,
			profileUpdateChan: make(chan uuid.UUID, 1000),
			stopChan:          make(chan struct{}),
		}

		// Simulate concurrent profile update requests
		numGoroutines := 100
		numUpdatesPerGoroutine := 10
		var wg sync.WaitGroup

		start := time.Now()

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < numUpdatesPerGoroutine; j++ {
					userID := uuid.New()
					service.triggerProfileUpdate(userID)
				}
			}()
		}

		wg.Wait()
		duration := time.Since(start)

		// Should handle 1000 profile updates quickly
		assert.Less(t, duration, 100*time.Millisecond, "Profile update queuing should be fast")

		// Verify channel has expected number of items (up to buffer size)
		channelSize := len(service.profileUpdateChan)
		assert.LessOrEqual(t, channelSize, 1000, "Channel should not overflow")
		assert.Greater(t, channelSize, 0, "Channel should have some items")
	})

	// Test vector operations performance
	t.Run("vector normalization performance", func(t *testing.T) {
		service := &UserInteractionService{}

		// Test with typical embedding dimension
		vector := make([]float32, 768)
		for i := range vector {
			vector[i] = float32(i % 100) // Some test values
		}

		numOperations := 10000
		start := time.Now()

		for i := 0; i < numOperations; i++ {
			testVector := make([]float32, len(vector))
			copy(testVector, vector)
			service.normalizeVector(testVector)
		}

		duration := time.Since(start)
		avgDuration := duration / time.Duration(numOperations)

		// Should normalize vectors quickly (< 1ms per operation)
		assert.Less(t, avgDuration, time.Millisecond, "Vector normalization should be fast")

		t.Logf("Vector normalization: %d operations in %v (avg: %v per operation)",
			numOperations, duration, avgDuration)
	})

	// Test interaction weight calculation performance
	t.Run("interaction weight calculation performance", func(t *testing.T) {
		service := &UserInteractionService{}

		interactions := []struct {
			interactionType string
			value           *float64
			duration        *int
		}{
			{"rating", func() *float64 { v := 4.5; return &v }(), nil},
			{"like", nil, nil},
			{"view", nil, func() *int { d := 120; return &d }()},
			{"click", nil, nil},
		}

		numOperations := 100000
		start := time.Now()

		for i := 0; i < numOperations; i++ {
			interaction := interactions[i%len(interactions)]
			service.getInteractionWeight(interaction.interactionType, interaction.value, interaction.duration)
		}

		duration := time.Since(start)
		avgDuration := duration / time.Duration(numOperations)

		// Should calculate weights very quickly (< 1μs per operation)
		assert.Less(t, avgDuration, time.Microsecond, "Weight calculation should be very fast")

		t.Logf("Weight calculation: %d operations in %v (avg: %v per operation)",
			numOperations, duration, avgDuration)
	})

	// Test confidence calculation performance
	t.Run("confidence calculation performance", func(t *testing.T) {
		service := &UserInteractionService{}

		interactions := []*models.UserInteraction{
			{InteractionType: "rating"},
			{InteractionType: "like"},
			{InteractionType: "view", Duration: func() *int { d := 60; return &d }()},
			{InteractionType: "click"},
		}

		numOperations := 100000
		start := time.Now()

		for i := 0; i < numOperations; i++ {
			interaction := interactions[i%len(interactions)]
			service.calculateConfidence(interaction)
		}

		duration := time.Since(start)
		avgDuration := duration / time.Duration(numOperations)

		// Should calculate confidence very quickly (< 1μs per operation)
		assert.Less(t, avgDuration, time.Microsecond, "Confidence calculation should be very fast")

		t.Logf("Confidence calculation: %d operations in %v (avg: %v per operation)",
			numOperations, duration, avgDuration)
	})
}

// BenchmarkUserInteractionService_ProfileUpdate benchmarks profile update operations
func BenchmarkUserInteractionService_ProfileUpdate(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	service := &UserInteractionService{
		logger:            logger,
		profileUpdateChan: make(chan uuid.UUID, 10000),
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			userID := uuid.New()
			service.triggerProfileUpdate(userID)
		}
	})
}

// BenchmarkUserInteractionService_VectorOperations benchmarks vector operations
func BenchmarkUserInteractionService_VectorOperations(b *testing.B) {
	service := &UserInteractionService{}
	vector := make([]float32, 768)

	// Initialize with test values
	for i := range vector {
		vector[i] = float32(i % 100)
	}

	b.Run("normalize_vector", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			testVector := make([]float32, len(vector))
			copy(testVector, vector)
			service.normalizeVector(testVector)
		}
	})

	b.Run("calculate_vector_norm", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			service.calculateVectorNorm(vector)
		}
	})
}

// BenchmarkUserInteractionService_UtilityFunctions benchmarks utility functions
func BenchmarkUserInteractionService_UtilityFunctions(b *testing.B) {
	service := &UserInteractionService{}

	b.Run("get_interaction_weight", func(b *testing.B) {
		value := 4.5
		duration := 120
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			service.getInteractionWeight("rating", &value, &duration)
		}
	})

	b.Run("calculate_confidence", func(b *testing.B) {
		duration := 60
		interaction := &models.UserInteraction{
			InteractionType: "view",
			Duration:        &duration,
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			service.calculateConfidence(interaction)
		}
	})

	b.Run("map_interaction_type", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			service.mapInteractionTypeToRelationship("rating")
		}
	})
}
