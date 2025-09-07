// Neo4j Sample Data Population Script
// This script populates Neo4j with sample data that corresponds to the PostgreSQL data

// Clear existing data
MATCH (n) DETACH DELETE n;

// Create User nodes
CREATE 
(:User {
    id: '123e4567-e89b-12d3-a456-426614174001',
    created_at: datetime('2024-01-01T10:00:00Z'),
    last_interaction: datetime('2024-01-15T14:00:00Z'),
    interaction_count: 45,
    user_tier: 'premium'
}),
(:User {
    id: '123e4567-e89b-12d3-a456-426614174002',
    created_at: datetime('2024-01-05T15:30:00Z'),
    last_interaction: datetime('2024-01-14T16:00:00Z'),
    interaction_count: 32,
    user_tier: 'free'
}),
(:User {
    id: '123e4567-e89b-12d3-a456-426614174003',
    created_at: datetime('2024-01-03T09:15:00Z'),
    last_interaction: datetime('2024-01-12T11:30:00Z'),
    interaction_count: 28,
    user_tier: 'premium'
}),
(:User {
    id: '123e4567-e89b-12d3-a456-426614174004',
    created_at: datetime('2024-01-10T12:45:00Z'),
    last_interaction: datetime('2024-01-15T08:30:00Z'),
    interaction_count: 67,
    user_tier: 'free'
}),
(:User {
    id: '123e4567-e89b-12d3-a456-426614174005',
    created_at: datetime('2024-01-02T20:00:00Z'),
    last_interaction: datetime('2024-01-15T10:00:00Z'),
    interaction_count: 89,
    user_tier: 'premium'
}),
(:User {
    id: '123e4567-e89b-12d3-a456-426614174006',
    created_at: datetime('2024-01-08T07:30:00Z'),
    last_interaction: datetime('2024-01-14T08:00:00Z'),
    interaction_count: 41,
    user_tier: 'premium'
}),
(:User {
    id: '123e4567-e89b-12d3-a456-426614174007',
    created_at: datetime('2024-01-15T13:30:00Z'),
    last_interaction: datetime('2024-01-15T14:00:00Z'),
    interaction_count: 3,
    user_tier: 'free'
}),
(:User {
    id: '123e4567-e89b-12d3-a456-426614174008',
    created_at: datetime('2023-12-15T10:00:00Z'),
    last_interaction: datetime('2024-01-15T06:00:00Z'),
    interaction_count: 156,
    user_tier: 'premium'
});

