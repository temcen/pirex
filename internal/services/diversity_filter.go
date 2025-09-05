package services

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"

	"github.com/temcen/pirex/internal/config"
	"github.com/temcen/pirex/pkg/models"
)

// DiversityFilter applies various diversity filters to recommendation lists
type DiversityFilter struct {
	db     *pgxpool.Pool
	config *config.DiversityConfig
	logger *logrus.Logger
}

// NewDiversityFilter creates a new diversity filter
func NewDiversityFilter(
	db *pgxpool.Pool,
	config *config.DiversityConfig,
	logger *logrus.Logger,
) *DiversityFilter {
	return &DiversityFilter{
		db:     db,
		config: config,
		logger: logger,
	}
}

// FilteredRecommendation represents a recommendation with diversity metadata
type FilteredRecommendation struct {
	models.Recommendation
	CategorySimilarity  float64
	EmbeddingSimilarity float64
	IsSerendipitous     bool
	TemporalPenalty     float64
}

// ApplyDiversityFilters applies all diversity filters to the recommendation list
func (df *DiversityFilter) ApplyDiversityFilters(
	ctx context.Context,
	userID uuid.UUID,
	recommendations []models.Recommendation,
) ([]models.Recommendation, error) {

	if len(recommendations) == 0 {
		return recommendations, nil
	}

	// If no database connection, return original recommendations
	if df.db == nil {
		df.logger.Debug("No database connection available for diversity filtering, returning original recommendations")
		return recommendations, nil
	}

	// Get content items for the recommendations
	contentItems, err := df.getContentItems(ctx, recommendations)
	if err != nil {
		df.logger.Warn("Failed to get content items for diversity filtering", "error", err)
		return recommendations, nil // Return original recommendations if we can't enhance them
	}

	// Get user's recent interactions for temporal filtering
	recentInteractions, err := df.getUserRecentInteractions(ctx, userID)
	if err != nil {
		df.logger.Warn("Failed to get recent interactions", "error", err)
		recentInteractions = []models.UserInteraction{} // Continue without temporal filtering
	}

	// Convert to filtered recommendations with metadata
	filteredRecs := df.prepareFilteredRecommendations(recommendations, contentItems)

	// Apply filters in sequence
	filteredRecs = df.applyIntraListDiversityFilter(filteredRecs, contentItems)
	filteredRecs = df.applyCategoryDiversityFilter(filteredRecs, contentItems)
	filteredRecs = df.applyTemporalDiversityFilter(filteredRecs, contentItems, recentInteractions)
	filteredRecs = df.applySerendipityFilter(ctx, userID, filteredRecs, contentItems)

	// Convert back to regular recommendations
	result := make([]models.Recommendation, len(filteredRecs))
	for i, rec := range filteredRecs {
		result[i] = rec.Recommendation
		result[i].Position = i + 1 // Update positions after filtering
	}

	df.logger.Debug("Applied diversity filters",
		"user_id", userID,
		"original_count", len(recommendations),
		"filtered_count", len(result),
	)

	return result, nil
}

// applyIntraListDiversityFilter implements greedy algorithm for intra-list diversity
func (df *DiversityFilter) applyIntraListDiversityFilter(
	recommendations []FilteredRecommendation,
	contentItems map[uuid.UUID]*models.ContentItem,
) []FilteredRecommendation {

	if len(recommendations) <= 1 {
		return recommendations
	}

	diversityWeight := df.config.IntraListDiversity
	maxSimilarity := df.config.MaxSimilarityThreshold

	// Start with the highest-scored item
	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].Score > recommendations[j].Score
	})

	selected := []FilteredRecommendation{recommendations[0]}
	remaining := recommendations[1:]

	// Greedy selection: iteratively add items that maximize diversity score
	for len(remaining) > 0 && len(selected) < len(recommendations) {
		bestIdx := -1
		bestScore := -1.0

		for i, candidate := range remaining {
			// Calculate average similarity to already selected items
			avgSimilarity := df.calculateAverageSimilarity(candidate, selected, contentItems)

			// Skip if too similar to existing items
			if avgSimilarity > maxSimilarity {
				df.logger.Debug("Skipping item due to high similarity",
					"item_id", candidate.ItemID,
					"avg_similarity", avgSimilarity,
					"max_threshold", maxSimilarity)
				continue
			}

			// Diversity score: relevance * (1 - diversity_weight) + (1 - avg_similarity) * diversity_weight
			diversityScore := candidate.Score*(1-diversityWeight) + (1-avgSimilarity)*diversityWeight

			df.logger.Debug("Evaluating candidate",
				"item_id", candidate.ItemID,
				"avg_similarity", avgSimilarity,
				"diversity_score", diversityScore)

			if diversityScore > bestScore {
				bestScore = diversityScore
				bestIdx = i
			}
		}

		// If no suitable candidate found, break
		if bestIdx == -1 {
			break
		}

		// Add the best candidate and remove from remaining
		selected = append(selected, remaining[bestIdx])
		remaining = append(remaining[:bestIdx], remaining[bestIdx+1:]...)
	}

	return selected
}

