package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

// MonitoringService handles system monitoring and health checks
type MonitoringService struct {
	db           *sql.DB
	redisClients map[string]*redis.Client

	// System metrics
	cpuUsage       prometheus.Gauge
	memoryUsage    prometheus.Gauge
	goroutineCount prometheus.Gauge
	gcPauseTime    prometheus.Histogram

	// Application metrics
	httpRequestsTotal   *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec
	activeConnections   prometheus.Gauge

	// Health check metrics
	healthCheckStatus *prometheus.GaugeVec
	lastHealthCheck   *prometheus.GaugeVec

	// Cache metrics
	cacheOperations *prometheus.CounterVec
	cacheHitRate    *prometheus.GaugeVec
	cacheSize       *prometheus.GaugeVec

	// Database metrics
	dbConnections   *prometheus.GaugeVec
	dbQueryDuration *prometheus.HistogramVec
	dbQueryErrors   *prometheus.CounterVec

	mu           sync.RWMutex
	healthStatus map[string]HealthStatus
}

// NewMonitoringService creates a new monitoring service
func NewMonitoringService(db *sql.DB, redisClients map[string]*redis.Client) *MonitoringService {
	ms := &MonitoringService{
		db:           db,
		redisClients: redisClients,
		healthStatus: make(map[string]HealthStatus),

		// System metrics
		cpuUsage: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "system_cpu_usage_percent",
			Help: "Current CPU usage percentage",
		}),

		memoryUsage: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "system_memory_usage_bytes",
			Help: "Current memory usage in bytes",
		}),

		goroutineCount: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "system_goroutines_count",
			Help: "Current number of goroutines",
		}),

		gcPauseTime: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "system_gc_pause_seconds",
			Help:    "Garbage collection pause time in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0},
		}),

		// Application metrics
		httpRequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		}, []string{"method", "endpoint", "status"}),

		httpRequestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1.0, 2.0, 5.0},
		}, []string{"method", "endpoint"}),

		activeConnections: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "http_active_connections",
			Help: "Number of active HTTP connections",
		}),

		// Health check metrics
		healthCheckStatus: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "health_check_status",
			Help: "Health check status (1 = healthy, 0 = unhealthy)",
		}, []string{"service"}),

		lastHealthCheck: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "health_check_timestamp",
			Help: "Timestamp of last health check",
		}, []string{"service"}),

		// Cache metrics
		cacheOperations: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "cache_operations_total",
			Help: "Total number of cache operations",
		}, []string{"cache_type", "operation", "result"}),

		cacheHitRate: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "cache_hit_rate",
			Help: "Cache hit rate percentage",
		}, []string{"cache_type"}),

		cacheSize: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "cache_size_bytes",
			Help: "Cache size in bytes",
		}, []string{"cache_type"}),

		// Database metrics
		dbConnections: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "database_connections",
			Help: "Number of database connections",
		}, []string{"database", "state"}),

		dbQueryDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "database_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0},
		}, []string{"database", "query_type"}),

		dbQueryErrors: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "database_query_errors_total",
			Help: "Total number of database query errors",
		}, []string{"database", "error_type"}),
	}

	// Start background monitoring
	go ms.collectSystemMetrics()
	go ms.performHealthChecks()
	go ms.collectCacheMetrics()
	go ms.collectDatabaseMetrics()

	return ms
}

// SetupPrometheusEndpoint sets up the Prometheus metrics endpoint
func (ms *MonitoringService) SetupPrometheusEndpoint(router *gin.Engine) {
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
}

// HTTPMetricsMiddleware provides HTTP request metrics middleware
func (ms *MonitoringService) HTTPMetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Increment active connections
		ms.activeConnections.Inc()
		defer ms.activeConnections.Dec()

		// Process request
		c.Next()

		// Record metrics
		duration := time.Since(start)
		status := fmt.Sprintf("%d", c.Writer.Status())

		ms.httpRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), status).Inc()
		ms.httpRequestDuration.WithLabelValues(c.Request.Method, c.FullPath()).Observe(duration.Seconds())
	}
}

