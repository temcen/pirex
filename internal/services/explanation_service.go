package services

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"

	"github.com/temcen/pirex/pkg/models"
)

// ExplanationService generates explanations for recommendations
type ExplanationService struct {
	db     *pgxpool.Pool
	logger *logrus.Logger
}

// NewExplanationService creates a new explanation service
func NewExplanationService(db *pgxpool.Pool, logger *logrus.Logger) *ExplanationService {
	return &ExplanationService{
		db:     db,
		logger: logger,
	}
}

// ExplanationType represents different types of explanations
type ExplanationType string

const (
	ContentBasedExplanation    ExplanationType = "content_based"
	CollaborativeExplanation   ExplanationType = "collaborative"
	GraphBasedExplanation      ExplanationType = "graph_based"
	PopularityBasedExplanation ExplanationType = "popularity_based"
	SerendipityExplanation     ExplanationType = "serendipity"
	GenericExplanation         ExplanationType = "generic"
)

// ExplanationData contains data needed to generate explanations
type ExplanationData struct {
	Type              ExplanationType
	Confidence        float64
	SimilarItems      []SimilarItemInfo
	SharedUsers       []SharedUserInfo
	GraphConnections  []GraphConnectionInfo
	PopularityStats   *PopularityStats
	SerendipityReason *SerendipityReason
}

// SimilarItemInfo contains information about similar items for content-based explanations
type SimilarItemInfo struct {
	ItemID           uuid.UUID
	Title            string
	SharedCategories []string
	SimilarityScore  float64
}

// SharedUserInfo contains information about users with similar preferences
type SharedUserInfo struct {
	UserCount     int
	SharedItems   []string
	AverageRating float64
}

// GraphConnectionInfo contains information about graph-based connections
type GraphConnectionInfo struct {
	ConnectedItem   string
	ConnectionType  string
	PathDescription string
	Strength        float64
}

// PopularityStats contains popularity-based statistics
type PopularityStats struct {
	UserCount     int
	AverageRating float64
	IsTrending    bool
	TimeFrame     string
}

// SerendipityReason contains information about why an item is serendipitous
type SerendipityReason struct {
	UnexploredCategories []string
	SimilarUserLikes     int
	CategoryNovelty      float64
}

// GenerateExplanations generates explanations for a list of recommendations
func (es *ExplanationService) GenerateExplanations(
	ctx context.Context,
	userID uuid.UUID,
	recommendations []models.Recommendation,
) ([]models.Recommendation, error) {

	if len(recommendations) == 0 {
		return recommendations, nil
	}

	// If no database connection, generate generic explanations
	if es.db == nil {
		es.logger.Debug("No database connection available for explanations, using generic explanations")
		for i := range recommendations {
			genericExplanation := "Personalized recommendation based on your preferences"
			recommendations[i].Explanation = &genericExplanation
		}
		return recommendations, nil
	}

	// Process each recommendation
	for i, rec := range recommendations {
		explanation, confidence := es.generateSingleExplanation(ctx, userID, rec)

		if explanation != "" {
			recommendations[i].Explanation = &explanation
			// Update confidence if explanation provides better insight
			if confidence > rec.Confidence {
				recommendations[i].Confidence = confidence
			}
		}
	}

	return recommendations, nil
}

// generateSingleExplanation generates an explanation for a single recommendation
func (es *ExplanationService) generateSingleExplanation(
	ctx context.Context,
	userID uuid.UUID,
	recommendation models.Recommendation,
) (string, float64) {

	// Determine explanation type based on algorithm and available data
	explanationData := es.gatherExplanationData(ctx, userID, recommendation)

	// Select the best explanation type based on confidence
	bestExplanation := es.selectBestExplanation(explanationData)

	// Generate the actual explanation text
	explanationText := es.generateExplanationText(bestExplanation)

	return explanationText, bestExplanation.Confidence
}