// applyCategoryDiversityFilter enforces maximum items per category
func (df *DiversityFilter) applyCategoryDiversityFilter(
	recommendations []FilteredRecommendation,
	contentItems map[uuid.UUID]*models.ContentItem,
) []FilteredRecommendation {

	maxPerCategory := df.config.CategoryMaxItems
	categoryCount := make(map[string]int)
	var filtered []FilteredRecommendation

	for _, rec := range recommendations {
		item := contentItems[rec.ItemID]
		if item == nil {
			continue
		}

		// Check category limits
		canAdd := true
		for _, category := range item.Categories {
			if categoryCount[category] >= maxPerCategory {
				canAdd = false
				break
			}
		}

		if canAdd {
			// Update category counts
			for _, category := range item.Categories {
				categoryCount[category]++
			}
			filtered = append(filtered, rec)
		}
	}

	return filtered
}

// applyTemporalDiversityFilter reduces recommendations similar to recent interactions
func (df *DiversityFilter) applyTemporalDiversityFilter(
	recommendations []FilteredRecommendation,
	contentItems map[uuid.UUID]*models.ContentItem,
	recentInteractions []models.UserInteraction,
) []FilteredRecommendation {

	if len(recentInteractions) == 0 {
		return recommendations
	}

	decayFactor := df.config.TemporalDecayFactor
	maxRecentSimilar := df.config.MaxRecentSimilarItems
	recentSimilarCount := 0

	// Get recent interaction items
	recentItemIDs := make(map[uuid.UUID]time.Time)
	for _, interaction := range recentInteractions {
		if interaction.ItemID != nil {
			recentItemIDs[*interaction.ItemID] = interaction.Timestamp
		}
	}

	var filtered []FilteredRecommendation

	for _, rec := range recommendations {
		item := contentItems[rec.ItemID]
		if item == nil {
			continue
		}

		// Calculate similarity penalty based on recent interactions
		maxPenalty := 0.0
		for recentItemID, interactionTime := range recentItemIDs {
			recentItem := contentItems[recentItemID]
			if recentItem == nil {
				continue
			}

			// Calculate similarity between current item and recent item
			similarity := df.calculateItemSimilarity(item, recentItem)

			// Calculate temporal penalty
			daysSince := time.Since(interactionTime).Hours() / 24
			penalty := similarity * math.Exp(-daysSince/decayFactor)

			if penalty > maxPenalty {
				maxPenalty = penalty
			}
		}

		rec.TemporalPenalty = maxPenalty

		// Apply penalty to score
		penalizedScore := rec.Score * (1 - maxPenalty)

		// Count items with high similarity to recent interactions
		if maxPenalty > 0.7 { // High similarity threshold
			recentSimilarCount++
			if recentSimilarCount > maxRecentSimilar {
				continue // Skip this item
			}
		}

		rec.Score = penalizedScore
		filtered = append(filtered, rec)
	}

	// Re-sort by penalized scores
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Score > filtered[j].Score
	})

	return filtered
}

