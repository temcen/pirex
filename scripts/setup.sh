#!/bin/bash

# Setup script for recommendation engine development environment

set -e

echo "Setting up recommendation engine development environment..."

# Create necessary directories
echo "Creating directories..."
mkdir -p models
mkdir -p logs
mkdir -p data

# Download ONNX models (placeholder URLs - replace with actual model URLs)
echo "Downloading ONNX models..."
if [ ! -f "models/all-MiniLM-L6-v2.onnx" ]; then
    echo "Note: Please download all-MiniLM-L6-v2.onnx model to models/ directory"
    echo "You can convert from Hugging Face: https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2"
fi

if [ ! -f "models/clip-vit-base-patch32.onnx" ]; then
    echo "Note: Please download clip-vit-base-patch32.onnx model to models/ directory"
    echo "You can convert from Hugging Face: https://huggingface.co/openai/clip-vit-base-patch32"
fi

# Set up environment variables
echo "Setting up environment variables..."
if [ ! -f ".env" ]; then
    cat > .env << EOF
# Database Configuration
DATABASE_URL=postgres://postgres:postgres@localhost:6432/recommendations
NEO4J_URL=bolt://localhost:7687
NEO4J_USERNAME=neo4j
NEO4J_PASSWORD=password

# Redis Configuration
REDIS_HOT_URL=redis://localhost:6379
REDIS_WARM_URL=redis://localhost:6380
REDIS_COLD_URL=redis://localhost:6381

# Kafka Configuration
KAFKA_BROKERS=localhost:9092

# Authentication
JWT_SECRET=your-super-secret-jwt-key-change-this-in-production
API_RATE_LIMIT_DEFAULT=1000

# Server Configuration
SERVER_PORT=8080
SERVER_MODE=development

# Logging
LOG_LEVEL=info
LOG_FORMAT=text
EOF
    echo "Created .env file with default values"
fi

# Initialize Go modules
echo "Initializing Go modules..."
go mod tidy

# Start Docker services
echo "Starting Docker services..."
docker-compose -f docker-compose.dev.yml up -d

# Wait for services to be ready
echo "Waiting for services to start..."
sleep 30

# Check service health
echo "Checking service health..."
docker-compose -f docker-compose.dev.yml ps

echo "Setup complete!"
echo ""
echo "Next steps:"
echo "1. Download ONNX models to the models/ directory"
echo "2. Run 'go run cmd/server/main.go' to start the server"
echo "3. Visit http://localhost:8080/health to check system health"
echo "4. Visit http://localhost:3000 for Grafana dashboard (admin/admin)"
echo "5. Visit http://localhost:7474 for Neo4j browser (neo4j/password)"