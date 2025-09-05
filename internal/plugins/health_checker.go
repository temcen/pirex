package plugins

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// HealthChecker monitors plugin health and provides health status
type HealthChecker struct {
	plugins       map[string]ExternalSystemPlugin
	healthStatus  map[string]*HealthStatus
	checkInterval time.Duration
	logger        *logrus.Logger
	mu            sync.RWMutex
	stopChan      chan struct{}
	running       bool
}

// HealthStatus represents the health status of a plugin
type HealthStatus struct {
	Plugin       string        `json:"plugin"`
	Healthy      bool          `json:"healthy"`
	LastCheck    time.Time     `json:"last_check"`
	LastSuccess  time.Time     `json:"last_success"`
	LastFailure  time.Time     `json:"last_failure"`
	FailureCount int           `json:"failure_count"`
	SuccessCount int           `json:"success_count"`
	ResponseTime time.Duration `json:"response_time"`
	ErrorMessage string        `json:"error_message,omitempty"`
	Uptime       time.Duration `json:"uptime"`
	StartTime    time.Time     `json:"start_time"`
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(logger *logrus.Logger) *HealthChecker {
	if logger == nil {
		logger = logrus.New()
	}

	return &HealthChecker{
		plugins:       make(map[string]ExternalSystemPlugin),
		healthStatus:  make(map[string]*HealthStatus),
		checkInterval: 30 * time.Second,
		logger:        logger,
		stopChan:      make(chan struct{}),
	}
}

// AddPlugin adds a plugin to health monitoring
func (hc *HealthChecker) AddPlugin(plugin ExternalSystemPlugin) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	name := plugin.Name()
	hc.plugins[name] = plugin
	hc.healthStatus[name] = &HealthStatus{
		Plugin:    name,
		StartTime: time.Now(),
		Healthy:   true, // Assume healthy initially
	}

	// Start monitoring if not already running
	if !hc.running {
		go hc.startMonitoring()
		hc.running = true
	}

	hc.logger.WithField("plugin", name).Info("Added plugin to health monitoring")
}

// RemovePlugin removes a plugin from health monitoring
func (hc *HealthChecker) RemovePlugin(name string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	delete(hc.plugins, name)
	delete(hc.healthStatus, name)

	hc.logger.WithField("plugin", name).Info("Removed plugin from health monitoring")
}

// GetHealthStatus returns the health status of a specific plugin
func (hc *HealthChecker) GetHealthStatus(name string) (*HealthStatus, bool) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	status, exists := hc.healthStatus[name]
	if !exists {
		return nil, false
	}

	// Calculate uptime
	statusCopy := *status
	statusCopy.Uptime = time.Since(status.StartTime)

	return &statusCopy, true
}

// GetAllHealthStatus returns the health status of all plugins
func (hc *HealthChecker) GetAllHealthStatus() map[string]*HealthStatus {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	result := make(map[string]*HealthStatus)

	for name, status := range hc.healthStatus {
		statusCopy := *status
		statusCopy.Uptime = time.Since(status.StartTime)
		result[name] = &statusCopy
	}

	return result
}

// IsPluginHealthy checks if a specific plugin is healthy
func (hc *HealthChecker) IsPluginHealthy(name string) bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	status, exists := hc.healthStatus[name]
	if !exists {
		return false
	}

	return status.Healthy
}

// GetUnhealthyPlugins returns a list of unhealthy plugins
func (hc *HealthChecker) GetUnhealthyPlugins() []string {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	var unhealthy []string

	for name, status := range hc.healthStatus {
		if !status.Healthy {
			unhealthy = append(unhealthy, name)
		}
	}

	return unhealthy
}

// SetCheckInterval sets the health check interval
func (hc *HealthChecker) SetCheckInterval(interval time.Duration) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	hc.checkInterval = interval
}

// Stop stops the health monitoring
func (hc *HealthChecker) Stop() {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if hc.running {
		close(hc.stopChan)
		hc.running = false
	}
}

// ForceCheck forces a health check for all plugins
func (hc *HealthChecker) ForceCheck() {
	hc.mu.RLock()
	plugins := make(map[string]ExternalSystemPlugin)
	for name, plugin := range hc.plugins {
		plugins[name] = plugin
	}
	hc.mu.RUnlock()

	for name, plugin := range plugins {
		hc.checkPluginHealth(name, plugin)
	}
}

