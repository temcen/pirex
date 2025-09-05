# Design Document

## Overview

The recommendation engine follows a monolithic architecture with clear service boundaries, designed for flexibility and extensibility. The system processes multi-modal content (text and images initially) through a sophisticated pipeline that combines semantic similarity, collaborative filtering, and graph-based signals to generate personalized recommendations.

The architecture emphasizes:
- **Multi-modal processing**: Text and visual embeddings with fusion capabilities
- **Hybrid recommendation strategies**: Content-based, collaborative, and graph-based approaches
- **Performance optimization**: Multi-layer caching with intelligent TTL management
- **Real-time learning**: Continuous feedback integration and model updates
- **Extensibility**: Plugin architecture for future advanced features (GNN, advanced diversity)

## Technology Stack

### Core Application Stack

**Monolithic Architecture**: Single Go application with separated data layer

**Single Go Application**:
- **Go 1.21+** - All services in one binary for simplicity
- **Gin** - HTTP framework for REST and GraphQL APIs
- **Embedded components**: All services run as goroutines within single process

**Monolithic Structure**:
```
┌─────────────────────────────────────────┐
│           Go Monolith                   │
├─────────────────────────────────────────┤
│ • HTTP Router (Gin)                     │
│ • GraphQL Handler                       │
│ • Authentication Middleware             │
│ • Content Ingestion Handler             │
│ • User Interaction Handler              │
│ • Data Preprocessor (goroutines)        │
│ • Embedding Service (goroutines)        │
│ • Computation Engine (goroutines)       │
│ • Recommendation Orchestrator           │
│ • Ranking Service                       │
│ • Diversity Filter                      │
│ • Cache Manager                         │
│ • Background Job Processor              │
└─────────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────┐
│            Data Layer                   │
├─────────────────────────────────────────┤
│ • PostgreSQL + pgvector                 │
│ • Neo4j                                 │
│ • Redis Cluster (3 instances)           │
│ • Kafka (optional for async)            │
└─────────────────────────────────────────┘
```

**Monolithic Advantages**:
- **Simple deployment**: Single binary, single container
- **Easy development**: No inter-service communication complexity
- **Faster iteration**: All code in one repository
- **Simplified debugging**: Single process to debug and monitor
- **No network latency**: In-process function calls instead of HTTP/gRPC
- **Easier testing**: Integration tests without service orchestration

### Database and Storage

**Primary Database**: 
- **PostgreSQL 15+** with **pgvector extension** - Vector similarity search with ACID compliance
- **Connection pooling**: pg-pool for efficient connection management

**Graph Database**: 
- **Neo4j 5.x** - Advanced graph algorithms and relationship modeling
- **Neo4j JavaScript Driver** - Official driver with connection pooling

**Caching Layer**: 
- **Redis 7.x Cluster** - Multi-tier caching with different configurations per tier
- **ioredis** - Redis client with cluster support and connection pooling

### Message Bus and Async Processing

**Message Broker Strategy**:
- **Apache Kafka** - For ingestion layer → processing layer communication and external integrations
- **Go Channels** - For all internal component communication within the monolith
- **segmentio/kafka-go** - Go Kafka client for producer/consumer operations

**Background Jobs**:
- **Go Goroutines with Worker Pools** - Native Go concurrency for background tasks
- **Pluggable architecture** - Designed for future migration to Redis queues if needed
- **Scheduled tasks** - Using Go's time.Ticker for model updates and cache warming

```go
// Background job processing with worker pools
type JobProcessor struct {
    workers    int
    jobQueue   chan Job
    workerPool chan chan Job
    quit       chan bool
}

type Job interface {
    Execute() error
    GetType() string
    GetPriority() int
}

// Future migration path to Redis queues
type JobQueue interface {
    Enqueue(job Job) error
    Dequeue() (Job, error)
    // Can be implemented with Redis later
}
```

### Machine Learning and Embeddings

### Machine Learning Stack (Go Backend)

**Text Embeddings**:
- **Primary**: **onnxruntime-go** with pre-trained ONNX models
  - Models: `all-MiniLM-L6-v2`, `all-mpnet-base-v2` (Sentence-BERT)
  - Format: ONNX for cross-platform compatibility and performance
- **Alternative**: **go-sentence-transformers** (native Go implementation)
- **Fallback**: HTTP calls to Hugging Face Inference API

**Image Embeddings**:
- **Primary**: **onnxruntime-go** with CLIP or ResNet ONNX models
  - Models: `clip-vit-base-patch32`, `resnet50`
  - Preprocessing: Go image processing libraries
- **Alternative**: **TensorFlow Go** (more complex setup)

**Vector Operations & ML Algorithms**:
- **gonum.org/v1/gonum** - High-performance matrix operations and linear algebra
- **gonum.org/v1/gonum/stat** - Statistical functions for recommendation algorithms
- **Custom implementations**: Collaborative filtering, PageRank, similarity calculations

**Model Management**:
```go
type MLStack struct {
    // Text embedding models
    textModels map[string]*onnxruntime.Session
    
    // Image embedding models  
    imageModels map[string]*onnxruntime.Session
    
    // Model metadata and versioning
    modelRegistry *ModelRegistry
    
    // Performance optimization
    modelPool sync.Pool
    
    // Vector operations
    vectorEngine *gonum.VectorEngine
}

// Example text embedding
func (ml *MLStack) GenerateTextEmbedding(text string) ([]float32, error) {
    session := ml.textModels["sentence-bert"]
    
    // Tokenize and preprocess
    tokens := ml.tokenize(text)
    
    // Run inference
    outputs, err := session.Run([]onnxruntime.Value{tokens})
    if err != nil {
        return nil, err
    }
    
    // Extract embeddings (768 dimensions)
    embeddings := outputs[0].GetTensorData().([]float32)
    
    // L2 normalize
    return ml.normalize(embeddings), nil
}

// Example collaborative filtering
func (ml *MLStack) ComputeUserSimilarity(user1, user2 []float32) float32 {
    // Cosine similarity using gonum
    return floats.Dot(user1, user2) / 
           (math.Sqrt(floats.Dot(user1, user1)) * 
            math.Sqrt(floats.Dot(user2, user2)))
}
```

**Recommendation Algorithms (Native Go)**:
```go
// Content-based filtering
func (ml *MLStack) ContentBasedRecommendations(
    userProfile []float32, 
    candidates []ContentItem) []ScoredItem {
    
    var results []ScoredItem
    for _, item := range candidates {
        similarity := ml.CosineSimilarity(userProfile, item.Embedding)
        results = append(results, ScoredItem{
            ItemID: item.ID,
            Score:  similarity,
        })
    }
    
    // Sort by score descending
    sort.Slice(results, func(i, j int) bool {
        return results[i].Score > results[j].Score
    })
    
    return results
}

// Collaborative filtering using matrix factorization
func (ml *MLStack) CollaborativeFiltering(
    userID string, 
    interactions []Interaction) []ScoredItem {
    
    // Build user-item matrix
    matrix := ml.buildUserItemMatrix(interactions)
    
    // Apply SVD using gonum
    var svd mat.SVD
    svd.Factorize(matrix, mat.SVDThin)
    
    // Generate recommendations
    return ml.generateFromFactorization(&svd, userID)
}
```

**Model Loading and Caching**:
```go
type ModelRegistry struct {
    models map[string]*ModelInfo
    cache  *sync.Map // Thread-safe model cache
}

type ModelInfo struct {
    Name        string
    Version     string
    Path        string
    Dimensions  int
    ModelType   string // "text", "image", "multimodal"
    LoadedAt    time.Time
    Performance ModelMetrics
}

// Lazy loading with caching
func (mr *ModelRegistry) LoadModel(name string) (*onnxruntime.Session, error) {
    if cached, ok := mr.cache.Load(name); ok {
        return cached.(*onnxruntime.Session), nil
    }
    
    modelInfo := mr.models[name]
    session, err := onnxruntime.NewSession(modelInfo.Path)
    if err != nil {
        return nil, err
    }
    
    mr.cache.Store(name, session)
    return session, nil
}
```

### Frontend Stack (Node.js)

**Frontend Application** (separate from Go backend):
- **Node.js 18+ with TypeScript** - Simple web application and admin dashboard
- **Express.js** - Lightweight web server for serving static files and API proxy
- **Vanilla JavaScript/TypeScript** - No complex framework, just DOM manipulation
- **Bootstrap 5** - Simple CSS framework for quick UI development
- **Chart.js** - Simple charting library for analytics

**Frontend Architecture**:
```
┌─────────────────────────────────────────┐
│           Node.js Frontend              │
├─────────────────────────────────────────┤
│ • Express.js Server                     │
│ • Static HTML/CSS/JS                    │
│ • Simple DOM manipulation               │
│ • Bootstrap UI Components               │
│ • Chart.js for Analytics                │
│ • Fetch API for Backend Communication   │
└─────────────────────────────────────────┘
                    │ HTTP/REST
                    ▼
┌─────────────────────────────────────────┐
│           Go Backend                    │
│        (Recommendation Engine)          │
└─────────────────────────────────────────┘
```

**Frontend Package.json** (Minimal):
```json
{
  "name": "recommendation-frontend",
  "dependencies": {
    "express": "^4.18.0",
    "express-static": "^1.2.6",
    "http-proxy-middleware": "^2.0.6"
  },
  "devDependencies": {
    "typescript": "^5.1.0",
    "@types/express": "^4.17.0",
    "nodemon": "^3.0.0"
  }
}
```

**Simple Frontend Structure**:
```
frontend/
├── server.js              # Express server
├── public/
│   ├── index.html         # Main dashboard
│   ├── admin.html         # Admin interface  
│   ├── css/
│   │   └── bootstrap.min.css
│   ├── js/
│   │   ├── dashboard.js   # Dashboard logic
│   │   ├── admin.js       # Admin logic
│   │   └── chart.min.js   # Charts
│   └── assets/
└── package.json
```

**Communication Pattern**:
- **Go Backend**: Serves GraphQL/REST APIs on port 8080
- **Node.js Frontend**: Serves web UI on port 3000, proxies API calls to Go backend
- **Real-time updates**: WebSocket connection from frontend to Go backend
- **Admin features**: Built in Next.js for managing content, monitoring, and analytics

### API and Web Framework

**API Layer**:
- **Gin** - HTTP framework for REST and GraphQL endpoints
- **JWT middleware** - Token-based authentication with user sessions
- **Rate limiting middleware** - Per API key/user rate limiting
- **Security middleware** - CORS, request validation, sanitization

**Authentication & Authorization**:
```go
type AuthService struct {
    jwtSecret []byte
    tokenTTL  time.Duration
}

type JWTClaims struct {
    UserID   string `json:"user_id"`
    APIKey   string `json:"api_key,omitempty"`
    UserTier string `json:"user_tier"` // For rate limiting
    jwt.StandardClaims
}

// JWT token with user sessions
func (as *AuthService) GenerateToken(userID, userTier string) (string, error) {
    claims := JWTClaims{
        UserID:   userID,
        UserTier: userTier,
        StandardClaims: jwt.StandardClaims{
            ExpiresAt: time.Now().Add(as.tokenTTL).Unix(),
            IssuedAt:  time.Now().Unix(),
        },
    }
    
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(as.jwtSecret)
}

// Per API key/user rate limiting
type RateLimiter struct {
    limits map[string]int // user_tier -> requests_per_minute
    store  *redis.Client
}

func (rl *RateLimiter) CheckLimit(userID, userTier string) bool {
    key := fmt.Sprintf("rate_limit:user:%s", userID)
    limit := rl.limits[userTier]
    
    // Sliding window rate limiting
    return rl.slidingWindowCheck(key, limit, time.Minute)
}
```

### Monitoring and Observability

**Application Monitoring**:
- **Prometheus** - Metrics collection and alerting
- **Grafana** - Visualization and dashboards
- **Winston** - Structured logging
- **Morgan** - HTTP request logging

**Performance Monitoring**:
- **New Relic** or **DataDog** - APM and infrastructure monitoring
- **Redis Insight** - Redis cluster monitoring
- **Neo4j Browser** - Graph database monitoring

### Development and Deployment

**Development Tools**:
- **TypeScript 5.x** - Static typing and modern JavaScript features
- **ESLint + Prettier** - Code linting and formatting
- **Jest** - Unit and integration testing
- **Supertest** - API endpoint testing