// Create Content nodes
CREATE 
(:Content {
    id: '550e8400-e29b-41d4-a716-446655440001',
    type: 'product',
    title: 'iPhone 15 Pro',
    categories: ['Electronics', 'Smartphones', 'Apple'],
    created_at: datetime('2024-01-01T00:00:00Z'),
    active: true,
    quality_score: 0.95
}),
(:Content {
    id: '550e8400-e29b-41d4-a716-446655440002',
    type: 'product',
    title: 'Samsung Galaxy S24 Ultra',
    categories: ['Electronics', 'Smartphones', 'Samsung'],
    created_at: datetime('2024-01-01T01:00:00Z'),
    active: true,
    quality_score: 0.92
}),
(:Content {
    id: '550e8400-e29b-41d4-a716-446655440003',
    type: 'product',
    title: 'MacBook Pro 16"',
    categories: ['Electronics', 'Laptops', 'Apple'],
    created_at: datetime('2024-01-01T02:00:00Z'),
    active: true,
    quality_score: 0.98
}),
(:Content {
    id: '550e8400-e29b-41d4-a716-446655440004',
    type: 'product',
    title: 'Dell XPS 13',
    categories: ['Electronics', 'Laptops', 'Dell'],
    created_at: datetime('2024-01-01T03:00:00Z'),
    active: true,
    quality_score: 0.88
}),
(:Content {
    id: '550e8400-e29b-41d4-a716-446655440005',
    type: 'product',
    title: 'Sony WH-1000XM5',
    categories: ['Electronics', 'Audio', 'Headphones'],
    created_at: datetime('2024-01-01T04:00:00Z'),
    active: true,
    quality_score: 0.91
}),
(:Content {
    id: '550e8400-e29b-41d4-a716-446655440006',
    type: 'product',
    title: 'Nike Air Max 270',
    categories: ['Fashion', 'Shoes', 'Athletic'],
    created_at: datetime('2024-01-01T05:00:00Z'),
    active: true,
    quality_score: 0.85
}),
(:Content {
    id: '550e8400-e29b-41d4-a716-446655440007',
    type: 'product',
    title: 'Levi\'s 501 Original Jeans',
    categories: ['Fashion', 'Clothing', 'Jeans'],
    created_at: datetime('2024-01-01T06:00:00Z'),
    active: true,
    quality_score: 0.82
}),
(:Content {
    id: '550e8400-e29b-41d4-a716-446655440008',
    type: 'product',
    title: 'Dyson V15 Detect',
    categories: ['Home & Garden', 'Appliances', 'Cleaning'],
    created_at: datetime('2024-01-01T07:00:00Z'),
    active: true,
    quality_score: 0.94
}),
(:Content {
    id: '550e8400-e29b-41d4-a716-446655440009',
    type: 'video',
    title: 'iPhone 15 Pro Review',
    categories: ['Technology', 'Reviews', 'Smartphones'],
    created_at: datetime('2024-01-01T08:00:00Z'),
    active: true,
    quality_score: 0.89
}),
(:Content {
    id: '550e8400-e29b-41d4-a716-446655440010',
    type: 'video',
    title: 'MacBook Pro M3 Unboxing',
    categories: ['Technology', 'Unboxing', 'Laptops'],
    created_at: datetime('2024-01-01T09:00:00Z'),
    active: true,
    quality_score: 0.86
}),
(:Content {
    id: '550e8400-e29b-41d4-a716-446655440011',
    type: 'article',
    title: 'Best Smartphones of 2024',
    categories: ['Technology', 'Guides', 'Smartphones'],
    created_at: datetime('2024-01-01T10:00:00Z'),
    active: true,
    quality_score: 0.93
}),
(:Content {
    id: '550e8400-e29b-41d4-a716-446655440012',
    type: 'article',
    title: 'Fashion Trends Spring 2024',
    categories: ['Fashion', 'Trends', 'Style'],
    created_at: datetime('2024-01-01T11:00:00Z'),
    active: true,
    quality_score: 0.87
}),
(:Content {
    id: '550e8400-e29b-41d4-a716-446655440013',
    type: 'product',
    title: 'The Psychology of Persuasion',
    categories: ['Books', 'Psychology', 'Business'],
    created_at: datetime('2024-01-01T12:00:00Z'),
    active: true,
    quality_score: 0.96
}),
(:Content {
    id: '550e8400-e29b-41d4-a716-446655440014',
    type: 'product',
    title: 'Patagonia Down Jacket',
    categories: ['Sports & Outdoors', 'Clothing', 'Jackets'],
    created_at: datetime('2024-01-01T13:00:00Z'),
    active: true,
    quality_score: 0.90
}),
(:Content {
    id: '550e8400-e29b-41d4-a716-446655440015',
    type: 'product',
    title: 'PlayStation 5',
    categories: ['Gaming', 'Consoles', 'Sony'],
    created_at: datetime('2024-01-01T14:00:00Z'),
    active: true,
    quality_score: 0.97
});

// Create RATED relationships
MATCH (u:User {id: '123e4567-e89b-12d3-a456-426614174001'}), (c:Content {id: '550e8400-e29b-41d4-a716-446655440001'})
CREATE (u)-[:RATED {rating: 5.0, timestamp: datetime('2024-01-15T12:00:00Z'), confidence: 0.95, session_id: '550e8400-e29b-41d4-a716-446655440101'}]->(c);