// gatherExplanationData collects data for different types of explanations
func (es *ExplanationService) gatherExplanationData(
	ctx context.Context,
	userID uuid.UUID,
	recommendation models.Recommendation,
) []ExplanationData {

	var explanations []ExplanationData

	// Content-based explanation
	if contentData := es.getContentBasedExplanation(ctx, userID, recommendation.ItemID); contentData != nil {
		explanations = append(explanations, *contentData)
	}

	// Collaborative explanation
	if collabData := es.getCollaborativeExplanation(ctx, userID, recommendation.ItemID); collabData != nil {
		explanations = append(explanations, *collabData)
	}

	// Graph-based explanation
	if graphData := es.getGraphBasedExplanation(ctx, userID, recommendation.ItemID); graphData != nil {
		explanations = append(explanations, *graphData)
	}

	// Popularity-based explanation
	if popData := es.getPopularityBasedExplanation(ctx, recommendation.ItemID); popData != nil {
		explanations = append(explanations, *popData)
	}

	// Serendipity explanation (if marked as serendipitous)
	if recommendation.Algorithm == "serendipity" {
		if serenData := es.getSerendipityExplanation(ctx, userID, recommendation.ItemID); serenData != nil {
			explanations = append(explanations, *serenData)
		}
	}

	// Always have a generic fallback
	explanations = append(explanations, ExplanationData{
		Type:       GenericExplanation,
		Confidence: 0.1,
	})

	return explanations
}

// selectBestExplanation chooses the explanation with highest confidence
func (es *ExplanationService) selectBestExplanation(explanations []ExplanationData) ExplanationData {
	if len(explanations) == 0 {
		return ExplanationData{Type: GenericExplanation, Confidence: 0.1}
	}

	// Sort by confidence descending
	sort.Slice(explanations, func(i, j int) bool {
		return explanations[i].Confidence > explanations[j].Confidence
	})

	return explanations[0]
}

// Content-based explanation methods

func (es *ExplanationService) getContentBasedExplanation(
	ctx context.Context,
	userID uuid.UUID,
	itemID uuid.UUID,
) *ExplanationData {

	if es.db == nil {
		return nil
	}

	query := `
		WITH user_liked_items AS (
			SELECT DISTINCT ui.item_id, c.title, c.categories
			FROM user_interactions ui
			JOIN content_items c ON ui.item_id = c.id
			WHERE ui.user_id = $1 
			  AND ui.interaction_type IN ('rating', 'like')
			  AND (ui.value IS NULL OR ui.value >= 4.0)
			  AND ui.timestamp >= NOW() - INTERVAL '90 days'
		),
		target_item AS (
			SELECT title, categories
			FROM content_items
			WHERE id = $2
		)
		SELECT uli.item_id, uli.title, uli.categories, ti.categories as target_categories
		FROM user_liked_items uli, target_item ti
		WHERE uli.categories && ti.categories  -- Array overlap operator
		ORDER BY array_length(uli.categories & ti.categories, 1) DESC  -- Most shared categories first
		LIMIT 5
	`

	rows, err := es.db.Query(ctx, query, userID, itemID)
	if err != nil {
		es.logger.Warn("Failed to get content-based explanation data", "error", err)
		return nil
	}
	defer rows.Close()

	var similarItems []SimilarItemInfo
	var targetCategories []string

	for rows.Next() {
		var similarItem SimilarItemInfo
		var likedCategories, targetCats []string

		err := rows.Scan(&similarItem.ItemID, &similarItem.Title, &likedCategories, &targetCats)
		if err != nil {
			continue
		}

		if len(targetCategories) == 0 {
			targetCategories = targetCats
		}

		// Calculate shared categories
		sharedCategories := es.findSharedCategories(likedCategories, targetCategories)
		if len(sharedCategories) > 0 {
			similarItem.SharedCategories = sharedCategories
			similarItem.SimilarityScore = float64(len(sharedCategories)) / float64(len(targetCategories))
			similarItems = append(similarItems, similarItem)
		}
	}

	if len(similarItems) == 0 {
		return nil
	}

	// Calculate confidence based on number of similar items and similarity strength
	confidence := math.Min(0.9, float64(len(similarItems))*0.2+similarItems[0].SimilarityScore*0.5)

	return &ExplanationData{
		Type:         ContentBasedExplanation,
		Confidence:   confidence,
		SimilarItems: similarItems,
	}
}

// Collaborative explanation methods