**Build and Deployment**:
- **Docker** - Containerization for all services
- **Docker Compose** - Local development environment
- **GitHub Actions** - CI/CD pipeline
- **Kubernetes** - Production orchestration (optional)

### Infrastructure Dependencies

**Required Services**:
```yaml
services:
  # Single Application
  recommendation-engine:
    build: .
    image: golang:1.21-alpine
    ports: ["8080:8080"]
    environment:
      - DATABASE_URL=postgres://user:pass@postgres:5432/recommendations
      - NEO4J_URL=bolt://neo4j:7687
      - REDIS_HOT_URL=redis://redis-hot:6379
      - REDIS_WARM_URL=redis://redis-warm:6379
      - REDIS_COLD_URL=redis://redis-cold:6379
    
  # Data Layer Only
  postgres:
    image: postgres:15
    environment:
      - POSTGRES_DB=recommendations
      - POSTGRES_USER=user
      - POSTGRES_PASSWORD=pass
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./init.sql:/docker-entrypoint-initdb.d/init.sql
    
  neo4j:
    image: neo4j:5.12
    environment:
      - NEO4J_AUTH=neo4j/password
      - NEO4J_PLUGINS=["graph-data-science"]
    volumes:
      - neo4j_data:/data
    
  redis-hot:
    image: redis:7-alpine
    command: redis-server --maxmemory 8gb --maxmemory-policy allkeys-lru
    
  redis-warm:
    image: redis:7-alpine
    command: redis-server --maxmemory 4gb --maxmemory-policy allkeys-lru
    
  redis-cold:
    image: redis:7-alpine
    command: redis-server --maxmemory 16gb --maxmemory-policy allkeys-lfu
    
  # Optional: Message queue for async processing
  kafka:
    image: confluentinc/cp-kafka:latest
    environment:
      - KAFKA_ZOOKEEPER_CONNECT=zookeeper:2181
      - KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://kafka:9092
    depends_on: [zookeeper]
    
  zookeeper:
    image: confluentinc/cp-zookeeper:latest
    environment:
      - ZOOKEEPER_CLIENT_PORT=2181
    
  # Monitoring (optional)
  prometheus:
    image: prom/prometheus:latest
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
    
  grafana:
    image: grafana/grafana:latest
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin

volumes:
  postgres_data:
  neo4j_data:
```

### Package Dependencies

**Go Monolith (go.mod)**:
```go
module recommendation-engine

go 1.21

require (
    // HTTP Framework
    github.com/gin-gonic/gin v1.9.1
    
    // Database drivers
    github.com/lib/pq v1.10.9
    github.com/neo4j/neo4j-go-driver/v5 v5.12.0
    github.com/go-redis/redis/v8 v8.11.5
    
    // Message bus
    github.com/segmentio/kafka-go v0.4.42
    
    // ML and vector operations
    github.com/onnx/onnx-go v1.9.0
    gonum.org/v1/gonum v0.14.0
    
    // Authentication
    github.com/golang-jwt/jwt/v4 v4.5.0
    golang.org/x/crypto v0.12.0
    
    // Configuration
    github.com/spf13/viper v1.16.0
    
    // Monitoring
    github.com/prometheus/client_golang v1.16.0
    
    // Logging
    github.com/sirupsen/logrus v1.9.3
    
    // Testing
    github.com/stretchr/testify v1.8.4
)
```

### Rationale for Monolithic Architecture

**Monolithic Go Application Benefits**:
- **Simplicity**: Single binary deployment, no service orchestration complexity
- **Performance**: In-process function calls, no network latency between components
- **Development speed**: All code in one repository, easier debugging and testing
- **Resource efficiency**: Shared memory, connection pools, and caches
- **Easier scaling**: Scale entire application as one unit initially

**Go Language Advantages**:
- **Performance**: 10-100x faster than Node.js for ML computations and vector operations
- **Concurrency**: Goroutines efficiently handle thousands of concurrent requests
- **Memory efficiency**: Lower memory footprint (~50MB vs ~200MB for Node.js)
- **Static compilation**: Single binary with no runtime dependencies
- **Type safety**: Compile-time guarantees for complex recommendation algorithms
- **Rich ecosystem**: Excellent libraries for databases, HTTP, and ML

**Internal Architecture**:
```go
// Main application structure
type RecommendationEngine struct {
    router           *gin.Engine
    dbPool          *sql.DB
    neo4jDriver     neo4j.Driver
    redisClients    map[string]*redis.Client
    embeddingService *EmbeddingService
    computeService   *ComputationService
    cacheManager     *CacheManager
}

// All services run as goroutines within single process
func (re *RecommendationEngine) Start() {
    // Start background workers
    go re.embeddingService.ProcessQueue()
    go re.computeService.UpdateModels()
    go re.cacheManager.EvictExpired()
    
    // Start HTTP server
    re.router.Run(":8080")
}
```

**Migration Path**:
- **Phase 1**: Start with monolith for rapid development and deployment
- **Phase 2**: Extract high-load components (embedding service) if needed
- **Phase 3**: Microservices only when team size and complexity justify it

**Performance Expectations**:
```
Single Go Monolith:
- Embedding Generation: ~100ms for 1000 items
- Vector Search: ~10ms for similarity queries  
- Memory Usage: ~100MB total
- Concurrent Users: 10,000+ with proper connection pooling
- Deployment: Single 20MB Docker container
```

This monolithic approach provides the best balance of simplicity, performance, and maintainability for getting started quickly.

## Configuration Management

### Configuration Strategy
**Environment Variables** for secrets and deployment-specific settings:
```bash
# Database connections
DATABASE_URL=postgres://user:pass@localhost:5432/recommendations
NEO4J_URL=bolt://localhost:7687
REDIS_HOT_URL=redis://localhost:6379
REDIS_WARM_URL=redis://localhost:6380
REDIS_COLD_URL=redis://localhost:6381

# Security
JWT_SECRET=your-secret-key
API_RATE_LIMIT_DEFAULT=1000

# External services
KAFKA_BROKERS=localhost:9092
```

**Config Files** for business logic and algorithm parameters:
```yaml
# config/app.yaml
recommendation:
  algorithms:
    semantic_search:
      enabled: true
      weight: 0.4
      similarity_threshold: 0.7
    collaborative_filtering:
      enabled: true
      weight: 0.3
      min_similarity: 0.5
    pagerank:
      enabled: true
      weight: 0.3
      damping_factor: 0.85
      iterations: 20

  diversity:
    intra_list_diversity: 0.3
    category_max_items: 3
    serendipity_ratio: 0.15

  caching:
    embeddings_ttl: "24h"
    recommendations_ttl: "15m"
    metadata_ttl: "1h"
    graph_results_ttl: "30m"

models:
  text_embedding:
    model_path: "./models/all-MiniLM-L6-v2.onnx"
    dimensions: 768
  image_embedding:
    model_path: "./models/clip-vit-base-patch32.onnx"
    dimensions: 512
```

### Configuration Loading
```go
type Config struct {
    Database    DatabaseConfig    `mapstructure:"database"`
    Redis       RedisConfig       `mapstructure:"redis"`
    Kafka       KafkaConfig       `mapstructure:"kafka"`
    Auth        AuthConfig        `mapstructure:"auth"`
    Algorithms  AlgorithmConfig   `mapstructure:"recommendation"`
    Models      ModelConfig       `mapstructure:"models"`
}

func LoadConfig() (*Config, error) {
    viper.SetConfigName("app")
    viper.SetConfigType("yaml")
    viper.AddConfigPath("./config")
    viper.AddConfigPath(".")
    
    // Environment variable overrides
    viper.AutomaticEnv()
    viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
    
    if err := viper.ReadInConfig(); err != nil {
        return nil, err
    }
    
    var config Config
    return &config, viper.Unmarshal(&config)
}
```

## External System Integration

### Plugin Architecture for External Systems
```go
type ExternalSystemPlugin interface {
    Name() string
    Connect(config map[string]interface{}) error
    EnrichUserProfile(userID string) (*UserEnrichment, error)
    IsHealthy() bool
}

type ExternalSystemManager struct {
    plugins map[string]ExternalSystemPlugin
    cache   *redis.Client
}

// CRM Integration Plugin
type CRMPlugin struct {
    apiURL    string
    apiKey    string
    client    *http.Client
    lastSync  time.Time
}

func (crm *CRMPlugin) EnrichUserProfile(userID string) (*UserEnrichment, error) {
    // Try cache first
    if cached := crm.getCachedData(userID); cached != nil {
        return cached, nil
    }
    
    // Fetch from CRM API with fallback
    data, err := crm.fetchFromAPI(userID)
    if err != nil {
        // Use cached data from previous successful call
        return crm.getFallbackData(userID), nil
    }
    
    // Cache successful response
    crm.cacheData(userID, data)
    return data, nil
}

// Social Media Plugin
type SocialMediaPlugin struct {
    platforms map[string]SocialPlatformClient
}

func (sm *SocialMediaPlugin) EnrichUserProfile(userID string) (*UserEnrichment, error) {
    enrichment := &UserEnrichment{}
    
    for platform, client := range sm.platforms {
        if data, err := client.GetUserInterests(userID); err == nil {
            enrichment.Interests = append(enrichment.Interests, data.Interests...)
        }
        // Continue with other platforms even if one fails
    }
    
    return enrichment, nil
}
```

### Fallback Strategy Implementation
```go
type FallbackManager struct {
    cache           *redis.Client
    defaultValues   map[string]interface{}
    degradedMode    bool
}

func (fm *FallbackManager) HandleExternalSystemFailure(systemName string, userID string) *UserEnrichment {
    // Try cached data first
    if cached := fm.getCachedEnrichment(systemName, userID); cached != nil {
        log.Warn("Using cached data for external system", "system", systemName)
        return cached
    }
    
    // Use default values
    log.Warn("Using default values for external system", "system", systemName)
    return &UserEnrichment{
        Demographics: fm.defaultValues["demographics"].(Demographics),
        Interests:    []string{"general"},
        Confidence:   0.3, // Lower confidence for default data
    }
}
```

## Monitoring and Business Metrics

### Metrics Collection in PostgreSQL
```sql
-- Business metrics table
CREATE TABLE recommendation_metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    item_id UUID NOT NULL,
    recommendation_id UUID NOT NULL,
    event_type VARCHAR(50) NOT NULL, -- 'impression', 'click', 'conversion'
    algorithm_used VARCHAR(100) NOT NULL,
    position_in_list INTEGER,
    confidence_score FLOAT,
    timestamp TIMESTAMP DEFAULT NOW(),
    session_id UUID,
    context JSONB
);

-- Aggregated metrics for fast queries
CREATE TABLE daily_metrics_summary (
    date DATE PRIMARY KEY,
    total_recommendations INTEGER,
    total_clicks INTEGER,
    total_conversions INTEGER,
    click_through_rate FLOAT,
    conversion_rate FLOAT,
    avg_confidence_score FLOAT,
    algorithm_performance JSONB,
    created_at TIMESTAMP DEFAULT NOW()
);
```

### Metrics Collection Service
```go
type MetricsCollector struct {
    db     *sql.DB
    buffer chan MetricEvent
}

type MetricEvent struct {
    UserID           string    `json:"user_id"`
    ItemID           string    `json:"item_id"`
    RecommendationID string    `json:"recommendation_id"`
    EventType        string    `json:"event_type"`
    AlgorithmUsed    string    `json:"algorithm_used"`
    PositionInList   int       `json:"position_in_list"`
    ConfidenceScore  float64   `json:"confidence_score"`
    Timestamp        time.Time `json:"timestamp"`
    SessionID        string    `json:"session_id"`
    Context          map[string]interface{} `json:"context"`
}

func (mc *MetricsCollector) RecordEvent(event MetricEvent) {
    select {
    case mc.buffer <- event:
        // Event buffered successfully
    default:
        // Buffer full, log warning but don't block
        log.Warn("Metrics buffer full, dropping event")
    }
}

func (mc *MetricsCollector) processBatch() {
    events := make([]MetricEvent, 0, 100)
    
    // Collect events for batch processing
    for i := 0; i < 100; i++ {
        select {
        case event := <-mc.buffer:
            events = append(events, event)
        case <-time.After(5 * time.Second):
            break // Process what we have
        }
    }
    
    if len(events) > 0 {
        mc.insertBatch(events)
    }
}
```