MATCH (u:User {id: '123e4567-e89b-12d3-a456-426614174002'}), (c:Content {id: '550e8400-e29b-41d4-a716-446655440006'})
CREATE (u)-[:RATED {rating: 4.0, timestamp: datetime('2024-01-14T16:00:00Z'), confidence: 0.87, session_id: '550e8400-e29b-41d4-a716-446655440102'}]->(c);

MATCH (u:User {id: '123e4567-e89b-12d3-a456-426614174003'}), (c:Content {id: '550e8400-e29b-41d4-a716-446655440008'})
CREATE (u)-[:RATED {rating: 5.0, timestamp: datetime('2024-01-12T11:30:00Z'), confidence: 0.94, session_id: '550e8400-e29b-41d4-a716-446655440103'}]->(c);

MATCH (u:User {id: '123e4567-e89b-12d3-a456-426614174004'}), (c:Content {id: '550e8400-e29b-41d4-a716-446655440013'})
CREATE (u)-[:RATED {rating: 4.0, timestamp: datetime('2024-01-15T08:30:00Z'), confidence: 0.82, session_id: '550e8400-e29b-41d4-a716-446655440104'}]->(c);

MATCH (u:User {id: '123e4567-e89b-12d3-a456-426614174005'}), (c:Content {id: '550e8400-e29b-41d4-a716-446655440015'})
CREATE (u)-[:RATED {rating: 5.0, timestamp: datetime('2024-01-15T10:00:00Z'), confidence: 0.97, session_id: '550e8400-e29b-41d4-a716-446655440105'}]->(c);

MATCH (u:User {id: '123e4567-e89b-12d3-a456-426614174006'}), (c:Content {id: '550e8400-e29b-41d4-a716-446655440014'})
CREATE (u)-[:RATED {rating: 4.0, timestamp: datetime('2024-01-14T08:00:00Z'), confidence: 0.90, session_id: '550e8400-e29b-41d4-a716-446655440106'}]->(c);

MATCH (u:User {id: '123e4567-e89b-12d3-a456-426614174008'}), (c:Content {id: '550e8400-e29b-41d4-a716-446655440001'})
CREATE (u)-[:RATED {rating: 4.0, timestamp: datetime('2024-01-15T06:00:00Z'), confidence: 0.95, session_id: '550e8400-e29b-41d4-a716-446655440108'}]->(c);

MATCH (u:User {id: '123e4567-e89b-12d3-a456-426614174008'}), (c:Content {id: '550e8400-e29b-41d4-a716-446655440013'})
CREATE (u)-[:RATED {rating: 5.0, timestamp: datetime('2024-01-15T05:45:00Z'), confidence: 0.96, session_id: '550e8400-e29b-41d4-a716-446655440108'}]->(c);

// Create VIEWED relationships
MATCH (u:User {id: '123e4567-e89b-12d3-a456-426614174001'}), (c:Content {id: '550e8400-e29b-41d4-a716-446655440003'})
CREATE (u)-[:VIEWED {duration: 180, progress: 75.0, timestamp: datetime('2024-01-15T11:45:00Z'), session_id: '550e8400-e29b-41d4-a716-446655440101'}]->(c);

MATCH (u:User {id: '123e4567-e89b-12d3-a456-426614174002'}), (c:Content {id: '550e8400-e29b-41d4-a716-446655440007'})
CREATE (u)-[:VIEWED {duration: 120, progress: 60.0, timestamp: datetime('2024-01-14T15:55:00Z'), session_id: '550e8400-e29b-41d4-a716-446655440102'}]->(c);

MATCH (u:User {id: '123e4567-e89b-12d3-a456-426614174002'}), (c:Content {id: '550e8400-e29b-41d4-a716-446655440012'})
CREATE (u)-[:VIEWED {duration: 300, progress: 90.0, timestamp: datetime('2024-01-14T15:50:00Z'), session_id: '550e8400-e29b-41d4-a716-446655440102'}]->(c);

