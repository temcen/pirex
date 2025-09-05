# Recommendation Algorithms

This document describes the core recommendation algorithms implemented in the recommendation engine.

## Overview

The recommendation engine implements four main algorithms that work together to provide personalized recommendations:

1. **Semantic Search Algorithm** - Vector similarity using pgvector
2. **Collaborative Filtering Algorithm** - User-based recommendations using Pearson correlation
3. **Personalized PageRank Algorithm** - Graph-based recommendations using Neo4j GDS
4. **Graph Signal Analysis** - Community detection and signal propagation

Each algorithm returns `[]ScoredItem` with ItemID, Score, Algorithm, and Confidence values that are later combined by the Recommendation Orchestrator.

## Algorithm Details

### 1. Semantic Search Algorithm

**Purpose**: Find items similar to user preferences using vector embeddings.

**Implementation**:
- Uses pgvector `<=>` operator for cosine similarity
- Filters by content type, categories, and active status
- Applies similarity threshold (default 0.7)
- Excludes items user has already interacted with

**Query Example**:
```sql
SELECT 
    id as item_id,
    1 - (embedding <=> $1) as similarity
FROM content_items 
WHERE active = true
    AND quality_score > 0.5
    AND 1 - (embedding <=> $1) >= 0.7
    AND type = ANY($2)
    AND categories && $3
    AND id NOT IN (
        SELECT DISTINCT item_id 
        FROM user_interactions 
        WHERE user_id = $4 
            AND item_id IS NOT NULL
            AND interaction_type IN ('rating', 'like', 'dislike')
    )
ORDER BY embedding <=> $1 
LIMIT 100
```

**Caching**: 30 minutes TTL in Redis-warm

**Performance**: ~10ms typical response time

### 2. Collaborative Filtering Algorithm

**Purpose**: Recommend items based on similar users' preferences.

**Implementation**:
- Finds similar users using Pearson correlation in Neo4j
- Requires minimum 3 shared rated items
- Generates weighted average recommendations
- Handles cold start with popularity-based fallback

**Neo4j Query for User Similarity**:
```cypher
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
WHERE correlation >= 0.5
RETURN u2.user_id AS user_id, correlation AS similarity_score, size(shared_ratings) AS shared_items
ORDER BY correlation DESC
LIMIT 50
```

**Caching**: 1 hour TTL for user similarities

**Performance**: ~50ms typical response time

### 3. Personalized PageRank Algorithm

**Purpose**: Leverage graph structure to find influential items in user's network.

**Implementation**:
- Creates dynamic user-centric graph projection
- Includes user + similar users + their interactions
- Configures damping factor (0.85), max iterations (20)
- Weights edges by interaction type

**Edge Weights**:
- RATED: rating/5.0 (0.2 to 1.0)
- VIEWED: progress/100 (0.0 to 1.0)
- INTERACTED_WITH: 0.5 (fixed)

**Neo4j GDS Query**:
```cypher
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
LIMIT $limit
```

**Caching**: 30 minutes TTL

**Performance**: ~100ms typical response time

### 4. Graph Signal Analysis

**Purpose**: Use community detection and signal propagation for serendipitous recommendations.

**Implementation**:
- Detects user communities using Louvain algorithm
- Calculates item similarities using Jaccard coefficient
- Propagates signals through user networks
- Combines similarity and propagation scores

**Community Detection**:
```cypher
CALL gds.louvain.stream('user-similarity-graph')
YIELD nodeId, communityId
MATCH (u:User) WHERE id(u) = nodeId AND u.user_id = $userId
RETURN collect(communityId) AS userCommunities
```

**Signal Propagation**:
```cypher
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
```

**Caching**: 2 hours TTL for community detection

**Performance**: ~200ms typical response time

## Confidence Scoring

Each algorithm calculates confidence scores to indicate result reliability:

### Semantic Search Confidence
```go
func calculateSemanticConfidence(similarity float64) float64 {
    return math.Min(similarity*1.2, 1.0)
}
```

### Collaborative Filtering Confidence
```go
func calculateCollaborativeConfidence(contributorCount int, weightSum float64) float64 {
    contributorFactor := math.Min(float64(contributorCount)/10.0, 1.0)
    weightFactor := math.Min(weightSum/5.0, 1.0)
    return (contributorFactor + weightFactor) / 2.0
}
```

### PageRank Confidence
```go
func calculatePageRankConfidence(score float64) float64 {
    return math.Min(score*10.0, 1.0)
}
```

### Graph Signal Confidence
```go
func calculateGraphSignalConfidence(score float64) float64 {
    return math.Min(score*0.8, 1.0)
}
```

## Caching Strategy

