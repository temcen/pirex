package services

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"github.com/temcen/pirex/internal/config"
	"github.com/temcen/pirex/pkg/models"
)

// DatabaseQuerier interface for database operations
type DatabaseQuerier interface {
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
}

// RecommendationAlgorithmsService implements core recommendation algorithms
type RecommendationAlgorithmsService struct {
	db     DatabaseQuerier
	neo4j  neo4j.DriverWithContext
	redis  *redis.Client // warm cache
	config *config.AlgorithmConfig
	logger *logrus.Logger
}

// NewRecommendationAlgorithmsService creates a new recommendation algorithms service
func NewRecommendationAlgorithmsService(
	db DatabaseQuerier,
	neo4j neo4j.DriverWithContext,
	redis *redis.Client,
	config *config.AlgorithmConfig,
	logger *logrus.Logger,
) *RecommendationAlgorithmsService {
	return &RecommendationAlgorithmsService{
		db:     db,
		neo4j:  neo4j,
		redis:  redis,
		config: config,
		logger: logger,
	}
}

// SemanticSearchRecommendations generates recommendations using semantic search with pgvector
func (s *RecommendationAlgorithmsService) SemanticSearchRecommendations(
	ctx context.Context,
	userID uuid.UUID,
	userEmbedding []float32,
	contentTypes []string,
	categories []string,
	limit int,
) ([]models.ScoredItem, error) {
	if !s.config.SemanticSearch.Enabled {
		return nil, nil
	}

	// Create cache key for frequent queries
	cacheKey := fmt.Sprintf("semantic_search:%s:%v:%v:%d",
		userID.String(), contentTypes, categories, limit)

	// Try cache first
	if cached, err := s.getCachedResults(ctx, cacheKey); err == nil && cached != nil {
		s.logger.Debug("Semantic search cache hit", "user_id", userID)
		return cached, nil
	}

	// Build query with metadata filtering
	query := `
		SELECT 
			id as item_id,
			1 - (embedding <=> $1) as similarity
		FROM content_items 
		WHERE active = true
			AND quality_score > 0.5
			AND 1 - (embedding <=> $1) >= $2`

	args := []interface{}{userEmbedding, s.config.SemanticSearch.SimilarityThreshold}
	argIndex := 3

	// Add content type filtering
	if len(contentTypes) > 0 {
		query += fmt.Sprintf(" AND type = ANY($%d)", argIndex)
		args = append(args, contentTypes)
		argIndex++
	}

	// Add category filtering
	if len(categories) > 0 {
		query += fmt.Sprintf(" AND categories && $%d", argIndex)
		args = append(args, categories)
		argIndex++
	}

	// Exclude items user has already interacted with
	query += fmt.Sprintf(`
		AND id NOT IN (
			SELECT DISTINCT item_id 
			FROM user_interactions 
			WHERE user_id = $%d 
				AND item_id IS NOT NULL
				AND interaction_type IN ('rating', 'like', 'dislike')
		)`, argIndex)
	args = append(args, userID)

	query += fmt.Sprintf(" ORDER BY embedding <=> $1 LIMIT $%d", argIndex+1)
	args = append(args, limit)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("semantic search query failed: %w", err)
	}
	defer rows.Close()

	var results []models.ScoredItem
	for rows.Next() {
		var itemID uuid.UUID
		var similarity float64

		if err := rows.Scan(&itemID, &similarity); err != nil {
			s.logger.Error("Failed to scan semantic search result", "error", err)
			continue
		}

		results = append(results, models.ScoredItem{
			ItemID:     itemID,
			Score:      similarity,
			Algorithm:  "semantic_search",
			Confidence: s.calculateSemanticConfidence(similarity),
		})
	}

	// Cache results for 30 minutes
	if err := s.cacheResults(ctx, cacheKey, results, 30*time.Minute); err != nil {
		s.logger.Warn("Failed to cache semantic search results", "error", err)
	}

	s.logger.Debug("Semantic search completed",
		"user_id", userID, "results", len(results))

	return results, nil
}

