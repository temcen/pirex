package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

// Manager handles plugin lifecycle and orchestration
type Manager struct {
	plugins       map[string]ExternalSystemPlugin
	configs       map[string]map[string]interface{}
	cache         *redis.Client
	logger        *logrus.Logger
	pluginDir     string
	cacheTTL      time.Duration
	timeout       time.Duration
	maxRetries    int
	retryDelay    time.Duration
	mu            sync.RWMutex
	healthChecker *HealthChecker
}

// ManagerConfig contains configuration for the plugin manager
type ManagerConfig struct {
	PluginDir   string        `json:"plugin_dir"`
	CacheTTL    time.Duration `json:"cache_ttl"`
	Timeout     time.Duration `json:"timeout"`
	MaxRetries  int           `json:"max_retries"`
	RetryDelay  time.Duration `json:"retry_delay"`
	RedisClient *redis.Client
	Logger      *logrus.Logger
}

// NewManager creates a new plugin manager
func NewManager(config *ManagerConfig) *Manager {
	if config.Logger == nil {
		config.Logger = logrus.New()
	}

	manager := &Manager{
		plugins:       make(map[string]ExternalSystemPlugin),
		configs:       make(map[string]map[string]interface{}),
		cache:         config.RedisClient,
		logger:        config.Logger,
		pluginDir:     config.PluginDir,
		cacheTTL:      config.CacheTTL,
		timeout:       config.Timeout,
		maxRetries:    config.MaxRetries,
		retryDelay:    config.RetryDelay,
		healthChecker: NewHealthChecker(config.Logger),
	}

	// Set defaults
	if manager.cacheTTL == 0 {
		manager.cacheTTL = 30 * time.Minute
	}
	if manager.timeout == 0 {
		manager.timeout = 10 * time.Second
	}
	if manager.maxRetries == 0 {
		manager.maxRetries = 3
	}
	if manager.retryDelay == 0 {
		manager.retryDelay = time.Second
	}

	return manager
}

// LoadPlugins discovers and loads all plugins from the plugin directory
func (m *Manager) LoadPlugins() error {
	if m.pluginDir == "" {
		m.logger.Info("No plugin directory specified, skipping plugin loading")
		return nil
	}

	return filepath.WalkDir(m.pluginDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}

		return m.loadPluginConfig(path)
	})
}

// RegisterPlugin registers a plugin with the manager
func (m *Manager) RegisterPlugin(plugin ExternalSystemPlugin, config map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := plugin.Name()

	// Validate plugin metadata
	metadata := plugin.GetMetadata()
	if err := m.validatePluginMetadata(metadata); err != nil {
		return fmt.Errorf("invalid plugin metadata for %s: %w", name, err)
	}

	// Validate configuration
	if err := m.validatePluginConfig(metadata.Config, config); err != nil {
		return fmt.Errorf("invalid configuration for plugin %s: %w", name, err)
	}

	// Connect plugin
	if err := plugin.Connect(config); err != nil {
		return fmt.Errorf("failed to connect plugin %s: %w", name, err)
	}

	m.plugins[name] = plugin
	m.configs[name] = config

	// Start health monitoring
	m.healthChecker.AddPlugin(plugin)

	m.logger.WithFields(logrus.Fields{
		"plugin":  name,
		"version": metadata.Version,
	}).Info("Plugin registered successfully")

	return nil
}

// UnregisterPlugin removes a plugin from the manager
func (m *Manager) UnregisterPlugin(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plugin, exists := m.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	// Cleanup plugin
	if err := plugin.Cleanup(); err != nil {
		m.logger.WithError(err).Warn("Error during plugin cleanup")
	}

	// Remove from health checker
	m.healthChecker.RemovePlugin(name)

	delete(m.plugins, name)
	delete(m.configs, name)

	m.logger.WithField("plugin", name).Info("Plugin unregistered")

	return nil
}