func (es *ExplanationService) getCollaborativeExplanation(
	ctx context.Context,
	userID uuid.UUID,
	itemID uuid.UUID,
) *ExplanationData {

	if es.db == nil {
		return nil
	}

	query := `
		WITH similar_users AS (
			SELECT ui2.user_id, COUNT(*) as shared_items,
			       array_agg(DISTINCT c.title) as shared_item_titles
			FROM user_interactions ui1
			JOIN user_interactions ui2 ON ui1.item_id = ui2.item_id
			JOIN content_items c ON ui1.item_id = c.id
			WHERE ui1.user_id = $1 
			  AND ui2.user_id != $1
			  AND ui1.interaction_type IN ('rating', 'like')
			  AND ui2.interaction_type IN ('rating', 'like')
			  AND (ui1.value IS NULL OR ui1.value >= 4.0)
			  AND (ui2.value IS NULL OR ui2.value >= 4.0)
			GROUP BY ui2.user_id
			HAVING COUNT(*) >= 3
		),
		target_item_ratings AS (
			SELECT AVG(COALESCE(ui.value, 4.5)) as avg_rating, COUNT(*) as user_count
			FROM user_interactions ui
			JOIN similar_users su ON ui.user_id = su.user_id
			WHERE ui.item_id = $2
			  AND ui.interaction_type IN ('rating', 'like')
		)
		SELECT su.shared_items, su.shared_item_titles, tir.avg_rating, tir.user_count
		FROM similar_users su, target_item_ratings tir
		WHERE tir.user_count > 0
		ORDER BY su.shared_items DESC
		LIMIT 1
	`

	var sharedItemCount int
	var sharedItemTitles []string
	var avgRating float64
	var userCount int

	err := es.db.QueryRow(ctx, query, userID, itemID).Scan(
		&sharedItemCount, &sharedItemTitles, &avgRating, &userCount,
	)
	if err != nil {
		if err != pgx.ErrNoRows {
			es.logger.Warn("Failed to get collaborative explanation data", "error", err)
		}
		return nil
	}

	if userCount == 0 {
		return nil
	}

	// Calculate confidence based on number of similar users and shared items
	confidence := math.Min(0.9, float64(userCount)*0.1+float64(sharedItemCount)*0.05)

	return &ExplanationData{
		Type:       CollaborativeExplanation,
		Confidence: confidence,
		SharedUsers: []SharedUserInfo{{
			UserCount:     userCount,
			SharedItems:   sharedItemTitles[:int(math.Min(3, float64(len(sharedItemTitles))))], // Limit to 3 items
			AverageRating: avgRating,
		}},
	}
}

// Graph-based explanation methods

func (es *ExplanationService) getGraphBasedExplanation(
	ctx context.Context,
	userID uuid.UUID,
	itemID uuid.UUID,
) *ExplanationData {

	if es.db == nil {
		return nil
	}

	// This would typically use Neo4j, but for now we'll simulate with SQL
	query := `
		WITH user_items AS (
			SELECT DISTINCT ui.item_id
			FROM user_interactions ui
			WHERE ui.user_id = $1
			  AND ui.interaction_type IN ('rating', 'like', 'view')
			  AND (ui.value IS NULL OR ui.value >= 3.0)
		),
		connected_items AS (
			SELECT c1.title as connected_title, 'similar_category' as connection_type,
			       array_length(c1.categories & c2.categories, 1) as connection_strength
			FROM content_items c1
			JOIN user_items ui ON c1.id = ui.item_id
			JOIN content_items c2 ON c2.id = $2
			WHERE c1.categories && c2.categories
			  AND c1.id != c2.id
			ORDER BY connection_strength DESC
			LIMIT 3
		)
		SELECT connected_title, connection_type, connection_strength
		FROM connected_items
		WHERE connection_strength > 0
	`

	rows, err := es.db.Query(ctx, query, userID, itemID)
	if err != nil {
		es.logger.Warn("Failed to get graph-based explanation data", "error", err)
		return nil
	}
	defer rows.Close()

	var connections []GraphConnectionInfo

	for rows.Next() {
		var connection GraphConnectionInfo
		var strength int

		err := rows.Scan(&connection.ConnectedItem, &connection.ConnectionType, &strength)
		if err != nil {
			continue
		}

		connection.Strength = float64(strength)
		connection.PathDescription = fmt.Sprintf("shared categories with %s", connection.ConnectedItem)
		connections = append(connections, connection)
	}

	if len(connections) == 0 {
		return nil
	}

	// Calculate confidence based on connection strength
	avgStrength := 0.0
	for _, conn := range connections {
		avgStrength += conn.Strength
	}
	avgStrength /= float64(len(connections))

	confidence := math.Min(0.8, avgStrength*0.2+float64(len(connections))*0.1)

	return &ExplanationData{
		Type:             GraphBasedExplanation,
		Confidence:       confidence,
		GraphConnections: connections,
	}
}