| Algorithm | Cache Location | TTL | Reason |
|-----------|---------------|-----|---------|
| Semantic Search | Redis-warm | 30min | Frequent queries, moderate computation |
| Collaborative Filtering | Redis-warm | 1hour | User similarities change slowly |
| PageRank | Redis-warm | 30min | Graph computations are expensive |
| Graph Signal Analysis | Redis-warm | 2hours | Community detection is very expensive |

## Performance Characteristics

| Algorithm | Typical Latency | Cache Hit Rate | Scalability |
|-----------|----------------|----------------|-------------|
| Semantic Search | ~10ms | 85% | Excellent (vector ops) |
| Collaborative Filtering | ~50ms | 70% | Good (limited by user similarity) |
| PageRank | ~100ms | 60% | Moderate (graph computation) |
| Graph Signal Analysis | ~200ms | 80% | Lower (community detection) |

## Algorithm Selection Strategy

The system selects algorithms based on user profile completeness:

### New Users (< 5 interactions)
- **Primary**: Popularity-based recommendations
- **Secondary**: Semantic search (if user preferences available)
- **Fallback**: Random high-quality items

### Active Users (5-50 interactions)
- **All algorithms enabled**
- **Weights**: Semantic (0.4), Collaborative (0.3), PageRank (0.3)
- **Graph Signal**: Used for diversity injection

### Power Users (> 50 interactions)
- **Primary**: Collaborative filtering and Graph-based algorithms
- **Secondary**: Semantic search for exploration
- **Enhanced**: Graph signal analysis for serendipity

## Integration with Recommendation Orchestrator

1. **Parallel Execution**: All enabled algorithms run concurrently
2. **Result Combination**: Weighted combination based on configuration
3. **Score Normalization**: Each algorithm's scores normalized to [0,1]
4. **Confidence Weighting**: Final scores adjusted by confidence
5. **Diversity Filtering**: Applied after initial ranking

## Testing and Validation

### Unit Tests
- Algorithm-specific logic testing
- Confidence calculation validation
- Cache operation verification
- Mock database integration

### Integration Tests
- End-to-end algorithm execution
- Database query validation
- Neo4j graph operations
- Redis caching behavior

### Performance Tests
- Benchmark confidence calculations
- Load testing with concurrent requests
- Memory usage profiling
- Cache hit rate optimization

### Synthetic Data Validation
- Known pattern recognition
- Algorithm output consistency
- Score distribution analysis
- Confidence range validation

## Configuration

Algorithms are configured via `config/app.yaml`:

```yaml
recommendation:
  algorithms:
    semantic_search:
      enabled: true
      weight: 0.4
      similarity_threshold: 0.7
    collaborative_filtering:
      enabled: true
      weight: 0.3
      similarity_threshold: 0.5
    pagerank:
      enabled: true
      weight: 0.3
      damping_factor: 0.85
      max_iterations: 20
    graph_signal_analysis:
      enabled: true
      community_cache_ttl: "2h"
      min_propagation_strength: 2
```

## Monitoring and Metrics

Key metrics tracked for each algorithm:

- **Execution Time**: P50, P95, P99 latencies
- **Cache Hit Rate**: Percentage of cached responses
- **Error Rate**: Failed algorithm executions
- **Confidence Distribution**: Average confidence scores
- **Result Quality**: Click-through rates by algorithm

## Future Enhancements

1. **Machine Learning Integration**: Replace heuristic confidence with learned models
2. **Dynamic Weight Adjustment**: A/B testing for optimal algorithm weights
3. **Advanced Graph Algorithms**: Graph Neural Networks for deeper insights
4. **Real-time Learning**: Online learning for immediate preference updates
5. **Multi-objective Optimization**: Balance relevance, diversity, and novelty

## Usage Example

```go
// Initialize service
service := services.NewRecommendationAlgorithmsService(
    db, neo4j, redis, config, logger,
)

// Get semantic search recommendations
semanticResults, err := service.SemanticSearchRecommendations(
    ctx, userID, userEmbedding, contentTypes, categories, 20,
)

// Get collaborative filtering recommendations
collaborativeResults, err := service.CollaborativeFilteringRecommendations(
    ctx, userID, 20,
)

// Get PageRank recommendations
pageRankResults, err := service.PersonalizedPageRankRecommendations(
    ctx, userID, 20,
)

// Get graph signal analysis recommendations
graphResults, err := service.GraphSignalAnalysisRecommendations(
    ctx, userID, 20,
)

// Results are combined by the Recommendation Orchestrator
```

For more details, see the implementation in `internal/services/recommendation_algorithms.go` and tests in `internal/services/recommendation_algorithms_test.go`.