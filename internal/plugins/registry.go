package plugins

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Registry manages plugin discovery, registration, and lifecycle
type Registry struct {
	plugins       map[string]*PluginEntry
	pluginDir     string
	logger        *logrus.Logger
	mu            sync.RWMutex
	autoDiscovery bool
	watchInterval time.Duration
	stopWatch     chan struct{}
}

// PluginEntry represents a registered plugin with metadata
type PluginEntry struct {
	Plugin       ExternalSystemPlugin   `json:"-"`
	Metadata     *PluginMetadata        `json:"metadata"`
	Config       map[string]interface{} `json:"config"`
	Status       PluginStatus           `json:"status"`
	InstallTime  time.Time              `json:"install_time"`
	LastUsed     time.Time              `json:"last_used"`
	UsageCount   int64                  `json:"usage_count"`
	Rating       float64                `json:"rating"`
	Reviews      []PluginReview         `json:"reviews"`
	Dependencies []string               `json:"dependencies"`
	Enabled      bool                   `json:"enabled"`
	ConfigPath   string                 `json:"config_path"`
}

// PluginStatus represents the current status of a plugin
type PluginStatus string

const (
	StatusInstalled   PluginStatus = "installed"
	StatusEnabled     PluginStatus = "enabled"
	StatusDisabled    PluginStatus = "disabled"
	StatusError       PluginStatus = "error"
	StatusUpdating    PluginStatus = "updating"
	StatusUninstalled PluginStatus = "uninstalled"
)

// PluginReview represents a user review of a plugin
type PluginReview struct {
	UserID   string    `json:"user_id"`
	Rating   int       `json:"rating"` // 1-5 stars
	Comment  string    `json:"comment"`
	Date     time.Time `json:"date"`
	Version  string    `json:"version"`
	Helpful  int       `json:"helpful"`  // helpful votes
	Verified bool      `json:"verified"` // verified purchase/usage
}

// PluginManifest represents plugin manifest file
type PluginManifest struct {
	Name         string        `json:"name"`
	Version      string        `json:"version"`
	Description  string        `json:"description"`
	Author       string        `json:"author"`
	Website      string        `json:"website,omitempty"`
	License      string        `json:"license,omitempty"`
	Tags         []string      `json:"tags,omitempty"`
	Dependencies []string      `json:"dependencies,omitempty"`
	Config       *ConfigSchema `json:"config_schema,omitempty"`
	Capabilities []string      `json:"capabilities"`
	MinVersion   string        `json:"min_engine_version,omitempty"`
	MaxVersion   string        `json:"max_engine_version,omitempty"`
	Checksum     string        `json:"checksum,omitempty"`
}

// RegistryConfig contains configuration for the plugin registry
type RegistryConfig struct {
	PluginDir     string        `json:"plugin_dir"`
	AutoDiscovery bool          `json:"auto_discovery"`
	WatchInterval time.Duration `json:"watch_interval"`
	Logger        *logrus.Logger
}

// SearchFilter represents search criteria for plugins
type SearchFilter struct {
	Name         string         `json:"name,omitempty"`
	Tags         []string       `json:"tags,omitempty"`
	Author       string         `json:"author,omitempty"`
	Capabilities []string       `json:"capabilities,omitempty"`
	MinRating    float64        `json:"min_rating,omitempty"`
	Status       []PluginStatus `json:"status,omitempty"`
	SortBy       string         `json:"sort_by,omitempty"`    // name, rating, usage, date
	SortOrder    string         `json:"sort_order,omitempty"` // asc, desc
	Limit        int            `json:"limit,omitempty"`
	Offset       int            `json:"offset,omitempty"`
}

// SearchResult represents search results
type SearchResult struct {
	Plugins []*PluginEntry `json:"plugins"`
	Total   int            `json:"total"`
	Offset  int            `json:"offset"`
	Limit   int            `json:"limit"`
	Query   *SearchFilter  `json:"query"`
}