### Health Check Implementation
```go
type HealthChecker struct {
    db          *sql.DB
    redisClients map[string]*redis.Client
    neo4jDriver neo4j.Driver
    kafkaReader *kafka.Reader
}

type HealthStatus struct {
    Status      string            `json:"status"`
    Timestamp   time.Time         `json:"timestamp"`
    Services    map[string]string `json:"services"`
    Critical    []string          `json:"critical_failures,omitempty"`
    NonCritical []string          `json:"non_critical_failures,omitempty"`
}

func (hc *HealthChecker) CheckHealth() *HealthStatus {
    status := &HealthStatus{
        Timestamp: time.Now(),
        Services:  make(map[string]string),
    }
    
    // Critical dependencies
    criticalServices := map[string]func() error{
        "postgresql": hc.checkPostgreSQL,
        "redis_hot":  func() error { return hc.checkRedis("hot") },
    }
    
    // Non-critical dependencies  
    nonCriticalServices := map[string]func() error{
        "neo4j":      hc.checkNeo4j,
        "redis_warm": func() error { return hc.checkRedis("warm") },
        "redis_cold": func() error { return hc.checkRedis("cold") },
        "kafka":      hc.checkKafka,
    }
    
    // Check critical services
    allCriticalHealthy := true
    for name, checkFunc := range criticalServices {
        if err := checkFunc(); err != nil {
            status.Services[name] = "unhealthy"
            status.Critical = append(status.Critical, name)
            allCriticalHealthy = false
        } else {
            status.Services[name] = "healthy"
        }
    }
    
    // Check non-critical services
    for name, checkFunc := range nonCriticalServices {
        if err := checkFunc(); err != nil {
            status.Services[name] = "unhealthy"
            status.NonCritical = append(status.NonCritical, name)
        } else {
            status.Services[name] = "healthy"
        }
    }
    
    // Overall status
    if allCriticalHealthy {
        if len(status.NonCritical) == 0 {
            status.Status = "healthy"
        } else {
            status.Status = "degraded"
        }
    } else {
        status.Status = "unhealthy"
    }
    
    return status
}
```

## Architecture

### High-Level Architecture

```mermaid
graph TB
    subgraph "Client Layer"
        Client[Client Applications]
        Admin[Admin Dashboard]
    end
    
    subgraph "API Layer"
        GW[API Gateway/BFF]
        Auth[Authentication Service]
        ES[Explanation Service]
    end
    
    subgraph "Ingestion Layer"
        CIA[Content Ingestion API]
        UIA[User Interaction API]
        ESA[External System APIs]
        MB[Message Bus]
    end
    
    subgraph "Processing Layer"
        DP[Data Preprocessor]
        UES[Unified Embedding Service]
    end
    
    subgraph "Storage Layer"
        PG[(PostgreSQL + pgvector)]
        N4J[(Neo4j)]
        CacheMeta[Metadata Cache]
        CacheEmbed[Embeddings Cache]
        CacheGraph[Graph Cache]
        CacheRec[Recommendations Cache]
    end
    
    subgraph "Computation Layer"
        UCS[Unified Computation Service]
        RO[Recommendation Orchestrator]
        RS[Ranking Service]
        DF[Diversity Filter]
    end
    
    subgraph "Monitoring Layer"
        MD[Monitoring Dashboard]
        MLO[ML Model Manager]
        CM[Cache Manager]
    end
    
    Client --> GW
    Admin --> MD
    GW --> Auth
    GW --> ES
    
    CIA --> MB
    UIA --> MB
    ESA --> MB
    MB --> DP
    
    DP --> UES
    UES --> PG
    UES --> CacheEmbed
    
    UCS --> PG
    UCS --> N4J
    UCS --> CacheGraph
    
    RO --> UCS
    RS --> RO
    DF --> RS
    
    GW --> DF
    GW --> CacheRec
```

### Service Communication Patterns

The system uses three primary communication patterns:

1. **Direct Function Calls**: Internal component communication within the Go monolith
2. **Kafka Pub/Sub**: Ingestion layer → Processing layer communication and external integrations
3. **Go Channels**: Background job processing and internal event handling

```go
// Internal component communication
type RecommendationEngine struct {
    // Direct service references
    embeddingService    *EmbeddingService
    computationService  *ComputationService
    cacheManager       *CacheManager
    
    // Kafka for external communication
    kafkaProducer      *kafka.Writer
    kafkaConsumer      *kafka.Reader
    
    // Internal channels for background jobs
    jobQueue           chan Job
    cacheInvalidation  chan CacheEvent
}

// Direct function calls for synchronous operations
func (re *RecommendationEngine) GetRecommendations(userID string) ([]Recommendation, error) {
    // Direct function call - no network overhead
    userProfile := re.embeddingService.GetUserProfile(userID)
    return re.computationService.GenerateRecommendations(userProfile)
}
```

## Components and Interfaces

### 1. Content Ingestion API

**Purpose**: Main entry point for catalog content with validation and normalization.

**Key Interfaces**:
```typescript
interface ContentIngestionAPI {
  ingestContent(content: ContentItem): Promise<IngestionResult>
  validateContent(content: ContentItem): ValidationResult
  getIngestionStatus(jobId: string): IngestionStatus
}

interface ContentItem {
  id: string
  type: 'product' | 'video' | 'article'
  title: string
  description: string
  imageUrls: string[]
  metadata: Record<string, any>
  categories: string[]
}
```

**Design Decisions**:
- RESTful API with JSON payload validation using JSON Schema
- Asynchronous processing through message bus to handle traffic spikes
- Content type abstraction to support multiple domains (e-commerce, media, etc.)

### 2. User Interaction API

**Purpose**: Captures all user behaviors and maintains real-time user profiles.

**Key Interfaces**:
```typescript
interface UserInteractionAPI {
  recordExplicitFeedback(userId: string, itemId: string, feedback: ExplicitFeedback): Promise<void>
  recordImplicitFeedback(userId: string, interaction: ImplicitInteraction): Promise<void>
  getUserProfile(userId: string): Promise<UserProfile>
}

interface ExplicitFeedback {
  type: 'rating' | 'like' | 'dislike' | 'share'
  value: number | boolean
  timestamp: Date
}

interface ImplicitInteraction {
  type: 'click' | 'view' | 'search' | 'browse'
  itemId?: string
  query?: string
  duration?: number
  timestamp: Date
}
```

**Design Decisions**:
- Separate explicit and implicit feedback handling with different processing priorities
- Real-time profile updates for explicit feedback, batched processing for implicit
- Graph relationship updates in Neo4j for user-item-interaction patterns

### 3. Unified Embedding Service

**Purpose**: Multi-modal embedding generation with model versioning and fusion capabilities.

**Key Interfaces**:
```typescript
interface UnifiedEmbeddingService {
  generateTextEmbedding(text: string, model?: string): Promise<number[]>
  generateImageEmbedding(imageUrl: string, model?: string): Promise<number[]>
  generateMultiModalEmbedding(text: string, imageUrl: string): Promise<number[]>
  getModelVersions(): Promise<ModelVersion[]>
  updateModel(modelId: string, version: string): Promise<void>
}

interface ModelVersion {
  id: string
  type: 'text' | 'image' | 'multimodal'
  version: string
  performance: ModelMetrics
  isActive: boolean
}
```

**Design Decisions**:
- Pluggable model architecture supporting BERT/RoBERTa for text, ResNet/ViT for images
- Multi-modal fusion using late fusion (concatenation) initially, with hooks for early fusion
- Model versioning with A/B testing capabilities for gradual rollouts
- Embedding dimensionality standardization (768 dimensions) for consistency

### 4. Unified Computation Service

**Purpose**: Executes multiple recommendation algorithms with specific implementations.

**Algorithm Implementations**:

#### 4.1 Semantic Search Algorithm
```go
type SemanticSearchAlgorithm struct {
    vectorDB *pgxpool.Pool
    dimension int
}

func (s *SemanticSearchAlgorithm) Search(queryEmbedding []float32, filters SearchFilters) ([]ScoredItem, error) {
    // Use pgvector for cosine similarity search
    query := `
        SELECT 
            content_id,
            title,
            1 - (text_embedding <=> $1) as similarity_score,
            metadata
        FROM content_items 
        WHERE 
            ($2::text IS NULL OR type = $2) AND
            ($3::text[] IS NULL OR categories && $3) AND
            is_active = true
        ORDER BY text_embedding <=> $1 
        LIMIT $4
    `
    
    rows, err := s.vectorDB.Query(context.Background(), query, 
        pgvector.NewVector(queryEmbedding),
        filters.ContentType,
        pq.Array(filters.Categories),
        filters.Limit)
    
    // Process results and return scored items
    return s.processResults(rows)
}
```

#### 4.2 Collaborative Filtering Algorithm
```go
type CollaborativeFilteringAlgorithm struct {
    neo4jDriver neo4j.Driver
    minSimilarity float64
}

func (c *CollaborativeFilteringAlgorithm) FindSimilarUsers(userID string) ([]SimilarUser, error) {
    // Find users with similar interaction patterns
    cypher := `
        MATCH (u1:User {user_id: $userId})-[r1:RATED]->(item:Content)<-[r2:RATED]-(u2:User)
        WHERE r1.rating >= 4 AND r2.rating >= 4 AND u1 <> u2
        WITH u2, count(item) as shared_items, 
             collect({item: item.content_id, r1: r1.rating, r2: r2.rating}) as ratings
        WHERE shared_items >= 3
        
        // Calculate Pearson correlation
        WITH u2, shared_items,
             reduce(sum = 0.0, r in ratings | sum + r.r1 * r.r2) as dot_product,
             reduce(sum1 = 0.0, r in ratings | sum1 + r.r1 * r.r1) as sum1_sq,
             reduce(sum2 = 0.0, r in ratings | sum2 + r.r2 * r.r2) as sum2_sq,
             reduce(sum1 = 0.0, r in ratings | sum1 + r.r1) as sum1,
             reduce(sum2 = 0.0, r in ratings | sum2 + r.r2) as sum2
        
        WITH u2, shared_items,
             (dot_product - (sum1 * sum2 / shared_items)) / 
             sqrt((sum1_sq - sum1^2/shared_items) * (sum2_sq - sum2^2/shared_items)) as correlation
        
        WHERE correlation > $minSimilarity
        RETURN u2.user_id as user_id, correlation, shared_items
        ORDER BY correlation DESC
        LIMIT 50
    `
    
    return c.executeCypher(cypher, map[string]interface{}{
        "userId": userID,
        "minSimilarity": c.minSimilarity,
    })
}

func (c *CollaborativeFilteringAlgorithm) GenerateRecommendations(userID string, similarUsers []SimilarUser) ([]ScoredItem, error) {
    // Get items liked by similar users but not seen by target user
    cypher := `
        MATCH (target:User {user_id: $userId})
        MATCH (similar:User)-[r:RATED]->(item:Content)
        WHERE similar.user_id IN $similarUserIds 
          AND r.rating >= 4
          AND NOT EXISTS((target)-[:RATED|:VIEWED|:INTERACTED_WITH]->(item))
        
        WITH item, 
             collect({user: similar.user_id, rating: r.rating, similarity: $similarities[similar.user_id]}) as user_ratings
        
        // Weighted average score based on user similarity
        WITH item,
             reduce(weighted_sum = 0.0, ur in user_ratings | 
                weighted_sum + (ur.rating * ur.similarity)) as weighted_sum,
             reduce(similarity_sum = 0.0, ur in user_ratings | 
                similarity_sum + ur.similarity) as similarity_sum,
             size(user_ratings) as rating_count
        
        WHERE rating_count >= 2 AND similarity_sum > 0
        
        RETURN item.content_id as item_id,
               item.title as title,
               weighted_sum / similarity_sum as predicted_score,
               rating_count
        ORDER BY predicted_score DESC
        LIMIT 100
    `
    
    return c.executeCypher(cypher, c.buildSimilarityMap(similarUsers))
}
```