// Popularity-based explanation methods

func (es *ExplanationService) getPopularityBasedExplanation(
	ctx context.Context,
	itemID uuid.UUID,
) *ExplanationData {

	if es.db == nil {
		return nil
	}

	query := `
		SELECT COUNT(DISTINCT ui.user_id) as user_count,
		       AVG(COALESCE(ui.value, 4.0)) as avg_rating,
		       COUNT(CASE WHEN ui.timestamp >= NOW() - INTERVAL '7 days' THEN 1 END) as recent_interactions
		FROM user_interactions ui
		WHERE ui.item_id = $1
		  AND ui.interaction_type IN ('rating', 'like', 'view')
	`

	var userCount, recentInteractions int
	var avgRating float64

	err := es.db.QueryRow(ctx, query, itemID).Scan(&userCount, &avgRating, &recentInteractions)
	if err != nil {
		es.logger.Warn("Failed to get popularity explanation data", "error", err)
		return nil
	}

	if userCount < 5 { // Not popular enough
		return nil
	}

	isTrending := recentInteractions > userCount/4 // More than 25% of interactions in last week

	// Calculate confidence based on user count and rating
	confidence := math.Min(0.8, math.Log10(float64(userCount))*0.2+avgRating*0.1)

	timeFrame := "overall"
	if isTrending {
		timeFrame = "recently"
		confidence += 0.1 // Boost confidence for trending items
	}

	return &ExplanationData{
		Type:       PopularityBasedExplanation,
		Confidence: confidence,
		PopularityStats: &PopularityStats{
			UserCount:     userCount,
			AverageRating: avgRating,
			IsTrending:    isTrending,
			TimeFrame:     timeFrame,
		},
	}
}

// Serendipity explanation methods

func (es *ExplanationService) getSerendipityExplanation(
	ctx context.Context,
	userID uuid.UUID,
	itemID uuid.UUID,
) *ExplanationData {

	if es.db == nil {
		return nil
	}

	query := `
		WITH user_categories AS (
			SELECT DISTINCT unnest(c.categories) as category, COUNT(*) as interaction_count
			FROM user_interactions ui
			JOIN content_items c ON ui.item_id = c.id
			WHERE ui.user_id = $1
			  AND ui.timestamp >= NOW() - INTERVAL '90 days'
			GROUP BY category
		),
		item_categories AS (
			SELECT unnest(categories) as category
			FROM content_items
			WHERE id = $2
		),
		unexplored_categories AS (
			SELECT ic.category
			FROM item_categories ic
			LEFT JOIN user_categories uc ON ic.category = uc.category
			WHERE uc.category IS NULL OR uc.interaction_count < 3
		),
		similar_user_likes AS (
			SELECT COUNT(DISTINCT ui.user_id) as user_count
			FROM user_interactions ui
			WHERE ui.item_id = $2
			  AND ui.interaction_type IN ('rating', 'like')
			  AND (ui.value IS NULL OR ui.value >= 4.0)
			  AND ui.user_id IN (
				  SELECT DISTINCT ui2.user_id
				  FROM user_interactions ui1
				  JOIN user_interactions ui2 ON ui1.item_id = ui2.item_id
				  WHERE ui1.user_id = $1 AND ui2.user_id != $1
			  )
		)
		SELECT array_agg(DISTINCT uc.category) as unexplored_categories, sul.user_count
		FROM unexplored_categories uc, similar_user_likes sul
		GROUP BY sul.user_count
	`

	var unexploredCategories []string
	var similarUserLikes int

	err := es.db.QueryRow(ctx, query, userID, itemID).Scan(&unexploredCategories, &similarUserLikes)
	if err != nil {
		if err != pgx.ErrNoRows {
			es.logger.Warn("Failed to get serendipity explanation data", "error", err)
		}
		return nil
	}

	if len(unexploredCategories) == 0 && similarUserLikes == 0 {
		return nil
	}

	// Calculate novelty based on unexplored categories
	categoryNovelty := float64(len(unexploredCategories)) * 0.3

	// Calculate confidence based on similar user likes and category novelty
	confidence := math.Min(0.9, float64(similarUserLikes)*0.1+categoryNovelty)

	return &ExplanationData{
		Type:       SerendipityExplanation,
		Confidence: confidence,
		SerendipityReason: &SerendipityReason{
			UnexploredCategories: unexploredCategories,
			SimilarUserLikes:     similarUserLikes,
			CategoryNovelty:      categoryNovelty,
		},
	}
}

