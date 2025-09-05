# Monitoring, Analytics, and Frontend

This document describes the monitoring, analytics, and frontend components of the recommendation engine.

## Overview

The system includes comprehensive monitoring and analytics capabilities:

- **Business Metrics Collection**: Track CTR, conversion rates, user engagement
- **Prometheus Monitoring**: System metrics, performance monitoring, alerting
- **Node.js Frontend**: User dashboard and admin interface
- **Grafana Dashboards**: Visualization and monitoring
- **Real-time Updates**: WebSocket connections for live data

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Frontend      │    │   Go Backend    │    │   Monitoring    │
│   (Node.js)     │◄──►│ (Gin + Metrics) │◄──►│  (Prometheus)   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         │                       ▼                       ▼
         │              ┌─────────────────┐    ┌─────────────────┐
         │              │   PostgreSQL    │    │    Grafana      │
         │              │   (Metrics DB)  │    │ (Visualization) │
         │              └─────────────────┘    └─────────────────┘
         │
         ▼
┌─────────────────┐
│   WebSocket     │
│  (Real-time)    │
└─────────────────┘
```

## Components

### 1. Business Metrics Collection

**Location**: `internal/services/metrics_collector.go`

**Features**:
- Event-driven metrics collection
- Batch processing for performance
- Real-time and aggregated metrics
- Cohort analysis
- A/B testing support

**Key Metrics**:
- Click-through rate (CTR)
- Conversion rate
- User engagement time
- Recommendation diversity
- Algorithm performance

**Database Schema**:
```sql
-- Raw events
CREATE TABLE recommendation_metrics (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    item_id UUID NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    algorithm_used VARCHAR(100) NOT NULL,
    confidence_score FLOAT,
    timestamp TIMESTAMP DEFAULT NOW()
);

-- Aggregated metrics
CREATE TABLE daily_metrics_summary (
    date DATE NOT NULL,
    algorithm_used VARCHAR(100) NOT NULL,
    total_recommendations INTEGER DEFAULT 0,
    total_clicks INTEGER DEFAULT 0,
    click_through_rate FLOAT DEFAULT 0
);
```

### 2. Prometheus Monitoring

**Location**: `internal/services/health.go` (enhanced)

**Custom Metrics**:
- `recommendation_requests_total`: Counter by algorithm, user_tier
- `recommendation_latency_seconds`: Histogram with buckets
- `algorithm_performance_score`: Gauge by algorithm
- `cache_hit_ratio`: Gauge by cache_type
- `database_connection_pool_usage`: Gauge

**System Metrics**:
- CPU usage, memory consumption
- Goroutine count, GC pause times
- HTTP request metrics
- Database connection stats

**Alerting Rules**:
- High error rate (> 5% for 5 minutes)
- High latency (p95 > 2s for 10 minutes)
- Low cache hit rate (< 70% for 15 minutes)
- Database connection pool exhaustion (> 90%)

### 3. Node.js Frontend

**Location**: `frontend/`

**Features**:
- Express.js server with API proxy
- WebSocket support for real-time updates
- Bootstrap 5 UI framework
- Chart.js for data visualization
- Responsive design

**Pages**:
- **Dashboard** (`/`): User recommendations and metrics
- **Admin** (`/admin`): System administration and analytics
- **Health** (`/health`): Service health check

**Key Components**:
```javascript
// WebSocket connection for real-time updates
class RecommendationDashboard {
    setupWebSocket() {
        this.ws = new WebSocket(wsUrl);
        this.ws.onmessage = (event) => {
            this.handleWebSocketMessage(JSON.parse(event.data));
        };
    }
    
    handleRecommendationUpdate(data) {
        // Update UI with new recommendations
        this.displayRecommendations(data.recommendations);
    }
}
```

### 4. Admin Dashboard

**Features**:
- **System Health**: Real-time service status
- **Business Analytics**: Revenue impact, user engagement
- **Content Management**: Ingestion status, error logs
- **User Management**: Segmentation, privacy controls
- **Algorithm Configuration**: Weight adjustments, feature flags
- **A/B Testing**: Test management and results

**Configuration Management**:
```javascript
// Algorithm configuration
const config = {
    algorithms: {
        semantic_search: { weight: 0.4, similarity_threshold: 0.7 },
        collaborative_filtering: { weight: 0.3, similarity_threshold: 0.5 },
        pagerank: { weight: 0.3, damping_factor: 0.85 }
    },
    diversity: {
        intra_list_diversity: 0.3,
        category_max_items: 3,
        serendipity_ratio: 0.15
    }
};
```

## API Endpoints

### Metrics Endpoints

```
GET  /api/v1/metrics/business          # Business metrics
GET  /api/v1/metrics/performance       # Performance data for charts
POST /api/v1/metrics/interactions      # Record user interactions
```

### Admin Endpoints

```
GET  /api/v1/admin/metrics/overview    # Admin dashboard overview
GET  /api/v1/admin/analytics           # Detailed analytics
GET  /api/v1/admin/content/status      # Content ingestion status
GET  /api/v1/admin/users/analytics     # User analytics
GET  /api/v1/admin/monitoring/metrics  # System monitoring
GET  /api/v1/admin/alerts/recent       # Recent alerts
GET  /api/v1/admin/ab-tests            # A/B test data