// collectSystemMetrics collects system-level metrics
func (ms *MonitoringService) collectSystemMetrics() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	var memStats runtime.MemStats

	for range ticker.C {
		// Collect memory statistics
		runtime.ReadMemStats(&memStats)

		ms.memoryUsage.Set(float64(memStats.Alloc))
		ms.goroutineCount.Set(float64(runtime.NumGoroutine()))

		// Record GC pause time
		if len(memStats.PauseNs) > 0 {
			lastPause := memStats.PauseNs[(memStats.NumGC+255)%256]
			ms.gcPauseTime.Observe(float64(lastPause) / 1e9)
		}
	}
}

// performHealthChecks performs periodic health checks
func (ms *MonitoringService) performHealthChecks() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ms.checkAllServices()
	}
}

// checkAllServices checks the health of all services
func (ms *MonitoringService) checkAllServices() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	services := map[string]func(context.Context) error{
		"postgresql": ms.checkPostgreSQL,
		"redis_hot":  func(ctx context.Context) error { return ms.checkRedis(ctx, "hot") },
		"redis_warm": func(ctx context.Context) error { return ms.checkRedis(ctx, "warm") },
		"redis_cold": func(ctx context.Context) error { return ms.checkRedis(ctx, "cold") },
	}

	status := HealthStatus{
		Timestamp: time.Now(),
		Services:  make(map[string]string),
		Details:   make(map[string]interface{}),
	}

	allHealthy := true

	for serviceName, checkFunc := range services {
		start := time.Now()
		err := checkFunc(ctx)
		latency := time.Since(start)

		if err != nil {
			status.Services[serviceName] = "unhealthy"
			status.Critical = append(status.Critical, serviceName)
			status.Details[serviceName] = err.Error()
			ms.healthCheckStatus.WithLabelValues(serviceName).Set(0)
			allHealthy = false
			log.Printf("Health check failed for %s: %v", serviceName, err)
		} else {
			status.Services[serviceName] = "healthy"
			ms.healthCheckStatus.WithLabelValues(serviceName).Set(1)
		}

		ms.lastHealthCheck.WithLabelValues(serviceName).Set(float64(time.Now().Unix()))
		status.Details[serviceName+"_latency"] = latency.Milliseconds()
	}

	if allHealthy {
		status.Status = "healthy"
	} else {
		status.Status = "unhealthy"
	}

	ms.mu.Lock()
	ms.healthStatus["overall"] = status
	ms.mu.Unlock()
}

// checkPostgreSQL checks PostgreSQL health
func (ms *MonitoringService) checkPostgreSQL(ctx context.Context) error {
	if ms.db == nil {
		return fmt.Errorf("database connection is nil")
	}

	var result int
	err := ms.db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		return fmt.Errorf("database query failed: %w", err)
	}

	if result != 1 {
		return fmt.Errorf("unexpected query result: %d", result)
	}

	return nil
}

// checkRedis checks Redis health
func (ms *MonitoringService) checkRedis(ctx context.Context, cacheType string) error {
	client, exists := ms.redisClients[cacheType]
	if !exists {
		return fmt.Errorf("redis client for %s not found", cacheType)
	}

	if client == nil {
		return fmt.Errorf("redis client for %s is nil", cacheType)
	}

	result := client.Ping(ctx)
	if result.Err() != nil {
		return fmt.Errorf("redis ping failed: %w", result.Err())
	}

	return nil
}