#### 4.3 Personalized PageRank Algorithm
```go
type PersonalizedPageRankAlgorithm struct {
    neo4jDriver neo4j.Driver
    dampingFactor float64
    iterations int
}

func (p *PersonalizedPageRankAlgorithm) ComputePageRank(userID string) ([]RankedItem, error) {
    // Use Neo4j Graph Data Science library for PageRank
    cypher := `
        // Create user-centric subgraph projection
        CALL gds.graph.project.cypher(
            'user-content-graph-' + $userId,
            '
            MATCH (n) 
            WHERE n:User OR n:Content 
            RETURN id(n) as id, labels(n) as labels
            ',
            '
            MATCH (u:User)-[r:RATED|:VIEWED|:INTERACTED_WITH]->(c:Content)
            WHERE u.user_id = $userId OR 
                  EXISTS((u)-[:SIMILAR_TO]->(:User {user_id: $userId}))
            RETURN id(u) as source, id(c) as target, 
                   CASE 
                     WHEN type(r) = "RATED" THEN r.rating / 5.0
                     WHEN type(r) = "VIEWED" THEN r.progress_percentage / 100.0
                     ELSE r.weight 
                   END as weight
            '
        )
        
        // Run Personalized PageRank with user as source
        CALL gds.pageRank.stream('user-content-graph-' + $userId, {
            dampingFactor: $dampingFactor,
            maxIterations: $iterations,
            sourceNodes: [id((:User {user_id: $userId}))]
        })
        YIELD nodeId, score
        
        // Get content items with their PageRank scores
        MATCH (c:Content) WHERE id(c) = nodeId
        RETURN c.content_id as item_id, c.title as title, score
        ORDER BY score DESC
        LIMIT 100
    `
    
    return p.executeCypher(cypher, map[string]interface{}{
        "userId": userID,
        "dampingFactor": p.dampingFactor,
        "iterations": p.iterations,
    })
}
```

#### 4.4 Graph Signal Analysis
```go
type GraphSignalAnalysis struct {
    neo4jDriver neo4j.Driver
}

func (g *GraphSignalAnalysis) AnalyzeUserCommunities(userID string) ([]string, error) {
    // Find user communities using Louvain algorithm
    cypher := `
        CALL gds.louvain.stream('user-similarity-graph', {
            includeIntermediateCommunities: false
        })
        YIELD nodeId, communityId
        
        MATCH (u:User) WHERE id(u) = nodeId
        WITH communityId, collect(u.user_id) as community_members
        WHERE $userId IN community_members
        
        RETURN community_members
    `
    
    return g.executeCommunityQuery(cypher, userID)
}

func (g *GraphSignalAnalysis) ComputeItemSimilarities(contextItems []string) ([]ItemSimilarity, error) {
    // Compute item similarities based on user interaction patterns
    cypher := `
        MATCH (item1:Content)-[:INTERACTED_WITH|:RATED|:VIEWED]-(u:User)-[:INTERACTED_WITH|:RATED|:VIEWED]-(item2:Content)
        WHERE item1.content_id IN $contextItems 
          AND item2.content_id IN $contextItems 
          AND item1 <> item2
        
        WITH item1, item2, count(u) as shared_users
        WHERE shared_users >= 3
        
        // Calculate Jaccard similarity
        MATCH (item1)-[:INTERACTED_WITH|:RATED|:VIEWED]-(u1:User)
        WITH item1, item2, shared_users, count(u1) as item1_users
        
        MATCH (item2)-[:INTERACTED_WITH|:RATED|:VIEWED]-(u2:User)  
        WITH item1, item2, shared_users, item1_users, count(u2) as item2_users
        
        WITH item1, item2, 
             shared_users * 1.0 / (item1_users + item2_users - shared_users) as jaccard_similarity
        
        WHERE jaccard_similarity > 0.1
        
        RETURN item1.content_id as item1_id,
               item2.content_id as item2_id,
               jaccard_similarity as similarity_score
        ORDER BY jaccard_similarity DESC
    `
    
    return g.executeItemSimilarityQuery(cypher, contextItems)
}
```

**Design Decisions**:
- **Semantic Search**: Uses pgvector's native cosine similarity with SQL filtering
- **Collaborative Filtering**: Implements Pearson correlation for user similarity
- **PageRank**: Leverages Neo4j GDS library for graph-based ranking
- **Graph Signals**: Uses Louvain community detection and Jaccard similarity

### 5. Diversity Filter Algorithms

**Purpose**: Ensures variety and serendipity in recommendations while maintaining relevance.

#### 5.1 Intra-List Diversity Algorithm
```go
type IntraListDiversityFilter struct {
    diversityWeight float64
    maxSimilarity   float64
}

func (d *IntraListDiversityFilter) ApplyDiversity(items []ScoredItem, targetDiversity float64) ([]ScoredItem, error) {
    if len(items) <= 1 {
        return items, nil
    }
    
    // Greedy algorithm for diversity optimization
    var diverseItems []ScoredItem
    diverseItems = append(diverseItems, items[0]) // Start with highest scored item
    
    for len(diverseItems) < len(items) && len(diverseItems) < 50 {
        bestCandidate := ScoredItem{}
        bestDiversityScore := -1.0
        
        for _, candidate := range items {
            if d.isAlreadySelected(candidate, diverseItems) {
                continue
            }
            
            // Calculate diversity score
            avgSimilarity := d.calculateAverageSimilarity(candidate, diverseItems)
            diversityScore := candidate.Score * (1 - d.diversityWeight) + 
                            (1 - avgSimilarity) * d.diversityWeight
            
            if diversityScore > bestDiversityScore {
                bestDiversityScore = diversityScore
                bestCandidate = candidate
            }
        }
        
        if bestCandidate.ItemID != "" {
            diverseItems = append(diverseItems, bestCandidate)
        } else {
            break
        }
    }
    
    return diverseItems, nil
}

func (d *IntraListDiversityFilter) calculateAverageSimilarity(candidate ScoredItem, selected []ScoredItem) float64 {
    if len(selected) == 0 {
        return 0.0
    }
    
    totalSimilarity := 0.0
    for _, item := range selected {
        similarity := d.computeItemSimilarity(candidate, item)
        totalSimilarity += similarity
    }
    
    return totalSimilarity / float64(len(selected))
}

func (d *IntraListDiversityFilter) computeItemSimilarity(item1, item2 ScoredItem) float64 {
    // Use cached similarity from Neo4j or compute on-the-fly
    // This could be category overlap, embedding similarity, etc.
    
    // Category-based similarity
    categorySimilarity := d.calculateCategorySimilarity(item1.Categories, item2.Categories)
    
    // Embedding similarity (if available)
    embeddingSimilarity := 0.0
    if len(item1.Embedding) > 0 && len(item2.Embedding) > 0 {
        embeddingSimilarity = d.cosineSimilarity(item1.Embedding, item2.Embedding)
    }
    
    // Weighted combination
    return 0.6*categorySimilarity + 0.4*embeddingSimilarity
}
```

#### 5.2 Category Diversity Algorithm
```go
type CategoryDiversityFilter struct {
    maxItemsPerCategory int
    categoryWeights     map[string]float64
}

func (c *CategoryDiversityFilter) EnforceCategoryDiversity(items []ScoredItem) ([]ScoredItem, error) {
    categoryCount := make(map[string]int)
    var diverseItems []ScoredItem
    
    // Sort items by score descending
    sort.Slice(items, func(i, j int) bool {
        return items[i].Score > items[j].Score
    })
    
    for _, item := range items {
        // Check if we can add this item based on category limits
        canAdd := true
        for _, category := range item.Categories {
            if categoryCount[category] >= c.maxItemsPerCategory {
                canAdd = false
                break
            }
        }
        
        if canAdd {
            diverseItems = append(diverseItems, item)
            
            // Update category counts
            for _, category := range item.Categories {
                categoryCount[category]++
            }
        }
        
        // Stop if we have enough items
        if len(diverseItems) >= 50 {
            break
        }
    }
    
    return diverseItems, nil
}
```

#### 5.3 Serendipity Filter Algorithm
```go
type SerendipityFilter struct {
    serendipityRatio    float64  // Percentage of unexpected items
    unexpectednessThreshold float64
    neo4jDriver         neo4j.Driver
}

func (s *SerendipityFilter) InjectSerendipity(userID string, items []ScoredItem) ([]ScoredItem, error) {
    serendipityCount := int(float64(len(items)) * s.serendipityRatio)
    if serendipityCount == 0 {
        return items, nil
    }
    
    // Find unexpected items for the user
    unexpectedItems, err := s.findUnexpectedItems(userID, serendipityCount*2) // Get more candidates
    if err != nil {
        return items, err
    }
    
    // Filter for quality unexpected items
    qualityUnexpected := s.filterQualityUnexpected(unexpectedItems, serendipityCount)
    
    // Inject serendipitous items at strategic positions
    return s.injectAtPositions(items, qualityUnexpected), nil
}

func (s *SerendipityFilter) findUnexpectedItems(userID string, count int) ([]ScoredItem, error) {
    // Find items from categories user hasn't explored much
    cypher := `
        MATCH (u:User {user_id: $userId})
        OPTIONAL MATCH (u)-[r:RATED|:VIEWED|:INTERACTED_WITH]->(interacted:Content)
        
        WITH u, collect(DISTINCT interacted.categories) as user_categories
        
        // Find items in categories user hasn't explored
        MATCH (item:Content)
        WHERE item.is_active = true
          AND NOT EXISTS((u)-[:RATED|:VIEWED|:INTERACTED_WITH]->(item))
          AND NONE(cat IN item.categories WHERE cat IN user_categories)
        
        // But are popular among similar users
        OPTIONAL MATCH (similar:User)-[:SIMILAR_TO]->(u)
        OPTIONAL MATCH (similar)-[sr:RATED]->(item)
        WHERE sr.rating >= 4
        
        WITH item, count(sr) as similar_user_likes, avg(sr.rating) as avg_rating
        WHERE similar_user_likes >= 2
        
        RETURN item.content_id as item_id,
               item.title as title,
               item.categories as categories,
               similar_user_likes * avg_rating as unexpectedness_score
        ORDER BY unexpectedness_score DESC
        LIMIT $count
    `
    
    return s.executeCypher(cypher, map[string]interface{}{
        "userId": userID,
        "count":  count,
    })
}

func (s *SerendipityFilter) injectAtPositions(mainItems []ScoredItem, serendipitousItems []ScoredItem) []ScoredItem {
    if len(serendipitousItems) == 0 {
        return mainItems
    }
    
    result := make([]ScoredItem, 0, len(mainItems)+len(serendipitousItems))
    serendipityIndex := 0
    
    // Inject serendipitous items at positions 3, 7, 12, 18, etc.
    injectionPositions := []int{3, 7, 12, 18, 25, 33, 42}
    injectionMap := make(map[int]bool)
    for _, pos := range injectionPositions {
        if pos < len(mainItems)+len(serendipitousItems) {
            injectionMap[pos] = true
        }
    }
    
    for i := 0; i < len(mainItems)+len(serendipitousItems); i++ {
        if injectionMap[i] && serendipityIndex < len(serendipitousItems) {
            // Inject serendipitous item
            result = append(result, serendipitousItems[serendipityIndex])
            serendipityIndex++
        } else if len(result)-serendipityIndex < len(mainItems) {
            // Add main item
            result = append(result, mainItems[len(result)-serendipityIndex])
        }
    }
    
    return result
}
```

#### 5.4 Temporal Diversity Algorithm
```go
type TemporalDiversityFilter struct {
    recentInteractionWindow time.Duration
    maxRecentSimilar        int
}

func (t *TemporalDiversityFilter) FilterRecentSimilar(userID string, items []ScoredItem) ([]ScoredItem, error) {
    // Get user's recent interactions
    recentItems, err := t.getRecentInteractions(userID)
    if err != nil {
        return items, err
    }
    
    var filteredItems []ScoredItem
    recentSimilarCount := 0
    
    for _, item := range items {
        isSimilarToRecent := false
        
        for _, recentItem := range recentItems {
            similarity := t.computeItemSimilarity(item, recentItem)
            if similarity > 0.7 { // High similarity threshold
                isSimilarToRecent = true
                break
            }
        }
        
        if isSimilarToRecent {
            if recentSimilarCount < t.maxRecentSimilar {
                filteredItems = append(filteredItems, item)
                recentSimilarCount++
            }
            // Skip if too many similar to recent items
        } else {
            filteredItems = append(filteredItems, item)
        }
    }
    
    return filteredItems, nil
}
```

**Design Decisions**:
- **Intra-List Diversity**: Greedy algorithm balancing relevance and diversity
- **Category Diversity**: Enforces maximum items per category to prevent over-concentration
- **Serendipity**: Injects unexpected but quality items from unexplored categories
- **Temporal Diversity**: Prevents over-recommendation of items similar to recent interactions

### 6. Explanation Service Algorithms

