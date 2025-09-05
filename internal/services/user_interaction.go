package services

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/sirupsen/logrus"

	"github.com/temcen/pirex/internal/config"
	"github.com/temcen/pirex/internal/database"
	"github.com/temcen/pirex/pkg/models"
)

type UserInteractionService struct {
	db                *database.Database
	logger            *logrus.Logger
	config            *config.Config
	profileUpdateChan chan uuid.UUID
	batchUpdateChan   chan []Neo4jRelationship
	stopChan          chan struct{}
	wg                sync.WaitGroup
}

type Neo4jRelationship struct {
	UserID     uuid.UUID
	ItemID     uuid.UUID
	Type       string
	Properties map[string]interface{}
}

type ProfileUpdateStats struct {
	InteractionCount int
	LastUpdate       time.Time
	PendingUpdates   int
}

func NewUserInteractionService(db *database.Database, cfg *config.Config, logger *logrus.Logger) *UserInteractionService {
	service := &UserInteractionService{
		db:                db,
		logger:            logger,
		config:            cfg,
		profileUpdateChan: make(chan uuid.UUID, 1000),
		batchUpdateChan:   make(chan []Neo4jRelationship, 100),
		stopChan:          make(chan struct{}),
	}

	// Start background workers
	service.startBackgroundWorkers()

	return service
}

func (s *UserInteractionService) startBackgroundWorkers() {
	// Profile update worker
	s.wg.Add(1)
	go s.profileUpdateWorker()

	// Neo4j batch update worker
	s.wg.Add(1)
	go s.neo4jBatchWorker()

	// Periodic sync worker (every 10 minutes)
	s.wg.Add(1)
	go s.periodicSyncWorker()
}

func (s *UserInteractionService) Stop() {
	close(s.stopChan)
	s.wg.Wait()
}

// RecordExplicitInteraction records explicit user feedback (ratings, likes, etc.)
func (s *UserInteractionService) RecordExplicitInteraction(ctx context.Context, req *models.ExplicitInteractionRequest) (*models.UserInteraction, error) {
	interaction := &models.UserInteraction{
		ID:              uuid.New(),
		UserID:          req.UserID,
		ItemID:          &req.ItemID,
		InteractionType: req.Type,
		Value:           req.Value,
		SessionID:       req.SessionID,
		Timestamp:       time.Now(),
	}

	// Store in PostgreSQL
	if err := s.storeInteraction(ctx, interaction); err != nil {
		return nil, fmt.Errorf("failed to store explicit interaction: %w", err)
	}

	// Trigger profile update
	s.triggerProfileUpdate(req.UserID)

	// Queue Neo4j relationship update
	s.queueNeo4jUpdate(interaction)

	s.logger.WithFields(logrus.Fields{
		"user_id":          req.UserID,
		"item_id":          req.ItemID,
		"interaction_type": req.Type,
		"value":            req.Value,
	}).Info("Recorded explicit interaction")

	return interaction, nil
}

// RecordImplicitInteraction records implicit user behavior (clicks, views, etc.)
func (s *UserInteractionService) RecordImplicitInteraction(ctx context.Context, req *models.ImplicitInteractionRequest) (*models.UserInteraction, error) {
	interaction := &models.UserInteraction{
		ID:              uuid.New(),
		UserID:          req.UserID,
		ItemID:          req.ItemID,
		InteractionType: req.Type,
		Duration:        req.Duration,
		Query:           req.Query,
		SessionID:       req.SessionID,
		Context:         req.Context,
		Timestamp:       time.Now(),
	}

	// Store in PostgreSQL
	if err := s.storeInteraction(ctx, interaction); err != nil {
		return nil, fmt.Errorf("failed to store implicit interaction: %w", err)
	}

	// Trigger profile update (less frequent for implicit interactions)
	if req.Type == "click" || req.Type == "view" {
		s.triggerProfileUpdate(req.UserID)
	}

	// Queue Neo4j relationship update for significant interactions
	if req.ItemID != nil && (req.Type == "click" || req.Type == "view") {
		s.queueNeo4jUpdate(interaction)
	}

	s.logger.WithFields(logrus.Fields{
		"user_id":          req.UserID,
		"item_id":          req.ItemID,
		"interaction_type": req.Type,
		"duration":         req.Duration,
	}).Debug("Recorded implicit interaction")

	return interaction, nil
}