// NewRegistry creates a new plugin registry
func NewRegistry(config *RegistryConfig) *Registry {
	if config.Logger == nil {
		config.Logger = logrus.New()
	}

	if config.WatchInterval == 0 {
		config.WatchInterval = 30 * time.Second
	}

	registry := &Registry{
		plugins:       make(map[string]*PluginEntry),
		pluginDir:     config.PluginDir,
		logger:        config.Logger,
		autoDiscovery: config.AutoDiscovery,
		watchInterval: config.WatchInterval,
		stopWatch:     make(chan struct{}),
	}

	return registry
}

// Start starts the plugin registry
func (r *Registry) Start() error {
	r.logger.Info("Starting plugin registry")

	// Discover existing plugins
	if err := r.DiscoverPlugins(); err != nil {
		return fmt.Errorf("failed to discover plugins: %w", err)
	}

	// Start auto-discovery if enabled
	if r.autoDiscovery {
		go r.watchPluginDirectory()
	}

	return nil
}

// Stop stops the plugin registry
func (r *Registry) Stop() error {
	r.logger.Info("Stopping plugin registry")

	if r.autoDiscovery {
		close(r.stopWatch)
	}

	return nil
}

// DiscoverPlugins discovers plugins in the plugin directory
func (r *Registry) DiscoverPlugins() error {
	if r.pluginDir == "" {
		r.logger.Info("No plugin directory specified, skipping discovery")
		return nil
	}

	if _, err := os.Stat(r.pluginDir); os.IsNotExist(err) {
		r.logger.WithField("dir", r.pluginDir).Info("Plugin directory does not exist, creating it")
		if err := os.MkdirAll(r.pluginDir, 0755); err != nil {
			return fmt.Errorf("failed to create plugin directory: %w", err)
		}
		return nil
	}

	return filepath.WalkDir(r.pluginDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		// Look for plugin manifest files
		if strings.HasSuffix(path, "plugin.json") {
			return r.loadPluginFromManifest(path)
		}

		return nil
	})
}

// RegisterPlugin registers a plugin with the registry
func (r *Registry) RegisterPlugin(plugin ExternalSystemPlugin, config map[string]interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := plugin.Name()
	metadata := plugin.GetMetadata()

	// Validate plugin
	if err := r.validatePlugin(plugin, metadata); err != nil {
		return fmt.Errorf("plugin validation failed: %w", err)
	}

	// Create plugin entry
	entry := &PluginEntry{
		Plugin:      plugin,
		Metadata:    metadata,
		Config:      config,
		Status:      StatusInstalled,
		InstallTime: time.Now(),
		Enabled:     true,
		Reviews:     make([]PluginReview, 0),
	}

	r.plugins[name] = entry

	r.logger.WithFields(logrus.Fields{
		"plugin":  name,
		"version": metadata.Version,
		"author":  metadata.Author,
	}).Info("Plugin registered successfully")

	return nil
}

// UnregisterPlugin unregisters a plugin from the registry
func (r *Registry) UnregisterPlugin(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, exists := r.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	// Cleanup plugin
	if entry.Plugin != nil {
		if err := entry.Plugin.Cleanup(); err != nil {
			r.logger.WithError(err).Warn("Error during plugin cleanup")
		}
	}

	// Update status
	entry.Status = StatusUninstalled
	entry.Enabled = false

	delete(r.plugins, name)

	r.logger.WithField("plugin", name).Info("Plugin unregistered")

	return nil
}

// GetPlugin retrieves a plugin by name
func (r *Registry) GetPlugin(name string) (*PluginEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.plugins[name]
	return entry, exists
}

// ListPlugins returns a list of all registered plugins
func (r *Registry) ListPlugins() []*PluginEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugins := make([]*PluginEntry, 0, len(r.plugins))
	for _, entry := range r.plugins {
		plugins = append(plugins, entry)
	}

	return plugins
}