**Purpose**: Generates transparent, human-readable explanations for why items were recommended.

#### 6.1 Content-Based Explanations
```go
type ContentBasedExplainer struct {
    embeddingService *EmbeddingService
    neo4jDriver      neo4j.Driver
}

func (c *ContentBasedExplainer) ExplainContentSimilarity(userID, itemID string) (*Explanation, error) {
    // Find user's highly-rated items similar to recommended item
    cypher := `
        MATCH (u:User {user_id: $userId})-[r:RATED]->(liked:Content)
        WHERE r.rating >= 4
        
        MATCH (recommended:Content {content_id: $itemID})
        
        // Find similarity reasons
        WITH liked, recommended,
             // Category overlap
             [cat IN liked.categories WHERE cat IN recommended.categories] as shared_categories,
             // Check if same type
             CASE WHEN liked.type = recommended.type THEN liked.type ELSE null END as shared_type
        
        WHERE size(shared_categories) > 0 OR shared_type IS NOT NULL
        
        RETURN liked.title as similar_item,
               liked.content_id as similar_item_id,
               r.rating as user_rating,
               shared_categories,
               shared_type,
               // Calculate embedding similarity if available
               CASE 
                 WHEN liked.text_embedding IS NOT NULL AND recommended.text_embedding IS NOT NULL
                 THEN 1 - (liked.text_embedding <=> recommended.text_embedding)
                 ELSE null 
               END as embedding_similarity
        ORDER BY user_rating DESC, size(shared_categories) DESC
        LIMIT 3
    `
    
    results, err := c.executeCypher(cypher, map[string]interface{}{
        "userId": userID,
        "itemID": itemID,
    })
    if err != nil {
        return nil, err
    }
    
    return c.buildContentExplanation(results), nil
}

func (c *ContentBasedExplainer) buildContentExplanation(similarities []SimilarityResult) *Explanation {
    if len(similarities) == 0 {
        return &Explanation{
            Type:   "content_based",
            Reason: "This item matches your general preferences",
            Confidence: 0.3,
        }
    }
    
    // Build explanation based on strongest similarity
    strongest := similarities[0]
    
    var reason strings.Builder
    reason.WriteString(fmt.Sprintf("Because you rated '%s' highly", strongest.SimilarItem))
    
    if len(strongest.SharedCategories) > 0 {
        reason.WriteString(fmt.Sprintf(" and both are in %s", 
            strings.Join(strongest.SharedCategories, ", ")))
    }
    
    if strongest.SharedType != "" {
        reason.WriteString(fmt.Sprintf(" (both are %s)", strongest.SharedType))
    }
    
    confidence := c.calculateContentConfidence(similarities)
    
    return &Explanation{
        Type:       "content_based",
        Reason:     reason.String(),
        Confidence: confidence,
        Evidence: map[string]interface{}{
            "similar_items": similarities,
            "similarity_count": len(similarities),
        },
    }
}
```

#### 6.2 Collaborative Filtering Explanations
```go
type CollaborativeExplainer struct {
    neo4jDriver neo4j.Driver
}

func (c *CollaborativeExplainer) ExplainCollaborativeRecommendation(userID, itemID string) (*Explanation, error) {
    // Find similar users who liked this item
    cypher := `
        MATCH (target:User {user_id: $userId})
        MATCH (similar:User)-[:SIMILAR_TO]->(target)
        MATCH (similar)-[r:RATED]->(item:Content {content_id: $itemID})
        WHERE r.rating >= 4
        
        // Get shared preferences between users
        OPTIONAL MATCH (target)-[tr:RATED]->(shared:Content)<-[sr:RATED]-(similar)
        WHERE tr.rating >= 4 AND sr.rating >= 4
        
        WITH similar, r, collect(shared.title) as shared_items
        
        RETURN similar.user_id as similar_user,
               r.rating as their_rating,
               shared_items[0..3] as sample_shared_items,
               size(shared_items) as total_shared_items
        ORDER BY r.rating DESC, total_shared_items DESC
        LIMIT 3
    `
    
    results, err := c.executeCypher(cypher, map[string]interface{}{
        "userId": userID,
        "itemID": itemID,
    })
    if err != nil {
        return nil, err
    }
    
    return c.buildCollaborativeExplanation(results), nil
}

func (c *CollaborativeExplainer) buildCollaborativeExplanation(similarUsers []SimilarUserResult) *Explanation {
    if len(similarUsers) == 0 {
        return &Explanation{
            Type:   "collaborative",
            Reason: "Users with similar tastes liked this item",
            Confidence: 0.4,
        }
    }
    
    totalSharedItems := 0
    avgRating := 0.0
    
    for _, user := range similarUsers {
        totalSharedItems += user.TotalSharedItems
        avgRating += user.TheirRating
    }
    avgRating /= float64(len(similarUsers))
    
    var reason strings.Builder
    reason.WriteString(fmt.Sprintf("Users who share %d+ preferences with you ", 
        totalSharedItems/len(similarUsers)))
    reason.WriteString(fmt.Sprintf("rated this %.1f/5", avgRating))
    
    if len(similarUsers[0].SampleSharedItems) > 0 {
        reason.WriteString(fmt.Sprintf(". You both liked: %s", 
            strings.Join(similarUsers[0].SampleSharedItems, ", ")))
    }
    
    confidence := c.calculateCollaborativeConfidence(similarUsers)
    
    return &Explanation{
        Type:       "collaborative",
        Reason:     reason.String(),
        Confidence: confidence,
        Evidence: map[string]interface{}{
            "similar_users": len(similarUsers),
            "avg_rating": avgRating,
            "shared_preferences": totalSharedItems,
        },
    }
}
```

#### 6.3 Graph-Based Explanations
```go
type GraphExplainer struct {
    neo4jDriver neo4j.Driver
}

func (g *GraphExplainer) ExplainGraphRecommendation(userID, itemID string) (*Explanation, error) {
    // Find the path/connection that led to this recommendation
    cypher := `
        MATCH path = (u:User {user_id: $userId})-[*1..3]-(item:Content {content_id: $itemID})
        WHERE NONE(r IN relationships(path) WHERE type(r) = 'RATED' AND r.rating < 3)
        
        WITH path, 
             [n IN nodes(path) WHERE n:Content | n.title] as content_titles,
             [r IN relationships(path) | {type: type(r), strength: coalesce(r.rating, r.weight, r.score)}] as relationship_info
        
        RETURN content_titles,
               relationship_info,
               length(path) as path_length
        ORDER BY path_length ASC
        LIMIT 1
    `
    
    result, err := g.executeCypher(cypher, map[string]interface{}{
        "userId": userID,
        "itemID": itemID,
    })
    if err != nil {
        return nil, err
    }
    
    return g.buildGraphExplanation(result), nil
}

func (g *GraphExplainer) buildGraphExplanation(pathResult PathResult) *Explanation {
    if pathResult.PathLength == 0 {
        return &Explanation{
            Type:   "graph_based",
            Reason: "This item is connected to your preferences through our recommendation network",
            Confidence: 0.5,
        }
    }
    
    var reason strings.Builder
    
    switch pathResult.PathLength {
    case 2:
        reason.WriteString("You're directly connected to this item through your preferences")
    case 3:
        if len(pathResult.ContentTitles) > 1 {
            reason.WriteString(fmt.Sprintf("This connects to '%s' which you've shown interest in", 
                pathResult.ContentTitles[0]))
        }
    case 4:
        reason.WriteString("This item is part of a recommendation chain based on your activity patterns")
    }
    
    confidence := 0.8 - (float64(pathResult.PathLength-2) * 0.15) // Shorter paths = higher confidence
    
    return &Explanation{
        Type:       "graph_based",
        Reason:     reason.String(),
        Confidence: math.Max(confidence, 0.3),
        Evidence: map[string]interface{}{
            "path_length": pathResult.PathLength,
            "connection_strength": g.calculatePathStrength(pathResult.RelationshipInfo),
        },
    }
}
```

#### 6.4 Popularity-Based Explanations
```go
type PopularityExplainer struct {
    neo4jDriver neo4j.Driver
}

func (p *PopularityExplainer) ExplainPopularityRecommendation(itemID string, userContext UserContext) (*Explanation, error) {
    // Get popularity metrics for the item
    cypher := `
        MATCH (item:Content {content_id: $itemID})
        
        // Overall popularity
        OPTIONAL MATCH (item)<-[r:RATED]-(u:User)
        WITH item, count(r) as total_ratings, avg(r.rating) as avg_rating
        
        // Recent popularity (last 30 days)
        OPTIONAL MATCH (item)<-[recent:VIEWED|:INTERACTED_WITH]-(u2:User)
        WHERE recent.timestamp > datetime() - duration('P30D')
        WITH item, total_ratings, avg_rating, count(recent) as recent_interactions
        
        // Category popularity
        OPTIONAL MATCH (similar:Content)<-[cr:RATED]-(u3:User)
        WHERE any(cat IN item.categories WHERE cat IN similar.categories)
        WITH item, total_ratings, avg_rating, recent_interactions, 
             count(cr) as category_popularity
        
        RETURN total_ratings, avg_rating, recent_interactions, category_popularity
    `
    
    result, err := p.executeCypher(cypher, map[string]interface{}{
        "itemID": itemID,
    })
    if err != nil {
        return nil, err
    }
    
    return p.buildPopularityExplanation(result, userContext), nil
}

func (p *PopularityExplainer) buildPopularityExplanation(popularity PopularityResult, userContext UserContext) *Explanation {
    var reason strings.Builder
    var confidence float64
    
    if popularity.TotalRatings > 100 && popularity.AvgRating > 4.0 {
        reason.WriteString(fmt.Sprintf("Highly rated by %d users (%.1f/5 stars)", 
            popularity.TotalRatings, popularity.AvgRating))
        confidence = 0.7
    } else if popularity.RecentInteractions > 50 {
        reason.WriteString("Trending - many users have been engaging with this recently")
        confidence = 0.6
    } else if userContext.IsNewUser {
        reason.WriteString("Popular choice for new users getting started")
        confidence = 0.5
    } else {
        reason.WriteString("Recommended based on general popularity")
        confidence = 0.4
    }
    
    return &Explanation{
        Type:       "popularity_based",
        Reason:     reason.String(),
        Confidence: confidence,
        Evidence: map[string]interface{}{
            "total_ratings": popularity.TotalRatings,
            "avg_rating": popularity.AvgRating,
            "recent_interactions": popularity.RecentInteractions,
        },
    }
}
```

#### 6.5 Explanation Aggregation and Selection
```go
type ExplanationService struct {
    contentExplainer      *ContentBasedExplainer
    collaborativeExplainer *CollaborativeExplainer
    graphExplainer        *GraphExplainer
    popularityExplainer   *PopularityExplainer
}

func (e *ExplanationService) GenerateExplanation(userID, itemID string, algorithmUsed string, userContext UserContext) (*Explanation, error) {
    var explanations []*Explanation
    
    // Generate explanations from different perspectives
    if contentExp, err := e.contentExplainer.ExplainContentSimilarity(userID, itemID); err == nil {
        explanations = append(explanations, contentExp)
    }
    
    if collabExp, err := e.collaborativeExplainer.ExplainCollaborativeRecommendation(userID, itemID); err == nil {
        explanations = append(explanations, collabExp)
    }
    
    if graphExp, err := e.graphExplainer.ExplainGraphRecommendation(userID, itemID); err == nil {
        explanations = append(explanations, graphExp)
    }
    
    if popExp, err := e.popularityExplainer.ExplainPopularityRecommendation(itemID, userContext); err == nil {
        explanations = append(explanations, popExp)
    }
    
    // Select the best explanation based on confidence and algorithm used
    return e.selectBestExplanation(explanations, algorithmUsed), nil
}

func (e *ExplanationService) selectBestExplanation(explanations []*Explanation, algorithmUsed string) *Explanation {
    if len(explanations) == 0 {
        return &Explanation{
            Type:   "default",
            Reason: "This item matches your preferences",
            Confidence: 0.3,
        }
    }
    
    // Prefer explanation that matches the algorithm used
    for _, exp := range explanations {
        if strings.Contains(algorithmUsed, exp.Type) && exp.Confidence > 0.5 {
            return exp
        }
    }
    
    // Otherwise, return highest confidence explanation
    sort.Slice(explanations, func(i, j int) bool {
        return explanations[i].Confidence > explanations[j].Confidence
    })
    
    return explanations[0]
}

// Explanation data structure
type Explanation struct {
    Type       string                 `json:"type"`
    Reason     string                 `json:"reason"`
    Confidence float64                `json:"confidence"`
    Evidence   map[string]interface{} `json:"evidence,omitempty"`
}
```

