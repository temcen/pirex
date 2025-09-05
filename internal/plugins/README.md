# Plugin Development Framework

The Plugin Development Framework provides a comprehensive system for integrating external data sources into the recommendation engine. This framework allows developers to create plugins that enrich user profiles with data from CRM systems, social media platforms, analytics services, and other external APIs.

## Overview

The plugin framework consists of several key components:

- **Plugin Interface**: Standardized interface that all plugins must implement
- **Plugin Manager**: Manages plugin lifecycle, configuration, and orchestration
- **Plugin Registry**: Handles plugin discovery, registration, and metadata management
- **Health Checker**: Monitors plugin health and performance
- **Testing Framework**: Comprehensive testing utilities for plugin development
- **Configuration System**: Flexible configuration management with validation

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Plugin Framework                         │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │   Plugin    │  │   Plugin    │  │    Health Checker   │  │
│  │   Manager   │  │  Registry   │  │                     │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
│                                                             │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │              Plugin Interface                           │  │
│  └─────────────────────────────────────────────────────────┘  │
│                                                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │ CRM Plugin  │  │Social Media │  │  Custom Plugins     │  │
│  │             │  │   Plugin    │  │                     │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
│                                                             │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                External Systems                             │
├─────────────────────────────────────────────────────────────┤
│  Salesforce  │  HubSpot  │  Facebook  │  Twitter  │  etc.   │
└─────────────────────────────────────────────────────────────┘
```

## Quick Start

### 1. Create a New Plugin

```go
package plugins

import (
    "fmt"
    "time"
    "github.com/sirupsen/logrus"
)

type MyPlugin struct {
    name      string
    version   string
    apiURL    string
    apiKey    string
    client    *http.Client
    logger    *logrus.Logger
    connected bool
}

func NewMyPlugin(logger *logrus.Logger) *MyPlugin {
    return &MyPlugin{
        name:    "my-plugin",
        version: "1.0.0",
        client:  &http.Client{Timeout: 30 * time.Second},
        logger:  logger,
    }
}
```

### 2. Implement the Plugin Interface

```go
func (p *MyPlugin) Name() string {
    return p.name
}

func (p *MyPlugin) Connect(config map[string]interface{}) error {
    // Parse and validate configuration
    // Test connection to external system
    // Set p.connected = true on success
    return nil
}

func (p *MyPlugin) EnrichUserProfile(userID string) (*UserEnrichment, error) {
    // Fetch data from external system
    // Transform data to UserEnrichment format
    // Return enriched profile
    return enrichment, nil
}

func (p *MyPlugin) IsHealthy() bool {
    // Check plugin health
    return p.connected
}

func (p *MyPlugin) Cleanup() error {
    // Cleanup resources
    return nil
}

