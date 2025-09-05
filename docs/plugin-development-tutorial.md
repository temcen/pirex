# Plugin Development Tutorial

This tutorial will guide you through developing external system plugins for the recommendation engine. Plugins allow you to enrich user profiles with data from external systems like CRM platforms, social media APIs, analytics services, and more.

## Table of Contents

1. [Getting Started](#getting-started)
2. [Plugin Interface Overview](#plugin-interface-overview)
3. [Step 1: Plugin Structure and Boilerplate](#step-1-plugin-structure-and-boilerplate)
4. [Step 2: Implementing the Plugin Interface](#step-2-implementing-the-plugin-interface)
5. [Step 3: Configuration Management](#step-3-configuration-management)
6. [Step 4: API Client Implementation](#step-4-api-client-implementation)
7. [Step 5: Data Transformation and Enrichment](#step-5-data-transformation-and-enrichment)
8. [Step 6: Testing Your Plugin](#step-6-testing-your-plugin)
9. [Step 7: Deployment and Monitoring](#step-7-deployment-and-monitoring)
10. [Best Practices](#best-practices)
11. [Troubleshooting](#troubleshooting)

## Getting Started

### Prerequisites

- Go 1.21 or later
- Basic understanding of HTTP APIs
- Access to the external system you want to integrate

### Development Environment Setup

1. Clone the recommendation engine repository
2. Navigate to the plugins directory: `cd internal/plugins`
3. Create a new file for your plugin: `touch my_plugin.go`

## Plugin Interface Overview

All plugins must implement the `ExternalSystemPlugin` interface:

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

### Key Data Structures

```go
type UserEnrichment struct {
    Demographics      *Demographics      `json:"demographics,omitempty"`
    Interests         []string          `json:"interests,omitempty"`
    SocialConnections *SocialConnections `json:"social_connections,omitempty"`
    BehaviorPatterns  *BehaviorPatterns  `json:"behavior_patterns,omitempty"`
    ContextualData    map[string]interface{} `json:"contextual_data,omitempty"`
    Confidence        float64           `json:"confidence"`
    Source            string            `json:"source"`
    Timestamp         time.Time         `json:"timestamp"`
    TTL               time.Duration     `json:"ttl"`
}
```

## Step 1: Plugin Structure and Boilerplate

### Basic Plugin Structure

Create a new plugin file with the following structure:

```go
package plugins

import (
    "fmt"
    "net/http"
    "time"
    
    "github.com/sirupsen/logrus"
)

// MyPlugin implements ExternalSystemPlugin for [Your Service]
type MyPlugin struct {
    name      string
    version   string
    apiURL    string
    apiKey    string
    client    *http.Client
    logger    *logrus.Logger
    connected bool
    lastError error
}

// NewMyPlugin creates a new instance of your plugin
func NewMyPlugin(logger *logrus.Logger) *MyPlugin {
    if logger == nil {
        logger = logrus.New()
    }
    
    return &MyPlugin{
        name:    "my-plugin",
        version: "1.0.0",
        client: &http.Client{
            Timeout: 30 * time.Second,
        },
        logger: logger,
    }
}
```

### Plugin Metadata

Define your plugin's metadata:

```go
func (p *MyPlugin) GetMetadata() *PluginMetadata {
    return &PluginMetadata{
        Name:        p.name,
        Version:     p.version,
        Description: "Integration with [Your Service] for user profile enrichment",
        Author:      "Your Name",
        License:     "MIT",
        Capabilities: []string{
            "user_demographics",
            "interests_extraction",
            // Add your capabilities
        },
        Config: &ConfigSchema{
            Properties: map[string]*ConfigProperty{
                "api_url": {
                    Type:        "string",
                    Description: "API base URL",
                    Required:    true,
                },
                "api_key": {
                    Type:        "string",
                    Description: "API key for authentication",
                    Required:    true,
                    Sensitive:   true,
                },
            },
            Required: []string{"api_url", "api_key"},
        },
        Tags: []string{"external-api", "user-data"},
    }
}
```

## Step 2: Implementing the Plugin Interface

### Name Method

```go
func (p *MyPlugin) Name() string {
    return p.name
}
```

### Connect Method

```go
func (p *MyPlugin) Connect(config map[string]interface{}) error {
    // Parse configuration
    apiURL, ok := config["api_url"].(string)
    if !ok || apiURL == "" {
        return fmt.Errorf("api_url is required")
    }
    
    apiKey, ok := config["api_key"].(string)
    if !ok || apiKey == "" {
        return fmt.Errorf("api_key is required")
    }
    
    p.apiURL = apiURL
    p.apiKey = apiKey
    
    // Test connection
    if err := p.testConnection(); err != nil {
        p.lastError = err
        p.connected = false
        return fmt.Errorf("connection test failed: %w", err)
    }
    
    p.connected = true
    p.lastError = nil
    
    p.logger.WithField("api_url", apiURL).Info("Plugin connected successfully")
    
    return nil
}
```

### EnrichUserProfile Method

```go
func (p *MyPlugin) EnrichUserProfile(userID string) (*UserEnrichment, error) {
    if !p.connected {
        return nil, &PluginError{
            Plugin:    p.name,
            Operation: "enrich",
            Message:   "plugin not connected",
            Code:      ErrorCodeConnection,
            Retryable: false,
        }
    }
    
    // Fetch user data from external API
    userData, err := p.fetchUserData(userID)
    if err != nil {
        p.lastError = err
        return nil, err
    }
    
    // Convert to UserEnrichment
    enrichment := p.convertToEnrichment(userData)
    
    p.logger.WithFields(logrus.Fields{
        "user_id":    userID,
        "confidence": enrichment.Confidence,
    }).Debug("User profile enriched")
    
    return enrichment, nil
}
```

### Health Check Methods

```go
func (p *MyPlugin) IsHealthy() bool {
    if !p.connected {
        return false
    }
    
    // Perform lightweight health check
    err := p.testConnection()
    if err != nil {
        p.lastError = err
        return false
    }
    
    return true
}

func (p *MyPlugin) Cleanup() error {
    p.connected = false
    p.logger.Info("Plugin cleaned up")
    return nil
}
```

## Step 3: Configuration Management

### Configuration Parsing

```go
type MyPluginConfig struct {
    APIURL    string `json:"api_url"`
    APIKey    string `json:"api_key"`
    Timeout   int    `json:"timeout"`
    RateLimit int    `json:"rate_limit"`
}

func (p *MyPlugin) parseConfig(config map[string]interface{}) (*MyPluginConfig, error) {
    pluginConfig := &MyPluginConfig{}
    
    // Required fields
    if apiURL, ok := config["api_url"].(string); ok {
        pluginConfig.APIURL = apiURL
    } else {
        return nil, fmt.Errorf("api_url is required")
    }
    
    if apiKey, ok := config["api_key"].(string); ok {
        pluginConfig.APIKey = apiKey
    } else {
        return nil, fmt.Errorf("api_key is required")
    }
    
    // Optional fields with defaults
    if timeout, ok := config["timeout"]; ok {
        if timeoutInt, ok := timeout.(int); ok {
            pluginConfig.Timeout = timeoutInt
        } else if timeoutFloat, ok := timeout.(float64); ok {
            pluginConfig.Timeout = int(timeoutFloat)
        }
    }
    
    if rateLimit, ok := config["rate_limit"]; ok {
        if rateLimitInt, ok := rateLimit.(int); ok {
            pluginConfig.RateLimit = rateLimitInt
        }
    }
    
    return pluginConfig, nil
}
```

### Environment Variable Support

```go
import "os"

func (p *MyPlugin) loadFromEnvironment() {
    if apiURL := os.Getenv("MY_PLUGIN_API_URL"); apiURL != "" {
        p.apiURL = apiURL
    }
    
    if apiKey := os.Getenv("MY_PLUGIN_API_KEY"); apiKey != "" {
        p.apiKey = apiKey
    }
}
```

## Step 4: API Client Implementation

### HTTP Client with Retry Logic

```go
import (
    "bytes"
    "encoding/json"
    "io"
    "net/http"
    "time"
)

func (p *MyPlugin) makeAPIRequest(method, endpoint string, body interface{}) ([]byte, error) {
    var reqBody io.Reader
    
    if body != nil {
        jsonBody, err := json.Marshal(body)
        if err != nil {
            return nil, err
        }
        reqBody = bytes.NewBuffer(jsonBody)
    }
    
    req, err := http.NewRequest(method, p.apiURL+endpoint, reqBody)
    if err != nil {
        return nil, err
    }
    
    // Add authentication headers
    p.addAuthHeaders(req)
    
    // Retry logic
    var lastErr error
    for attempt := 0; attempt < 3; attempt++ {
        resp, err := p.client.Do(req)
        if err != nil {
            lastErr = err
            time.Sleep(time.Duration(attempt+1) * time.Second)
            continue
        }
        defer resp.Body.Close()
        
        respBody, err := io.ReadAll(resp.Body)
        if err != nil {
            lastErr = err
            continue
        }
        
        // Handle different status codes
        switch resp.StatusCode {
        case 200, 201:
            return respBody, nil
        case 404:
            return nil, &PluginError{
                Plugin:    p.name,
                Operation: "api_request",
                Message:   "resource not found",
                Code:      ErrorCodeNotFound,
                Retryable: false,
            }
        case 429:
            return nil, &PluginError{
                Plugin:    p.name,
                Operation: "api_request",
                Message:   "rate limit exceeded",
                Code:      ErrorCodeRateLimit,
                Retryable: true,
            }
        case 500, 502, 503, 504:
            lastErr = &PluginError{
                Plugin:    p.name,
                Operation: "api_request",
                Message:   fmt.Sprintf("server error: %d", resp.StatusCode),
                Code:      ErrorCodeConnection,
                Retryable: true,
            }
            time.Sleep(time.Duration(attempt+1) * time.Second)
            continue
        default:
            return nil, &PluginError{
                Plugin:    p.name,
                Operation: "api_request",
                Message:   fmt.Sprintf("unexpected status: %d", resp.StatusCode),
                Code:      ErrorCodeConnection,
                Retryable: false,
            }
        }
    }
    
    return nil, lastErr
}

func (p *MyPlugin) addAuthHeaders(req *http.Request) {
    req.Header.Set("Authorization", "Bearer "+p.apiKey)
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("User-Agent", "RecommendationEngine-MyPlugin/"+p.version)
}
```

### Rate Limiting

```go
type RateLimiter struct {
    requests []time.Time
    limit    int
    window   time.Duration
    mu       sync.Mutex
}

func (rl *RateLimiter) Allow() bool {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    
    now := time.Now()
    cutoff := now.Add(-rl.window)
    
    // Remove old requests
    validRequests := make([]time.Time, 0)
    for _, req := range rl.requests {
        if req.After(cutoff) {
            validRequests = append(validRequests, req)
        }
    }
    rl.requests = validRequests
    
    // Check if we can add another request
    if len(rl.requests) < rl.limit {
        rl.requests = append(rl.requests, now)
        return true
    }
    
    return false
}
```

## Step 5: Data Transformation and Enrichment

### Fetching User Data

```go
type ExternalUserData struct {
    ID          string                 `json:"id"`
    Name        string                 `json:"name"`
    Email       string                 `json:"email"`
    Location    string                 `json:"location"`
    Interests   []string               `json:"interests"`
    Demographics map[string]interface{} `json:"demographics"`
    // Add fields specific to your external system
}

func (p *MyPlugin) fetchUserData(userID string) (*ExternalUserData, error) {
    endpoint := fmt.Sprintf("/users/%s", userID)
    
    respBody, err := p.makeAPIRequest("GET", endpoint, nil)
    if err != nil {
        return nil, err
    }
    
    var userData ExternalUserData
    if err := json.Unmarshal(respBody, &userData); err != nil {
        return nil, fmt.Errorf("failed to parse user data: %w", err)
    }
    
    return &userData, nil
}
```

### Converting to UserEnrichment

```go
func (p *MyPlugin) convertToEnrichment(userData *ExternalUserData) *UserEnrichment {
    enrichment := &UserEnrichment{
        Source:    p.name,
        Timestamp: time.Now(),
        TTL:       1 * time.Hour, // Cache for 1 hour
    }
    
    // Convert demographics
    if userData.Location != "" {
        enrichment.Demographics = &Demographics{
            Location: userData.Location,
        }
    }
    
    // Set interests
    enrichment.Interests = userData.Interests
    
    // Calculate confidence based on data completeness
    confidence := p.calculateConfidence(userData)
    enrichment.Confidence = confidence
    
    // Add contextual data
    enrichment.ContextualData = map[string]interface{}{
        "external_id": userData.ID,
        "email":       userData.Email,
        "raw_data":    userData.Demographics,
    }
    
    return enrichment
}

func (p *MyPlugin) calculateConfidence(userData *ExternalUserData) float64 {
    confidence := 0.0
    dataPoints := 0
    
    if userData.Name != "" {
        confidence += 0.2
        dataPoints++
    }
    if userData.Email != "" {
        confidence += 0.2
        dataPoints++
    }
    if userData.Location != "" {
        confidence += 0.2
        dataPoints++
    }
    if len(userData.Interests) > 0 {
        confidence += 0.2
        dataPoints++
    }
    if len(userData.Demographics) > 0 {
        confidence += 0.2
        dataPoints++
    }
    
    return confidence
}
```

## Step 6: Testing Your Plugin

### Unit Tests

```go
package plugins

import (
    "testing"
    "time"
    
    "github.com/sirupsen/logrus"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestMyPlugin_Name(t *testing.T) {
    plugin := NewMyPlugin(logrus.New())
    assert.Equal(t, "my-plugin", plugin.Name())
}

func TestMyPlugin_Connect(t *testing.T) {
    plugin := NewMyPlugin(logrus.New())
    
    // Test with valid config
    config := map[string]interface{}{
        "api_url": "https://api.example.com",
        "api_key": "test-key",
    }
    
    // Note: This will fail without a real API, use mock server for real tests
    err := plugin.Connect(config)
    // assert.NoError(t, err) // Uncomment when using mock server
    
    // Test with invalid config
    invalidConfig := map[string]interface{}{}
    err = plugin.Connect(invalidConfig)
    assert.Error(t, err)
}

func TestMyPlugin_EnrichUserProfile(t *testing.T) {
    plugin := NewMyPlugin(logrus.New())
    
    // Test without connection
    _, err := plugin.EnrichUserProfile("user123")
    assert.Error(t, err)
    
    // Test with connection (requires mock server)
    // setupMockServer()
    // plugin.Connect(validConfig)
    // enrichment, err := plugin.EnrichUserProfile("user123")
    // assert.NoError(t, err)
    // assert.NotNil(t, enrichment)
}
```

### Integration Tests with Mock Server

```go
func TestMyPlugin_Integration(t *testing.T) {
    // Create plugin tester
    tester := NewPluginTester(logrus.New())
    
    // Setup mock responses
    responses := map[string]MockResponse{
        "GET /users/user123": {
            StatusCode: 200,
            Headers:    map[string]string{"Content-Type": "application/json"},
            Body: ExternalUserData{
                ID:        "user123",
                Name:      "Test User",
                Email:     "test@example.com",
                Location:  "San Francisco",
                Interests: []string{"technology", "music"},
            },
        },
        "GET /health": {
            StatusCode: 200,
            Body:       map[string]string{"status": "ok"},
        },
    }
    
    tester.SetupMockServer(responses)
    defer tester.TeardownMockServer()
    
    // Create plugin and connect
    plugin := NewMyPlugin(logrus.New())
    config := map[string]interface{}{
        "api_url": tester.GetMockServerURL(),
        "api_key": "test-key",
    }
    
    err := plugin.Connect(config)
    require.NoError(t, err)
    
    // Test enrichment
    enrichment, err := plugin.EnrichUserProfile("user123")
    require.NoError(t, err)
    require.NotNil(t, enrichment)
    
    assert.Equal(t, "my-plugin", enrichment.Source)
    assert.Contains(t, enrichment.Interests, "technology")
    assert.Greater(t, enrichment.Confidence, 0.0)
}
```

### Performance Testing

```go
func TestMyPlugin_Performance(t *testing.T) {
    plugin := NewMyPlugin(logrus.New())
    
    // Setup mock server
    tester := NewPluginTester(logrus.New())
    // ... setup mock responses ...
    
    // Connect plugin
    plugin.Connect(config)
    
    // Run benchmark
    result := tester.BenchmarkPlugin(plugin, "user123", 100)
    
    assert.Less(t, result.AverageDuration, 500*time.Millisecond)
    assert.Greater(t, result.SuccessRate, 95.0)
    assert.Equal(t, 0, result.ErrorCount)
}
```

## Step 7: Deployment and Monitoring

### Plugin Configuration File

Create a configuration file for your plugin:

```json
{
  "plugin": "my-plugin",
  "config": {
    "api_url": "https://api.example.com",
    "api_key": "${MY_PLUGIN_API_KEY}",
    "timeout": 30,
    "rate_limit": 100
  }
}
```

### Registration with Plugin Manager

```go
func RegisterMyPlugin(manager *Manager) error {
    plugin := NewMyPlugin(nil)
    
    config := map[string]interface{}{
        "api_url":    os.Getenv("MY_PLUGIN_API_URL"),
        "api_key":    os.Getenv("MY_PLUGIN_API_KEY"),
        "timeout":    30,
        "rate_limit": 100,
    }
    
    return manager.RegisterPlugin(plugin, config)
}
```

### Monitoring and Logging

```go
func (p *MyPlugin) EnrichUserProfile(userID string) (*UserEnrichment, error) {
    start := time.Now()
    
    // Log request
    p.logger.WithField("user_id", userID).Debug("Starting user enrichment")
    
    enrichment, err := p.doEnrichment(userID)
    
    duration := time.Since(start)
    
    // Log result
    if err != nil {
        p.logger.WithFields(logrus.Fields{
            "user_id":  userID,
            "duration": duration,
            "error":    err,
        }).Error("User enrichment failed")
    } else {
        p.logger.WithFields(logrus.Fields{
            "user_id":    userID,
            "duration":   duration,
            "confidence": enrichment.Confidence,
        }).Info("User enrichment completed")
    }
    
    return enrichment, err
}
```

## Best Practices

### Error Handling

1. **Use Structured Errors**: Always use `PluginError` for consistent error handling
2. **Classify Errors**: Mark errors as retryable or non-retryable
3. **Provide Context**: Include operation context in error messages
4. **Log Appropriately**: Use different log levels for different error types

```go
// Good error handling
if resp.StatusCode == 404 {
    return nil, &PluginError{
        Plugin:    p.name,
        Operation: "fetch_user",
        Message:   "user not found in external system",
        Code:      ErrorCodeNotFound,
        Retryable: false,
    }
}
```

### Performance Optimization

1. **Connection Pooling**: Reuse HTTP connections
2. **Caching**: Cache frequently accessed data
3. **Rate Limiting**: Respect external API limits
4. **Timeouts**: Set appropriate timeouts for all operations
5. **Batch Operations**: Use batch APIs when available

```go
// Connection pooling
client := &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
    },
}
```

### Security

1. **Secure Configuration**: Mark sensitive fields as sensitive
2. **Input Validation**: Validate all inputs
3. **Output Sanitization**: Sanitize data from external systems
4. **Credential Management**: Use environment variables for secrets

```go
// Secure configuration
"api_key": {
    Type:        "string",
    Description: "API key for authentication",
    Required:    true,
    Sensitive:   true, // This field contains sensitive data
},
```

### Logging

1. **Structured Logging**: Use structured fields
2. **Appropriate Levels**: Use correct log levels
3. **No Sensitive Data**: Never log sensitive information
4. **Performance Metrics**: Log timing and performance data

```go
// Good logging
p.logger.WithFields(logrus.Fields{
    "user_id":    userID,
    "duration":   duration,
    "confidence": enrichment.Confidence,
    "data_points": len(enrichment.Interests),
}).Info("User enrichment completed")
```

## Troubleshooting

### Common Issues

#### Connection Failures

**Problem**: Plugin fails to connect to external API

**Solutions**:
1. Check API URL and credentials
2. Verify network connectivity
3. Check firewall rules
4. Validate SSL certificates

```go
// Debug connection issues
func (p *MyPlugin) testConnection() error {
    resp, err := p.client.Get(p.apiURL + "/health")
    if err != nil {
        p.logger.WithError(err).Error("Connection test failed")
        return err
    }
    defer resp.Body.Close()
    
    p.logger.WithField("status", resp.StatusCode).Debug("Connection test response")
    
    if resp.StatusCode >= 400 {
        return fmt.Errorf("API returned status %d", resp.StatusCode)
    }
    
    return nil
}
```

#### Rate Limiting

**Problem**: Plugin hits rate limits

**Solutions**:
1. Implement exponential backoff
2. Add jitter to retry delays
3. Use rate limiting middleware
4. Cache responses when possible

```go
// Exponential backoff with jitter
func (p *MyPlugin) retryWithBackoff(operation func() error) error {
    var err error
    for attempt := 0; attempt < 5; attempt++ {
        err = operation()
        if err == nil {
            return nil
        }
        
        // Check if error is retryable
        if pluginErr, ok := err.(*PluginError); ok && !pluginErr.Retryable {
            return err
        }
        
        // Calculate delay with jitter
        delay := time.Duration(1<<attempt) * time.Second
        jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
        time.Sleep(delay + jitter)
    }
    
    return err
}
```

#### Data Quality Issues

**Problem**: External API returns inconsistent or poor quality data

**Solutions**:
1. Implement data validation
2. Use confidence scoring
3. Provide fallback values
4. Log data quality metrics

```go
// Data validation
func (p *MyPlugin) validateUserData(data *ExternalUserData) error {
    if data.ID == "" {
        return fmt.Errorf("user ID is required")
    }
    
    if data.Email != "" && !isValidEmail(data.Email) {
        p.logger.WithField("email", data.Email).Warn("Invalid email format")
        data.Email = "" // Clear invalid data
    }
    
    return nil
}
```

### Debugging Tips

1. **Enable Debug Logging**: Set log level to debug during development
2. **Use Mock Servers**: Test with controlled responses
3. **Monitor Health Checks**: Implement comprehensive health checks
4. **Track Metrics**: Monitor success rates, latency, and error rates

```go
// Comprehensive health check
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
    if err := p.testConnection(); err != nil {
        p.lastError = err
        p.lastErrorTime = time.Now()
        return false
    }
    
    return true
}
```

### Performance Debugging

1. **Profile Memory Usage**: Use Go's built-in profiler
2. **Monitor Goroutines**: Check for goroutine leaks
3. **Track Response Times**: Log and monitor API response times
4. **Analyze Bottlenecks**: Identify slow operations

```go
// Performance monitoring
func (p *MyPlugin) EnrichUserProfile(userID string) (*UserEnrichment, error) {
    start := time.Now()
    defer func() {
        duration := time.Since(start)
        p.logger.WithFields(logrus.Fields{
            "operation": "enrich_user_profile",
            "duration":  duration,
            "user_id":   userID,
        }).Debug("Operation completed")
    }()
    
    // ... implementation ...
}
```

## Example Plugins

The repository includes several example plugins:

1. **CRM Plugin** (`crm_plugin.go`): Integrates with CRM systems like Salesforce
2. **Social Media Plugin** (`social_media_plugin.go`): Connects to social media APIs
3. **Weather Plugin**: Provides contextual weather data
4. **Analytics Plugin**: Extracts user behavior from analytics platforms

Study these examples to understand different integration patterns and best practices.

## Next Steps

1. **Implement Your Plugin**: Follow this tutorial to create your plugin
2. **Write Tests**: Create comprehensive unit and integration tests
3. **Deploy and Monitor**: Deploy your plugin and monitor its performance
4. **Contribute**: Consider contributing your plugin to the community

For additional help, check the troubleshooting guide or reach out to the development team.