**Design Decisions**:
- **Multi-perspective explanations**: Generate explanations from content, collaborative, graph, and popularity perspectives
- **Confidence scoring**: Each explanation includes a confidence score based on evidence strength
- **Algorithm alignment**: Prefer explanations that match the algorithm that generated the recommendation
- **Fallback explanations**: Always provide some explanation, even with low confidence
- **Evidence tracking**: Include supporting data for transparency and debugging

### Caching Architecture

The system uses a multi-tier Redis-based caching strategy with hierarchical keys and hybrid invalidation:

#### Cache Key Strategy
```go
type CacheKeyBuilder struct {
    prefix string
}

// Hierarchical cache keys for easy invalidation
func (ckb *CacheKeyBuilder) BuildKey(components ...string) string {
    return strings.Join(append([]string{ckb.prefix}, components...), ":")
}

// Examples:
// rec:user:123:context:home -> User recommendations
// embed:model:v1:item:456 -> Item embeddings  
// graph:pagerank:user:123 -> Graph computation results
// meta:item:789:v2 -> Item metadata
```

#### Cache Invalidation Strategy
```go
type CacheInvalidationStrategy struct {
    immediate []string // Critical caches - invalidate immediately
    ttlBased  []string // Non-critical - rely on TTL
}

// Hybrid approach
func (cis *CacheInvalidationStrategy) InvalidateOnUserInteraction(userID string) {
    // Immediate invalidation for user-specific caches
    cis.invalidatePattern(fmt.Sprintf("rec:user:%s:*", userID))
    
    // TTL-based for less critical caches
    cis.scheduleInvalidation(fmt.Sprintf("graph:*:user:%s", userID), 5*time.Minute)
}
```

#### Cache Tier Design

```mermaid
graph TB
    subgraph "Redis Cluster"
        subgraph "Hot Cache (Redis Instance 1)"
            CacheMeta[Metadata Cache<br/>TTL: 1 hour]
            CacheRec[Recommendations Cache<br/>TTL: 15 minutes]
        end
        
        subgraph "Warm Cache (Redis Instance 2)"
            CacheGraph[Graph Results Cache<br/>TTL: 30 minutes]
        end
        
        subgraph "Cold Cache (Redis Instance 3)"
            CacheEmbed[Embeddings Cache<br/>TTL: 24 hours]
        end
    end
    
    App[Application Layer] --> CacheMeta
    App --> CacheRec
    App --> CacheGraph
    App --> CacheEmbed
```

#### Cache Configuration

**Single Redis Cluster with 3 Instances**:
- **Instance 1 (Hot)**: High-frequency, short-lived data (metadata, user recommendations)
  - Memory: 8GB, optimized for speed
  - Eviction policy: `allkeys-lru`
  - Persistence: AOF with 1-second fsync

- **Instance 2 (Warm)**: Medium-frequency, medium-lived data (graph computation results)
  - Memory: 4GB, balanced configuration
  - Eviction policy: `allkeys-lru`
  - Persistence: RDB snapshots every 5 minutes

- **Instance 3 (Cold)**: Low-frequency, long-lived data (embeddings, model artifacts)
  - Memory: 16GB, optimized for capacity
  - Eviction policy: `allkeys-lfu`
  - Persistence: RDB snapshots every 15 minutes

#### Cache Key Strategies

```typescript
interface CacheKeyStrategy {
  metadata: `meta:${itemType}:${itemId}:${version}`
  recommendations: `rec:${userId}:${context}:${timestamp}`
  graphResults: `graph:${algorithm}:${userId}:${params}`
  embeddings: `embed:${modelVersion}:${contentId}:${type}`
}
```

#### Cache Invalidation

- **Event-driven invalidation**: User interactions trigger recommendation cache clearing
- **Time-based expiration**: Different TTLs based on data volatility
- **Version-based invalidation**: Model updates invalidate related embedding caches
- **Dependency tracking**: Content updates cascade to related recommendation caches

## Data Models

### Core Data Models

#### User Profile Model
```typescript
interface UserProfile {
  userId: string
  preferenceVector: number[]  // Aggregated embedding representation
  explicitPreferences: ExplicitPreference[]
  behaviorPatterns: BehaviorPattern[]
  demographics?: Demographics
  createdAt: Date
  updatedAt: Date
}

interface ExplicitPreference {
  itemId: string
  category: string
  rating: number
  confidence: number
  timestamp: Date
}

interface BehaviorPattern {
  pattern: 'browsing' | 'purchasing' | 'searching'
  frequency: number
  timeOfDay: number[]
  categories: string[]
  seasonality?: SeasonalPattern
}
```

#### Content Item Model
```typescript
interface ContentItem {
  id: string
  type: ContentType
  title: string
  description: string
  textEmbedding: number[]
  imageEmbeddings: number[][]
  multiModalEmbedding: number[]
  metadata: ContentMetadata
  categories: Category[]
  qualityScore: number
  createdAt: Date
  updatedAt: Date
}

interface ContentMetadata {
  price?: number
  brand?: string
  tags: string[]
  attributes: Record<string, any>
  availability: boolean
  popularity: PopularityMetrics
}
```

#### Graph Relationship Model (Neo4j)

Your detailed relationship model provides much richer behavioral tracking than the initial design. Here's the enhanced Neo4j schema:

```cypher
// Core Nodes
CREATE CONSTRAINT user_id_unique FOR (u:User) REQUIRE u.user_id IS UNIQUE;
CREATE CONSTRAINT content_id_unique FOR (c:Content) REQUIRE c.content_id IS UNIQUE;

// User Node
(:User {
  user_id: string,
  name: string,
  created_at: datetime,
  segment: string,
  preference_vector: list<float>  // Synced from PostgreSQL
})

// Content Node  
(:Content {
  content_id: string,
  title: string,
  type: string,
  categories: list<string>,
  release_date: datetime,
  is_active: boolean,
  embedding_version: string  // Track which embedding model was used
})

// Detailed Relationship Types
(:User)-[:INTERACTED_WITH {
  timestamp: datetime,
  interaction_type: string,
  duration_seconds: int,
  weight: float,
  session_id: string,
  context: map
}]->(:Content)

(:User)-[:RATED {
  timestamp: datetime,
  rating: int,
  comment_id: string,
  confidence: float,
  review_text: string
}]->(:Content)

(:User)-[:VIEWED {
  timestamp: datetime,
  progress_percentage: int,
  device: string,
  view_duration: int,
  completion_status: string
}]->(:Content)

(:User)-[:ABANDONED {
  timestamp: datetime,
  progress_percentage: int,
  reason: string,
  session_duration: int,
  exit_point: string
}]->(:Content)

(:Content)-[:SIMILAR_TO {
  similarity_type: string,  // 'semantic', 'collaborative', 'content_based'
  score: float,
  computed_at: datetime,
  algorithm_version: string
}]->(:Content)

// Additional relationships for enhanced recommendations
(:User)-[:SIMILAR_TO {
  similarity_type: string,
  score: float,
  shared_preferences: list<string>,
  computed_at: datetime
}]->(:User)

(:Content)-[:BELONGS_TO]->(:Category {
  name: string,
  parent_category: string,
  level: int
})

(:User)-[:PREFERS {
  strength: float,
  updated_at: datetime,
  interaction_count: int
}]->(:Category)
```

#### Key Improvements in Your Model

**1. Granular Interaction Tracking**:
- **VIEWED**: Captures engagement depth with progress_percentage and device context
- **ABANDONED**: Critical for understanding content friction points and user drop-off patterns
- **RATED**: Explicit feedback with comment linking for sentiment analysis
- **INTERACTED_WITH**: General interaction wrapper with flexible context

**2. Rich Behavioral Context**:
- **Duration tracking**: Both interaction duration and session context
- **Device information**: Enables device-specific recommendation optimization
- **Progress tracking**: Understanding of content consumption patterns
- **Abandonment reasons**: Valuable for content quality and UX improvements

**3. Enhanced Similarity Modeling**:
- **Multiple similarity types**: Semantic, collaborative, and content-based similarities tracked separately
- **Algorithm versioning**: Track which model version computed similarities
- **Temporal tracking**: When similarities were computed for freshness management

#### Integration with PostgreSQL

The Neo4j model complements the PostgreSQL design by:

```typescript
interface GraphSyncStrategy {
  // Sync user preference vectors from PostgreSQL to Neo4j
  syncUserPreferences(userId: string): Promise<void>
  
  // Update PostgreSQL user profiles based on Neo4j relationship analysis
  updateUserProfileFromGraph(userId: string): Promise<UserProfile>
  
  // Compute collaborative signals from Neo4j relationships
  computeCollaborativeSignals(userId: string): Promise<CollaborativeSignals>
}

interface CollaborativeSignals {
  similarUsers: SimilarUser[]
  categoryAffinities: CategoryAffinity[]
  abandonmentPatterns: AbandonmentPattern[]
  engagementPredictors: EngagementPredictor[]
}
```

#### Graph Algorithm Applications

Your model enables sophisticated graph algorithms:

```cypher
// Find users with similar abandonment patterns
MATCH (u1:User)-[a1:ABANDONED]->(c:Content)<-[a2:ABANDONED]-(u2:User)
WHERE u1.user_id = $userId 
  AND a1.progress_percentage > 50 
  AND a2.progress_percentage > 50
  AND a1.reason = a2.reason
RETURN u2, count(c) as shared_abandonments
ORDER BY shared_abandonments DESC

// Identify content that leads to high engagement
MATCH (c:Content)<-[v:VIEWED]-(u:User)
WHERE v.progress_percentage > 80
WITH c, avg(v.progress_percentage) as avg_completion, count(v) as view_count
WHERE view_count > 10
RETURN c.title, avg_completion, view_count
ORDER BY avg_completion DESC

// Find content similar to highly-rated items
MATCH (u:User)-[r:RATED]->(c1:Content)-[s:SIMILAR_TO]->(c2:Content)
WHERE u.user_id = $userId AND r.rating >= 4
RETURN c2, avg(s.score) as similarity_score, count(r) as rating_count
ORDER BY similarity_score DESC
```

This model is significantly more sophisticated than my initial design and provides much richer data for generating high-quality recommendations. It captures the full user journey including abandonment patterns, which is crucial for understanding user preferences and content quality.

### Database Schema Design

#### PostgreSQL Connection Optimization

**PgBouncer Connection Pooling Strategy**:

**PgBouncer Configuration** (`pgbouncer.ini`):
```ini
[databases]
recommendations_write = host=postgres-primary port=5432 dbname=recommendations user=app_user
recommendations_read = host=postgres-replica port=5432 dbname=recommendations user=app_user

[pgbouncer]
# Connection pooling mode
pool_mode = transaction  # Most efficient for recommendation workloads

# Connection limits
max_client_conn = 1000   # Max client connections
default_pool_size = 25   # Connections per database
reserve_pool_size = 5    # Emergency connections
max_db_connections = 30  # Max connections to PostgreSQL

# Timeouts
server_idle_timeout = 600      # 10 minutes
server_connect_timeout = 15    # 15 seconds
server_login_retry = 15        # 15 seconds
query_timeout = 30             # 30 seconds for long vector queries
query_wait_timeout = 120       # 2 minutes wait in queue

# Performance settings
server_reset_query = DISCARD ALL
server_check_query = SELECT 1
server_check_delay = 30

# Logging
log_connections = 1
log_disconnections = 1
log_pooler_errors = 1

# Authentication
auth_type = md5
auth_file = /etc/pgbouncer/userlist.txt

# Admin interface
admin_users = admin
stats_users = stats, admin
```