func (p *MyPlugin) GetMetadata() *PluginMetadata {
    // Return plugin metadata
    return &PluginMetadata{...}
}
```

### 3. Register Your Plugin

```go
func main() {
    // Create plugin manager
    manager := NewManager(&ManagerConfig{
        PluginDir:   "./plugins",
        CacheTTL:    30 * time.Minute,
        Timeout:     10 * time.Second,
        MaxRetries:  3,
        RetryDelay:  time.Second,
    })

    // Create and register plugin
    plugin := NewMyPlugin(nil)
    config := map[string]interface{}{
        "api_url": "https://api.example.com",
        "api_key": "your-api-key",
    }
    
    err := manager.RegisterPlugin(plugin, config)
    if err != nil {
        log.Fatal(err)
    }
}
```

## Core Components

### Plugin Interface

The `ExternalSystemPlugin` interface defines the contract that all plugins must implement:

```go
type ExternalSystemPlugin interface {
    Name() string
    Connect(config map[string]interface{}) error
    EnrichUserProfile(userID string) (*UserEnrichment, error)
    IsHealthy() bool
    Cleanup() error
    GetMetadata() *PluginMetadata
}
```

#### Method Descriptions

- **Name()**: Returns the unique name of the plugin
- **Connect()**: Establishes connection to the external system using provided configuration
- **EnrichUserProfile()**: Fetches and returns user enrichment data for a given user ID
- **IsHealthy()**: Performs a health check and returns the plugin's health status
- **Cleanup()**: Performs cleanup when the plugin is being shut down
- **GetMetadata()**: Returns plugin metadata including version, capabilities, and configuration schema

### Data Structures

#### UserEnrichment

The primary data structure returned by plugins:

```go
type UserEnrichment struct {
    Demographics      *Demographics           `json:"demographics,omitempty"`
    Interests         []string               `json:"interests,omitempty"`
    SocialConnections *SocialConnections     `json:"social_connections,omitempty"`
    BehaviorPatterns  *BehaviorPatterns      `json:"behavior_patterns,omitempty"`
    ContextualData    map[string]interface{} `json:"contextual_data,omitempty"`
    Confidence        float64                `json:"confidence"`
    Source            string                 `json:"source"`
    Timestamp         time.Time              `json:"timestamp"`
    TTL               time.Duration          `json:"ttl"`
}
```

#### Demographics

User demographic information:

```go
type Demographics struct {
    Age           *int    `json:"age,omitempty"`
    Gender        string  `json:"gender,omitempty"`
    Location      string  `json:"location,omitempty"`
    Country       string  `json:"country,omitempty"`
    City          string  `json:"city,omitempty"`
    Income        *int    `json:"income,omitempty"`
    Education     string  `json:"education,omitempty"`
    Occupation    string  `json:"occupation,omitempty"`
    MaritalStatus string  `json:"marital_status,omitempty"`
}
```

#### SocialConnections

Social network information:

```go
type SocialConnections struct {
    Platforms     []string               `json:"platforms,omitempty"`
    Connections   int                    `json:"connections,omitempty"`
    Influence     float64                `json:"influence,omitempty"`
    Communities   []string               `json:"communities,omitempty"`
    Interactions  map[string]interface{} `json:"interactions,omitempty"`
}
```

#### BehaviorPatterns

User behavior analysis:

```go
type BehaviorPatterns struct {
    SessionDuration    time.Duration          `json:"session_duration,omitempty"`
    PageViews          int                    `json:"page_views,omitempty"`
    ConversionRate     float64                `json:"conversion_rate,omitempty"`
    PreferredChannels  []string               `json:"preferred_channels,omitempty"`
    ActivityTimes      []string               `json:"activity_times,omitempty"`
    PurchaseHistory    []PurchaseEvent        `json:"purchase_history,omitempty"`
    SearchPatterns     []string               `json:"search_patterns,omitempty"`
    EngagementMetrics  map[string]interface{} `json:"engagement_metrics,omitempty"`
}
```

### Plugin Manager

The Plugin Manager handles plugin lifecycle and orchestration:

```go
type Manager struct {
    plugins       map[string]ExternalSystemPlugin
    configs       map[string]map[string]interface{}
    cache         *redis.Client
    logger        *logrus.Logger
    // ... other fields
}
```

#### Key Methods

- **LoadPlugins()**: Discovers and loads plugins from the plugin directory
- **RegisterPlugin()**: Registers a plugin with configuration
- **UnregisterPlugin()**: Removes a plugin from the manager
- **EnrichUserProfile()**: Enriches user profile using all available plugins
- **EnrichWithPlugin()**: Enriches user profile using a specific plugin
- **GetPluginStatus()**: Returns the status of all plugins
- **Shutdown()**: Gracefully shuts down all plugins

### Plugin Registry

The Plugin Registry manages plugin discovery, registration, and metadata:

```go
type Registry struct {
    plugins       map[string]*PluginEntry
    pluginDir     string
    logger        *logrus.Logger
    // ... other fields
}
```

#### Features

- **Auto-discovery**: Automatically discovers plugins in the plugin directory
- **Metadata management**: Stores and manages plugin metadata
- **Search and filtering**: Provides search capabilities for plugins
- **Version management**: Handles plugin versioning and compatibility
- **Usage tracking**: Tracks plugin usage statistics
- **Rating system**: Supports plugin ratings and reviews

### Health Checker

The Health Checker monitors plugin health and performance:

```go
type HealthChecker struct {
    plugins       map[string]ExternalSystemPlugin
    healthStatus  map[string]*HealthStatus
    checkInterval time.Duration
    logger        *logrus.Logger
    // ... other fields
}
```

#### Features

- **Continuous monitoring**: Regularly checks plugin health
- **Health status tracking**: Maintains detailed health status for each plugin
- **Performance metrics**: Tracks response times and error rates
- **Alerting**: Provides health status information for monitoring systems

## Configuration

### Plugin Configuration

Plugins are configured using YAML files or environment variables:

```yaml
plugins:
  my_plugin:
    enabled: true
    config:
      api_url: "https://api.example.com"
      api_key: "${MY_PLUGIN_API_KEY}"
      timeout: 30
      rate_limit: 100
```

### Configuration Schema

Plugins define their configuration schema using the `ConfigSchema` structure:

```go
type ConfigSchema struct {
    Properties map[string]*ConfigProperty `json:"properties"`
    Required   []string                   `json:"required"`
}