GET  /api/v1/admin/algorithms/config   # Get algorithm config
PUT  /api/v1/admin/algorithms/config   # Update algorithm config
POST /api/v1/admin/algorithms/test     # Test configuration
```

### Health and Monitoring

```
GET  /health                           # Basic health check
GET  /health/detailed                  # Detailed health status
GET  /metrics                          # Prometheus metrics
```

## Setup and Deployment

### 1. Database Setup

Initialize the metrics database:

```bash
# Run metrics schema
psql -h localhost -U postgres -d recommendations -f scripts/init-metrics.sql
```

### 2. Frontend Setup

```bash
cd frontend
npm install
npm start  # Development mode
```

### 3. Docker Compose

```bash
# Start all services including frontend and monitoring
docker-compose -f docker-compose.dev.yml up -d

# Services will be available at:
# - Frontend: http://localhost:3000
# - Backend API: http://localhost:8080
# - Grafana: http://localhost:3001
# - Prometheus: http://localhost:9090
```

### 4. Grafana Setup

1. Access Grafana at http://localhost:3001
2. Login with admin/admin
3. Dashboards are automatically provisioned
4. Prometheus datasource is pre-configured

## Configuration

### Environment Variables

```bash
# Frontend
NODE_ENV=development
API_URL=http://localhost:8080

# Backend
PROMETHEUS_ENABLED=true
METRICS_PORT=9090
```

### Algorithm Configuration

The admin dashboard allows real-time configuration of:

- Algorithm weights and thresholds
- Diversity and serendipity settings
- Feature flags (ML ranking, real-time learning)
- Caching parameters

## Monitoring and Alerting

### Key Metrics to Monitor

1. **Business Metrics**:
   - Click-through rate trends
   - Conversion rate by algorithm
   - User engagement patterns
   - Revenue impact

2. **System Metrics**:
   - Response time percentiles
   - Error rates
   - Cache hit ratios
   - Database connection pool usage

3. **Algorithm Performance**:
   - Individual algorithm scores
   - A/B test results
   - Model accuracy metrics

### Alert Conditions

```yaml
# Prometheus alerting rules
groups:
  - name: recommendation-engine
    rules:
      - alert: HighErrorRate
        expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High error rate detected"
          
      - alert: HighLatency
        expr: histogram_quantile(0.95, rate(recommendation_latency_seconds_bucket[10m])) > 2
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "High response latency detected"
```

## Performance Optimization

### Frontend Optimization

- **Caching**: Browser caching for static assets
- **Compression**: Gzip compression for responses
- **CDN**: Image optimization and delivery
- **Service Worker**: Offline functionality

### Backend Optimization

- **Connection Pooling**: Database and Redis connections
- **Query Optimization**: Efficient database queries
- **Monitoring Overhead**: < 5% performance impact
- **Batch Processing**: Metrics collection in batches

## Testing

### Frontend Tests

```bash
cd frontend
npm test  # Run Jest tests
```

### Load Testing

```bash
# Test monitoring system performance
ab -n 1000 -c 10 http://localhost:8080/metrics
```

### Integration Tests

```bash
# Test API endpoints
curl -X GET http://localhost:8080/api/v1/metrics/business
curl -X GET http://localhost:8080/health/detailed
```

## Troubleshooting

### Common Issues

1. **WebSocket Connection Failed**:
   - Check frontend server is running
   - Verify WebSocket URL configuration
   - Check firewall settings

2. **Metrics Not Appearing**:
   - Verify Prometheus scraping configuration
   - Check metrics endpoint accessibility
   - Validate metric names and labels

3. **High Memory Usage**:
   - Monitor metrics buffer size
   - Check for metric label cardinality
   - Optimize aggregation queries

4. **Dashboard Loading Slowly**:
   - Optimize database queries
   - Implement proper indexing
   - Use connection pooling

### Debug Commands

```bash
# Check service health
curl http://localhost:8080/health/detailed

# View Prometheus metrics
curl http://localhost:8080/metrics

# Test WebSocket connection
wscat -c ws://localhost:3000

# Check database metrics
psql -h localhost -U postgres -d recommendations -c "SELECT COUNT(*) FROM recommendation_metrics;"
```

## Future Enhancements

1. **Advanced Analytics**:
   - Machine learning model performance tracking
   - Predictive analytics dashboard
   - Automated anomaly detection

2. **Enhanced Monitoring**:
   - Distributed tracing with Jaeger
   - Log aggregation with ELK stack
   - Custom alerting integrations

3. **Frontend Improvements**:
   - Progressive Web App (PWA) features
   - Advanced data visualization
   - Mobile-responsive design

4. **Performance Optimization**:
   - Edge caching for metrics
   - Real-time streaming analytics
   - Automated scaling based on metrics