// applySerendipityFilter injects unexpected but potentially interesting items
func (df *DiversityFilter) applySerendipityFilter(
	ctx context.Context,
	userID uuid.UUID,
	recommendations []FilteredRecommendation,
	contentItems map[uuid.UUID]*models.ContentItem,
) []FilteredRecommendation {

	serendipityRatio := df.config.SerendipityRatio
	if serendipityRatio <= 0 {
		return recommendations
	}

	totalCount := len(recommendations)
	serendipityCount := int(float64(totalCount) * serendipityRatio)

	if serendipityCount == 0 {
		return recommendations
	}

	// Get user's category familiarity
	categoryFamiliarity, err := df.getUserCategoryFamiliarity(ctx, userID)
	if err != nil {
		df.logger.Warn("Failed to get category familiarity", "error", err)
		return recommendations
	}

	// Find serendipitous items
	serendipitousItems, err := df.findSerendipitousItems(ctx, userID, serendipityCount, categoryFamiliarity)
	if err != nil {
		df.logger.Warn("Failed to find serendipitous items", "error", err)
		return recommendations
	}

	// Inject serendipitous items at strategic positions (3rd, 7th, 12th, etc.)
	positions := []int{2, 6, 11, 16, 21} // 0-indexed positions
	injected := 0

	for _, pos := range positions {
		if injected >= len(serendipitousItems) || pos >= len(recommendations) {
			break
		}

		// Create serendipitous recommendation
		serendipitousRec := FilteredRecommendation{
			Recommendation: models.Recommendation{
				ItemID:     serendipitousItems[injected].ItemID,
				Score:      serendipitousItems[injected].Score,
				Algorithm:  "serendipity",
				Confidence: serendipitousItems[injected].Confidence,
				Position:   pos + 1,
			},
			IsSerendipitous: true,
		}

		// Insert at position, shifting others down
		recommendations = append(recommendations[:pos+1], recommendations[pos:]...)
		recommendations[pos] = serendipitousRec
		injected++
	}

	return recommendations
}

// Helper methods

func (df *DiversityFilter) prepareFilteredRecommendations(
	recommendations []models.Recommendation,
	contentItems map[uuid.UUID]*models.ContentItem,
) []FilteredRecommendation {

	filtered := make([]FilteredRecommendation, len(recommendations))
	for i, rec := range recommendations {
		filtered[i] = FilteredRecommendation{
			Recommendation: rec,
		}
	}
	return filtered
}

func (df *DiversityFilter) calculateAverageSimilarity(
	candidate FilteredRecommendation,
	selected []FilteredRecommendation,
	contentItems map[uuid.UUID]*models.ContentItem,
) float64 {

	if len(selected) == 0 {
		return 0.0
	}

	candidateItem := contentItems[candidate.ItemID]
	if candidateItem == nil {
		return 0.0
	}

	totalSimilarity := 0.0
	for _, selectedRec := range selected {
		selectedItem := contentItems[selectedRec.ItemID]
		if selectedItem == nil {
			continue
		}

		similarity := df.calculateItemSimilarity(candidateItem, selectedItem)
		totalSimilarity += similarity
	}

	return totalSimilarity / float64(len(selected))
}

func (df *DiversityFilter) calculateItemSimilarity(item1, item2 *models.ContentItem) float64 {
	// Calculate category overlap (Jaccard similarity)
	categoryJaccard := df.calculateJaccardSimilarity(item1.Categories, item2.Categories)

	// Calculate embedding cosine similarity if available
	embeddingSimilarity := 0.0
	if len(item1.Embedding) > 0 && len(item2.Embedding) > 0 {
		embeddingSimilarity = df.calculateCosineSimilarity(item1.Embedding, item2.Embedding)
	}

	// Combine similarities (weight category more heavily if no embeddings)
	if embeddingSimilarity > 0 {
		return 0.3*categoryJaccard + 0.7*embeddingSimilarity
	}
	return categoryJaccard
}