type ConfigProperty struct {
    Type        string      `json:"type"`
    Description string      `json:"description"`
    Default     interface{} `json:"default,omitempty"`
    Enum        []string    `json:"enum,omitempty"`
    Pattern     string      `json:"pattern,omitempty"`
    Minimum     *float64    `json:"minimum,omitempty"`
    Maximum     *float64    `json:"maximum,omitempty"`
    Required    bool        `json:"required"`
    Sensitive   bool        `json:"sensitive"`
}
```

## Testing

### Unit Testing

The framework provides comprehensive testing utilities:

```go
func TestMyPlugin(t *testing.T) {
    plugin := NewMyPlugin(logrus.New())
    
    // Test plugin interface compliance
    tester := NewPluginTester(logrus.New())
    errors := tester.ValidatePluginInterface(plugin)
    assert.Empty(t, errors)
    
    // Test with mock server
    responses := map[string]MockResponse{
        "GET /users/123": {
            StatusCode: 200,
            Body: map[string]interface{}{
                "id": "123",
                "name": "Test User",
            },
        },
    }
    
    tester.SetupMockServer(responses)
    defer tester.TeardownMockServer()
    
    // Connect plugin
    config := map[string]interface{}{
        "api_url": tester.GetMockServerURL(),
        "api_key": "test-key",
    }
    
    err := plugin.Connect(config)
    assert.NoError(t, err)
    
    // Test enrichment
    enrichment, err := plugin.EnrichUserProfile("123")
    assert.NoError(t, err)
    assert.NotNil(t, enrichment)
}
```

### Performance Testing

```go
func TestMyPlugin_Performance(t *testing.T) {
    plugin := NewMyPlugin(logrus.New())
    tester := NewPluginTester(logrus.New())
    
    // Setup and connect plugin
    // ...
    
    // Run benchmark
    result := tester.BenchmarkPlugin(plugin, "user123", 100)
    
    assert.Less(t, result.AverageDuration, 500*time.Millisecond)
    assert.Greater(t, result.SuccessRate, 95.0)
}
```

### Integration Testing

```go
func TestMyPlugin_Integration(t *testing.T) {
    suite := &TestSuite{
        Plugin: NewMyPlugin(logrus.New()),
        TestCases: []TestCase{
            {
                Name:   "Valid User",
                UserID: "user123",
                ExpectedResult: &UserEnrichment{
                    Source: "my-plugin",
                    Interests: []string{"technology"},
                },
            },
        },
        Performance: &PerformanceTest{
            ConcurrentUsers: 10,
            Duration:        30 * time.Second,
            MaxLatency:      time.Second,
        },
        Integration: &IntegrationTest{
            ConfigValidation: true,
            ErrorHandling:    true,
            HealthCheck:      true,
        },
    }
    
    tester := NewPluginTester(logrus.New())
    result, err := tester.RunTestSuite(suite)
    
    assert.NoError(t, err)
    assert.True(t, result.Success)
}
```

## Error Handling

### Plugin Errors

Use the `PluginError` type for consistent error handling:

```go
type PluginError struct {
    Plugin    string
    Operation string
    Message   string
    Cause     error
    Code      string
    Retryable bool
}
```

### Error Codes

Standard error codes are defined:

- `ErrorCodeConnection`: Connection-related errors
- `ErrorCodeAuthentication`: Authentication failures
- `ErrorCodeRateLimit`: Rate limit exceeded
- `ErrorCodeNotFound`: Resource not found
- `ErrorCodeInvalidConfig`: Invalid configuration
- `ErrorCodeTimeout`: Request timeout
- `ErrorCodeQuotaExceeded`: API quota exceeded

### Example Error Handling

```go
func (p *MyPlugin) EnrichUserProfile(userID string) (*UserEnrichment, error) {
    resp, err := p.makeAPIRequest("GET", "/users/"+userID, nil)
    if err != nil {
        return nil, &PluginError{
            Plugin:    p.name,
            Operation: "fetch_user",
            Message:   "failed to fetch user data",
            Cause:     err,
            Code:      ErrorCodeConnection,
            Retryable: true,
        }
    }
    
    if resp.StatusCode == 404 {
        return nil, &PluginError{
            Plugin:    p.name,
            Operation: "fetch_user",
            Message:   "user not found",
            Code:      ErrorCodeNotFound,
            Retryable: false,
        }
    }
    
    // ... process response
}
```

## Best Practices

### Performance

1. **Use connection pooling** for HTTP clients
2. **Implement caching** for frequently accessed data
3. **Respect rate limits** of external APIs
4. **Use timeouts** for all external requests
5. **Implement retry logic** with exponential backoff

### Security

1. **Mark sensitive configuration** fields as sensitive
2. **Validate all inputs** from external systems
3. **Use HTTPS** for all external communications
4. **Store credentials securely** using environment variables
5. **Implement proper authentication** handling

### Reliability

1. **Implement comprehensive health checks**
2. **Handle all error conditions gracefully**
3. **Provide meaningful error messages**
4. **Log important events and errors**
5. **Test with various failure scenarios**

### Maintainability

1. **Follow consistent naming conventions**
2. **Document all public methods and types**
3. **Use structured logging with appropriate levels**
4. **Implement comprehensive unit tests**
5. **Keep plugins focused and single-purpose**

## Example Plugins

The framework includes several example plugins:

### CRM Plugin (`crm_plugin.go`)

Integrates with CRM systems like Salesforce, HubSpot, and Pipedrive to provide:
- User demographics
- Purchase history
- Lead scoring
- Interaction tracking

### Social Media Plugin (`social_media_plugin.go`)

Connects to social media platforms to extract:
- User interests
- Social connections
- Engagement metrics
- Activity patterns

### Custom Plugin Template

Use the plugin development tutorial to create custom plugins for your specific needs.

## Deployment

### Plugin Manifest

Create a `plugin.json` manifest file for your plugin:

```json
{
  "name": "my-plugin",
  "version": "1.0.0",
  "description": "My custom plugin",
  "author": "Your Name",
  "capabilities": ["user_demographics"],
  "config_schema": {
    "properties": {
      "api_url": {
        "type": "string",
        "required": true
      }
    }
  }
}
```

### Configuration

Add your plugin configuration to the main configuration file:

```yaml
plugins:
  my_plugin:
    enabled: true
    config:
      api_url: "https://api.example.com"
      api_key: "${MY_PLUGIN_API_KEY}"