MATCH (u:User {id: '123e4567-e89b-12d3-a456-426614174003'}), (c:Content {id: '550e8400-e29b-41d4-a716-446655440008'})
CREATE (u)-[:VIEWED {duration: 240, progress: 85.0, timestamp: datetime('2024-01-12T11:20:00Z'), session_id: '550e8400-e29b-41d4-a716-446655440103'}]->(c);

MATCH (u:User {id: '123e4567-e89b-12d3-a456-426614174004'}), (c:Content {id: '550e8400-e29b-41d4-a716-446655440011'})
CREATE (u)-[:VIEWED {duration: 480, progress: 95.0, timestamp: datetime('2024-01-15T08:15:00Z'), session_id: '550e8400-e29b-41d4-a716-446655440104'}]->(c);

MATCH (u:User {id: '123e4567-e89b-12d3-a456-426614174005'}), (c:Content {id: '550e8400-e29b-41d4-a716-446655440009'})
CREATE (u)-[:VIEWED {duration: 720, progress: 100.0, timestamp: datetime('2024-01-15T09:30:00Z'), session_id: '550e8400-e29b-41d4-a716-446655440105'}]->(c);

MATCH (u:User {id: '123e4567-e89b-12d3-a456-426614174007'}), (c:Content {id: '550e8400-e29b-41d4-a716-446655440001'})
CREATE (u)-[:VIEWED {duration: 60, progress: 25.0, timestamp: datetime('2024-01-15T14:00:00Z'), session_id: '550e8400-e29b-41d4-a716-446655440107'}]->(c);

MATCH (u:User {id: '123e4567-e89b-12d3-a456-426614174008'}), (c:Content {id: '550e8400-e29b-41d4-a716-446655440008'})
CREATE (u)-[:VIEWED {duration: 200, progress: 80.0, timestamp: datetime('2024-01-15T05:30:00Z'), session_id: '550e8400-e29b-41d4-a716-446655440108'}]->(c);

// Create INTERACTED_WITH relationships (likes, shares, etc.)
MATCH (u:User {id: '123e4567-e89b-12d3-a456-426614174001'}), (c:Content {id: '550e8400-e29b-41d4-a716-446655440005'})
CREATE (u)-[:INTERACTED_WITH {type: 'like', value: 1.0, timestamp: datetime('2024-01-15T11:30:00Z'), session_id: '550e8400-e29b-41d4-a716-446655440101'}]->(c);

MATCH (u:User {id: '123e4567-e89b-12d3-a456-426614174006'}), (c:Content {id: '550e8400-e29b-41d4-a716-446655440006'})
CREATE (u)-[:INTERACTED_WITH {type: 'like', value: 1.0, timestamp: datetime('2024-01-14T07:40:00Z'), session_id: '550e8400-e29b-41d4-a716-446655440106'}]->(c);

// Create user similarity relationships based on shared preferences
MATCH (u1:User {id: '123e4567-e89b-12d3-a456-426614174001'}), (u2:User {id: '123e4567-e89b-12d3-a456-426614174008'})
CREATE (u1)-[:SIMILAR_TO {score: 0.85, basis: 'shared_ratings', computed_at: datetime('2024-01-15T00:00:00Z')}]->(u2);

MATCH (u1:User {id: '123e4567-e89b-12d3-a456-426614174008'}), (u2:User {id: '123e4567-e89b-12d3-a456-426614174001'})
CREATE (u1)-[:SIMILAR_TO {score: 0.85, basis: 'shared_ratings', computed_at: datetime('2024-01-15T00:00:00Z')}]->(u2);

MATCH (u1:User {id: '123e4567-e89b-12d3-a456-426614174002'}), (u2:User {id: '123e4567-e89b-12d3-a456-426614174006'})
CREATE (u1)-[:SIMILAR_TO {score: 0.72, basis: 'category_preferences', computed_at: datetime('2024-01-15T00:00:00Z')}]->(u2);