func (df *DiversityFilter) calculateJaccardSimilarity(set1, set2 []string) float64 {
	if len(set1) == 0 && len(set2) == 0 {
		return 1.0
	}

	// Convert to maps for efficient lookup
	map1 := make(map[string]bool)
	for _, item := range set1 {
		map1[item] = true
	}

	map2 := make(map[string]bool)
	for _, item := range set2 {
		map2[item] = true
	}

	// Calculate intersection and union
	intersection := 0
	union := len(map1)

	for item := range map2 {
		if map1[item] {
			intersection++
		} else {
			union++
		}
	}

	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

func (df *DiversityFilter) calculateCosineSimilarity(vec1, vec2 []float32) float64 {
	if len(vec1) != len(vec2) {
		return 0.0
	}

	var dotProduct, norm1, norm2 float64

	for i := 0; i < len(vec1); i++ {
		dotProduct += float64(vec1[i]) * float64(vec2[i])
		norm1 += float64(vec1[i]) * float64(vec1[i])
		norm2 += float64(vec2[i]) * float64(vec2[i])
	}

	if norm1 == 0 || norm2 == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(norm1) * math.Sqrt(norm2))
}

// Database operations

func (df *DiversityFilter) getContentItems(
	ctx context.Context,
	recommendations []models.Recommendation,
) (map[uuid.UUID]*models.ContentItem, error) {

	if len(recommendations) == 0 {
		return make(map[uuid.UUID]*models.ContentItem), nil
	}

	// Build query with placeholders
	itemIDs := make([]interface{}, len(recommendations))
	placeholders := make([]string, len(recommendations))
	for i, rec := range recommendations {
		itemIDs[i] = rec.ItemID
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}

	query := fmt.Sprintf(`
		SELECT id, type, title, description, image_urls, metadata, categories, 
		       embedding, quality_score, active, created_at, updated_at
		FROM content_items 
		WHERE id IN (%s) AND active = true
	`, fmt.Sprintf("%s", placeholders))

	rows, err := df.db.Query(ctx, query, itemIDs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query content items: %w", err)
	}
	defer rows.Close()

	items := make(map[uuid.UUID]*models.ContentItem)
	for rows.Next() {
		var item models.ContentItem
		var description *string
		var imageURLs, categories []string
		var metadata map[string]interface{}

		err := rows.Scan(
			&item.ID, &item.Type, &item.Title, &description,
			&imageURLs, &metadata, &categories,
			&item.Embedding, &item.QualityScore, &item.Active,
			&item.CreatedAt, &item.UpdatedAt,
		)
		if err != nil {
			continue // Skip problematic rows
		}

		item.Description = description
		item.ImageURLs = imageURLs
		item.Categories = categories
		item.Metadata = metadata

		items[item.ID] = &item
	}

	return items, nil
}

func (df *DiversityFilter) getUserRecentInteractions(
	ctx context.Context,
	userID uuid.UUID,
) ([]models.UserInteraction, error) {

	// Get interactions from last 7 days
	query := `
		SELECT user_id, item_id, interaction_type, value, timestamp, session_id, context
		FROM user_interactions 
		WHERE user_id = $1 AND timestamp >= $2
		ORDER BY timestamp DESC
		LIMIT 100
	`

	sevenDaysAgo := time.Now().AddDate(0, 0, -7)
	rows, err := df.db.Query(ctx, query, userID, sevenDaysAgo)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent interactions: %w", err)
	}
	defer rows.Close()

	var interactions []models.UserInteraction
	for rows.Next() {
		var interaction models.UserInteraction
		var value *float64
		var itemID uuid.UUID
		var sessionID uuid.UUID
		var contextMap map[string]interface{}

		err := rows.Scan(
			&interaction.UserID, &itemID, &interaction.InteractionType,
			&value, &interaction.Timestamp, &sessionID, &contextMap,
		)
		if err != nil {
			continue
		}

		interaction.ItemID = &itemID
		interaction.SessionID = sessionID
		interaction.Value = value
		interaction.Context = contextMap

		interactions = append(interactions, interaction)
	}

	return interactions, nil
}

func (df *DiversityFilter) getUserCategoryFamiliarity(
	ctx context.Context,
	userID uuid.UUID,
) (map[string]float64, error) {

	query := `
		SELECT c.categories, COUNT(*) as interaction_count
		FROM user_interactions ui
		JOIN content_items c ON ui.item_id = c.id
		WHERE ui.user_id = $1 AND ui.timestamp >= $2
		GROUP BY c.categories
	`

	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	rows, err := df.db.Query(ctx, query, userID, thirtyDaysAgo)
	if err != nil {
		return nil, fmt.Errorf("failed to query category familiarity: %w", err)
	}
	defer rows.Close()

	categoryCount := make(map[string]int)
	totalInteractions := 0

	for rows.Next() {
		var categories []string
		var count int

		err := rows.Scan(&categories, &count)
		if err != nil {
			continue
		}

		for _, category := range categories {
			categoryCount[category] += count
			totalInteractions += count
		}
	}

	// Convert to familiarity scores (0-1)
	familiarity := make(map[string]float64)
	for category, count := range categoryCount {
		if totalInteractions > 0 {
			familiarity[category] = float64(count) / float64(totalInteractions)
		}
	}

	return familiarity, nil
}