// Text generation methods

func (es *ExplanationService) generateExplanationText(data ExplanationData) string {
	switch data.Type {
	case ContentBasedExplanation:
		return es.generateContentBasedText(data)
	case CollaborativeExplanation:
		return es.generateCollaborativeText(data)
	case GraphBasedExplanation:
		return es.generateGraphBasedText(data)
	case PopularityBasedExplanation:
		return es.generatePopularityBasedText(data)
	case SerendipityExplanation:
		return es.generateSerendipityText(data)
	default:
		return "Personalized recommendation based on your preferences"
	}
}

func (es *ExplanationService) generateContentBasedText(data ExplanationData) string {
	if len(data.SimilarItems) == 0 {
		return "Based on your preferences and similar content"
	}

	similarItem := data.SimilarItems[0]
	sharedCats := strings.Join(similarItem.SharedCategories, ", ")

	if len(data.SimilarItems) == 1 {
		return fmt.Sprintf("Because you liked \"%s\" and both are %s",
			similarItem.Title, sharedCats)
	}

	return fmt.Sprintf("Because you liked \"%s\" and %d other similar items in %s",
		similarItem.Title, len(data.SimilarItems)-1, sharedCats)
}

func (es *ExplanationService) generateCollaborativeText(data ExplanationData) string {
	if len(data.SharedUsers) == 0 {
		return "Recommended by users with similar tastes"
	}

	sharedUser := data.SharedUsers[0]

	if len(sharedUser.SharedItems) > 0 {
		sharedItemsText := strings.Join(sharedUser.SharedItems[:int(math.Min(2, float64(len(sharedUser.SharedItems))))], "\", \"")
		return fmt.Sprintf("Users who liked \"%s\" also rated this %.1f/5 (%d users)",
			sharedItemsText, sharedUser.AverageRating, sharedUser.UserCount)
	}

	return fmt.Sprintf("Highly rated by %d users with similar preferences (%.1f/5 stars)",
		sharedUser.UserCount, sharedUser.AverageRating)
}

func (es *ExplanationService) generateGraphBasedText(data ExplanationData) string {
	if len(data.GraphConnections) == 0 {
		return "Connected to items in your network"
	}

	connection := data.GraphConnections[0]
	return fmt.Sprintf("This connects to \"%s\" through %s",
		connection.ConnectedItem, connection.PathDescription)
}

func (es *ExplanationService) generatePopularityBasedText(data ExplanationData) string {
	if data.PopularityStats == nil {
		return "Popular recommendation"
	}

	stats := data.PopularityStats

	if stats.IsTrending {
		return fmt.Sprintf("Trending recently - highly rated by %d users (%.1f/5 stars)",
			stats.UserCount, stats.AverageRating)
	}

	return fmt.Sprintf("Highly rated by %d users (%.1f/5 stars)",
		stats.UserCount, stats.AverageRating)
}

func (es *ExplanationService) generateSerendipityText(data ExplanationData) string {
	if data.SerendipityReason == nil {
		return "Something new you might enjoy"
	}

	reason := data.SerendipityReason

	if len(reason.UnexploredCategories) > 0 && reason.SimilarUserLikes > 0 {
		categories := strings.Join(reason.UnexploredCategories[:int(math.Min(2, float64(len(reason.UnexploredCategories))))], ", ")
		return fmt.Sprintf("Explore %s - liked by %d users with similar tastes",
			categories, reason.SimilarUserLikes)
	}

	if len(reason.UnexploredCategories) > 0 {
		categories := strings.Join(reason.UnexploredCategories[:int(math.Min(2, float64(len(reason.UnexploredCategories))))], ", ")
		return fmt.Sprintf("Discover something new in %s", categories)
	}

	return "Something different you might enjoy"
}

// Helper methods

func (es *ExplanationService) findSharedCategories(categories1, categories2 []string) []string {
	categorySet := make(map[string]bool)
	for _, cat := range categories1 {
		categorySet[cat] = true
	}

	var shared []string
	for _, cat := range categories2 {
		if categorySet[cat] {
			shared = append(shared, cat)
		}
	}

	return shared
}