// EnrichUserProfile enriches user profile using all available plugins
func (m *Manager) EnrichUserProfile(userID string) (*CombinedEnrichment, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	// Check cache first
	if cached := m.getCachedEnrichment(userID); cached != nil {
		return cached, nil
	}

	m.mu.RLock()
	plugins := make(map[string]ExternalSystemPlugin)
	for name, plugin := range m.plugins {
		plugins[name] = plugin
	}
	m.mu.RUnlock()

	// Enrich using all plugins concurrently
	enrichments := make(chan *PluginEnrichmentResult, len(plugins))

	for name, plugin := range plugins {
		go m.enrichWithPlugin(ctx, name, plugin, userID, enrichments)
	}

	// Collect results
	results := make([]*PluginEnrichmentResult, 0, len(plugins))
	for i := 0; i < len(plugins); i++ {
		select {
		case result := <-enrichments:
			results = append(results, result)
		case <-ctx.Done():
			m.logger.WithError(ctx.Err()).Warn("Timeout waiting for plugin enrichments")
			break
		}
	}

	// Combine enrichments
	combined := m.combineEnrichments(results)

	// Cache result
	m.cacheEnrichment(userID, combined)

	return combined, nil
}

// EnrichWithPlugin enriches user profile using a specific plugin
func (m *Manager) EnrichWithPlugin(pluginName, userID string) (*UserEnrichment, error) {
	m.mu.RLock()
	plugin, exists := m.plugins[pluginName]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("plugin %s not found", pluginName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	// Check plugin health
	if !plugin.IsHealthy() {
		return nil, &PluginError{
			Plugin:    pluginName,
			Operation: "enrich",
			Message:   "plugin is unhealthy",
			Code:      ErrorCodeConnection,
			Retryable: true,
		}
	}

	// Enrich with retry logic
	return m.enrichWithRetry(ctx, plugin, userID)
}

// GetPluginStatus returns the status of all plugins
func (m *Manager) GetPluginStatus() map[string]*PluginHealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[string]*PluginHealthStatus)

	for name, plugin := range m.plugins {
		metadata := plugin.GetMetadata()
		status[name] = &PluginHealthStatus{
			Name:         name,
			Version:      metadata.Version,
			Healthy:      plugin.IsHealthy(),
			LastCheck:    time.Now(),
			Capabilities: metadata.Capabilities,
		}
	}

	return status
}

// GetPlugin returns a specific plugin by name
func (m *Manager) GetPlugin(name string) (ExternalSystemPlugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plugin, exists := m.plugins[name]
	return plugin, exists
}

// ListPlugins returns a list of all registered plugins
func (m *Manager) ListPlugins() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.plugins))
	for name := range m.plugins {
		names = append(names, name)
	}

	return names
}

// Shutdown gracefully shuts down all plugins
func (m *Manager) Shutdown() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errors []error

	for name, plugin := range m.plugins {
		if err := plugin.Cleanup(); err != nil {
			errors = append(errors, fmt.Errorf("error cleaning up plugin %s: %w", name, err))
		}
	}

	m.healthChecker.Stop()

	if len(errors) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errors)
	}

	return nil
}

// Private methods