func (df *DiversityFilter) findSerendipitousItems(
	ctx context.Context,
	userID uuid.UUID,
	count int,
	categoryFamiliarity map[string]float64,
) ([]models.ScoredItem, error) {

	// Find items from categories user hasn't explored much, but liked by similar users
	query := `
		WITH similar_users AS (
			SELECT DISTINCT ui2.user_id, COUNT(*) as shared_items
			FROM user_interactions ui1
			JOIN user_interactions ui2 ON ui1.item_id = ui2.item_id
			WHERE ui1.user_id = $1 AND ui2.user_id != $1
			GROUP BY ui2.user_id
			HAVING COUNT(*) >= 3
			ORDER BY shared_items DESC
			LIMIT 50
		),
		serendipitous_candidates AS (
			SELECT c.id, c.categories, AVG(ui.value) as avg_rating, COUNT(*) as user_count
			FROM content_items c
			JOIN user_interactions ui ON c.id = ui.item_id
			JOIN similar_users su ON ui.user_id = su.user_id
			WHERE ui.interaction_type IN ('rating', 'like') 
			  AND ui.value >= 4.0
			  AND c.active = true
			  AND c.id NOT IN (
				  SELECT item_id FROM user_interactions WHERE user_id = $1
			  )
			GROUP BY c.id, c.categories
			HAVING COUNT(*) >= 3
		)
		SELECT id, avg_rating, user_count, categories
		FROM serendipitous_candidates
		ORDER BY avg_rating * user_count DESC
		LIMIT $2
	`

	rows, err := df.db.Query(ctx, query, userID, count*3) // Get more candidates
	if err != nil {
		return nil, fmt.Errorf("failed to query serendipitous items: %w", err)
	}
	defer rows.Close()

	var candidates []struct {
		ItemID     uuid.UUID
		AvgRating  float64
		UserCount  int
		Categories []string
	}

	for rows.Next() {
		var candidate struct {
			ItemID     uuid.UUID
			AvgRating  float64
			UserCount  int
			Categories []string
		}

		err := rows.Scan(&candidate.ItemID, &candidate.AvgRating, &candidate.UserCount, &candidate.Categories)
		if err != nil {
			continue
		}

		candidates = append(candidates, candidate)
	}

	// Calculate unexpectedness scores and select best items
	var serendipitousItems []models.ScoredItem

	for _, candidate := range candidates {
		// Calculate category unfamiliarity (1 - familiarity)
		avgFamiliarity := 0.0
		for _, category := range candidate.Categories {
			if familiarity, exists := categoryFamiliarity[category]; exists {
				avgFamiliarity += familiarity
			}
		}
		if len(candidate.Categories) > 0 {
			avgFamiliarity /= float64(len(candidate.Categories))
		}

		categoryUnfamiliarity := 1.0 - avgFamiliarity

		// Calculate unexpectedness score: similar_user_likes * avg_rating * (1 - category_familiarity)
		unexpectednessScore := float64(candidate.UserCount) * candidate.AvgRating * categoryUnfamiliarity

		serendipitousItems = append(serendipitousItems, models.ScoredItem{
			ItemID:     candidate.ItemID,
			Score:      unexpectednessScore,
			Algorithm:  "serendipity",
			Confidence: math.Min(0.8, categoryUnfamiliarity), // Higher confidence for more unfamiliar categories
		})

		if len(serendipitousItems) >= count {
			break
		}
	}

	// Sort by unexpectedness score
	sort.Slice(serendipitousItems, func(i, j int) bool {
		return serendipitousItems[i].Score > serendipitousItems[j].Score
	})

	return serendipitousItems, nil
}