// RecordBatchInteractions processes multiple interactions with deduplication
func (s *UserInteractionService) RecordBatchInteractions(ctx context.Context, req *models.InteractionBatchRequest) ([]models.UserInteraction, error) {
	var allInteractions []models.UserInteraction
	userUpdates := make(map[uuid.UUID]bool)

	// Process explicit interactions
	for _, explicitReq := range req.ExplicitInteractions {
		interaction, err := s.RecordExplicitInteraction(ctx, &explicitReq)
		if err != nil {
			s.logger.WithError(err).Error("Failed to record explicit interaction in batch")
			continue
		}
		allInteractions = append(allInteractions, *interaction)
		userUpdates[explicitReq.UserID] = true
	}

	// Process implicit interactions
	for _, implicitReq := range req.ImplicitInteractions {
		interaction, err := s.RecordImplicitInteraction(ctx, &implicitReq)
		if err != nil {
			s.logger.WithError(err).Error("Failed to record implicit interaction in batch")
			continue
		}
		allInteractions = append(allInteractions, *interaction)
		userUpdates[implicitReq.UserID] = true
	}

	s.logger.WithFields(logrus.Fields{
		"total_interactions": len(allInteractions),
		"affected_users":     len(userUpdates),
	}).Info("Processed batch interactions")

	return allInteractions, nil
}

// GetUserInteractions retrieves user interaction history with pagination and filtering
func (s *UserInteractionService) GetUserInteractions(ctx context.Context, userID uuid.UUID, interactionType string, limit, offset int, startDate, endDate *time.Time) ([]models.UserInteraction, int, error) {
	query := `
		SELECT id, user_id, item_id, interaction_type, value, duration, query, session_id, context, timestamp
		FROM user_interactions 
		WHERE user_id = $1`

	args := []interface{}{userID}
	argCount := 1

	// Add filters
	if interactionType != "" {
		argCount++
		query += fmt.Sprintf(" AND interaction_type = $%d", argCount)
		args = append(args, interactionType)
	}

	if startDate != nil {
		argCount++
		query += fmt.Sprintf(" AND timestamp >= $%d", argCount)
		args = append(args, *startDate)
	}

	if endDate != nil {
		argCount++
		query += fmt.Sprintf(" AND timestamp <= $%d", argCount)
		args = append(args, *endDate)
	}

	// Get total count
	countQuery := "SELECT COUNT(*) FROM (" + query + ") as filtered"
	var totalCount int
	if err := s.db.PG.QueryRow(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("failed to get interaction count: %w", err)
	}

	// Add pagination
	query += " ORDER BY timestamp DESC"
	if limit > 0 {
		argCount++
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, limit)
	}
	if offset > 0 {
		argCount++
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args = append(args, offset)
	}

	rows, err := s.db.PG.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query interactions: %w", err)
	}
	defer rows.Close()

	var interactions []models.UserInteraction
	for rows.Next() {
		var interaction models.UserInteraction
		var contextJSON []byte

		err := rows.Scan(
			&interaction.ID,
			&interaction.UserID,
			&interaction.ItemID,
			&interaction.InteractionType,
			&interaction.Value,
			&interaction.Duration,
			&interaction.Query,
			&interaction.SessionID,
			&contextJSON,
			&interaction.Timestamp,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan interaction: %w", err)
		}

		if len(contextJSON) > 0 {
			if err := json.Unmarshal(contextJSON, &interaction.Context); err != nil {
				s.logger.WithError(err).Warn("Failed to unmarshal interaction context")
			}
		}

		interactions = append(interactions, interaction)
	}

	return interactions, totalCount, nil
}