// collectCacheMetrics collects cache-related metrics
func (ms *MonitoringService) collectCacheMetrics() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		for cacheType, client := range ms.redisClients {
			if client == nil {
				continue
			}

			// Get cache info
			info := client.Info(ctx, "memory")
			if info.Err() == nil {
				// Parse memory usage from info string
				// This is a simplified version - in production, you'd parse the actual info
				ms.cacheSize.WithLabelValues(cacheType).Set(1024 * 1024) // Placeholder
			}

			// Get cache statistics (if available)
			stats := client.Info(ctx, "stats")
			if stats.Err() == nil {
				// Parse hit/miss statistics
				// This is a simplified version - in production, you'd parse the actual stats
				ms.cacheHitRate.WithLabelValues(cacheType).Set(85.0) // Placeholder
			}
		}

		cancel()
	}
}

// collectDatabaseMetrics collects database-related metrics
func (ms *MonitoringService) collectDatabaseMetrics() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		if ms.db == nil {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		// Get database connection statistics
		stats := ms.db.Stats()
		ms.dbConnections.WithLabelValues("postgresql", "open").Set(float64(stats.OpenConnections))
		ms.dbConnections.WithLabelValues("postgresql", "in_use").Set(float64(stats.InUse))
		ms.dbConnections.WithLabelValues("postgresql", "idle").Set(float64(stats.Idle))

		// Test query performance
		start := time.Now()
		var result int
		err := ms.db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
		duration := time.Since(start)

		if err != nil {
			ms.dbQueryErrors.WithLabelValues("postgresql", "connection").Inc()
		} else {
			ms.dbQueryDuration.WithLabelValues("postgresql", "health_check").Observe(duration.Seconds())
		}

		cancel()
	}
}

// RecordCacheOperation records a cache operation for metrics
func (ms *MonitoringService) RecordCacheOperation(cacheType, operation, result string) {
	ms.cacheOperations.WithLabelValues(cacheType, operation, result).Inc()
}

// RecordDatabaseQuery records a database query for metrics
func (ms *MonitoringService) RecordDatabaseQuery(database, queryType string, duration time.Duration, err error) {
	ms.dbQueryDuration.WithLabelValues(database, queryType).Observe(duration.Seconds())

	if err != nil {
		ms.dbQueryErrors.WithLabelValues(database, "query_error").Inc()
	}
}

// GetHealthStatus returns the current health status
func (ms *MonitoringService) GetHealthStatus() HealthStatus {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if status, exists := ms.healthStatus["overall"]; exists {
		return status
	}

	return HealthStatus{
		Status:    "unknown",
		Timestamp: time.Now(),
		Services:  make(map[string]string),
	}
}

// GetDetailedHealthStatus returns detailed health information
func (ms *MonitoringService) GetDetailedHealthStatus() map[string]interface{} {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	details := make(map[string]interface{})

	// System information
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	details["system"] = map[string]interface{}{
		"goroutines":   runtime.NumGoroutine(),
		"memory_alloc": memStats.Alloc,
		"memory_sys":   memStats.Sys,
		"gc_runs":      memStats.NumGC,
		"last_gc":      time.Unix(0, int64(memStats.LastGC)),
	}

	// Database information
	if ms.db != nil {
		stats := ms.db.Stats()
		details["database"] = map[string]interface{}{
			"open_connections": stats.OpenConnections,
			"in_use":           stats.InUse,
			"idle":             stats.Idle,
			"wait_count":       stats.WaitCount,
			"wait_duration":    stats.WaitDuration,
		}
	}

	// Cache information
	cacheInfo := make(map[string]interface{})
	for cacheType, client := range ms.redisClients {
		if client != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			info := client.Info(ctx, "server")
			if info.Err() == nil {
				cacheInfo[cacheType] = "connected"
			} else {
				cacheInfo[cacheType] = "disconnected"
			}
			cancel()
		}
	}
	details["cache"] = cacheInfo

	// Health status
	details["health"] = ms.healthStatus

	return details
}

// Close gracefully shuts down the monitoring service
func (ms *MonitoringService) Close() error {
	log.Println("Shutting down monitoring service")
	return nil
}