func (m *Manager) loadPluginConfig(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read plugin config %s: %w", configPath, err)
	}

	var config struct {
		Plugin string                 `json:"plugin"`
		Config map[string]interface{} `json:"config"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse plugin config %s: %w", configPath, err)
	}

	m.logger.WithFields(logrus.Fields{
		"plugin": config.Plugin,
		"config": configPath,
	}).Info("Loaded plugin configuration")

	return nil
}

func (m *Manager) enrichWithPlugin(ctx context.Context, name string, plugin ExternalSystemPlugin, userID string, results chan<- *PluginEnrichmentResult) {
	result := &PluginEnrichmentResult{
		Plugin:    name,
		Timestamp: time.Now(),
	}

	defer func() {
		results <- result
	}()

	// Check plugin health
	if !plugin.IsHealthy() {
		result.Error = &PluginError{
			Plugin:    name,
			Operation: "enrich",
			Message:   "plugin is unhealthy",
			Code:      ErrorCodeConnection,
			Retryable: true,
		}
		return
	}

	// Enrich with timeout
	enrichment, err := m.enrichWithRetry(ctx, plugin, userID)
	if err != nil {
		result.Error = err
		return
	}

	result.Enrichment = enrichment
	result.Success = true
}

func (m *Manager) enrichWithRetry(ctx context.Context, plugin ExternalSystemPlugin, userID string) (*UserEnrichment, error) {
	var lastErr error

	for attempt := 0; attempt < m.maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		enrichment, err := plugin.EnrichUserProfile(userID)
		if err == nil {
			return enrichment, nil
		}

		lastErr = err

		// Check if error is retryable
		if pluginErr, ok := err.(*PluginError); ok && !pluginErr.Retryable {
			break
		}

		// Wait before retry
		if attempt < m.maxRetries-1 {
			select {
			case <-time.After(m.retryDelay * time.Duration(attempt+1)):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	return nil, lastErr
}

func (m *Manager) combineEnrichments(results []*PluginEnrichmentResult) *CombinedEnrichment {
	combined := &CombinedEnrichment{
		Enrichments: make(map[string]*UserEnrichment),
		Errors:      make(map[string]error),
		Timestamp:   time.Now(),
	}

	var totalConfidence float64
	var successCount int

	for _, result := range results {
		if result.Success && result.Enrichment != nil {
			combined.Enrichments[result.Plugin] = result.Enrichment
			totalConfidence += result.Enrichment.Confidence
			successCount++
		} else if result.Error != nil {
			combined.Errors[result.Plugin] = result.Error
		}
	}

	// Calculate overall confidence
	if successCount > 0 {
		combined.OverallConfidence = totalConfidence / float64(successCount)
	}

	combined.SuccessCount = successCount
	combined.ErrorCount = len(combined.Errors)

	return combined
}

func (m *Manager) getCachedEnrichment(userID string) *CombinedEnrichment {
	if m.cache == nil {
		return nil
	}

	key := fmt.Sprintf("plugin_enrichment:%s", userID)
	data, err := m.cache.Get(context.Background(), key).Result()
	if err != nil {
		return nil
	}

	var enrichment CombinedEnrichment
	if err := json.Unmarshal([]byte(data), &enrichment); err != nil {
		m.logger.WithError(err).Warn("Failed to unmarshal cached enrichment")
		return nil
	}

	return &enrichment
}

func (m *Manager) cacheEnrichment(userID string, enrichment *CombinedEnrichment) {
	if m.cache == nil {
		return
	}

	key := fmt.Sprintf("plugin_enrichment:%s", userID)
	data, err := json.Marshal(enrichment)
	if err != nil {
		m.logger.WithError(err).Warn("Failed to marshal enrichment for caching")
		return
	}

	if err := m.cache.Set(context.Background(), key, data, m.cacheTTL).Err(); err != nil {
		m.logger.WithError(err).Warn("Failed to cache enrichment")
	}
}

func (m *Manager) validatePluginMetadata(metadata *PluginMetadata) error {
	if metadata == nil {
		return fmt.Errorf("metadata is required")
	}

	if metadata.Name == "" {
		return fmt.Errorf("plugin name is required")
	}

	if metadata.Version == "" {
		return fmt.Errorf("plugin version is required")
	}

	return nil
}

func (m *Manager) validatePluginConfig(schema *ConfigSchema, config map[string]interface{}) error {
	if schema == nil {
		return nil // No validation required
	}

	// Check required properties
	for _, required := range schema.Required {
		if _, exists := config[required]; !exists {
			return fmt.Errorf("required property %s is missing", required)
		}
	}

	// Validate property types and constraints
	for key, value := range config {
		property, exists := schema.Properties[key]
		if !exists {
			continue // Unknown properties are allowed
		}

		if err := m.validatePropertyValue(property, value); err != nil {
			return fmt.Errorf("invalid value for property %s: %w", key, err)
		}
	}

	return nil
}

func (m *Manager) validatePropertyValue(property *ConfigProperty, value interface{}) error {
	// Type validation would go here
	// This is a simplified version
	return nil
}

// Supporting types

type PluginEnrichmentResult struct {
	Plugin     string          `json:"plugin"`
	Success    bool            `json:"success"`
	Enrichment *UserEnrichment `json:"enrichment,omitempty"`
	Error      error           `json:"error,omitempty"`
	Timestamp  time.Time       `json:"timestamp"`
}

type CombinedEnrichment struct {
	Enrichments       map[string]*UserEnrichment `json:"enrichments"`
	Errors            map[string]error           `json:"errors"`
	OverallConfidence float64                    `json:"overall_confidence"`
	SuccessCount      int                        `json:"success_count"`
	ErrorCount        int                        `json:"error_count"`
	Timestamp         time.Time                  `json:"timestamp"`
}

type PluginHealthStatus struct {
	Name         string    `json:"name"`
	Version      string    `json:"version"`
	Healthy      bool      `json:"healthy"`
	LastCheck    time.Time `json:"last_check"`
	Capabilities []string  `json:"capabilities"`
}
