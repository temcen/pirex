package services

import (
	"context"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/temcen/pirex/internal/config"
	"github.com/temcen/pirex/internal/database"
)

type HealthService struct {
	config *config.Config
	logger *logrus.Logger
	db     *database.Database

	// Prometheus metrics
	healthCheckStatus   *prometheus.GaugeVec
	lastHealthCheck     *prometheus.GaugeVec
	systemMetrics       *prometheus.GaugeVec
	dbConnectionMetrics *prometheus.GaugeVec
}

type HealthStatus struct {
	Status      string                 `json:"status"`
	Timestamp   time.Time              `json:"timestamp"`
	Services    map[string]string      `json:"services"`
	Critical    []string               `json:"critical_failures,omitempty"`
	NonCritical []string               `json:"non_critical_failures,omitempty"`
	Latency     time.Duration          `json:"latency,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

func NewHealthService(cfg *config.Config, logger *logrus.Logger, db *database.Database) *HealthService {
	hs := &HealthService{
		config: cfg,
		logger: logger,
		db:     db,
	}

	// Initialize Prometheus metrics with error handling
	hs.healthCheckStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "health_check_status",
		Help: "Health check status (1 = healthy, 0 = unhealthy)",
	}, []string{"service"})

	hs.lastHealthCheck = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "health_check_timestamp",
		Help: "Timestamp of last health check",
	}, []string{"service"})

	hs.systemMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "system_info",
		Help: "System information metrics",
	}, []string{"metric_type"})

	hs.dbConnectionMetrics = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "database_connection_pool_usage",
		Help: "Database connection pool usage percentage",
	}, []string{"database", "state"})

	// Register metrics with error handling - ignore if already registered
	if err := prometheus.Register(hs.healthCheckStatus); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			logger.WithError(err).Warn("Failed to register health_check_status metric")
		}
	}
	if err := prometheus.Register(hs.lastHealthCheck); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			logger.WithError(err).Warn("Failed to register health_check_timestamp metric")
		}
	}
	if err := prometheus.Register(hs.systemMetrics); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			logger.WithError(err).Warn("Failed to register system_info metric")
		}
	}
	if err := prometheus.Register(hs.dbConnectionMetrics); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			logger.WithError(err).Warn("Failed to register database_connection_pool_usage metric")
		}
	}

	// Start background metrics collection
	go hs.collectSystemMetrics()
	go hs.collectDatabaseMetrics()

	return hs
}

func (s *HealthService) CheckHealth() *HealthStatus {
	status := &HealthStatus{
		Timestamp: time.Now(),
		Services:  make(map[string]string),
	}

	// Critical dependencies
	criticalServices := map[string]func() error{
		"postgresql": s.checkPostgreSQL,
		"redis_hot":  s.checkRedisHot,
	}

	// Non-critical dependencies
	nonCriticalServices := map[string]func() error{
		"neo4j":      s.checkNeo4j,
		"redis_warm": s.checkRedisWarm,
		"redis_cold": s.checkRedisCold,
	}

	// Check critical services
	allCriticalHealthy := true
	for name, checkFunc := range criticalServices {
		if err := checkFunc(); err != nil {
			status.Services[name] = "unhealthy"
			status.Critical = append(status.Critical, name)
			allCriticalHealthy = false
			s.logger.WithError(err).Errorf("Critical service %s is unhealthy", name)
			s.UpdateHealthMetrics(name, false)
		} else {
			status.Services[name] = "healthy"
			s.UpdateHealthMetrics(name, true)
		}
	}

	// Check non-critical services
	for name, checkFunc := range nonCriticalServices {
		if err := checkFunc(); err != nil {
			status.Services[name] = "unhealthy"
			status.NonCritical = append(status.NonCritical, name)
			s.logger.WithError(err).Warnf("Non-critical service %s is unhealthy", name)
			s.UpdateHealthMetrics(name, false)
		} else {
			status.Services[name] = "healthy"
			s.UpdateHealthMetrics(name, true)
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

func (s *HealthService) checkPostgreSQL() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.db.PG.Ping(ctx)
}

func (s *HealthService) checkNeo4j() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.db.Neo4j.VerifyConnectivity(ctx)
}

func (s *HealthService) checkRedisHot() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.db.Redis.Hot.Ping(ctx).Err()
}

func (s *HealthService) checkRedisWarm() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.db.Redis.Warm.Ping(ctx).Err()
}

func (s *HealthService) checkRedisCold() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.db.Redis.Cold.Ping(ctx).Err()
}

// collectSystemMetrics collects system-level metrics
func (s *HealthService) collectSystemMetrics() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	var memStats runtime.MemStats

	for range ticker.C {
		// Collect memory statistics
		runtime.ReadMemStats(&memStats)

		s.systemMetrics.WithLabelValues("memory_alloc_bytes").Set(float64(memStats.Alloc))
		s.systemMetrics.WithLabelValues("memory_sys_bytes").Set(float64(memStats.Sys))
		s.systemMetrics.WithLabelValues("goroutines_count").Set(float64(runtime.NumGoroutine()))
		s.systemMetrics.WithLabelValues("gc_runs_total").Set(float64(memStats.NumGC))

		// Record GC pause time
		if len(memStats.PauseNs) > 0 {
			lastPause := memStats.PauseNs[(memStats.NumGC+255)%256]
			s.systemMetrics.WithLabelValues("gc_pause_ns").Set(float64(lastPause))
		}
	}
}

// collectDatabaseMetrics collects database connection metrics
func (s *HealthService) collectDatabaseMetrics() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if s.db != nil && s.db.PG != nil {
			// Get PostgreSQL connection pool stats
			stats := s.db.PG.Stat()

			s.dbConnectionMetrics.WithLabelValues("postgresql", "acquired_conns").Set(float64(stats.AcquiredConns()))
			s.dbConnectionMetrics.WithLabelValues("postgresql", "constructing_conns").Set(float64(stats.ConstructingConns()))
			s.dbConnectionMetrics.WithLabelValues("postgresql", "idle_conns").Set(float64(stats.IdleConns()))
			s.dbConnectionMetrics.WithLabelValues("postgresql", "max_conns").Set(float64(stats.MaxConns()))
			s.dbConnectionMetrics.WithLabelValues("postgresql", "total_conns").Set(float64(stats.TotalConns()))

			// Calculate usage percentage
			if stats.MaxConns() > 0 {
				usage := float64(stats.AcquiredConns()) / float64(stats.MaxConns()) * 100
				s.dbConnectionMetrics.WithLabelValues("postgresql", "usage_percent").Set(usage)
			}
		}
	}
}

// UpdateHealthMetrics updates health check metrics
func (s *HealthService) UpdateHealthMetrics(serviceName string, healthy bool) {
	if healthy {
		s.healthCheckStatus.WithLabelValues(serviceName).Set(1)
	} else {
		s.healthCheckStatus.WithLabelValues(serviceName).Set(0)
	}
	s.lastHealthCheck.WithLabelValues(serviceName).Set(float64(time.Now().Unix()))
}
