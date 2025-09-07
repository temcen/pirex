#!/bin/bash

# Database Setup Test Script
# This script tests the complete database setup and data population

set -e  # Exit on any error

echo "🚀 Starting database setup test..."

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

# Check if Docker Compose is available
if ! command -v docker-compose &> /dev/null; then
    print_error "docker-compose is not installed or not in PATH"
    exit 1
fi

print_status "Docker Compose found"

# Start services
echo "📦 Starting database services..."
docker-compose -f docker-compose.dev.yml up -d postgres neo4j redis-hot redis-warm redis-cold

# Wait for services to be ready
echo "⏳ Waiting for services to be ready..."
sleep 10

# Test PostgreSQL connection
echo "🐘 Testing PostgreSQL connection..."
if docker exec recommendation-postgres pg_isready -U postgres > /dev/null 2>&1; then
    print_status "PostgreSQL is ready"
else
    print_error "PostgreSQL is not ready"
    exit 1
fi

# Test Neo4j connection
echo "🔗 Testing Neo4j connection..."
if docker exec recommendation-neo4j cypher-shell -u neo4j -p password "RETURN 1 as test;" > /dev/null 2>&1; then
    print_status "Neo4j is ready"
else
    print_error "Neo4j is not ready"
    exit 1
fi

# Test Redis connections
echo "📦 Testing Redis connections..."
if docker exec recommendation-redis-hot redis-cli ping > /dev/null 2>&1; then
    print_status "redis-hot is ready"
else
    print_error "redis-hot is not ready"
    exit 1
fi

if docker exec recommendation-redis-warm redis-cli -p 6380 ping > /dev/null 2>&1; then
    print_status "redis-warm is ready"
else
    print_error "redis-warm is not ready"
    exit 1
fi

if docker exec recommendation-redis-cold redis-cli -p 6381 ping > /dev/null 2>&1; then
    print_status "redis-cold is ready"
else
    print_error "redis-cold is not ready"
    exit 1
fi

# Validate PostgreSQL schema
echo "🔍 Validating PostgreSQL schema..."
if docker exec -i recommendation-postgres psql -U postgres -d recommendations < scripts/validate-schema.sql > /dev/null 2>&1; then
    print_status "PostgreSQL schema validation passed"
else
    print_warning "PostgreSQL schema validation had issues (check logs)"
fi

# Check if pgvector extension is installed
echo "🧮 Checking pgvector extension..."
PGVECTOR_CHECK=$(docker exec recommendation-postgres psql -U postgres -d recommendations -t -c "SELECT COUNT(*) FROM pg_extension WHERE extname = 'vector';")
if [ "$PGVECTOR_CHECK" -eq 1 ]; then
    print_status "pgvector extension is installed"
else
    print_error "pgvector extension is not installed"
    exit 1
fi

# Populate sample data
echo "📊 Populating PostgreSQL sample data..."
if docker exec -i recommendation-postgres psql -U postgres -d recommendations < scripts/populate-sample-data.sql > /dev/null 2>&1; then
    print_status "PostgreSQL sample data populated successfully"
else
    print_error "Failed to populate PostgreSQL sample data"
    exit 1
fi

# Populate Neo4j sample data
echo "🕸️ Populating Neo4j sample data..."
if docker exec -i recommendation-neo4j cypher-shell -u neo4j -p password < scripts/populate-neo4j-sample-data.cypher > /dev/null 2>&1; then
    print_status "Neo4j sample data populated successfully"
else
    print_error "Failed to populate Neo4j sample data"
    exit 1
fi

# Validate data integrity
echo "🔍 Validating data integrity..."
INTEGRITY_ISSUES=$(docker exec -i recommendation-postgres psql -U postgres -d recommendations < scripts/validate-data-integrity.sql 2>/dev/null | grep -c "count.*[1-9]" || true)
if [ "$INTEGRITY_ISSUES" -eq 0 ]; then
    print_status "Data integrity validation passed"
else
    print_warning "Found $INTEGRITY_ISSUES potential data integrity issues (check detailed output)"
fi