// storeInteraction stores interaction in PostgreSQL
func (s *UserInteractionService) storeInteraction(ctx context.Context, interaction *models.UserInteraction) error {
	contextJSON, _ := json.Marshal(interaction.Context)

	query := `
		INSERT INTO user_interactions (id, user_id, item_id, interaction_type, value, duration, query, session_id, context, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := s.db.PG.Exec(ctx, query,
		interaction.ID,
		interaction.UserID,
		interaction.ItemID,
		interaction.InteractionType,
		interaction.Value,
		interaction.Duration,
		interaction.Query,
		interaction.SessionID,
		contextJSON,
		interaction.Timestamp,
	)

	return err
}

// triggerProfileUpdate queues a user for profile update
func (s *UserInteractionService) triggerProfileUpdate(userID uuid.UUID) {
	select {
	case s.profileUpdateChan <- userID:
		// Successfully queued
	default:
		// Channel full, log warning
		s.logger.WithField("user_id", userID).Warn("Profile update queue full")
	}
}

// queueNeo4jUpdate queues a Neo4j relationship update
func (s *UserInteractionService) queueNeo4jUpdate(interaction *models.UserInteraction) {
	if interaction.ItemID == nil {
		return // Skip interactions without item ID
	}

	relationship := Neo4jRelationship{
		UserID: interaction.UserID,
		ItemID: *interaction.ItemID,
		Type:   s.mapInteractionTypeToRelationship(interaction.InteractionType),
		Properties: map[string]interface{}{
			"timestamp":  interaction.Timestamp.Unix(),
			"confidence": s.calculateConfidence(interaction),
		},
	}

	// Add type-specific properties
	if interaction.Value != nil {
		relationship.Properties["rating"] = *interaction.Value
	}
	if interaction.Duration != nil {
		relationship.Properties["duration"] = *interaction.Duration
	}

	select {
	case s.batchUpdateChan <- []Neo4jRelationship{relationship}:
		// Successfully queued
	default:
		// Channel full, log warning
		s.logger.WithField("user_id", interaction.UserID).Warn("Neo4j update queue full")
	}
}

// mapInteractionTypeToRelationship maps interaction types to Neo4j relationship types
func (s *UserInteractionService) mapInteractionTypeToRelationship(interactionType string) string {
	switch interactionType {
	case "rating", "like", "dislike":
		return "RATED"
	case "view", "click":
		return "VIEWED"
	case "share":
		return "SHARED"
	default:
		return "INTERACTED_WITH"
	}
}

// calculateConfidence calculates confidence score for the interaction
func (s *UserInteractionService) calculateConfidence(interaction *models.UserInteraction) float64 {
	switch interaction.InteractionType {
	case "rating":
		return 0.9 // High confidence for explicit ratings
	case "like", "dislike":
		return 0.8 // High confidence for explicit feedback
	case "share":
		return 0.85 // Very high confidence for sharing
	case "click":
		return 0.6 // Medium confidence for clicks
	case "view":
		if interaction.Duration != nil && *interaction.Duration > 30 {
			return 0.7 // Higher confidence for longer views
		}
		return 0.4 // Lower confidence for short views
	default:
		return 0.3 // Low confidence for other interactions
	}
}

// profileUpdateWorker processes profile updates in the background
func (s *UserInteractionService) profileUpdateWorker() {
	defer s.wg.Done()

	updateCounts := make(map[uuid.UUID]int)
	lastUpdate := make(map[uuid.UUID]time.Time)
	ticker := time.NewTicker(5 * time.Minute) // Update every 5 minutes
	defer ticker.Stop()

	for {
		select {
		case userID := <-s.profileUpdateChan:
			updateCounts[userID]++

			// Update immediately if 10 interactions or first interaction
			if updateCounts[userID] >= 10 || lastUpdate[userID].IsZero() {
				if err := s.updateUserProfile(context.Background(), userID); err != nil {
					s.logger.WithError(err).WithField("user_id", userID).Error("Failed to update user profile")
				} else {
					updateCounts[userID] = 0
					lastUpdate[userID] = time.Now()
				}
			}

		case <-ticker.C:
			// Periodic update for users with pending interactions
			for userID, count := range updateCounts {
				if count > 0 && time.Since(lastUpdate[userID]) > 5*time.Minute {
					if err := s.updateUserProfile(context.Background(), userID); err != nil {
						s.logger.WithError(err).WithField("user_id", userID).Error("Failed to update user profile")
					} else {
						updateCounts[userID] = 0
						lastUpdate[userID] = time.Now()
					}
				}
			}

		case <-s.stopChan:
			return
		}
	}
}

// neo4jBatchWorker processes Neo4j relationship updates in batches
func (s *UserInteractionService) neo4jBatchWorker() {
	defer s.wg.Done()

	var batch []Neo4jRelationship
	ticker := time.NewTicker(30 * time.Second) // Process batch every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case relationships := <-s.batchUpdateChan:
			batch = append(batch, relationships...)

			// Process batch if it reaches 100 items
			if len(batch) >= 100 {
				s.processBatchNeo4jUpdates(batch)
				batch = nil
			}

		case <-ticker.C:
			// Process any pending batch
			if len(batch) > 0 {
				s.processBatchNeo4jUpdates(batch)
				batch = nil
			}

		case <-s.stopChan:
			// Process final batch before stopping
			if len(batch) > 0 {
				s.processBatchNeo4jUpdates(batch)
			}
			return
		}
	}
}

// periodicSyncWorker runs periodic synchronization tasks
func (s *UserInteractionService) periodicSyncWorker() {
	defer s.wg.Done()

	ticker := time.NewTicker(10 * time.Minute) // Sync every 10 minutes
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Update user similarities
			if err := s.updateUserSimilarities(context.Background()); err != nil {
				s.logger.WithError(err).Error("Failed to update user similarities")
			}

			// Update content similarities
			if err := s.updateContentSimilarities(context.Background()); err != nil {
				s.logger.WithError(err).Error("Failed to update content similarities")
			}

		case <-s.stopChan:
			return
		}
	}
}

// updateUserProfile calculates and updates user preference vector and behavior patterns
func (s *UserInteractionService) updateUserProfile(ctx context.Context, userID uuid.UUID) error {
	// Get recent interactions (last 90 days with exponential decay)
	cutoffDate := time.Now().AddDate(0, 0, -90)

	query := `
		SELECT ui.item_id, ui.interaction_type, ui.value, ui.duration, ui.timestamp, ci.embedding, ci.categories
		FROM user_interactions ui
		LEFT JOIN content_items ci ON ui.item_id = ci.id
		WHERE ui.user_id = $1 AND ui.timestamp >= $2 AND ui.item_id IS NOT NULL AND ci.embedding IS NOT NULL
		ORDER BY ui.timestamp DESC`

	rows, err := s.db.PG.Query(ctx, query, userID, cutoffDate)
	if err != nil {
		return fmt.Errorf("failed to query user interactions: %w", err)
	}
	defer rows.Close()

	var weightedEmbeddings [][]float32
	var weights []float64
	var totalWeight float64
	categoryPrefs := make(map[string]float64)
	behaviorPatterns := make(map[string]interface{})

	interactionCount := 0
	var lastInteraction time.Time

	for rows.Next() {
		var itemID uuid.UUID
		var interactionType string
		var value *float64
		var duration *int
		var timestamp time.Time
		var embedding []float32
		var categories []string

		err := rows.Scan(&itemID, &interactionType, &value, &duration, &timestamp, &embedding, &categories)
		if err != nil {
			continue // Skip invalid rows
		}

		interactionCount++
		if timestamp.After(lastInteraction) {
			lastInteraction = timestamp
		}

		// Calculate weight with exponential decay (30-day half-life)
		daysSince := time.Since(timestamp).Hours() / 24
		decayFactor := math.Exp(-daysSince * math.Ln2 / 30) // 30-day half-life

		// Base weight from interaction type and value
		baseWeight := s.getInteractionWeight(interactionType, value, duration)
		weight := baseWeight * decayFactor

		if weight > 0.01 { // Only include significant weights
			weightedEmbeddings = append(weightedEmbeddings, embedding)
			weights = append(weights, weight)
			totalWeight += weight

			// Update category preferences
			for _, category := range categories {
				categoryPrefs[category] += weight
			}
		}
	}

	// Calculate preference vector as weighted average
	var preferenceVector []float32
	if len(weightedEmbeddings) > 0 && totalWeight > 0 {
		preferenceVector = make([]float32, len(weightedEmbeddings[0]))

		for i, embedding := range weightedEmbeddings {
			weight := float32(weights[i] / totalWeight)
			for j, val := range embedding {
				preferenceVector[j] += val * weight
			}
		}

		// L2 normalize the preference vector
		s.normalizeVector(preferenceVector)
	} else {
		// Cold start: zero vector
		preferenceVector = make([]float32, 768)
	}

	// Calculate behavior patterns
	behaviorPatterns["interaction_frequency"] = s.calculateInteractionFrequency(ctx, userID)
	behaviorPatterns["time_patterns"] = s.calculateTimePatterns(ctx, userID)
	behaviorPatterns["session_duration"] = s.calculateAvgSessionDuration(ctx, userID)
	behaviorPatterns["category_preferences"] = categoryPrefs

	// Update user profile in PostgreSQL
	return s.updateUserProfileInDB(ctx, userID, preferenceVector, behaviorPatterns, interactionCount, lastInteraction)
}

// getInteractionWeight calculates weight for different interaction types
func (s *UserInteractionService) getInteractionWeight(interactionType string, value *float64, duration *int) float64 {
	switch interactionType {
	case "rating":
		if value != nil {
			return *value / 5.0 // Normalize rating to 0-1
		}
		return 0.5
	case "like":
		return 0.8
	case "dislike":
		return -0.8 // Negative weight for dislikes
	case "share":
		return 0.9
	case "click":
		return 0.6
	case "view":
		if duration != nil {
			// Weight based on view duration (max 5 minutes = weight 1.0)
			return math.Min(float64(*duration)/300.0, 1.0)
		}
		return 0.4
	default:
		return 0.3
	}
}

// normalizeVector performs L2 normalization
func (s *UserInteractionService) normalizeVector(vector []float32) {
	var norm float64
	for _, val := range vector {
		norm += float64(val * val)
	}
	norm = math.Sqrt(norm)

	if norm > 0 {
		for i := range vector {
			vector[i] = float32(float64(vector[i]) / norm)
		}
	}
}

// calculateInteractionFrequency calculates user's interaction frequency
func (s *UserInteractionService) calculateInteractionFrequency(ctx context.Context, userID uuid.UUID) map[string]float64 {
	query := `
		SELECT 
			DATE_TRUNC('day', timestamp) as day,
			COUNT(*) as count
		FROM user_interactions 
		WHERE user_id = $1 AND timestamp >= NOW() - INTERVAL '30 days'
		GROUP BY DATE_TRUNC('day', timestamp)
		ORDER BY day`

	rows, err := s.db.PG.Query(ctx, query, userID)
	if err != nil {
		return map[string]float64{"daily_avg": 0}
	}
	defer rows.Close()

	var totalInteractions int
	var days int
	for rows.Next() {
		var day time.Time
		var count int
		if err := rows.Scan(&day, &count); err == nil {
			totalInteractions += count
			days++
		}
	}

	dailyAvg := 0.0
	if days > 0 {
		dailyAvg = float64(totalInteractions) / float64(days)
	}

	return map[string]float64{
		"daily_avg":       dailyAvg,
		"total_last_30d":  float64(totalInteractions),
		"active_days_30d": float64(days),
	}
}

// calculateTimePatterns analyzes user's time-of-day interaction patterns
func (s *UserInteractionService) calculateTimePatterns(ctx context.Context, userID uuid.UUID) map[string]float64 {
	query := `
		SELECT 
			EXTRACT(hour FROM timestamp) as hour,
			COUNT(*) as count
		FROM user_interactions 
		WHERE user_id = $1 AND timestamp >= NOW() - INTERVAL '30 days'
		GROUP BY EXTRACT(hour FROM timestamp)
		ORDER BY hour`

	rows, err := s.db.PG.Query(ctx, query, userID)
	if err != nil {
		return map[string]float64{}
	}
	defer rows.Close()

	hourlyPatterns := make(map[string]float64)
	var totalInteractions int

	for rows.Next() {
		var hour int
		var count int
		if err := rows.Scan(&hour, &count); err == nil {
			hourlyPatterns[fmt.Sprintf("hour_%d", hour)] = float64(count)
			totalInteractions += count
		}
	}

	// Normalize to percentages
	if totalInteractions > 0 {
		for hour, count := range hourlyPatterns {
			hourlyPatterns[hour] = count / float64(totalInteractions)
		}
	}

	return hourlyPatterns
}

// calculateAvgSessionDuration calculates average session duration
func (s *UserInteractionService) calculateAvgSessionDuration(ctx context.Context, userID uuid.UUID) float64 {
	query := `
		SELECT 
			session_id,
			MIN(timestamp) as session_start,
			MAX(timestamp) as session_end
		FROM user_interactions 
		WHERE user_id = $1 AND timestamp >= NOW() - INTERVAL '30 days'
		GROUP BY session_id
		HAVING COUNT(*) > 1`

	rows, err := s.db.PG.Query(ctx, query, userID)
	if err != nil {
		return 0
	}
	defer rows.Close()

	var totalDuration time.Duration
	var sessionCount int

	for rows.Next() {
		var sessionID uuid.UUID
		var sessionStart, sessionEnd time.Time
		if err := rows.Scan(&sessionID, &sessionStart, &sessionEnd); err == nil {
			duration := sessionEnd.Sub(sessionStart)
			if duration > 0 && duration < 24*time.Hour { // Reasonable session duration
				totalDuration += duration
				sessionCount++
			}
		}
	}

	if sessionCount > 0 {
		return totalDuration.Seconds() / float64(sessionCount)
	}
	return 0
}

// updateUserProfileInDB updates user profile in PostgreSQL
func (s *UserInteractionService) updateUserProfileInDB(ctx context.Context, userID uuid.UUID, preferenceVector []float32, behaviorPatterns map[string]interface{}, interactionCount int, lastInteraction time.Time) error {
	behaviorJSON, _ := json.Marshal(behaviorPatterns)

	query := `
		INSERT INTO user_profiles (user_id, preference_vector, behavior_patterns, interaction_count, last_interaction, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		ON CONFLICT (user_id) 
		DO UPDATE SET 
			preference_vector = EXCLUDED.preference_vector,
			behavior_patterns = EXCLUDED.behavior_patterns,
			interaction_count = EXCLUDED.interaction_count,
			last_interaction = EXCLUDED.last_interaction,
			updated_at = NOW()`

	_, err := s.db.PG.Exec(ctx, query, userID, preferenceVector, behaviorJSON, interactionCount, lastInteraction)
	if err != nil {
		return fmt.Errorf("failed to update user profile: %w", err)
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("user_profile:%s", userID.String())
	s.db.Redis.Hot.Del(ctx, cacheKey)

	s.logger.WithFields(logrus.Fields{
		"user_id":           userID,
		"interaction_count": interactionCount,
		"vector_norm":       s.calculateVectorNorm(preferenceVector),
	}).Debug("Updated user profile")

	return nil
}

// calculateVectorNorm calculates the L2 norm of a vector
func (s *UserInteractionService) calculateVectorNorm(vector []float32) float64 {
	var sum float64
	for _, val := range vector {
		sum += float64(val * val)
	}
	return math.Sqrt(sum)
}

// processBatchNeo4jUpdates processes a batch of Neo4j relationship updates
func (s *UserInteractionService) processBatchNeo4jUpdates(batch []Neo4jRelationship) {
	ctx := context.Background()
	session := s.db.Neo4j.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	// Create relationships in batch
	cypher := `
		UNWIND $relationships AS rel
		MERGE (u:User {id: rel.user_id})
		MERGE (c:Content {id: rel.item_id})
		MERGE (u)-[r:` + "`" + `rel.type` + "`" + `]->(c)
		SET r += rel.properties
		SET r.updated_at = datetime()`

	relationships := make([]map[string]interface{}, len(batch))
	for i, rel := range batch {
		relationships[i] = map[string]interface{}{
			"user_id":    rel.UserID.String(),
			"item_id":    rel.ItemID.String(),
			"type":       rel.Type,
			"properties": rel.Properties,
		}
	}

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		result, err := tx.Run(ctx, cypher, map[string]interface{}{
			"relationships": relationships,
		})
		if err != nil {
			return nil, err
		}

		summary, err := result.Consume(ctx)
		if err != nil {
			return nil, err
		}

		return summary.Counters(), nil
	})

	if err != nil {
		s.logger.WithError(err).WithField("batch_size", len(batch)).Error("Failed to process Neo4j batch update")
	} else {
		s.logger.WithField("batch_size", len(batch)).Debug("Processed Neo4j batch update")
	}
}

// updateUserSimilarities calculates and stores user similarity relationships
func (s *UserInteractionService) updateUserSimilarities(ctx context.Context) error {
	session := s.db.Neo4j.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	// Calculate user similarities based on shared ratings (Pearson correlation)
	cypher := `
		MATCH (u1:User)-[r1:RATED]->(item:Content)<-[r2:RATED]-(u2:User)
		WHERE u1.id < u2.id AND r1.rating IS NOT NULL AND r2.rating IS NOT NULL
		WITH u1, u2, 
			 COUNT(item) as shared_items,
			 COLLECT({r1: r1.rating, r2: r2.rating}) as ratings
		WHERE shared_items >= 3
		WITH u1, u2, shared_items,
			 REDUCE(sum = 0.0, rating IN ratings | sum + rating.r1 * rating.r2) as dot_product,
			 REDUCE(sum1 = 0.0, rating IN ratings | sum1 + rating.r1 * rating.r1) as sum_sq1,
			 REDUCE(sum2 = 0.0, rating IN ratings | sum2 + rating.r2 * rating.r2) as sum_sq2,
			 REDUCE(sum1 = 0.0, rating IN ratings | sum1 + rating.r1) as sum1,
			 REDUCE(sum2 = 0.0, rating IN ratings | sum2 + rating.r2) as sum2
		WITH u1, u2, shared_items,
			 (dot_product - (sum1 * sum2 / shared_items)) / 
			 SQRT((sum_sq1 - (sum1 * sum1 / shared_items)) * (sum_sq2 - (sum2 * sum2 / shared_items))) as correlation
		WHERE correlation > 0.5
		MERGE (u1)-[s:SIMILAR_TO]-(u2)
		SET s.score = correlation,
			s.basis = 'collaborative_filtering',
			s.shared_items = shared_items,
			s.computed_at = datetime()
		RETURN COUNT(s) as similarities_created`

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		result, err := tx.Run(ctx, cypher, nil)
		if err != nil {
			return nil, err
		}

		record, err := result.Single(ctx)
		if err != nil {
			return nil, err
		}

		count, _ := record.Get("similarities_created")
		s.logger.WithField("similarities_created", count).Info("Updated user similarities")
		return count, nil
	})

	return err
}

// updateContentSimilarities calculates and stores content similarity relationships
func (s *UserInteractionService) updateContentSimilarities(ctx context.Context) error {
	session := s.db.Neo4j.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	// Calculate content similarities based on users who interacted with both (Jaccard similarity)
	cypher := `
		MATCH (c1:Content)<-[:RATED|VIEWED|INTERACTED_WITH]-(user:User)-[:RATED|VIEWED|INTERACTED_WITH]->(c2:Content)
		WHERE c1.id < c2.id
		WITH c1, c2, COUNT(DISTINCT user) as shared_users
		WHERE shared_users >= 3
		MATCH (c1)<-[:RATED|VIEWED|INTERACTED_WITH]-(u1:User)
		WITH c1, c2, shared_users, COUNT(DISTINCT u1) as users_c1
		MATCH (c2)<-[:RATED|VIEWED|INTERACTED_WITH]-(u2:User)
		WITH c1, c2, shared_users, users_c1, COUNT(DISTINCT u2) as users_c2
		WITH c1, c2, shared_users, users_c1, users_c2,
			 toFloat(shared_users) / (users_c1 + users_c2 - shared_users) as jaccard_similarity
		WHERE jaccard_similarity > 0.1
		MERGE (c1)-[s:SIMILAR_TO]-(c2)
		SET s.score = jaccard_similarity,
			s.algorithm = 'jaccard_similarity',
			s.shared_users = shared_users,
			s.computed_at = datetime()
		RETURN COUNT(s) as similarities_created`

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		result, err := tx.Run(ctx, cypher, nil)
		if err != nil {
			return nil, err
		}

		record, err := result.Single(ctx)
		if err != nil {
			return nil, err
		}

		count, _ := record.Get("similarities_created")
		s.logger.WithField("similarities_created", count).Info("Updated content similarities")
		return count, nil
	})

	return err
}

// GetUserProfile retrieves user profile with caching
func (s *UserInteractionService) GetUserProfile(ctx context.Context, userID uuid.UUID) (*models.UserProfile, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("user_profile:%s", userID.String())
	cached, err := s.db.Redis.Hot.Get(ctx, cacheKey).Result()
	if err == nil {
		var profile models.UserProfile
		if json.Unmarshal([]byte(cached), &profile) == nil {
			return &profile, nil
		}
	}

	// Query from database
	query := `
		SELECT user_id, preference_vector, explicit_preferences, behavior_patterns, 
			   demographics, interaction_count, last_interaction, created_at, updated_at
		FROM user_profiles 
		WHERE user_id = $1`

	var profile models.UserProfile
	var explicitPrefsJSON, behaviorPatternsJSON, demographicsJSON []byte

	err = s.db.PG.QueryRow(ctx, query, userID).Scan(
		&profile.UserID,
		&profile.PreferenceVector,
		&explicitPrefsJSON,
		&behaviorPatternsJSON,
		&demographicsJSON,
		&profile.InteractionCount,
		&profile.LastInteraction,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			// Create new profile for new user
			profile = models.UserProfile{
				UserID:           userID,
				PreferenceVector: make([]float32, 768), // Zero vector for cold start
				ExplicitPrefs:    make(map[string]interface{}),
				BehaviorPatterns: make(map[string]interface{}),
				Demographics:     make(map[string]interface{}),
				InteractionCount: 0,
				CreatedAt:        time.Now(),
				UpdatedAt:        time.Now(),
			}

			// Insert new profile
			if err := s.createUserProfile(ctx, &profile); err != nil {
				return nil, fmt.Errorf("failed to create user profile: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to query user profile: %w", err)
		}
	} else {
		// Unmarshal JSON fields
		if len(explicitPrefsJSON) > 0 {
			json.Unmarshal(explicitPrefsJSON, &profile.ExplicitPrefs)
		}
		if len(behaviorPatternsJSON) > 0 {
			json.Unmarshal(behaviorPatternsJSON, &profile.BehaviorPatterns)
		}
		if len(demographicsJSON) > 0 {
			json.Unmarshal(demographicsJSON, &profile.Demographics)
		}
	}

	// Cache the profile
	profileJSON, _ := json.Marshal(profile)
	s.db.Redis.Hot.Set(ctx, cacheKey, profileJSON, time.Hour)

	return &profile, nil
}

// createUserProfile creates a new user profile
func (s *UserInteractionService) createUserProfile(ctx context.Context, profile *models.UserProfile) error {
	explicitPrefsJSON, _ := json.Marshal(profile.ExplicitPrefs)
	behaviorPatternsJSON, _ := json.Marshal(profile.BehaviorPatterns)
	demographicsJSON, _ := json.Marshal(profile.Demographics)

	query := `
		INSERT INTO user_profiles (user_id, preference_vector, explicit_preferences, behavior_patterns, demographics, interaction_count, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := s.db.PG.Exec(ctx, query,
		profile.UserID,
		profile.PreferenceVector,
		explicitPrefsJSON,
		behaviorPatternsJSON,
		demographicsJSON,
		profile.InteractionCount,
		profile.CreatedAt,
		profile.UpdatedAt,
	)

	return err
}

// GetSimilarUsers finds similar users using Neo4j
func (s *UserInteractionService) GetSimilarUsers(ctx context.Context, userID uuid.UUID, limit int) ([]models.SimilarUser, error) {
	session := s.db.Neo4j.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	cypher := `
		MATCH (u:User {id: $user_id})-[s:SIMILAR_TO]-(similar:User)
		RETURN similar.id as user_id, s.score as similarity_score, s.basis as basis, s.shared_items as shared_items
		ORDER BY s.score DESC
		LIMIT $limit`

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		result, err := tx.Run(ctx, cypher, map[string]interface{}{
			"user_id": userID.String(),
			"limit":   limit,
		})
		if err != nil {
			return nil, err
		}

		var similarUsers []models.SimilarUser
		for result.Next(ctx) {
			record := result.Record()
			userIDStr, _ := record.Get("user_id")
			score, _ := record.Get("similarity_score")
			basis, _ := record.Get("basis")
			sharedItems, _ := record.Get("shared_items")

			if userUUID, err := uuid.Parse(userIDStr.(string)); err == nil {
				similarUsers = append(similarUsers, models.SimilarUser{
					UserID:          userUUID,
					SimilarityScore: score.(float64),
					Basis:           basis.(string),
					SharedItems:     int(sharedItems.(int64)),
				})
			}
		}

		return similarUsers, result.Err()
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get similar users: %w", err)
	}

	return result.([]models.SimilarUser), nil
}

// ValidateDataConsistency checks consistency between PostgreSQL and Neo4j
func (s *UserInteractionService) ValidateDataConsistency(ctx context.Context) error {
	// Check if user profiles in PostgreSQL have corresponding nodes in Neo4j
	query := `SELECT user_id FROM user_profiles WHERE interaction_count > 0`
	rows, err := s.db.PG.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query user profiles: %w", err)
	}
	defer rows.Close()

	var inconsistencies []string
	session := s.db.Neo4j.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			continue
		}

		// Check if user exists in Neo4j
		cypher := `MATCH (u:User {id: $user_id}) RETURN COUNT(u) as count`
		result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
			result, err := tx.Run(ctx, cypher, map[string]interface{}{
				"user_id": userID.String(),
			})
			if err != nil {
				return 0, err
			}

			record, err := result.Single(ctx)
			if err != nil {
				return 0, err
			}

			count, _ := record.Get("count")
			return count.(int64), nil
		})

		if err != nil || result.(int64) == 0 {
			inconsistencies = append(inconsistencies, fmt.Sprintf("User %s missing in Neo4j", userID))
		}
	}

	if len(inconsistencies) > 0 {
		s.logger.WithField("inconsistencies", inconsistencies).Warn("Data consistency issues found")
		return fmt.Errorf("found %d data consistency issues", len(inconsistencies))
	}

	return nil
}
