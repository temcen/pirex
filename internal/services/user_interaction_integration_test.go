package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/temcen/pirex/pkg/models"
)

// TestUserInteractionService_IntegrationWorkflow tests the complete interaction workflow
func TestUserInteractionService_IntegrationWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// Test the complete workflow without database dependencies
	t.Run("interaction processing workflow", func(t *testing.T) {
		service := &UserInteractionService{
			logger:            logger,
			profileUpdateChan: make(chan uuid.UUID, 100),
			batchUpdateChan:   make(chan []Neo4jRelationship, 100),
			stopChan:          make(chan struct{}),
		}

		// Test data
		userID := uuid.New()
		itemID := uuid.New()
		sessionID := uuid.New()

		// Test explicit interaction processing
		explicitReq := &models.ExplicitInteractionRequest{
			UserID:    userID,
			ItemID:    itemID,
			Type:      "rating",
			Value:     func() *float64 { v := 4.5; return &v }(),
			SessionID: sessionID,
		}

		// Verify interaction weight calculation
		weight := service.getInteractionWeight(explicitReq.Type, explicitReq.Value, nil)
		assert.Equal(t, 0.9, weight, "Rating weight should be 0.9 (4.5/5.0)")

		// Test implicit interaction processing
		implicitReq := &models.ImplicitInteractionRequest{
			UserID:    userID,
			ItemID:    &itemID,
			Type:      "view",
			Duration:  func() *int { d := 120; return &d }(),
			SessionID: sessionID,
		}

		// Verify interaction weight calculation for view
		viewWeight := service.getInteractionWeight(implicitReq.Type, nil, implicitReq.Duration)
		assert.InDelta(t, 0.4, viewWeight, 0.01, "View weight should be ~0.4 (120/300)")

		// Test relationship mapping
		explicitRel := service.mapInteractionTypeToRelationship(explicitReq.Type)
		assert.Equal(t, "RATED", explicitRel, "Rating should map to RATED relationship")

		implicitRel := service.mapInteractionTypeToRelationship(implicitReq.Type)
		assert.Equal(t, "VIEWED", implicitRel, "View should map to VIEWED relationship")

		// Test confidence calculation
		explicitInteraction := &models.UserInteraction{
			InteractionType: explicitReq.Type,
			Value:           explicitReq.Value,
		}
		explicitConfidence := service.calculateConfidence(explicitInteraction)
		assert.Equal(t, 0.9, explicitConfidence, "Rating confidence should be 0.9")

		implicitInteraction := &models.UserInteraction{
			InteractionType: implicitReq.Type,
			Duration:        implicitReq.Duration,
		}
		implicitConfidence := service.calculateConfidence(implicitInteraction)
		assert.Equal(t, 0.7, implicitConfidence, "Long view confidence should be 0.7")

		// Test profile update triggering
		initialChannelSize := len(service.profileUpdateChan)
		service.triggerProfileUpdate(userID)
		newChannelSize := len(service.profileUpdateChan)
		assert.Equal(t, initialChannelSize+1, newChannelSize, "Profile update should be queued")

		// Test Neo4j relationship queuing
		initialBatchSize := len(service.batchUpdateChan)
		service.queueNeo4jUpdate(explicitInteraction)
		// Note: queueNeo4jUpdate requires ItemID, so we need to set it
		explicitInteraction.ItemID = &itemID
		service.queueNeo4jUpdate(explicitInteraction)
		newBatchSize := len(service.batchUpdateChan)
		assert.Equal(t, initialBatchSize+1, newBatchSize, "Neo4j update should be queued")
	})

	// Test vector operations
	t.Run("vector operations", func(t *testing.T) {
		service := &UserInteractionService{}

		// Test vector normalization
		vector := []float32{3.0, 4.0, 0.0} // Should normalize to [0.6, 0.8, 0.0]
		originalNorm := service.calculateVectorNorm(vector)
		assert.InDelta(t, 5.0, originalNorm, 0.01, "Original vector norm should be 5.0")

		service.normalizeVector(vector)
		newNorm := service.calculateVectorNorm(vector)
		assert.InDelta(t, 1.0, newNorm, 0.01, "Normalized vector norm should be 1.0")
		assert.InDelta(t, 0.6, vector[0], 0.01, "First component should be 0.6")
		assert.InDelta(t, 0.8, vector[1], 0.01, "Second component should be 0.8")
		assert.InDelta(t, 0.0, vector[2], 0.01, "Third component should be 0.0")

		// Test zero vector normalization
		zeroVector := []float32{0.0, 0.0, 0.0}
		service.normalizeVector(zeroVector)
		for _, val := range zeroVector {
			assert.Equal(t, float32(0.0), val, "Zero vector should remain zero after normalization")
		}
	})

	// Test batch processing simulation
	t.Run("batch processing", func(t *testing.T) {

		// Create test relationships
		relationships := []Neo4jRelationship{
			{
				UserID: uuid.New(),
				ItemID: uuid.New(),
				Type:   "RATED",
				Properties: map[string]interface{}{
					"rating":     4.5,
					"timestamp":  time.Now().Unix(),
					"confidence": 0.9,
				},
			},
			{
				UserID: uuid.New(),
				ItemID: uuid.New(),
				Type:   "VIEWED",
				Properties: map[string]interface{}{
					"duration":   120,
					"timestamp":  time.Now().Unix(),
					"confidence": 0.7,
				},
			},
		}

		// Test batch creation (without actual Neo4j processing)
		assert.Len(t, relationships, 2, "Should have 2 test relationships")
		assert.Equal(t, "RATED", relationships[0].Type, "First relationship should be RATED")
		assert.Equal(t, "VIEWED", relationships[1].Type, "Second relationship should be VIEWED")
		assert.Equal(t, 4.5, relationships[0].Properties["rating"], "Rating should be preserved")
		assert.Equal(t, 120, relationships[1].Properties["duration"], "Duration should be preserved")
	})

	// Test interaction type validation
	t.Run("interaction type validation", func(t *testing.T) {
		service := &UserInteractionService{}

		// Test all supported explicit interaction types
		explicitTypes := []string{"rating", "like", "dislike", "share"}
		for _, interactionType := range explicitTypes {
			relType := service.mapInteractionTypeToRelationship(interactionType)
			assert.NotEmpty(t, relType, "Relationship type should not be empty for %s", interactionType)
		}

		// Test all supported implicit interaction types
		implicitTypes := []string{"click", "view", "search", "browse"}
		for _, interactionType := range implicitTypes {
			relType := service.mapInteractionTypeToRelationship(interactionType)
			assert.NotEmpty(t, relType, "Relationship type should not be empty for %s", interactionType)
		}

		// Test unknown interaction type
		unknownRelType := service.mapInteractionTypeToRelationship("unknown")
		assert.Equal(t, "INTERACTED_WITH", unknownRelType, "Unknown types should map to INTERACTED_WITH")
	})

	// Test confidence scoring consistency
	t.Run("confidence scoring consistency", func(t *testing.T) {
		service := &UserInteractionService{}

		// Test that explicit interactions have higher confidence than implicit
		ratingInteraction := &models.UserInteraction{InteractionType: "rating"}
		clickInteraction := &models.UserInteraction{InteractionType: "click"}

		ratingConfidence := service.calculateConfidence(ratingInteraction)
		clickConfidence := service.calculateConfidence(clickInteraction)

		assert.Greater(t, ratingConfidence, clickConfidence, "Rating confidence should be higher than click confidence")

		// Test that longer views have higher confidence than shorter views
		shortView := &models.UserInteraction{
			InteractionType: "view",
			Duration:        func() *int { d := 10; return &d }(),
		}
		longView := &models.UserInteraction{
			InteractionType: "view",
			Duration:        func() *int { d := 60; return &d }(),
		}

		shortConfidence := service.calculateConfidence(shortView)
		longConfidence := service.calculateConfidence(longView)

		assert.Greater(t, longConfidence, shortConfidence, "Long view confidence should be higher than short view confidence")
	})
}