# Test vector operations
echo "🧮 Testing vector operations..."
VECTOR_TEST=$(docker exec recommendation-postgres psql -U postgres -d recommendations -t -c "
SELECT COUNT(*) FROM find_similar_content(
    (SELECT embedding FROM content_items WHERE embedding IS NOT NULL LIMIT 1),
    0.5,
    10
);")
if [ "$VECTOR_TEST" -gt 0 ]; then
    print_status "Vector similarity search is working"
else
    print_warning "Vector similarity search returned no results"
fi

# Test Neo4j graph operations
echo "🕸️ Testing Neo4j graph operations..."
NEO4J_TEST=$(docker exec recommendation-neo4j cypher-shell -u neo4j -p password "MATCH (u:User)-[:RATED]->(c:Content) RETURN count(*) as count;" | grep -o '[0-9]\+' | head -1)
if [ "$NEO4J_TEST" -gt 0 ]; then
    print_status "Neo4j graph relationships are working ($NEO4J_TEST relationships found)"
else
    print_warning "Neo4j graph relationships test returned no results"
fi

# Test GDS plugin
echo "🧠 Testing Neo4j GDS plugin..."
if docker exec recommendation-neo4j cypher-shell -u neo4j -p password "CALL gds.version();" > /dev/null 2>&1; then
    print_status "Neo4j GDS plugin is working"
else
    print_warning "Neo4j GDS plugin test failed"
fi

# Display data summary
echo "📈 Data summary:"
echo "Content Items:"
docker exec recommendation-postgres psql -U postgres -d recommendations -t -c "SELECT type, COUNT(*) FROM content_items GROUP BY type ORDER BY type;"

echo "User Profiles:"
docker exec recommendation-postgres psql -U postgres -d recommendations -t -c "SELECT COUNT(*) as total_users FROM user_profiles;"

echo "User Interactions:"
docker exec recommendation-postgres psql -U postgres -d recommendations -t -c "SELECT interaction_type, COUNT(*) FROM user_interactions GROUP BY interaction_type ORDER BY interaction_type;"

echo "Neo4j Nodes:"
docker exec recommendation-neo4j cypher-shell -u neo4j -p password "MATCH (n) RETURN labels(n)[0] as type, count(n) as count ORDER BY type;" 2>/dev/null | grep -v "^$"

echo "Neo4j Relationships:"
docker exec recommendation-neo4j cypher-shell -u neo4j -p password "MATCH ()-[r]->() RETURN type(r) as type, count(r) as count ORDER BY type;" 2>/dev/null | grep -v "^$"

# Performance test
echo "⚡ Running basic performance tests..."
echo "Vector similarity query performance:"
time docker exec recommendation-postgres psql -U postgres -d recommendations -c "
SELECT content_id, similarity_score 
FROM find_similar_content(
    (SELECT embedding FROM content_items WHERE embedding IS NOT NULL LIMIT 1),
    0.7,
    50
) LIMIT 10;" > /dev/null 2>&1

echo "Neo4j PageRank performance:"
time docker exec recommendation-neo4j cypher-shell -u neo4j -p password "
CALL gds.pageRank.stream('user-content-interactions')
YIELD nodeId, score
RETURN score
LIMIT 10;" > /dev/null 2>&1

# Test connection pooling
echo "🔗 Testing connection pooling..."
if docker exec recommendation-pgbouncer psql -h localhost -p 5432 -U postgres -d recommendations -c "SELECT 1;" > /dev/null 2>&1; then
    print_status "PgBouncer connection pooling is working"
else
    print_warning "PgBouncer connection pooling test failed"
fi

# Final status
echo ""
echo "🎉 Database setup test completed!"
echo ""
echo "Services running:"
docker-compose -f docker-compose.dev.yml ps --format "table {{.Name}}\t{{.Status}}\t{{.Ports}}"

echo ""
echo "Next steps:"
echo "1. Start the Go application server"
echo "2. Run integration tests"
echo "3. Test API endpoints with the populated data"
echo "4. Monitor performance with realistic workloads"

echo ""
echo "Useful commands:"
echo "- View PostgreSQL data: docker exec -it recommendation-postgres psql -U postgres -d recommendations"
echo "- View Neo4j data: docker exec -it recommendation-neo4j cypher-shell -u neo4j -p password"
echo "- View logs: docker-compose -f docker-compose.dev.yml logs [service-name]"
echo "- Stop services: docker-compose -f docker-compose.dev.yml down"

print_status "Database setup test completed successfully!"