MATCH (u1:User {id: '123e4567-e89b-12d3-a456-426614174006'}), (u2:User {id: '123e4567-e89b-12d3-a456-426614174002'})
CREATE (u1)-[:SIMILAR_TO {score: 0.72, basis: 'category_preferences', computed_at: datetime('2024-01-15T00:00:00Z')}]->(u2);

MATCH (u1:User {id: '123e4567-e89b-12d3-a456-426614174004'}), (u2:User {id: '123e4567-e89b-12d3-a456-426614174008'})
CREATE (u1)-[:SIMILAR_TO {score: 0.68, basis: 'reading_behavior', computed_at: datetime('2024-01-15T00:00:00Z')}]->(u2);

// Create content similarity relationships
MATCH (c1:Content {id: '550e8400-e29b-41d4-a716-446655440001'}), (c2:Content {id: '550e8400-e29b-41d4-a716-446655440002'})
CREATE (c1)-[:SIMILAR_TO {score: 0.92, algorithm: 'category_similarity', computed_at: datetime('2024-01-15T00:00:00Z')}]->(c2);

MATCH (c1:Content {id: '550e8400-e29b-41d4-a716-446655440002'}), (c2:Content {id: '550e8400-e29b-41d4-a716-446655440001'})
CREATE (c1)-[:SIMILAR_TO {score: 0.92, algorithm: 'category_similarity', computed_at: datetime('2024-01-15T00:00:00Z')}]->(c2);

MATCH (c1:Content {id: '550e8400-e29b-41d4-a716-446655440003'}), (c2:Content {id: '550e8400-e29b-41d4-a716-446655440004'})
CREATE (c1)-[:SIMILAR_TO {score: 0.88, algorithm: 'category_similarity', computed_at: datetime('2024-01-15T00:00:00Z')}]->(c2);

MATCH (c1:Content {id: '550e8400-e29b-41d4-a716-446655440004'}), (c2:Content {id: '550e8400-e29b-41d4-a716-446655440003'})
CREATE (c1)-[:SIMILAR_TO {score: 0.88, algorithm: 'category_similarity', computed_at: datetime('2024-01-15T00:00:00Z')}]->(c2);

MATCH (c1:Content {id: '550e8400-e29b-41d4-a716-446655440006'}), (c2:Content {id: '550e8400-e29b-41d4-a716-446655440007'})
CREATE (c1)-[:SIMILAR_TO {score: 0.75, algorithm: 'category_similarity', computed_at: datetime('2024-01-15T00:00:00Z')}]->(c2);

MATCH (c1:Content {id: '550e8400-e29b-41d4-a716-446655440009'}), (c2:Content {id: '550e8400-e29b-41d4-a716-446655440011'})
CREATE (c1)-[:SIMILAR_TO {score: 0.83, algorithm: 'content_similarity', computed_at: datetime('2024-01-15T00:00:00Z')}]->(c2);

// Create graph projections for algorithms
CALL gds.graph.project(
  'user-content-interactions',
  ['User', 'Content'],
  {
    RATED: {
      properties: 'rating'
    },
    VIEWED: {
      properties: 'progress'
    },
    INTERACTED_WITH: {
      properties: 'value'
    }
  }
);

// Create user similarity graph
CALL gds.graph.project(
  'user-similarity',
  'User',
  {
    SIMILAR_TO: {
      properties: 'score'
    }
  }
);

// Display summary of created data
MATCH (u:User) RETURN 'Users' as type, count(u) as count
UNION ALL
MATCH (c:Content) RETURN 'Content' as type, count(c) as count
UNION ALL
MATCH ()-[r:RATED]->() RETURN 'Ratings' as type, count(r) as count
UNION ALL
MATCH ()-[r:VIEWED]->() RETURN 'Views' as type, count(r) as count
UNION ALL
MATCH ()-[r:INTERACTED_WITH]->() RETURN 'Interactions' as type, count(r) as count
UNION ALL
MATCH ()-[r:SIMILAR_TO]->() RETURN 'Similarities' as type, count(r) as count;