```

### Registration

Register your plugin with the plugin manager:

```go
func init() {
    // Register plugin factory
    RegisterPluginFactory("my-plugin", func(logger *logrus.Logger) ExternalSystemPlugin {
        return NewMyPlugin(logger)
    })
}
```

## Monitoring

### Health Monitoring

The framework provides comprehensive health monitoring:

```go
// Check overall plugin health
healthSummary := healthChecker.GetHealthSummary()
fmt.Printf("Healthy plugins: %d/%d\n", healthSummary.HealthyPlugins, healthSummary.TotalPlugins)

// Check specific plugin health
status, exists := healthChecker.GetHealthStatus("my-plugin")
if exists {
    fmt.Printf("Plugin %s is healthy: %v\n", status.Plugin, status.Healthy)
}
```

### Performance Metrics

Track plugin performance metrics:

```go
// Get plugin statistics
stats := registry.GetPluginStats()
fmt.Printf("Total usage: %d\n", stats.TotalUsage)
fmt.Printf("Average rating: %.2f\n", stats.AverageRating)
```

### Logging

Use structured logging for better observability:

```go
p.logger.WithFields(logrus.Fields{
    "user_id":    userID,
    "duration":   duration,
    "confidence": enrichment.Confidence,
}).Info("User profile enriched successfully")
```

## Troubleshooting

### Common Issues

1. **Connection failures**: Check API credentials and network connectivity
2. **Rate limiting**: Implement proper rate limiting and retry logic
3. **Authentication errors**: Verify API keys and authentication methods
4. **Data quality issues**: Implement data validation and confidence scoring
5. **Performance problems**: Use profiling tools and optimize bottlenecks

### Debug Mode

Enable debug logging for detailed troubleshooting:

```go
logger := logrus.New()
logger.SetLevel(logrus.DebugLevel)
plugin := NewMyPlugin(logger)
```

### Health Checks

Implement comprehensive health checks:

```go
func (p *MyPlugin) IsHealthy() bool {
    // Check connection
    if !p.connected {
        return false
    }
    
    // Check recent errors
    if p.lastError != nil && time.Since(p.lastErrorTime) < 5*time.Minute {
        return false
    }
    
    // Perform lightweight API call
    return p.testConnection() == nil
}
```

## Contributing

To contribute to the plugin framework:

1. Fork the repository
2. Create a feature branch
3. Implement your changes with tests
4. Submit a pull request

### Plugin Contributions

To contribute a new plugin:

1. Follow the plugin development tutorial
2. Implement comprehensive tests
3. Create documentation and examples
4. Submit for review

## Support

For support and questions:

- **Documentation**: [Plugin Development Tutorial](../docs/plugin-development-tutorial.md)
- **Issues**: GitHub Issues
- **Email**: support@recommendation-engine.com
- **Community**: Discord/Slack channels

## License

This plugin framework is licensed under the MIT License. See LICENSE file for details.