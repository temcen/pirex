// Create constraints for User nodes
CREATE CONSTRAINT user_id_unique IF NOT EXISTS FOR (u:User) REQUIRE u.id IS UNIQUE;
CREATE CONSTRAINT user_id_not_null IF NOT EXISTS FOR (u:User) REQUIRE u.id IS NOT NULL;

// Create constraints for Content nodes
CREATE CONSTRAINT content_id_unique IF NOT EXISTS FOR (c:Content) REQUIRE c.id IS UNIQUE;
CREATE CONSTRAINT content_id_not_null IF NOT EXISTS FOR (c:Content) REQUIRE c.id IS NOT NULL;

// Create indexes for performance
CREATE INDEX user_created_at IF NOT EXISTS FOR (u:User) ON (u.created_at);
CREATE INDEX user_last_interaction IF NOT EXISTS FOR (u:User) ON (u.last_interaction);
CREATE INDEX content_type IF NOT EXISTS FOR (c:Content) ON (c.type);
CREATE INDEX content_categories IF NOT EXISTS FOR (c:Content) ON (c.categories);
CREATE INDEX content_created_at IF NOT EXISTS FOR (c:Content) ON (c.created_at);

// Create indexes for relationship properties
CREATE INDEX rated_timestamp IF NOT EXISTS FOR ()-[r:RATED]-() ON (r.timestamp);
CREATE INDEX rated_rating IF NOT EXISTS FOR ()-[r:RATED]-() ON (r.rating);
CREATE INDEX viewed_timestamp IF NOT EXISTS FOR ()-[r:VIEWED]-() ON (r.timestamp);
CREATE INDEX viewed_duration IF NOT EXISTS FOR ()-[r:VIEWED]-() ON (r.duration);
CREATE INDEX interacted_timestamp IF NOT EXISTS FOR ()-[r:INTERACTED_WITH]-() ON (r.timestamp);
CREATE INDEX similar_to_score IF NOT EXISTS FOR ()-[r:SIMILAR_TO]-() ON (r.score);
CREATE INDEX similar_to_computed_at IF NOT EXISTS FOR ()-[r:SIMILAR_TO]-() ON (r.computed_at);

// Create sample data structure (will be populated by application)
// User node properties: id, created_at, last_interaction, interaction_count, user_tier
// Content node properties: id, type, title, categories, created_at, active, quality_score

// Relationship types and their properties:
// (:User)-[:RATED {rating: float, timestamp: datetime, confidence: float, session_id: string}]->(:Content)
// (:User)-[:VIEWED {duration: int, progress: float, timestamp: datetime, session_id: string}]->(:Content)  
// (:User)-[:INTERACTED_WITH {type: string, value: float, timestamp: datetime, session_id: string}]->(:Content)
// (:User)-[:SIMILAR_TO {score: float, basis: string, computed_at: datetime}]->(:User)
// (:Content)-[:SIMILAR_TO {score: float, algorithm: string, computed_at: datetime}]->(:Content)

// Create procedures for common graph operations
CALL gds.graph.project.cypher(
  'user-content-graph',
  'MATCH (n) WHERE n:User OR n:Content RETURN id(n) AS id, labels(n) AS labels',
  'MATCH (u:User)-[r:RATED|VIEWED|INTERACTED_WITH]->(c:Content) 
   RETURN id(u) AS source, id(c) AS target, 
   CASE type(r) 
     WHEN "RATED" THEN r.rating/5.0 
     WHEN "VIEWED" THEN COALESCE(r.progress/100.0, 0.5)
     ELSE 0.3 
   END AS weight'
) YIELD graphName;

// Create user similarity graph projection
CALL gds.graph.project.cypher(
  'user-similarity-graph',
  'MATCH (u:User) RETURN id(u) AS id',
  'MATCH (u1:User)-[:SIMILAR_TO]->(u2:User) 
   RETURN id(u1) AS source, id(u2) AS target, 1.0 AS weight'
) YIELD graphName;

// Example queries for common operations:

// Find users similar to a given user based on shared ratings
// MATCH (u1:User {id: $userId})-[:RATED]->(c:Content)<-[:RATED]-(u2:User)
// WHERE u1 <> u2
// WITH u1, u2, COUNT(c) AS shared_items,
//      AVG(ABS(r1.rating - r2.rating)) AS avg_rating_diff
// WHERE shared_items >= 3
// RETURN u2.id, shared_items, (5 - avg_rating_diff) / 5 AS similarity_score
// ORDER BY similarity_score DESC
// LIMIT 50;

// Find content similar to a given item based on user interactions
// MATCH (c1:Content {id: $itemId})<-[:RATED|VIEWED]-(u:User)-[:RATED|VIEWED]->(c2:Content)
// WHERE c1 <> c2
// WITH c1, c2, COUNT(u) AS shared_users
// WHERE shared_users >= 3
// RETURN c2.id, shared_users, shared_users * 1.0 / 10 AS similarity_score
// ORDER BY similarity_score DESC
// LIMIT 100;

// PageRank for content recommendation
// CALL gds.pageRank.stream('user-content-graph', {
//   sourceNodes: [$userId],
//   dampingFactor: 0.85,
//   maxIterations: 20
// })
// YIELD nodeId, score
// WITH gds.util.asNode(nodeId) AS node, score
// WHERE node:Content AND node.active = true
// RETURN node.id, score
// ORDER BY score DESC
// LIMIT 100;