// CollaborativeFilteringRecommendations generates recommendations using collaborative filtering
func (s *RecommendationAlgorithmsService) CollaborativeFilteringRecommendations(
	ctx context.Context,
	userID uuid.UUID,
	limit int,
) ([]models.ScoredItem, error) {
	if !s.config.CollaborativeFilter.Enabled {
		return nil, nil
	}

	cacheKey := fmt.Sprintf("collaborative_filtering:%s:%d", userID.String(), limit)

	// Try cache first
	if cached, err := s.getCachedResults(ctx, cacheKey); err == nil && cached != nil {
		s.logger.Debug("Collaborative filtering cache hit", "user_id", userID)
		return cached, nil
	}

	// Find similar users using Pearson correlation in Neo4j
	similarUsers, err := s.findSimilarUsers(ctx, userID, 50)
	if err != nil {
		return nil, fmt.Errorf("failed to find similar users: %w", err)
	}

	if len(similarUsers) == 0 {
		// Cold start: return popularity-based recommendations
		return s.getPopularityBasedRecommendations(ctx, userID, limit)
	}

	// Generate recommendations based on similar users' ratings
	recommendations, err := s.generateCollaborativeRecommendations(ctx, userID, similarUsers, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate collaborative recommendations: %w", err)
	}

	// Cache results for 1 hour
	if err := s.cacheResults(ctx, cacheKey, recommendations, time.Hour); err != nil {
		s.logger.Warn("Failed to cache collaborative filtering results", "error", err)
	}

	s.logger.Debug("Collaborative filtering completed",
		"user_id", userID, "similar_users", len(similarUsers), "results", len(recommendations))

	return recommendations, nil
}

