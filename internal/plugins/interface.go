package plugins

import (
	"context"
	"time"
)

// ExternalSystemPlugin defines the interface that all external system plugins must implement
type ExternalSystemPlugin interface {
	// Name returns the unique name of the plugin
	Name() string

	// Connect establishes connection to the external system with provided configuration
	Connect(config map[string]interface{}) error

	// EnrichUserProfile fetches and returns user enrichment data from the external system
	EnrichUserProfile(userID string) (*UserEnrichment, error)

	// IsHealthy checks if the plugin and its external system connection are healthy
	IsHealthy() bool

	// Cleanup performs any necessary cleanup when the plugin is being shut down
	Cleanup() error

	// GetMetadata returns plugin metadata including version, description, and capabilities
	GetMetadata() *PluginMetadata
}

// UserEnrichment represents enriched user data from external systems
type UserEnrichment struct {
	// Demographics contains user demographic information
	Demographics *Demographics `json:"demographics,omitempty"`

	// Interests contains user interests and preferences
	Interests []string `json:"interests,omitempty"`

	// SocialConnections contains social network information
	SocialConnections *SocialConnections `json:"social_connections,omitempty"`

	// BehaviorPatterns contains user behavior analysis
	BehaviorPatterns *BehaviorPatterns `json:"behavior_patterns,omitempty"`

	// ContextualData contains contextual information (location, weather, etc.)
	ContextualData map[string]interface{} `json:"contextual_data,omitempty"`

	// Confidence represents the confidence level of the enrichment data (0.0 to 1.0)
	Confidence float64 `json:"confidence"`

	// Source identifies which plugin provided this enrichment
	Source string `json:"source"`

	// Timestamp when the data was collected
	Timestamp time.Time `json:"timestamp"`

	// TTL indicates how long this data should be considered valid
	TTL time.Duration `json:"ttl"`
}

// Demographics represents user demographic information
type Demographics struct {
	Age           *int   `json:"age,omitempty"`
	Gender        string `json:"gender,omitempty"`
	Location      string `json:"location,omitempty"`
	Country       string `json:"country,omitempty"`
	City          string `json:"city,omitempty"`
	Income        *int   `json:"income,omitempty"`
	Education     string `json:"education,omitempty"`
	Occupation    string `json:"occupation,omitempty"`
	MaritalStatus string `json:"marital_status,omitempty"`
}

// SocialConnections represents social network information
type SocialConnections struct {
	Platforms    []string               `json:"platforms,omitempty"`
	Connections  int                    `json:"connections,omitempty"`
	Influence    float64                `json:"influence,omitempty"`
	Communities  []string               `json:"communities,omitempty"`
	Interactions map[string]interface{} `json:"interactions,omitempty"`
}

// BehaviorPatterns represents user behavior analysis
type BehaviorPatterns struct {
	SessionDuration   time.Duration          `json:"session_duration,omitempty"`
	PageViews         int                    `json:"page_views,omitempty"`
	ConversionRate    float64                `json:"conversion_rate,omitempty"`
	PreferredChannels []string               `json:"preferred_channels,omitempty"`
	ActivityTimes     []string               `json:"activity_times,omitempty"`
	PurchaseHistory   []PurchaseEvent        `json:"purchase_history,omitempty"`
	SearchPatterns    []string               `json:"search_patterns,omitempty"`
	EngagementMetrics map[string]interface{} `json:"engagement_metrics,omitempty"`
}

// PurchaseEvent represents a purchase event
type PurchaseEvent struct {
	ProductID string    `json:"product_id"`
	Category  string    `json:"category"`
	Amount    float64   `json:"amount"`
	Currency  string    `json:"currency"`
	Timestamp time.Time `json:"timestamp"`
	Channel   string    `json:"channel"`
}

// PluginMetadata contains plugin information
type PluginMetadata struct {
	Name         string        `json:"name"`
	Version      string        `json:"version"`
	Description  string        `json:"description"`
	Author       string        `json:"author"`
	Website      string        `json:"website,omitempty"`
	License      string        `json:"license,omitempty"`
	Capabilities []string      `json:"capabilities"`
	Dependencies []string      `json:"dependencies,omitempty"`
	Config       *ConfigSchema `json:"config_schema"`
	Tags         []string      `json:"tags,omitempty"`
}

// ConfigSchema defines the configuration schema for a plugin
type ConfigSchema struct {
	Properties map[string]*ConfigProperty `json:"properties"`
	Required   []string                   `json:"required"`
}

// ConfigProperty defines a configuration property
type ConfigProperty struct {
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Default     interface{} `json:"default,omitempty"`
	Enum        []string    `json:"enum,omitempty"`
	Pattern     string      `json:"pattern,omitempty"`
	Minimum     *float64    `json:"minimum,omitempty"`
	Maximum     *float64    `json:"maximum,omitempty"`
	Required    bool        `json:"required"`
	Sensitive   bool        `json:"sensitive"` // For passwords, API keys, etc.
}

// PluginContext provides context for plugin operations
type PluginContext struct {
	Context   context.Context
	UserID    string
	RequestID string
	Timeout   time.Duration
	Logger    Logger
}

// Logger interface for plugin logging
type Logger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
}

// PluginError represents plugin-specific errors
type PluginError struct {
	Plugin    string
	Operation string
	Message   string
	Cause     error
	Code      string
	Retryable bool
}

func (e *PluginError) Error() string {
	if e.Cause != nil {
		return e.Plugin + ": " + e.Operation + ": " + e.Message + ": " + e.Cause.Error()
	}
	return e.Plugin + ": " + e.Operation + ": " + e.Message
}

// Common error codes
const (
	ErrorCodeConnection     = "CONNECTION_ERROR"
	ErrorCodeAuthentication = "AUTH_ERROR"
	ErrorCodeRateLimit      = "RATE_LIMIT_ERROR"
	ErrorCodeNotFound       = "NOT_FOUND"
	ErrorCodeInvalidConfig  = "INVALID_CONFIG"
	ErrorCodeTimeout        = "TIMEOUT_ERROR"
	ErrorCodeQuotaExceeded  = "QUOTA_EXCEEDED"
)