// SearchPlugins searches for plugins based on criteria
func (r *Registry) SearchPlugins(filter *SearchFilter) *SearchResult {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matches []*PluginEntry

	// Filter plugins
	for _, entry := range r.plugins {
		if r.matchesFilter(entry, filter) {
			matches = append(matches, entry)
		}
	}

	// Sort results
	r.sortPlugins(matches, filter.SortBy, filter.SortOrder)

	// Apply pagination
	total := len(matches)
	start := filter.Offset
	end := start + filter.Limit

	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	if filter.Limit == 0 {
		end = total
	}

	if start < end {
		matches = matches[start:end]
	} else {
		matches = []*PluginEntry{}
	}

	return &SearchResult{
		Plugins: matches,
		Total:   total,
		Offset:  filter.Offset,
		Limit:   filter.Limit,
		Query:   filter,
	}
}

// EnablePlugin enables a plugin
func (r *Registry) EnablePlugin(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, exists := r.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	entry.Enabled = true
	entry.Status = StatusEnabled

	r.logger.WithField("plugin", name).Info("Plugin enabled")

	return nil
}

// DisablePlugin disables a plugin
func (r *Registry) DisablePlugin(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, exists := r.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	entry.Enabled = false
	entry.Status = StatusDisabled

	r.logger.WithField("plugin", name).Info("Plugin disabled")

	return nil
}

// UpdatePluginUsage updates plugin usage statistics
func (r *Registry) UpdatePluginUsage(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if entry, exists := r.plugins[name]; exists {
		entry.UsageCount++
		entry.LastUsed = time.Now()
	}
}

// AddPluginReview adds a review for a plugin
func (r *Registry) AddPluginReview(name string, review PluginReview) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, exists := r.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	review.Date = time.Now()
	entry.Reviews = append(entry.Reviews, review)

	// Recalculate average rating
	r.calculateAverageRating(entry)

	r.logger.WithFields(logrus.Fields{
		"plugin": name,
		"rating": review.Rating,
		"user":   review.UserID,
	}).Info("Plugin review added")

	return nil
}

// GetPluginStats returns statistics about plugins
func (r *Registry) GetPluginStats() *PluginStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := &PluginStats{
		TotalPlugins:    len(r.plugins),
		EnabledPlugins:  0,
		DisabledPlugins: 0,
		StatusCounts:    make(map[PluginStatus]int),
		TagCounts:       make(map[string]int),
		AuthorCounts:    make(map[string]int),
	}

	var totalUsage int64
	var totalRating float64
	var ratedPlugins int

	for _, entry := range r.plugins {
		// Status counts
		stats.StatusCounts[entry.Status]++

		if entry.Enabled {
			stats.EnabledPlugins++
		} else {
			stats.DisabledPlugins++
		}

		// Usage stats
		totalUsage += entry.UsageCount

		// Rating stats
		if entry.Rating > 0 {
			totalRating += entry.Rating
			ratedPlugins++
		}

		// Tag counts
		for _, tag := range entry.Metadata.Tags {
			stats.TagCounts[tag]++
		}

		// Author counts
		stats.AuthorCounts[entry.Metadata.Author]++
	}

	stats.TotalUsage = totalUsage
	if ratedPlugins > 0 {
		stats.AverageRating = totalRating / float64(ratedPlugins)
	}

	return stats
}

// Private methods

func (r *Registry) loadPluginFromManifest(manifestPath string) error {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest %s: %w", manifestPath, err)
	}

	var manifest PluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("failed to parse manifest %s: %w", manifestPath, err)
	}

	// Create plugin entry from manifest
	entry := &PluginEntry{
		Metadata: &PluginMetadata{
			Name:         manifest.Name,
			Version:      manifest.Version,
			Description:  manifest.Description,
			Author:       manifest.Author,
			Website:      manifest.Website,
			License:      manifest.License,
			Capabilities: manifest.Capabilities,
			Dependencies: manifest.Dependencies,
			Config:       manifest.Config,
			Tags:         manifest.Tags,
		},
		Status:      StatusInstalled,
		InstallTime: time.Now(),
		Enabled:     false, // Disabled by default until connected
		Reviews:     make([]PluginReview, 0),
		ConfigPath:  manifestPath,
	}

	r.mu.Lock()
	r.plugins[manifest.Name] = entry
	r.mu.Unlock()

	r.logger.WithFields(logrus.Fields{
		"plugin":   manifest.Name,
		"version":  manifest.Version,
		"manifest": manifestPath,
	}).Info("Plugin discovered from manifest")

	return nil
}