**Go Application Database Configuration**:
```go
type DatabaseConfig struct {
    // PgBouncer endpoints
    WriteURL    string `default:"postgres://app_user:password@pgbouncer:6432/recommendations_write"`
    ReadURL     string `default:"postgres://app_user:password@pgbouncer:6433/recommendations_read"`
    
    // Application-level pool settings (smaller since PgBouncer handles pooling)
    MaxOpenConns    int           `default:"50"`   // Higher since PgBouncer manages efficiently
    MaxIdleConns    int           `default:"10"`   // Fewer idle connections needed
    ConnMaxLifetime time.Duration `default:"1h"`
    ConnMaxIdleTime time.Duration `default:"15m"`
    
    // Query timeouts
    QueryTimeout    time.Duration `default:"30s"`
    TxTimeout       time.Duration `default:"5m"`
}

// Simplified connection setup with PgBouncer
func NewDatabaseConnections(config DatabaseConfig) (*DatabaseManager, error) {
    // Write connection through PgBouncer
    writeDB, err := sql.Open("postgres", config.WriteURL)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to write database: %w", err)
    }
    
    // Read connection through PgBouncer
    readDB, err := sql.Open("postgres", config.ReadURL)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to read database: %w", err)
    }
    
    // Configure connection pools (smaller since PgBouncer handles the heavy lifting)
    writeDB.SetMaxOpenConns(config.MaxOpenConns)
    writeDB.SetMaxIdleConns(config.MaxIdleConns)
    writeDB.SetConnMaxLifetime(config.ConnMaxLifetime)
    writeDB.SetConnMaxIdleTime(config.ConnMaxIdleTime)
    
    readDB.SetMaxOpenConns(config.MaxOpenConns)
    readDB.SetMaxIdleConns(config.MaxIdleConns)
    readDB.SetConnMaxLifetime(config.ConnMaxLifetime)
    readDB.SetConnMaxIdleTime(config.ConnMaxIdleTime)
    
    return &DatabaseManager{
        writeDB: writeDB,
        readDB:  readDB,
    }, nil
}

type DatabaseManager struct {
    writeDB *sql.DB
    readDB  *sql.DB
}

func (dm *DatabaseManager) GetReadDB() *sql.DB {
    return dm.readDB
}

func (dm *DatabaseManager) GetWriteDB() *sql.DB {
    return dm.writeDB
}
```

**Docker Compose Setup with PgBouncer**:
```yaml
services:
  # Application
  recommendation-engine:
    build: .
    environment:
      - DB_WRITE_URL=postgres://app_user:password@pgbouncer-write:6432/recommendations
      - DB_READ_URL=postgres://app_user:password@pgbouncer-read:6432/recommendations
    depends_on: [pgbouncer-write, pgbouncer-read]
  
  # PgBouncer for write operations
  pgbouncer-write:
    image: pgbouncer/pgbouncer:latest
    environment:
      - DATABASES_HOST=postgres-primary
      - DATABASES_PORT=5432
      - DATABASES_USER=app_user
      - DATABASES_PASSWORD=password
      - DATABASES_DBNAME=recommendations
      - POOL_MODE=transaction
      - MAX_CLIENT_CONN=500
      - DEFAULT_POOL_SIZE=25
      - SERVER_RESET_QUERY=DISCARD ALL
    volumes:
      - ./pgbouncer-write.ini:/etc/pgbouncer/pgbouncer.ini
    depends_on: [postgres-primary]
  
  # PgBouncer for read operations  
  pgbouncer-read:
    image: pgbouncer/pgbouncer:latest
    environment:
      - DATABASES_HOST=postgres-replica
      - DATABASES_PORT=5432
      - DATABASES_USER=app_user
      - DATABASES_PASSWORD=password
      - DATABASES_DBNAME=recommendations
      - POOL_MODE=transaction
      - MAX_CLIENT_CONN=1000
      - DEFAULT_POOL_SIZE=30
      - SERVER_RESET_QUERY=DISCARD ALL
    volumes:
      - ./pgbouncer-read.ini:/etc/pgbouncer/pgbouncer.ini
    depends_on: [postgres-replica]
  
  # Primary PostgreSQL
  postgres-primary:
    image: postgres:15
    environment:
      - POSTGRES_DB=recommendations
      - POSTGRES_USER=app_user
      - POSTGRES_PASSWORD=password
    volumes:
      - postgres_primary_data:/var/lib/postgresql/data
      - ./init.sql:/docker-entrypoint-initdb.d/init.sql
    command: >
      postgres
      -c shared_buffers=2GB
      -c work_mem=256MB
      -c maintenance_work_mem=1GB
      -c shared_preload_libraries=pgvector
      -c max_connections=100
  
  # Read Replica PostgreSQL
  postgres-replica:
    image: postgres:15
    environment:
      - POSTGRES_DB=recommendations
      - POSTGRES_USER=app_user
      - POSTGRES_PASSWORD=password
      - PGUSER=postgres
    volumes:
      - postgres_replica_data:/var/lib/postgresql/data
    command: >
      postgres
      -c shared_buffers=2GB
      -c work_mem=256MB
      -c shared_preload_libraries=pgvector
      -c max_connections=100
      -c hot_standby=on
    depends_on: [postgres-primary]

volumes:
  postgres_primary_data:
  postgres_replica_data:
```

**Usage in Recommendation Service**:
```go
// Usage with PgBouncer - much simpler!
func (rs *RecommendationService) SearchSimilarItems(embedding []float32) ([]ScoredItem, error) {
    // PgBouncer automatically handles connection pooling
    db := rs.dbManager.GetReadDB()
    
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    query := `
        SELECT content_id, title, 1 - (text_embedding <=> $1) as similarity
        FROM content_items 
        WHERE is_active = true
        ORDER BY text_embedding <=> $1 
        LIMIT 100
    `
    
    rows, err := db.QueryContext(ctx, query, pq.Array(embedding))
    if err != nil {
        return nil, fmt.Errorf("similarity search failed: %w", err)
    }
    defer rows.Close()
    
    return rs.processResults(rows)
}

func (rs *RecommendationService) CreateUserProfile(profile UserProfile) error {
    // Write operations go through write PgBouncer
    db := rs.dbManager.GetWriteDB()
    
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    query := `
        INSERT INTO user_profiles (user_id, preference_vector, explicit_preferences, behavior_patterns)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (user_id) DO UPDATE SET
            preference_vector = EXCLUDED.preference_vector,
            explicit_preferences = EXCLUDED.explicit_preferences,
            behavior_patterns = EXCLUDED.behavior_patterns,
            updated_at = NOW()
    `
    
    _, err := db.ExecContext(ctx, query, 
        profile.UserID,
        pq.Array(profile.PreferenceVector),
        profile.ExplicitPreferences,
        profile.BehaviorPatterns)
    
    return err
}
```

**PostgreSQL Configuration Optimizations**:
```sql
-- postgresql.conf optimizations for recommendation workload

-- Memory settings
shared_buffers = '2GB'                    -- 25% of RAM for caching
work_mem = '256MB'                        -- Per-operation memory for vector ops
maintenance_work_mem = '1GB'              -- For index maintenance
effective_cache_size = '6GB'              -- Total system cache estimate

-- Vector-specific settings
shared_preload_libraries = 'pgvector'
max_parallel_workers_per_gather = 4      -- Parallel query execution
max_parallel_workers = 8
max_worker_processes = 8

-- Connection settings
max_connections = 100
superuser_reserved_connections = 3

-- Checkpoint and WAL settings
checkpoint_completion_target = 0.9
wal_buffers = '64MB'
checkpoint_timeout = '15min'

-- Query planner settings for vector operations
random_page_cost = 1.1                   -- SSD-optimized
effective_io_concurrency = 200           -- SSD concurrent I/O

-- Logging for performance monitoring
log_min_duration_statement = 1000        -- Log slow queries (>1s)
log_checkpoints = on
log_connections = on
log_disconnections = on
```

**Vector Index Optimization**:
```sql
-- Optimized vector indexes with proper parameters
CREATE INDEX CONCURRENTLY idx_content_text_embedding 
ON content_items 
USING ivfflat (text_embedding vector_cosine_ops) 
WITH (lists = 1000);  -- Adjust based on data size

CREATE INDEX CONCURRENTLY idx_content_multimodal_embedding 
ON content_items 
USING ivfflat (multimodal_embedding vector_cosine_ops) 
WITH (lists = 1000);

CREATE INDEX CONCURRENTLY idx_user_preference_vector 
ON user_profiles 
USING ivfflat (preference_vector vector_cosine_ops) 
WITH (lists = 500);   -- Fewer lists for smaller user table

-- Composite indexes for filtered vector searches
CREATE INDEX CONCURRENTLY idx_content_type_embedding 
ON content_items (type, is_active) 
INCLUDE (text_embedding);

CREATE INDEX CONCURRENTLY idx_content_categories_embedding 
ON content_items USING GIN (categories) 
INCLUDE (text_embedding);

-- Regular indexes for non-vector queries
CREATE INDEX CONCURRENTLY idx_user_interactions_user_time 
ON user_interactions (user_id, timestamp DESC);

CREATE INDEX CONCURRENTLY idx_user_interactions_item_time 
ON user_interactions (item_id, timestamp DESC);

CREATE INDEX CONCURRENTLY idx_content_items_active_type 
ON content_items (is_active, type) WHERE is_active = true;
```

**Database Schema (Enhanced)**:
```sql
-- Content items with vector embeddings and optimizations
CREATE TABLE content_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(50) NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    text_embedding vector(768),
    multimodal_embedding vector(768),
    metadata JSONB,
    categories TEXT[],
    quality_score FLOAT DEFAULT 0.0,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- User profiles with preference vectors
CREATE TABLE user_profiles (
    user_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    preference_vector vector(768),
    explicit_preferences JSONB,
    behavior_patterns JSONB,
    demographics JSONB,
    last_activity TIMESTAMP DEFAULT NOW(),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Partitioned user interactions table for better performance
CREATE TABLE user_interactions (
    id UUID DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    item_id UUID NOT NULL,
    interaction_type VARCHAR(50) NOT NULL,
    value FLOAT,
    timestamp TIMESTAMP DEFAULT NOW(),
    context JSONB,
    PRIMARY KEY (id, timestamp)
) PARTITION BY RANGE (timestamp);

-- Create monthly partitions
CREATE TABLE user_interactions_2024_01 PARTITION OF user_interactions
    FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');
-- ... create more partitions as needed

-- Foreign key constraints
ALTER TABLE user_interactions ADD CONSTRAINT fk_user_interactions_user 
    FOREIGN KEY (user_id) REFERENCES user_profiles(user_id);
ALTER TABLE user_interactions ADD CONSTRAINT fk_user_interactions_item 
    FOREIGN KEY (item_id) REFERENCES content_items(id);
```

**PgBouncer Monitoring and Health Checks**:
```go
type DatabaseHealthChecker struct {
    writeDB *sql.DB
    readDB  *sql.DB
    pgBouncerAdminURL string
}

func (dhc *DatabaseHealthChecker) CheckHealth() error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    // Check write database connectivity
    if err := dhc.writeDB.PingContext(ctx); err != nil {
        return fmt.Errorf("write database ping failed: %w", err)
    }
    
    // Check read database connectivity
    if err := dhc.readDB.PingContext(ctx); err != nil {
        return fmt.Errorf("read database ping failed: %w", err)
    }
    
    // Check pgvector extension
    var version string
    err := dhc.readDB.QueryRowContext(ctx, "SELECT extversion FROM pg_extension WHERE extname = 'vector'").Scan(&version)
    if err != nil {
        return fmt.Errorf("pgvector extension check failed: %w", err)
    }
    
    // Check PgBouncer stats
    if err := dhc.checkPgBouncerStats(); err != nil {
        log.Warn("PgBouncer stats check failed", "error", err)
        // Don't fail health check for stats issues
    }
    
    return nil
}

func (dhc *DatabaseHealthChecker) checkPgBouncerStats() error {
    // Connect to PgBouncer admin interface
    adminDB, err := sql.Open("postgres", dhc.pgBouncerAdminURL)
    if err != nil {
        return err
    }
    defer adminDB.Close()
    
    // Check pool stats
    rows, err := adminDB.Query("SHOW POOLS")
    if err != nil {
        return err
    }
    defer rows.Close()
    
    for rows.Next() {
        var database, user string
        var clActive, clWaiting, svActive, svIdle, svUsed, svTested, svLogin, maxwait int
        
        err := rows.Scan(&database, &user, &clActive, &clWaiting, &svActive, &svIdle, &svUsed, &svTested, &svLogin, &maxwait)
        if err != nil {
            continue
        }
        
        // Alert if too many waiting clients
        if clWaiting > 10 {
            log.Warn("High client wait queue in PgBouncer", 
                "database", database, 
                "waiting_clients", clWaiting)
        }
        
        // Alert if max wait time is high
        if maxwait > 30 {
            log.Warn("High max wait time in PgBouncer", 
                "database", database, 
                "max_wait_seconds", maxwait)
        }
    }
    
    return nil
}

// PgBouncer metrics for monitoring
type PgBouncerMetrics struct {
    ActiveClients  int
    WaitingClients int
    ActiveServers  int
    IdleServers    int
    MaxWaitTime    int
}

func (dhc *DatabaseHealthChecker) GetPgBouncerMetrics() (*PgBouncerMetrics, error) {
    adminDB, err := sql.Open("postgres", dhc.pgBouncerAdminURL)
    if err != nil {
        return nil, err
    }
    defer adminDB.Close()
    
    var metrics PgBouncerMetrics
    
    // Get aggregated stats
    err = adminDB.QueryRow(`
        SELECT 
            sum(cl_active) as active_clients,
            sum(cl_waiting) as waiting_clients,
            sum(sv_active) as active_servers,
            sum(sv_idle) as idle_servers,
            max(maxwait) as max_wait_time
        FROM (SELECT * FROM SHOW POOLS) AS pools
    `).Scan(
        &metrics.ActiveClients,
        &metrics.WaitingClients,
        &metrics.ActiveServers,
        &metrics.IdleServers,
        &metrics.MaxWaitTime,
    )
    
    return &metrics, err
}
```

