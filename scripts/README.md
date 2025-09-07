# Database Scripts Documentation

This directory contains all the scripts necessary for setting up, validating, and populating the recommendation engine databases with sample data.

## Scripts Overview

### Initialization Scripts
- **`init-postgres.sql`** - Base PostgreSQL schema with tables, indexes, and triggers
- **`init-pgvector.sql`** - pgvector extension setup and vector indexes
- **`init-neo4j.cypher`** - Neo4j constraints, indexes, and graph projections
- **`init-content-ingestion.sql`** - Content ingestion pipeline schema
- **`init-metrics.sql`** - Business metrics and analytics tables

### Validation Scripts
- **`validate-schema.sql`** - Validates database schema matches expected structure
- **`validate-data-integrity.sql`** - Checks data integrity and relationships

### Sample Data Scripts
- **`populate-sample-data.sql`** - Comprehensive PostgreSQL sample data
- **`populate-neo4j-sample-data.cypher`** - Neo4j graph data corresponding to PostgreSQL

### Setup and Testing
- **`test-database-setup.sh`** - Automated test script for complete database setup
- **`database-setup-guide.md`** - Detailed step-by-step setup guide

## Quick Start

### 1. Automated Setup (Recommended)

```bash
# Run the automated test script
./scripts/test-database-setup.sh
```

This script will:
- Start all database services via Docker Compose
- Validate schema initialization
- Populate sample data
- Run integrity checks
- Perform basic performance tests

### 2. Manual Setup

```bash
# Start services
docker-compose -f docker-compose.dev.yml up -d postgres neo4j redis-hot redis-warm redis-cold

# Validate schema
docker exec -i recommendation-postgres psql -U postgres -d recommendations < scripts/validate-schema.sql

# Populate PostgreSQL data
docker exec -i recommendation-postgres psql -U postgres -d recommendations < scripts/populate-sample-data.sql

# Populate Neo4j data
docker exec -i recommendation-neo4j cypher-shell -u neo4j -p password < scripts/populate-neo4j-sample-data.cypher

# Validate data integrity
docker exec -i recommendation-postgres psql -U postgres -d recommendations < scripts/validate-data-integrity.sql
```

## Sample Data Overview

### PostgreSQL Data
- **15 Content Items**: Products, videos, and articles across multiple categories
- **8 User Profiles**: Different user types (tech enthusiast, fashion-focused, budget-conscious, etc.)
- **20+ User Interactions**: Ratings, views, likes with realistic patterns
- **Sample Metrics**: Recommendation analytics data for testing dashboards
- **Content Jobs**: Various ingestion job statuses

### Neo4j Data
- **User Nodes**: 8 users with metadata and interaction counts
- **Content Nodes**: 15 content items with categories and quality scores
- **Relationships**: RATED, VIEWED, INTERACTED_WITH, SIMILAR_TO
- **Graph Projections**: Pre-configured for PageRank and similarity algorithms

## Testing Scenarios

The sample data supports testing these scenarios:

### User Types
1. **New User** (User 007): Minimal interactions, cold start scenario
2. **Tech Enthusiast** (User 001): High-value electronics preferences
3. **Fashion User** (User 002): Fashion and style focused
4. **Budget User** (User 004): Price-conscious, high interaction volume
5. **Power User** (User 008): Diverse interests, high engagement

### Content Types
1. **Electronics**: Smartphones, laptops, headphones
2. **Fashion**: Shoes, clothing, style articles
3. **Home & Garden**: Appliances, tools
4. **Entertainment**: Videos, gaming consoles
5. **Books**: Educational and business content

### Interaction Patterns
1. **Explicit Feedback**: Ratings (1-5 stars), likes/dislikes
2. **Implicit Feedback**: Views with duration, clicks
3. **Search Behavior**: Query-based interactions
4. **Session Tracking**: Multi-interaction sessions

## Validation Checks

### Schema Validation
- ✅ All required tables exist
- ✅ Vector columns have correct dimensions (768)
- ✅ Indexes are properly created (HNSW, IVFFlat)
- ✅ Foreign key relationships are valid
- ✅ Triggers and functions are working

### Data Integrity
- ✅ No orphaned records
- ✅ Embedding coverage > 90%
- ✅ Consistent interaction counts
- ✅ Valid value ranges (ratings 1-5, quality scores 0-1)
- ✅ Proper timestamp ordering

### Performance
- ✅ Vector similarity queries < 100ms
- ✅ Graph algorithms functional
- ✅ Connection pooling working
- ✅ Index usage optimized

## Customization

### Adding More Sample Data

To add more content items:

```sql
INSERT INTO content_items (type, title, description, categories, quality_score) VALUES
('product', 'Your Product', 'Description', ARRAY['Category1', 'Category2'], 0.85);
```

To add more users:

```sql
INSERT INTO user_profiles (user_id, explicit_preferences, behavior_patterns, demographics) VALUES
('new-user-uuid', '{"preferred_categories": ["Electronics"]}', '{}', '{}');
```

### Modifying Categories

Update the categories in both PostgreSQL and Neo4j:

```sql
-- PostgreSQL
UPDATE content_items SET categories = ARRAY['New', 'Categories'] WHERE id = 'item-uuid';
```

```cypher
// Neo4j
MATCH (c:Content {id: 'item-uuid'}) SET c.categories = ['New', 'Categories'];
```

## Troubleshooting

### Common Issues

1. **pgvector not found**: Ensure using `pgvector/pgvector:pg15` Docker image
2. **Neo4j GDS plugin missing**: Check `NEO4J_PLUGINS` environment variable
3. **Vector dimension mismatch**: All embeddings must be 768 dimensions
4. **Connection refused**: Wait for services to fully start (use health checks)

### Debug Commands

```bash
# Check service status
docker-compose -f docker-compose.dev.yml ps

# View logs
docker-compose -f docker-compose.dev.yml logs postgres
docker-compose -f docker-compose.dev.yml logs neo4j

# Test connections
docker exec recommendation-postgres pg_isready -U postgres
docker exec recommendation-neo4j cypher-shell -u neo4j -p password "RETURN 1;"

# Check data counts
docker exec recommendation-postgres psql -U postgres -d recommendations -c "SELECT COUNT(*) FROM content_items;"
docker exec recommendation-neo4j cypher-shell -u neo4j -p password "MATCH (n) RETURN count(n);"
```

## Performance Benchmarks

Expected performance with sample data:

- **Vector similarity search**: < 50ms for 100 results
- **Neo4j PageRank**: < 200ms for full graph
- **User profile lookup**: < 10ms
- **Recommendation generation**: < 500ms end-to-end

## Next Steps

After successful database setup:

1. **Start the Go application server**
2. **Run the integration test suite**
3. **Test API endpoints with sample data**
4. **Load test with concurrent users**
5. **Monitor performance metrics**

## File Structure

```
scripts/
├── README.md                           # This file
├── database-setup-guide.md             # Detailed setup guide
├── test-database-setup.sh              # Automated test script
├── validate-schema.sql                 # Schema validation
├── validate-data-integrity.sql         # Data integrity checks
├── populate-sample-data.sql            # PostgreSQL sample data
├── populate-neo4j-sample-data.cypher   # Neo4j sample data
├── init-postgres.sql                   # PostgreSQL initialization
├── init-pgvector.sql                   # pgvector setup
├── init-neo4j.cypher                   # Neo4j initialization
├── init-content-ingestion.sql          # Content pipeline schema
└── init-metrics.sql                    # Metrics and analytics schema
```

For detailed setup instructions, see `database-setup-guide.md`.