func (r *Registry) validatePlugin(plugin ExternalSystemPlugin, metadata *PluginMetadata) error {
	if metadata == nil {
		return fmt.Errorf("plugin metadata is required")
	}

	if metadata.Name == "" {
		return fmt.Errorf("plugin name is required")
	}

	if metadata.Version == "" {
		return fmt.Errorf("plugin version is required")
	}

	// Check for name conflicts
	if existing, exists := r.plugins[metadata.Name]; exists {
		if existing.Metadata.Version == metadata.Version {
			return fmt.Errorf("plugin %s version %s already registered", metadata.Name, metadata.Version)
		}
	}

	return nil
}

func (r *Registry) matchesFilter(entry *PluginEntry, filter *SearchFilter) bool {
	if filter == nil {
		return true
	}

	// Name filter
	if filter.Name != "" && !strings.Contains(strings.ToLower(entry.Metadata.Name), strings.ToLower(filter.Name)) {
		return false
	}

	// Author filter
	if filter.Author != "" && !strings.Contains(strings.ToLower(entry.Metadata.Author), strings.ToLower(filter.Author)) {
		return false
	}

	// Rating filter
	if filter.MinRating > 0 && entry.Rating < filter.MinRating {
		return false
	}

	// Status filter
	if len(filter.Status) > 0 {
		found := false
		for _, status := range filter.Status {
			if entry.Status == status {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Tags filter
	if len(filter.Tags) > 0 {
		for _, filterTag := range filter.Tags {
			found := false
			for _, entryTag := range entry.Metadata.Tags {
				if strings.EqualFold(entryTag, filterTag) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	// Capabilities filter
	if len(filter.Capabilities) > 0 {
		for _, filterCap := range filter.Capabilities {
			found := false
			for _, entryCap := range entry.Metadata.Capabilities {
				if strings.EqualFold(entryCap, filterCap) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	return true
}

func (r *Registry) sortPlugins(plugins []*PluginEntry, sortBy, sortOrder string) {
	if sortBy == "" {
		sortBy = "name"
	}
	if sortOrder == "" {
		sortOrder = "asc"
	}

	sort.Slice(plugins, func(i, j int) bool {
		var less bool

		switch sortBy {
		case "name":
			less = plugins[i].Metadata.Name < plugins[j].Metadata.Name
		case "rating":
			less = plugins[i].Rating < plugins[j].Rating
		case "usage":
			less = plugins[i].UsageCount < plugins[j].UsageCount
		case "date":
			less = plugins[i].InstallTime.Before(plugins[j].InstallTime)
		default:
			less = plugins[i].Metadata.Name < plugins[j].Metadata.Name
		}

		if sortOrder == "desc" {
			return !less
		}
		return less
	})
}

func (r *Registry) calculateAverageRating(entry *PluginEntry) {
	if len(entry.Reviews) == 0 {
		entry.Rating = 0
		return
	}

	var total float64
	for _, review := range entry.Reviews {
		total += float64(review.Rating)
	}

	entry.Rating = total / float64(len(entry.Reviews))
}

func (r *Registry) watchPluginDirectory() {
	ticker := time.NewTicker(r.watchInterval)
	defer ticker.Stop()

	r.logger.Info("Started plugin directory watcher")

	for {
		select {
		case <-ticker.C:
			if err := r.DiscoverPlugins(); err != nil {
				r.logger.WithError(err).Warn("Error during plugin discovery")
			}
		case <-r.stopWatch:
			r.logger.Info("Stopped plugin directory watcher")
			return
		}
	}
}

// Supporting types

type PluginStats struct {
	TotalPlugins    int                  `json:"total_plugins"`
	EnabledPlugins  int                  `json:"enabled_plugins"`
	DisabledPlugins int                  `json:"disabled_plugins"`
	TotalUsage      int64                `json:"total_usage"`
	AverageRating   float64              `json:"average_rating"`
	StatusCounts    map[PluginStatus]int `json:"status_counts"`
	TagCounts       map[string]int       `json:"tag_counts"`
	AuthorCounts    map[string]int       `json:"author_counts"`
}