// Private methods

func (hc *HealthChecker) startMonitoring() {
	ticker := time.NewTicker(hc.checkInterval)
	defer ticker.Stop()

	hc.logger.Info("Started plugin health monitoring")

	for {
		select {
		case <-ticker.C:
			hc.performHealthChecks()
		case <-hc.stopChan:
			hc.logger.Info("Stopped plugin health monitoring")
			return
		}
	}
}

func (hc *HealthChecker) performHealthChecks() {
	hc.mu.RLock()
	plugins := make(map[string]ExternalSystemPlugin)
	for name, plugin := range hc.plugins {
		plugins[name] = plugin
	}
	hc.mu.RUnlock()

	// Check each plugin concurrently
	var wg sync.WaitGroup

	for name, plugin := range plugins {
		wg.Add(1)
		go func(name string, plugin ExternalSystemPlugin) {
			defer wg.Done()
			hc.checkPluginHealth(name, plugin)
		}(name, plugin)
	}

	wg.Wait()
}

func (hc *HealthChecker) checkPluginHealth(name string, plugin ExternalSystemPlugin) {
	start := time.Now()

	// Perform health check
	healthy := plugin.IsHealthy()
	responseTime := time.Since(start)

	hc.mu.Lock()
	defer hc.mu.Unlock()

	status, exists := hc.healthStatus[name]
	if !exists {
		return // Plugin was removed
	}

	// Update status
	status.LastCheck = time.Now()
	status.ResponseTime = responseTime

	if healthy {
		status.Healthy = true
		status.LastSuccess = time.Now()
		status.SuccessCount++
		status.ErrorMessage = ""

		// Log recovery if previously unhealthy
		if status.FailureCount > 0 {
			hc.logger.WithFields(logrus.Fields{
				"plugin":        name,
				"failure_count": status.FailureCount,
			}).Info("Plugin recovered")
		}
	} else {
		wasHealthy := status.Healthy
		status.Healthy = false
		status.LastFailure = time.Now()
		status.FailureCount++

		// Log failure
		if wasHealthy {
			hc.logger.WithField("plugin", name).Warn("Plugin became unhealthy")
		}

		// Try to get error details
		if err := hc.getPluginError(plugin); err != nil {
			status.ErrorMessage = err.Error()
		}
	}

	// Log periodic status for debugging
	if status.SuccessCount%100 == 0 || status.FailureCount > 0 {
		hc.logger.WithFields(logrus.Fields{
			"plugin":        name,
			"healthy":       status.Healthy,
			"success_count": status.SuccessCount,
			"failure_count": status.FailureCount,
			"response_time": responseTime,
		}).Debug("Plugin health check completed")
	}
}

func (hc *HealthChecker) getPluginError(plugin ExternalSystemPlugin) error {
	// Try to get more detailed error information
	// This could be enhanced to call a GetLastError() method if plugins implement it
	return nil
}

// HealthSummary provides an overall health summary
type HealthSummary struct {
	TotalPlugins     int                      `json:"total_plugins"`
	HealthyPlugins   int                      `json:"healthy_plugins"`
	UnhealthyPlugins int                      `json:"unhealthy_plugins"`
	OverallHealthy   bool                     `json:"overall_healthy"`
	LastCheck        time.Time                `json:"last_check"`
	PluginDetails    map[string]*HealthStatus `json:"plugin_details"`
}

// GetHealthSummary returns an overall health summary
func (hc *HealthChecker) GetHealthSummary() *HealthSummary {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	summary := &HealthSummary{
		TotalPlugins:  len(hc.healthStatus),
		LastCheck:     time.Now(),
		PluginDetails: make(map[string]*HealthStatus),
	}

	for name, status := range hc.healthStatus {
		statusCopy := *status
		statusCopy.Uptime = time.Since(status.StartTime)
		summary.PluginDetails[name] = &statusCopy

		if status.Healthy {
			summary.HealthyPlugins++
		} else {
			summary.UnhealthyPlugins++
		}
	}

	// Overall healthy if all plugins are healthy and there are plugins
	summary.OverallHealthy = summary.UnhealthyPlugins == 0 && summary.TotalPlugins > 0

	return summary
}
