# Recommendation Engine

A comprehensive, multi-modal recommendation system built with Go, designed for e-commerce product recommendations with support for videos, articles, and other content types.

## Features

- **Multi-modal Content Processing**: Text and image embeddings with fusion capabilities
- **Hybrid Recommendation Strategies**: Content-based, collaborative filtering, and graph-based approaches
- **Real-time Learning**: Continuous feedback integration and model updates
- **High Performance**: Multi-layer caching with intelligent TTL management
- **Scalable Architecture**: Monolithic design with clear service boundaries for easy scaling
- **Comprehensive Monitoring**: Health checks, metrics collection, and observability

## Quick Start

### Prerequisites

- Go 1.21+
- Docker and Docker Compose
- 8GB+ RAM recommended

### Setup

1. Clone the repository and navigate to the project directory

2. Run the setup script:
```bash
./scripts/setup.sh
```

3. Download ONNX models (see [Model Setup](#model-setup))

4. Start the application:
```bash
go run cmd/server/main.go
```

5. Check system health:
```bash
curl http://localhost:8080/health
```

## Architecture

The system follows a monolithic architecture with clear service boundaries:

```
┌─────────────────────────────────────────┐
│           Go Monolith                   │
├─────────────────────────────────────────┤
│ • HTTP Router (Gin)                     │
│ • Authentication & Rate Limiting        │
│ • Content Ingestion Pipeline           │
│ • ML Models & Embedding Generation      │
│ • Recommendation Algorithms             │
│ • Multi-layer Caching                   │
└─────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────┐
│            Data Layer                   │
├─────────────────────────────────────────┤
│ • PostgreSQL + pgvector                 │
│ • Neo4j + Graph Data Science            │
│ • Redis Cluster (3 tiers)               │
│ • Kafka (optional)                      │
└─────────────────────────────────────────┘
```

## API Endpoints

### Health Check
- `GET /health` - System health status

### Content Management
- `POST /api/v1/content` - Create single content item
- `POST /api/v1/content/batch` - Bulk content creation
- `GET /api/v1/content/jobs/:jobId` - Check processing job status

### User Interactions
- `POST /api/v1/interactions/explicit` - Record explicit feedback (ratings, likes)
- `POST /api/v1/interactions/implicit` - Record implicit feedback (clicks, views)
- `POST /api/v1/interactions/batch` - Bulk interaction recording

### Recommendations
- `GET /api/v1/recommendations/:userId` - Get personalized recommendations
- `POST /api/v1/recommendations/batch` - Bulk recommendation requests

### User Management
- `GET /api/v1/users/:userId/interactions` - Get user interaction history

## Authentication

The system supports two authentication methods:

### API Key Authentication
```bash
curl -H "Authorization: Bearer your-api-key" \
     -H "X-User-ID: user-uuid" \
     http://localhost:8080/api/v1/recommendations/user-uuid
```

### JWT Token Authentication
```bash
curl -H "Authorization: Bearer jwt-token" \
     http://localhost:8080/api/v1/recommendations/user-uuid
```

## Configuration

Configuration is managed through:
- `config/app.yaml` - Business logic and algorithm parameters
- Environment variables - Secrets and deployment-specific settings

### Key Configuration Options

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
    pagerank:
      enabled: true
      weight: 0.3

  diversity:
    intra_list_diversity: 0.3
    category_max_items: 3
    serendipity_ratio: 0.15
```

## Model Setup

The system requires ONNX models for text and image embeddings:

1. **Text Embeddings**: `all-MiniLM-L6-v2.onnx` (384 dimensions)
2. **Image Embeddings**: `clip-vit-base-patch32.onnx` (512 dimensions)

Download or convert models from Hugging Face and place them in the `models/` directory.

## Development

### Project Structure

```
├── cmd/server/          # Application entry point
├── internal/
│   ├── app/            # Application setup and routing
│   ├── config/         # Configuration management
│   ├── database/       # Database connections
│   ├── handlers/       # HTTP request handlers
│   ├── middleware/     # HTTP middleware
│   └── services/       # Business logic services
├── pkg/models/         # Shared data models
├── config/            # Configuration files
├── scripts/           # Setup and utility scripts
└── docs/              # Documentation
```

### Running Tests

```bash
go test ./...
```

### Database Migrations

Database schemas are automatically applied when starting the Docker services. See `scripts/init-postgres.sql` and `scripts/init-neo4j.cypher`.

## Monitoring

### Health Checks

The system implements tiered health checks:
- **Critical**: PostgreSQL, Redis Hot
- **Non-critical**: Neo4j, Redis Warm/Cold, Kafka

### Metrics

Prometheus metrics are available at `/metrics`:
- Request latency and throughput
- Algorithm performance
- Cache hit rates
- Database connection pool usage

### Dashboards

- **Grafana**: http://localhost:3000 (admin/admin)
- **Neo4j Browser**: http://localhost:7474 (neo4j/password)
- **Prometheus**: http://localhost:9090

## Rate Limiting

Rate limits are enforced per user tier:
- **Free**: 1,000 requests/hour
- **Premium**: 10,000 requests/hour
- **Enterprise**: 100,000 requests/hour

## Caching Strategy

Multi-tier Redis caching:
- **Hot Cache**: User sessions, rate limiting (2GB, LRU)
- **Warm Cache**: Recommendations, metadata (1GB, LRU)
- **Cold Cache**: Embeddings, long-term data (4GB, LFU)

## Contributing

1. Follow Go best practices and conventions
2. Add tests for new functionality
3. Update documentation for API changes
4. Ensure all health checks pass

## License

[Add your license information here]