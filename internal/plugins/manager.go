package plugins

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// PluginManager manages external system plugins
type PluginManager struct {
	plugins  map[string]ExternalSystemPlugin
	configs  map[string]*PluginConfig
	statuses map[string]*PluginStatus
	redis    *redis.Client
	logger   *logrus.Logger
	mu       sync.RWMutex

	// Worker pool for async processing
	workerCount int
	jobQueue    chan *EnrichmentJob
	workers     []*Worker
	stopChan    chan struct{}
}

// EnrichmentJob represents a user enrichment job
type EnrichmentJob struct {
	UserID     uuid.UUID
	Plugins    []string
	Timeout    time.Duration
	ResultChan chan *EnrichmentResult
}

// EnrichmentResult represents the result of user enrichment
type EnrichmentResult struct {
	UserID      uuid.UUID
	Enrichments []*UserEnrichment
	Errors      map[string]error
	ProcessTime time.Duration
}

// Worker processes enrichment jobs
type Worker struct {
	id      int
	manager *PluginManager
	jobChan chan *EnrichmentJob
	quit    chan struct{}
}

// NewPluginManager creates a new plugin manager
func NewPluginManager(redis *redis.Client, logger *logrus.Logger, workerCount int) *PluginManager {
	return &PluginManager{
		plugins:     make(map[string]ExternalSystemPlugin),
		configs:     make(map[string]*PluginConfig),
		statuses:    make(map[string]*PluginStatus),
		redis:       redis,
		logger:      logger,
		workerCount: workerCount,
		jobQueue:    make(chan *EnrichmentJob, 100),
		stopChan:    make(chan struct{}),
	}
}

// LoadConfig loads plugin configurations from YAML
func (pm *PluginManager) LoadConfig(configData []byte) error {
	var configs []PluginConfig
	if err := yaml.Unmarshal(configData, &configs); err != nil {
		return fmt.Errorf("failed to parse plugin config: %w", err)
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, config := range configs {
		pm.configs[config.Name] = &config
		pm.statuses[config.Name] = &PluginStatus{
			Name:      config.Name,
			Healthy:   false,
			LastCheck: time.Now(),
		}
	}

	return nil
}

// RegisterPlugin registers a plugin with the manager
func (pm *PluginManager) RegisterPlugin(plugin ExternalSystemPlugin) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	name := plugin.Name()
	config, exists := pm.configs[name]
	if !exists {
		return fmt.Errorf("no configuration found for plugin: %s", name)
	}

	if !config.Enabled {
		pm.logger.Info("Plugin is disabled", "plugin", name)
		return nil
	}

	// Initialize plugin
	if err := plugin.Initialize(config.Config); err != nil {
		pm.logger.Error("Failed to initialize plugin", "plugin", name, "error", err)
		return fmt.Errorf("failed to initialize plugin %s: %w", name, err)
	}

	pm.plugins[name] = plugin
	pm.statuses[name].Healthy = true
	pm.statuses[name].LastCheck = time.Now()

	pm.logger.Info("Plugin registered successfully", "plugin", name)
	return nil
}

// Start starts the plugin manager and worker pool
func (pm *PluginManager) Start() error {
	pm.logger.Info("Starting plugin manager", "workers", pm.workerCount)

	// Start workers
	for i := 0; i < pm.workerCount; i++ {
		worker := &Worker{
			id:      i,
			manager: pm,
			jobChan: make(chan *EnrichmentJob),
			quit:    make(chan struct{}),
		}
		pm.workers = append(pm.workers, worker)
		go worker.start()
	}

	// Start job dispatcher
	go pm.dispatch()

	// Start health check routine
	go pm.healthCheckRoutine()

	return nil
}

// Stop stops the plugin manager and all workers
func (pm *PluginManager) Stop() error {
	pm.logger.Info("Stopping plugin manager")

	close(pm.stopChan)

	// Stop all workers
	for _, worker := range pm.workers {
		close(worker.quit)
	}

	// Cleanup all plugins
	pm.mu.RLock()
	for name, plugin := range pm.plugins {
		if err := plugin.Cleanup(); err != nil {
			pm.logger.Error("Failed to cleanup plugin", "plugin", name, "error", err)
		}
	}
	pm.mu.RUnlock()

	return nil
}