**PgBouncer Benefits**:
- **Massive connection efficiency**: Handle 1000+ client connections with only 25-30 database connections
- **Transaction-level pooling**: Optimal for recommendation workloads with short transactions
- **Automatic failover**: Built-in connection retry and error handling
- **Monitoring**: Admin interface for real-time pool statistics
- **Resource optimization**: Dramatically reduces PostgreSQL connection overhead

**Design Decisions**:
- **pgxpool**: High-performance connection pooling with health checks
- **Read/Write splitting**: Separate read replicas for similarity searches
- **Connection limits**: 25 max connections with 5 minimum for optimal resource usage
- **Vector index optimization**: IVFFlat indexes with tuned list parameters
- **Table partitioning**: Monthly partitions for user_interactions to improve query performance
- **Memory optimization**: Increased work_mem for vector operations, shared_buffers for caching

## Error Handling

### Error Categories and Strategies

#### 1. Input Validation Errors
- **Strategy**: Fail fast with detailed error messages
- **Implementation**: JSON Schema validation with custom error formatting
- **Recovery**: Client-side retry with corrected input

#### 2. External Service Failures
- **Strategy**: Circuit breaker pattern with graceful degradation
- **Implementation**: Hystrix-style circuit breakers with fallback mechanisms
- **Recovery**: Cached responses, default recommendations, or simplified algorithms

#### 3. Model Inference Errors
- **Strategy**: Model fallback hierarchy
- **Implementation**: Primary model → Backup model → Rule-based fallback
- **Recovery**: Automatic model health monitoring and switching

#### 4. Database Connection Issues
- **Strategy**: Connection pooling with retry logic
- **Implementation**: Exponential backoff with jitter
- **Recovery**: Read replicas for queries, write queuing for updates

### Error Response Format
```typescript
interface ErrorResponse {
  error: {
    code: string
    message: string
    details?: Record<string, any>
    timestamp: string
    requestId: string
  }
  fallback?: {
    used: boolean
    strategy: string
    confidence: number
  }
}
```

## Testing Strategy

### 1. Unit Testing
- **Component isolation**: Mock external dependencies
- **Algorithm testing**: Validate recommendation logic with synthetic data
- **Model testing**: Embedding quality and consistency tests

### 2. Integration Testing
- **API contract testing**: Ensure interface compatibility
- **Database integration**: Test query performance and data consistency
- **Cache integration**: Validate cache hit/miss scenarios

### 3. Performance Testing
- **Load testing**: Simulate concurrent recommendation requests
- **Embedding generation**: Test batch processing performance
- **Cache performance**: Measure hit rates and response times

### 4. A/B Testing Framework
- **Strategy comparison**: Test different recommendation algorithms
- **Model versions**: Compare embedding model performance
- **User experience**: Measure engagement and conversion metrics

### Test Data Strategy
```typescript
interface TestDataGenerator {
  generateSyntheticUsers(count: number): UserProfile[]
  generateSyntheticContent(count: number): ContentItem[]
  generateInteractionHistory(users: UserProfile[], items: ContentItem[]): Interaction[]
  createTestScenarios(): TestScenario[]
}

interface TestScenario {
  name: string
  userContext: UserContext
  expectedBehavior: ExpectedResult
  metrics: PerformanceMetrics
}
```

### Quality Metrics
- **Recommendation Quality**: Precision@K, Recall@K, NDCG
- **Diversity Metrics**: Intra-list diversity, coverage
- **Business Metrics**: Click-through rate, conversion rate, user engagement
- **System Metrics**: Response time, cache hit rate, error rate

## Development and Testing Strategy

### Unit Testing Approach
```go
// Example unit test for recommendation algorithm
func TestSemanticSearchAlgorithm(t *testing.T) {
    // Setup test database with known data
    db := setupTestDB(t)
    defer db.Close()
    
    algorithm := &SemanticSearchAlgorithm{
        vectorDB:  db,
        dimension: 768,
    }
    
    // Test with known embedding
    queryEmbedding := []float32{0.1, 0.2, 0.3, /* ... */}
    filters := SearchFilters{
        ContentType: "product",
        Categories:  []string{"electronics"},
        Limit:       10,
    }
    
    results, err := algorithm.Search(queryEmbedding, filters)
    
    assert.NoError(t, err)
    assert.Len(t, results, 10)
    assert.True(t, results[0].Score >= results[1].Score) // Sorted by score
}

// Test with mocked dependencies
func TestRecommendationOrchestrator(t *testing.T) {
    mockComputation := &MockComputationService{}
    mockCache := &MockCacheManager{}
    
    orchestrator := &RecommendationOrchestrator{
        computationService: mockComputation,
        cacheManager:      mockCache,
    }
    
    // Setup expectations
    mockComputation.On("GenerateRecommendations", mock.Anything).Return([]ScoredItem{
        {ItemID: "item1", Score: 0.9},
        {ItemID: "item2", Score: 0.8},
    }, nil)
    
    // Test
    request := RecommendationRequest{
        UserID: "user123",
        Count:  10,
    }
    
    result, err := orchestrator.GenerateRecommendations(request)
    
    assert.NoError(t, err)
    assert.Len(t, result.Items, 2)
    mockComputation.AssertExpectations(t)
}
```

### Docker Compose for Local Development
```yaml
# docker-compose.dev.yml
version: '3.8'

services:
  # Application
  recommendation-engine:
    build:
      context: .
      dockerfile: Dockerfile.dev
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=postgres://dev_user:dev_pass@postgres:5432/recommendations_dev
      - NEO4J_URL=bolt://neo4j:7687
      - REDIS_HOT_URL=redis://redis-hot:6379
      - REDIS_WARM_URL=redis://redis-warm:6379
      - REDIS_COLD_URL=redis://redis-cold:6379
      - KAFKA_BROKERS=kafka:9092
      - JWT_SECRET=dev-secret-key
      - LOG_LEVEL=debug
    volumes:
      - .:/app
      - ./models:/app/models
    depends_on:
      - postgres
      - neo4j
      - redis-hot
      - kafka
    command: go run main.go
  
  # Frontend (separate container)
  frontend:
    build:
      context: ./frontend
      dockerfile: Dockerfile.dev
    ports:
      - "3000:3000"
    environment:
      - API_URL=http://recommendation-engine:8080
    volumes:
      - ./frontend:/app
    command: npm run dev
  
  # Development databases
  postgres:
    image: postgres:15
    environment:
      - POSTGRES_DB=recommendations_dev
      - POSTGRES_USER=dev_user
      - POSTGRES_PASSWORD=dev_pass
    ports:
      - "5432:5432"
    volumes:
      - postgres_dev_data:/var/lib/postgresql/data
      - ./scripts/init-dev.sql:/docker-entrypoint-initdb.d/init.sql
  
  neo4j:
    image: neo4j:5.12
    environment:
      - NEO4J_AUTH=neo4j/dev_password
      - NEO4J_PLUGINS=["graph-data-science"]
    ports:
      - "7474:7474"
      - "7687:7687"
    volumes:
      - neo4j_dev_data:/data
  
  # Redis instances
  redis-hot:
    image: redis:7-alpine
    ports: ["6379:6379"]
    command: redis-server --maxmemory 1gb --maxmemory-policy allkeys-lru
  
  redis-warm:
    image: redis:7-alpine
    ports: ["6380:6379"]
    command: redis-server --maxmemory 512mb --maxmemory-policy allkeys-lru
  
  redis-cold:
    image: redis:7-alpine
    ports: ["6381:6379"]
    command: redis-server --maxmemory 2gb --maxmemory-policy allkeys-lfu
  
  # Kafka for development
  kafka:
    image: confluentinc/cp-kafka:latest
    ports: ["9092:9092"]
    environment:
      - KAFKA_ZOOKEEPER_CONNECT=zookeeper:2181
      - KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://kafka:9092
      - KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR=1
    depends_on: [zookeeper]
  
  zookeeper:
    image: confluentinc/cp-zookeeper:latest
    environment:
      - ZOOKEEPER_CLIENT_PORT=2181
  
  # Development tools
  pgbouncer:
    image: pgbouncer/pgbouncer:latest
    environment:
      - DATABASES_HOST=postgres
      - DATABASES_PORT=5432
      - DATABASES_USER=dev_user
      - DATABASES_PASSWORD=dev_pass
      - DATABASES_DBNAME=recommendations_dev
    ports: ["6432:6432"]
    depends_on: [postgres]

volumes:
  postgres_dev_data:
  neo4j_dev_data:
```

### Application Lifecycle Management
```go
type Application struct {
    config           *Config
    server           *gin.Engine
    db               *sql.DB
    neo4jDriver      neo4j.Driver
    redisClients     map[string]*redis.Client
    kafkaProducer    *kafka.Writer
    kafkaConsumer    *kafka.Reader
    
    // Services
    embeddingService *EmbeddingService
    computeService   *ComputationService
    cacheManager     *CacheManager
    
    // Lifecycle
    shutdownChan     chan os.Signal
    healthChecker    *HealthChecker
}

func (app *Application) Start() error {
    // Initialize services in order
    if err := app.initializeDatabase(); err != nil {
        return fmt.Errorf("database initialization failed: %w", err)
    }
    
    if err := app.initializeRedis(); err != nil {
        return fmt.Errorf("redis initialization failed: %w", err)
    }
    
    if err := app.initializeNeo4j(); err != nil {
        return fmt.Errorf("neo4j initialization failed: %w", err)
    }
    
    if err := app.initializeKafka(); err != nil {
        return fmt.Errorf("kafka initialization failed: %w", err)
    }
    
    // Initialize ML models
    if err := app.initializeMLModels(); err != nil {
        return fmt.Errorf("ML models initialization failed: %w", err)
    }
    
    // Start background services
    app.startBackgroundServices()
    
    // Setup HTTP routes
    app.setupRoutes()
    
    // Start health checker
    go app.healthChecker.Start()
    
    // Setup graceful shutdown
    app.setupGracefulShutdown()
    
    log.Info("Application started successfully")
    return app.server.Run(":8080")
}

func (app *Application) setupGracefulShutdown() {
    signal.Notify(app.shutdownChan, syscall.SIGINT, syscall.SIGTERM)
    
    go func() {
        <-app.shutdownChan
        log.Info("Shutdown signal received, starting graceful shutdown...")
        
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        
        // Stop accepting new requests
        // Finish processing existing requests
        // Close database connections
        // Stop background services
        
        app.shutdown(ctx)
        os.Exit(0)
    }()
}

func (app *Application) shutdown(ctx context.Context) {
    // Close services in reverse order
    if app.kafkaConsumer != nil {
        app.kafkaConsumer.Close()
    }
    if app.kafkaProducer != nil {
        app.kafkaProducer.Close()
    }
    
    for name, client := range app.redisClients {
        if err := client.Close(); err != nil {
            log.Error("Failed to close Redis client", "name", name, "error", err)
        }
    }
    
    if app.neo4jDriver != nil {
        app.neo4jDriver.Close()
    }
    
    if app.db != nil {
        app.db.Close()
    }
    
    log.Info("Graceful shutdown completed")
}
```