// PersonalizedPageRankRecommendations generates recommendations using personalized PageRank
func (s *RecommendationAlgorithmsService) PersonalizedPageRankRecommendations(
	ctx context.Context,
	userID uuid.UUID,
	limit int,
) ([]models.ScoredItem, error) {
	if !s.config.PageRank.Enabled {
		return nil, nil
	}

	cacheKey := fmt.Sprintf("pagerank:%s:%d", userID.String(), limit)

	// Try cache first
	if cached, err := s.getCachedResults(ctx, cacheKey); err == nil && cached != nil {
		s.logger.Debug("PageRank cache hit", "user_id", userID)
		return cached, nil
	}

	// Create user-centric graph projection and run PageRank
	session := s.neo4j.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	// First, create a dynamic subgraph including user + similar users + their interactions
	projectionQuery := `
		CALL gds.graph.project.cypher(
			'user-centric-' + $userId,
			'MATCH (n) WHERE n:User OR n:Content RETURN id(n) AS id, labels(n) AS labels',
			'MATCH (u:User)-[r:RATED|VIEWED|INTERACTED_WITH]-(c:Content) 
			 WHERE u.user_id = $userId OR 
				   u.user_id IN [user.user_id | user IN [(u2:User)-[:SIMILAR_TO]-(u3:User) WHERE u3.user_id = $userId | u2][0..50]]
			 RETURN id(startNode(r)) AS source, id(endNode(r)) AS target, 
					CASE type(r) 
						WHEN "RATED" THEN coalesce(r.rating, 3.0) / 5.0
						WHEN "VIEWED" THEN coalesce(r.progress, 50.0) / 100.0
						WHEN "INTERACTED_WITH" THEN 0.5
						ELSE 0.3
					END AS weight'
		) YIELD graphName
		RETURN graphName`

	result, err := session.Run(ctx, projectionQuery, map[string]interface{}{
		"userId": userID.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create graph projection: %w", err)
	}

	var graphName string
	if result.Next(ctx) {
		record := result.Record()
		graphName = record.Values[0].(string)
	}
	if err := result.Err(); err != nil {
		return nil, fmt.Errorf("failed to get graph projection name: %w", err)
	}

	// Run personalized PageRank
	pageRankQuery := `
		CALL gds.pageRank.stream($graphName, {
			dampingFactor: 0.85,
			maxIterations: 20,
			tolerance: 0.0001,
			sourceNodes: [id(u) | u IN [(user:User) WHERE user.user_id = $userId | user]]
		})
		YIELD nodeId, score
		MATCH (n) WHERE id(n) = nodeId AND n:Content
		RETURN n.content_id AS item_id, score
		ORDER BY score DESC
		LIMIT $limit`

	pageRankResult, err := session.Run(ctx, pageRankQuery, map[string]interface{}{
		"graphName": graphName,
		"userId":    userID.String(),
		"limit":     limit,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to run PageRank: %w", err)
	}

	var results []models.ScoredItem
	for pageRankResult.Next(ctx) {
		record := pageRankResult.Record()
		itemIDStr := record.Values[0].(string)
		score := record.Values[1].(float64)

		itemID, err := uuid.Parse(itemIDStr)
		if err != nil {
			s.logger.Error("Failed to parse item ID from PageRank result", "item_id", itemIDStr)
			continue
		}

		results = append(results, models.ScoredItem{
			ItemID:     itemID,
			Score:      score,
			Algorithm:  "pagerank",
			Confidence: s.calculatePageRankConfidence(score),
		})
	}

	// Clean up graph projection
	cleanupQuery := `CALL gds.graph.drop($graphName)`
	_, err = session.Run(ctx, cleanupQuery, map[string]interface{}{
		"graphName": graphName,
	})
	if err != nil {
		s.logger.Warn("Failed to cleanup graph projection", "graph_name", graphName, "error", err)
	}

	// Cache results for 30 minutes
	if err := s.cacheResults(ctx, cacheKey, results, 30*time.Minute); err != nil {
		s.logger.Warn("Failed to cache PageRank results", "error", err)
	}

	s.logger.Debug("PageRank completed",
		"user_id", userID, "results", len(results))

	return results, nil
}

// GraphSignalAnalysisRecommendations generates recommendations using graph signal analysis
func (s *RecommendationAlgorithmsService) GraphSignalAnalysisRecommendations(
	ctx context.Context,
	userID uuid.UUID,
	limit int,
) ([]models.ScoredItem, error) {
	cacheKey := fmt.Sprintf("graph_signal:%s:%d", userID.String(), limit)

	// Try cache first
	if cached, err := s.getCachedResults(ctx, cacheKey); err == nil && cached != nil {
		s.logger.Debug("Graph signal analysis cache hit", "user_id", userID)
		return cached, nil
	}

	session := s.neo4j.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	// Community detection using Louvain algorithm
	communities, err := s.detectUserCommunities(ctx, session, userID)
	if err != nil {
		s.logger.Warn("Failed to detect communities", "error", err)
		communities = []int{0} // Default community
	}

	// Item similarity based on shared users (Jaccard similarity)
	itemSimilarities, err := s.calculateItemSimilarities(ctx, session, userID, communities)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate item similarities: %w", err)
	}

	// Signal propagation through user networks
	propagatedScores, err := s.propagateSignalThroughNetwork(ctx, session, userID, communities, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to propagate signal: %w", err)
	}

	// Combine item similarities and propagated scores
	results := s.combineGraphSignals(itemSimilarities, propagatedScores, limit)

	// Cache results for 2 hours
	if err := s.cacheResults(ctx, cacheKey, results, 2*time.Hour); err != nil {
		s.logger.Warn("Failed to cache graph signal results", "error", err)
	}

	s.logger.Debug("Graph signal analysis completed",
		"user_id", userID, "communities", len(communities), "results", len(results))

	return results, nil
}

// Helper methods

func (s *RecommendationAlgorithmsService) findSimilarUsers(
	ctx context.Context,
	userID uuid.UUID,
	limit int,
) ([]models.SimilarUser, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("similar_users:%s:%d", userID.String(), limit)
	if cached := s.redis.Get(ctx, cacheKey).Val(); cached != "" {
		var users []models.SimilarUser
		if err := json.Unmarshal([]byte(cached), &users); err == nil {
			return users, nil
		}
	}

	session := s.neo4j.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	// Find users with shared ratings (minimum 3 items) and calculate Pearson correlation
	query := `
		MATCH (u1:User {user_id: $userId})-[r1:RATED]->(item:Content)<-[r2:RATED]-(u2:User)
		WHERE u1 <> u2
		WITH u1, u2, collect({item: item.content_id, rating1: r1.rating, rating2: r2.rating}) AS shared_ratings
		WHERE size(shared_ratings) >= 3
		WITH u1, u2, shared_ratings,
			 reduce(sum = 0.0, rating IN shared_ratings | sum + rating.rating1) / size(shared_ratings) AS avg1,
			 reduce(sum = 0.0, rating IN shared_ratings | sum + rating.rating2) / size(shared_ratings) AS avg2
		WITH u1, u2, shared_ratings, avg1, avg2,
			 reduce(num = 0.0, rating IN shared_ratings | num + (rating.rating1 - avg1) * (rating.rating2 - avg2)) AS numerator,
			 sqrt(reduce(sum = 0.0, rating IN shared_ratings | sum + (rating.rating1 - avg1)^2)) AS denom1,
			 sqrt(reduce(sum = 0.0, rating IN shared_ratings | sum + (rating.rating2 - avg2)^2)) AS denom2
		WITH u2, shared_ratings, 
			 CASE WHEN denom1 * denom2 = 0 THEN 0 ELSE numerator / (denom1 * denom2) END AS correlation
		WHERE correlation >= $threshold
		RETURN u2.user_id AS user_id, correlation AS similarity_score, size(shared_ratings) AS shared_items
		ORDER BY correlation DESC
		LIMIT $limit`

	result, err := session.Run(ctx, query, map[string]interface{}{
		"userId":    userID.String(),
		"threshold": s.config.CollaborativeFilter.SimilarityThreshold,
		"limit":     limit,
	})
	if err != nil {
		return nil, err
	}

	var users []models.SimilarUser
	for result.Next(ctx) {
		record := result.Record()
		userIDStr := record.Values[0].(string)
		similarity := record.Values[1].(float64)
		sharedItems := int(record.Values[2].(int64))

		similarUserID, err := uuid.Parse(userIDStr)
		if err != nil {
			continue
		}

		users = append(users, models.SimilarUser{
			UserID:          similarUserID,
			SimilarityScore: similarity,
			Basis:           "pearson_correlation",
			SharedItems:     sharedItems,
		})
	}

	// Cache for 1 hour
	if data, err := json.Marshal(users); err == nil {
		s.redis.Set(ctx, cacheKey, data, time.Hour)
	}

	return users, nil
}

func (s *RecommendationAlgorithmsService) generateCollaborativeRecommendations(
	ctx context.Context,
	userID uuid.UUID,
	similarUsers []models.SimilarUser,
	limit int,
) ([]models.ScoredItem, error) {
	session := s.neo4j.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	// Get weighted average ratings from similar users
	userIDs := make([]string, len(similarUsers))
	weights := make(map[string]float64)

	for i, user := range similarUsers {
		userIDs[i] = user.UserID.String()
		weights[user.UserID.String()] = user.SimilarityScore
	}

	query := `
		MATCH (u:User)-[r:RATED]->(item:Content)
		WHERE u.user_id IN $userIds
			AND NOT EXISTS {
				MATCH (target:User {user_id: $targetUserId})-[:RATED|LIKED|DISLIKED]->(item)
			}
		WITH item, collect({userId: u.user_id, rating: r.rating}) AS ratings
		RETURN item.content_id AS item_id, ratings
		LIMIT $limit`

	result, err := session.Run(ctx, query, map[string]interface{}{
		"userIds":      userIDs,
		"targetUserId": userID.String(),
		"limit":        limit * 2, // Get more to account for filtering
	})
	if err != nil {
		return nil, err
	}

	type itemScore struct {
		itemID           uuid.UUID
		weightedSum      float64
		weightSum        float64
		contributorCount int
	}

	itemScores := make(map[uuid.UUID]*itemScore)

	for result.Next(ctx) {
		record := result.Record()
		itemIDStr := record.Values[0].(string)
		ratings := record.Values[1].([]interface{})

		itemID, err := uuid.Parse(itemIDStr)
		if err != nil {
			continue
		}

		score := &itemScore{itemID: itemID}

		for _, ratingData := range ratings {
			ratingMap := ratingData.(map[string]interface{})
			userIDStr := ratingMap["userId"].(string)
			rating := ratingMap["rating"].(float64)

			if weight, exists := weights[userIDStr]; exists {
				score.weightedSum += rating * weight
				score.weightSum += weight
				score.contributorCount++
			}
		}

		if score.weightSum > 0 {
			itemScores[itemID] = score
		}
	}

	// Convert to scored items and sort
	var results []models.ScoredItem
	for _, score := range itemScores {
		finalScore := score.weightedSum / score.weightSum
		confidence := s.calculateCollaborativeConfidence(score.contributorCount, score.weightSum)

		results = append(results, models.ScoredItem{
			ItemID:     score.itemID,
			Score:      finalScore,
			Algorithm:  "collaborative_filtering",
			Confidence: confidence,
		})
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Limit results
	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

func (s *RecommendationAlgorithmsService) getPopularityBasedRecommendations(
	ctx context.Context,
	userID uuid.UUID,
	limit int,
) ([]models.ScoredItem, error) {
	query := `
		SELECT 
			ci.id,
			COALESCE(AVG(CASE WHEN ui.interaction_type = 'rating' THEN ui.value END), 0) as avg_rating,
			COUNT(CASE WHEN ui.interaction_type IN ('rating', 'like', 'view') THEN 1 END) as interaction_count,
			ci.quality_score
		FROM content_items ci
		LEFT JOIN user_interactions ui ON ci.id = ui.item_id
		WHERE ci.active = true
			AND ci.quality_score > 0.5
			AND ci.id NOT IN (
				SELECT DISTINCT item_id 
				FROM user_interactions 
				WHERE user_id = $1 
					AND item_id IS NOT NULL
					AND interaction_type IN ('rating', 'like', 'dislike')
			)
		GROUP BY ci.id, ci.quality_score
		HAVING COUNT(CASE WHEN ui.interaction_type IN ('rating', 'like', 'view') THEN 1 END) >= 5
		ORDER BY 
			(COALESCE(AVG(CASE WHEN ui.interaction_type = 'rating' THEN ui.value END), 0) * 0.4 +
			 LOG(COUNT(CASE WHEN ui.interaction_type IN ('rating', 'like', 'view') THEN 1 END) + 1) * 0.3 +
			 ci.quality_score * 0.3) DESC
		LIMIT $2`

	rows, err := s.db.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.ScoredItem
	for rows.Next() {
		var itemID uuid.UUID
		var avgRating, interactionCount, qualityScore float64

		if err := rows.Scan(&itemID, &avgRating, &interactionCount, &qualityScore); err != nil {
			continue
		}

		// Calculate popularity score
		popularityScore := avgRating*0.4 + math.Log(interactionCount+1)*0.3 + qualityScore*0.3

		results = append(results, models.ScoredItem{
			ItemID:     itemID,
			Score:      popularityScore,
			Algorithm:  "popularity_based",
			Confidence: 0.6, // Lower confidence for cold start
		})
	}

	return results, nil
}

func (s *RecommendationAlgorithmsService) detectUserCommunities(
	ctx context.Context,
	session neo4j.SessionWithContext,
	userID uuid.UUID,
) ([]int, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("user_communities:%s", userID.String())
	if cached := s.redis.Get(ctx, cacheKey).Val(); cached != "" {
		var communities []int
		if err := json.Unmarshal([]byte(cached), &communities); err == nil {
			return communities, nil
		}
	}

	query := `
		CALL gds.graph.project.cypher(
			'user-similarity-graph',
			'MATCH (u:User) RETURN id(u) AS id',
			'MATCH (u1:User)-[:SIMILAR_TO]-(u2:User) RETURN id(u1) AS source, id(u2) AS target'
		) YIELD graphName
		CALL gds.louvain.stream(graphName)
		YIELD nodeId, communityId
		MATCH (u:User) WHERE id(u) = nodeId AND u.user_id = $userId
		WITH collect(communityId) AS userCommunities
		CALL gds.graph.drop('user-similarity-graph')
		RETURN userCommunities`

	result, err := session.Run(ctx, query, map[string]interface{}{
		"userId": userID.String(),
	})
	if err != nil {
		return nil, err
	}

	var communities []int
	if result.Next(ctx) {
		record := result.Record()
		if communityList, ok := record.Values[0].([]interface{}); ok {
			for _, community := range communityList {
				if communityID, ok := community.(int64); ok {
					communities = append(communities, int(communityID))
				}
			}
		}
	}

	// Cache for 2 hours
	if data, err := json.Marshal(communities); err == nil {
		s.redis.Set(ctx, cacheKey, data, 2*time.Hour)
	}

	return communities, nil
}

func (s *RecommendationAlgorithmsService) calculateItemSimilarities(
	ctx context.Context,
	session neo4j.SessionWithContext,
	userID uuid.UUID,
	communities []int,
) (map[uuid.UUID]float64, error) {
	// Get items that users in the same communities have interacted with
	query := `
		MATCH (u:User)-[r:RATED|VIEWED|INTERACTED_WITH]->(item1:Content)
		WHERE u.community IN $communities
		MATCH (item1)<-[:RATED|VIEWED|INTERACTED_WITH]-(u2:User)-[:RATED|VIEWED|INTERACTED_WITH]->(item2:Content)
		WHERE item1 <> item2 AND u2.community IN $communities
		WITH item1, item2, count(DISTINCT u2) AS shared_users
		WHERE shared_users >= 3
		MATCH (item1)<-[:RATED|VIEWED|INTERACTED_WITH]-(all_users1:User)
		MATCH (item2)<-[:RATED|VIEWED|INTERACTED_WITH]-(all_users2:User)
		WITH item1, item2, shared_users, 
			 count(DISTINCT all_users1) AS total_users1,
			 count(DISTINCT all_users2) AS total_users2
		WITH item1, item2, 
			 toFloat(shared_users) / (total_users1 + total_users2 - shared_users) AS jaccard_similarity
		WHERE jaccard_similarity > 0.1
		RETURN item2.content_id AS item_id, jaccard_similarity
		ORDER BY jaccard_similarity DESC
		LIMIT 100`

	result, err := session.Run(ctx, query, map[string]interface{}{
		"communities": communities,
	})
	if err != nil {
		return nil, err
	}

	similarities := make(map[uuid.UUID]float64)
	for result.Next(ctx) {
		record := result.Record()
		itemIDStr := record.Values[0].(string)
		similarity := record.Values[1].(float64)

		if itemID, err := uuid.Parse(itemIDStr); err == nil {
			similarities[itemID] = similarity
		}
	}

	return similarities, nil
}

func (s *RecommendationAlgorithmsService) propagateSignalThroughNetwork(
	ctx context.Context,
	session neo4j.SessionWithContext,
	userID uuid.UUID,
	communities []int,
	limit int,
) (map[uuid.UUID]float64, error) {
	// Propagate user preferences through the network
	query := `
		MATCH (source:User {user_id: $userId})-[r1:RATED]->(item:Content)
		WHERE r1.rating >= 4.0
		MATCH (item)<-[r2:RATED]-(intermediate:User)-[r3:RATED]->(target:Content)
		WHERE intermediate.community IN $communities 
			AND r2.rating >= 4.0 
			AND r3.rating >= 4.0
			AND target <> item
		WITH target, 
			 count(DISTINCT intermediate) AS propagation_strength,
			 avg(r3.rating) AS avg_rating
		WHERE propagation_strength >= 2
		RETURN target.content_id AS item_id, 
			   (propagation_strength * avg_rating / 5.0) AS propagated_score
		ORDER BY propagated_score DESC
		LIMIT $limit`

	result, err := session.Run(ctx, query, map[string]interface{}{
		"userId":      userID.String(),
		"communities": communities,
		"limit":       limit,
	})
	if err != nil {
		return nil, err
	}

	scores := make(map[uuid.UUID]float64)
	for result.Next(ctx) {
		record := result.Record()
		itemIDStr := record.Values[0].(string)
		score := record.Values[1].(float64)

		if itemID, err := uuid.Parse(itemIDStr); err == nil {
			scores[itemID] = score
		}
	}

	return scores, nil
}

func (s *RecommendationAlgorithmsService) combineGraphSignals(
	itemSimilarities map[uuid.UUID]float64,
	propagatedScores map[uuid.UUID]float64,
	limit int,
) []models.ScoredItem {
	// Combine similarity and propagation scores
	combinedScores := make(map[uuid.UUID]float64)

	// Add similarity scores
	for itemID, similarity := range itemSimilarities {
		combinedScores[itemID] += similarity * 0.6
	}

	// Add propagated scores
	for itemID, score := range propagatedScores {
		combinedScores[itemID] += score * 0.4
	}

	// Convert to scored items
	var results []models.ScoredItem
	for itemID, score := range combinedScores {
		results = append(results, models.ScoredItem{
			ItemID:     itemID,
			Score:      score,
			Algorithm:  "graph_signal_analysis",
			Confidence: s.calculateGraphSignalConfidence(score),
		})
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Limit results
	if len(results) > limit {
		results = results[:limit]
	}

	return results
}

// Confidence calculation methods

func (s *RecommendationAlgorithmsService) calculateSemanticConfidence(similarity float64) float64 {
	// Higher similarity = higher confidence
	return math.Min(similarity*1.2, 1.0)
}

func (s *RecommendationAlgorithmsService) calculateCollaborativeConfidence(contributorCount int, weightSum float64) float64 {
	// More contributors and higher weight sum = higher confidence
	contributorFactor := math.Min(float64(contributorCount)/10.0, 1.0)
	weightFactor := math.Min(weightSum/5.0, 1.0)
	return (contributorFactor + weightFactor) / 2.0
}

func (s *RecommendationAlgorithmsService) calculatePageRankConfidence(score float64) float64 {
	// Normalize PageRank score to confidence
	return math.Min(score*10.0, 1.0)
}

func (s *RecommendationAlgorithmsService) calculateGraphSignalConfidence(score float64) float64 {
	// Graph signal confidence based on combined score strength
	return math.Min(score*0.8, 1.0)
}

// Cache helper methods

func (s *RecommendationAlgorithmsService) getCachedResults(ctx context.Context, key string) ([]models.ScoredItem, error) {
	cached := s.redis.Get(ctx, key).Val()
	if cached == "" {
		return nil, fmt.Errorf("cache miss")
	}

	var results []models.ScoredItem
	if err := json.Unmarshal([]byte(cached), &results); err != nil {
		return nil, err
	}

	return results, nil
}

func (s *RecommendationAlgorithmsService) cacheResults(ctx context.Context, key string, results []models.ScoredItem, ttl time.Duration) error {
	data, err := json.Marshal(results)
	if err != nil {
		return err
	}

	return s.redis.Set(ctx, key, data, ttl).Err()
}