// EnrichUser enriches user data using configured plugins
func (pm *PluginManager) EnrichUser(ctx context.Context, userID uuid.UUID, pluginNames []string) (*EnrichmentResult, error) {
	// If no specific plugins requested, use all enabled plugins
	if len(pluginNames) == 0 {
		pm.mu.RLock()
		for name, config := range pm.configs {
			if config.Enabled {
				pluginNames = append(pluginNames, name)
			}
		}
		pm.mu.RUnlock()
	}

	// Check cache first
	if cached := pm.getCachedEnrichment(ctx, userID); cached != nil {
		pm.logger.Debug("Using cached enrichment", "user_id", userID)
		return cached, nil
	}

	// Create enrichment job
	job := &EnrichmentJob{
		UserID:     userID,
		Plugins:    pluginNames,
		Timeout:    5 * time.Second, // Default timeout
		ResultChan: make(chan *EnrichmentResult, 1),
	}

	// Submit job
	select {
	case pm.jobQueue <- job:
		// Job submitted successfully
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return nil, fmt.Errorf("job queue is full")
	}

	// Wait for result
	select {
	case result := <-job.ResultChan:
		// Cache successful result
		if len(result.Enrichments) > 0 {
			pm.cacheEnrichment(ctx, userID, result)
		}
		return result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// GetPluginStatus returns the status of all plugins
func (pm *PluginManager) GetPluginStatus() map[string]*PluginStatus {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	status := make(map[string]*PluginStatus)
	for name, s := range pm.statuses {
		status[name] = &PluginStatus{
			Name:           s.Name,
			Healthy:        s.Healthy,
			LastCheck:      s.LastCheck,
			Error:          s.Error,
			ProcessedCount: s.ProcessedCount,
			ErrorCount:     s.ErrorCount,
		}
	}

	return status
}

// dispatch distributes jobs to workers
func (pm *PluginManager) dispatch() {
	for {
		select {
		case job := <-pm.jobQueue:
			// Find available worker
			jobHandled := false
			for _, worker := range pm.workers {
				select {
				case worker.jobChan <- job:
					jobHandled = true
				default:
					continue
				}
				if jobHandled {
					break
				}
			}
			// If no worker available, handle with fallback
			if !jobHandled {
				pm.handleJobFallback(job)
			}
		case <-pm.stopChan:
			return
		}
	}
}

// handleJobFallback handles jobs when no workers are available
func (pm *PluginManager) handleJobFallback(job *EnrichmentJob) {
	result := &EnrichmentResult{
		UserID:      job.UserID,
		Enrichments: []*UserEnrichment{},
		Errors:      make(map[string]error),
		ProcessTime: 0,
	}

	for _, pluginName := range job.Plugins {
		result.Errors[pluginName] = fmt.Errorf("no workers available")
	}

	select {
	case job.ResultChan <- result:
	default:
	}
}

// healthCheckRoutine periodically checks plugin health
func (pm *PluginManager) healthCheckRoutine() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pm.performHealthChecks()
		case <-pm.stopChan:
			return
		}
	}
}

// performHealthChecks checks the health of all plugins
func (pm *PluginManager) performHealthChecks() {
	pm.mu.RLock()
	plugins := make(map[string]ExternalSystemPlugin)
	for name, plugin := range pm.plugins {
		plugins[name] = plugin
	}
	pm.mu.RUnlock()

	for name, plugin := range plugins {
		err := plugin.HealthCheck()

		pm.mu.Lock()
		status := pm.statuses[name]
		status.LastCheck = time.Now()
		if err != nil {
			status.Healthy = false
			status.Error = err.Error()
			pm.logger.Warn("Plugin health check failed", "plugin", name, "error", err)
		} else {
			status.Healthy = true
			status.Error = ""
		}
		pm.mu.Unlock()
	}
}

// getCachedEnrichment retrieves cached enrichment data
func (pm *PluginManager) getCachedEnrichment(ctx context.Context, userID uuid.UUID) *EnrichmentResult {
	if pm.redis == nil {
		return nil
	}

	key := fmt.Sprintf("enrichment:%s", userID.String())
	data := pm.redis.Get(ctx, key).Val()
	if data == "" {
		return nil
	}

	// In a real implementation, this would deserialize the cached data
	// For now, return nil to force fresh enrichment
	return nil
}

// cacheEnrichment caches enrichment data
func (pm *PluginManager) cacheEnrichment(ctx context.Context, userID uuid.UUID, result *EnrichmentResult) {
	if pm.redis == nil {
		return
	}

	key := fmt.Sprintf("enrichment:%s", userID.String())
	// In a real implementation, this would serialize and cache the result
	// For now, just set a placeholder
	pm.redis.Set(ctx, key, "cached", 1*time.Hour)
}

// Worker implementation

func (w *Worker) start() {
	w.manager.logger.Debug("Starting worker", "worker_id", w.id)

	for {
		select {
		case job := <-w.jobChan:
			w.processJob(job)
		case <-w.quit:
			w.manager.logger.Debug("Stopping worker", "worker_id", w.id)
			return
		}
	}
}

func (w *Worker) processJob(job *EnrichmentJob) {
	startTime := time.Now()
	result := &EnrichmentResult{
		UserID:      job.UserID,
		Enrichments: []*UserEnrichment{},
		Errors:      make(map[string]error),
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), job.Timeout)
	defer cancel()

	// Process each plugin
	for _, pluginName := range job.Plugins {
		w.manager.mu.RLock()
		plugin, exists := w.manager.plugins[pluginName]
		status := w.manager.statuses[pluginName]
		w.manager.mu.RUnlock()

		if !exists {
			result.Errors[pluginName] = fmt.Errorf("plugin not found")
			continue
		}

		if !status.Healthy {
			result.Errors[pluginName] = fmt.Errorf("plugin is unhealthy")
			continue
		}

		// Process with plugin
		enrichment, err := plugin.Process(ctx, job.UserID)
		if err != nil {
			result.Errors[pluginName] = err
			w.manager.mu.Lock()
			status.ErrorCount++
			w.manager.mu.Unlock()
			w.manager.logger.Warn("Plugin processing failed",
				"plugin", pluginName, "user_id", job.UserID, "error", err)
		} else {
			result.Enrichments = append(result.Enrichments, enrichment)
			w.manager.mu.Lock()
			status.ProcessedCount++
			w.manager.mu.Unlock()
		}
	}

	result.ProcessTime = time.Since(startTime)

	// Send result
	select {
	case job.ResultChan <- result:
	default:
		w.manager.logger.Warn("Failed to send job result", "user_id", job.UserID)
	}
}
