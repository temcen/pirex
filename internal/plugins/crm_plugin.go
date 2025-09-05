package plugins

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// CRMPlugin implements ExternalSystemPlugin for CRM integration (Salesforce, HubSpot, etc.)
type CRMPlugin struct {
	name         string
	version      string
	apiURL       string
	apiKey       string
	clientID     string
	clientSecret string
	accessToken  string
	refreshToken string
	client       *http.Client
	logger       *logrus.Logger
	rateLimiter  *RateLimiter
	lastError    error
	connected    bool
	crmType      string // "salesforce", "hubspot", "pipedrive"
}

// CRMConfig represents CRM plugin configuration
type CRMConfig struct {
	APIURL       string `json:"api_url"`
	APIKey       string `json:"api_key"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	CRMType      string `json:"crm_type"`
	Timeout      int    `json:"timeout"`
	RateLimit    int    `json:"rate_limit"`
}

// CRMUserData represents user data from CRM
type CRMUserData struct {
	UserID          string                 `json:"user_id"`
	Email           string                 `json:"email"`
	FirstName       string                 `json:"first_name"`
	LastName        string                 `json:"last_name"`
	Company         string                 `json:"company"`
	JobTitle        string                 `json:"job_title"`
	Industry        string                 `json:"industry"`
	LeadScore       int                    `json:"lead_score"`
	LifecycleStage  string                 `json:"lifecycle_stage"`
	LastActivity    time.Time              `json:"last_activity"`
	TotalRevenue    float64                `json:"total_revenue"`
	PurchaseHistory []CRMPurchase          `json:"purchase_history"`
	Interactions    []CRMInteraction       `json:"interactions"`
	CustomFields    map[string]interface{} `json:"custom_fields"`
}

// CRMPurchase represents a purchase record from CRM
type CRMPurchase struct {
	ID       string    `json:"id"`
	Amount   float64   `json:"amount"`
	Currency string    `json:"currency"`
	Product  string    `json:"product"`
	Category string    `json:"category"`
	Date     time.Time `json:"date"`
	Status   string    `json:"status"`
	SalesRep string    `json:"sales_rep"`
}

// CRMInteraction represents an interaction record from CRM
type CRMInteraction struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"` // email, call, meeting, etc.
	Subject    string                 `json:"subject"`
	Date       time.Time              `json:"date"`
	Duration   int                    `json:"duration"` // in minutes
	Outcome    string                 `json:"outcome"`
	Notes      string                 `json:"notes"`
	Properties map[string]interface{} `json:"properties"`
}

// NewCRMPlugin creates a new CRM plugin instance
func NewCRMPlugin(logger *logrus.Logger) *CRMPlugin {
	if logger == nil {
		logger = logrus.New()
	}

	return &CRMPlugin{
		name:    "crm-plugin",
		version: "1.0.0",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger:      logger,
		rateLimiter: NewRateLimiter(100, time.Minute), // 100 requests per minute default
	}
}

// Name returns the plugin name
func (c *CRMPlugin) Name() string {
	return c.name
}

// Connect establishes connection to the CRM system
func (c *CRMPlugin) Connect(config map[string]interface{}) error {
	// Parse configuration
	crmConfig, err := c.parseConfig(config)
	if err != nil {
		return fmt.Errorf("invalid CRM configuration: %w", err)
	}

	c.apiURL = crmConfig.APIURL
	c.apiKey = crmConfig.APIKey
	c.clientID = crmConfig.ClientID
	c.clientSecret = crmConfig.ClientSecret
	c.crmType = crmConfig.CRMType

	// Set timeout
	if crmConfig.Timeout > 0 {
		c.client.Timeout = time.Duration(crmConfig.Timeout) * time.Second
	}

	// Set rate limit
	if crmConfig.RateLimit > 0 {
		c.rateLimiter = NewRateLimiter(crmConfig.RateLimit, time.Minute)
	}

	// Test connection
	if err := c.testConnection(); err != nil {
		c.lastError = err
		c.connected = false
		return fmt.Errorf("CRM connection test failed: %w", err)
	}

	c.connected = true
	c.lastError = nil

	c.logger.WithFields(logrus.Fields{
		"crm_type": c.crmType,
		"api_url":  c.apiURL,
	}).Info("CRM plugin connected successfully")

	return nil
}

// EnrichUserProfile enriches user profile with CRM data
func (c *CRMPlugin) EnrichUserProfile(userID string) (*UserEnrichment, error) {
	if !c.connected {
		return nil, &PluginError{
			Plugin:    c.name,
			Operation: "enrich",
			Message:   "CRM plugin not connected",
			Code:      ErrorCodeConnection,
			Retryable: false,
		}
	}

	// Rate limiting
	if !c.rateLimiter.Allow() {
		return nil, &PluginError{
			Plugin:    c.name,
			Operation: "enrich",
			Message:   "rate limit exceeded",
			Code:      ErrorCodeRateLimit,
			Retryable: true,
		}
	}

	// Fetch user data from CRM
	crmData, err := c.fetchUserData(userID)
	if err != nil {
		c.lastError = err
		return nil, err
	}

	// Convert to UserEnrichment
	enrichment := c.convertToEnrichment(crmData)

	c.logger.WithFields(logrus.Fields{
		"user_id":    userID,
		"confidence": enrichment.Confidence,
	}).Debug("CRM user profile enriched")

	return enrichment, nil
}

// IsHealthy checks if the CRM connection is healthy
func (c *CRMPlugin) IsHealthy() bool {
	if !c.connected {
		return false
	}

	// Perform a lightweight health check
	err := c.healthCheck()
	if err != nil {
		c.lastError = err
		return false
	}

	return true
}

// Cleanup performs cleanup when the plugin is shut down
func (c *CRMPlugin) Cleanup() error {
	c.connected = false
	c.logger.Info("CRM plugin cleaned up")
	return nil
}

// GetMetadata returns plugin metadata
func (c *CRMPlugin) GetMetadata() *PluginMetadata {
	return &PluginMetadata{
		Name:        c.name,
		Version:     c.version,
		Description: "CRM integration plugin for user profile enrichment",
		Author:      "Recommendation Engine Team",
		License:     "MIT",
		Capabilities: []string{
			"user_demographics",
			"purchase_history",
			"lead_scoring",
			"interaction_tracking",
		},
		Config: &ConfigSchema{
			Properties: map[string]*ConfigProperty{
				"api_url": {
					Type:        "string",
					Description: "CRM API base URL",
					Required:    true,
				},
				"api_key": {
					Type:        "string",
					Description: "CRM API key",
					Required:    true,
					Sensitive:   true,
				},
				"client_id": {
					Type:        "string",
					Description: "OAuth client ID",
					Required:    false,
				},
				"client_secret": {
					Type:        "string",
					Description: "OAuth client secret",
					Required:    false,
					Sensitive:   true,
				},
				"crm_type": {
					Type:        "string",
					Description: "Type of CRM system",
					Required:    true,
					Enum:        []string{"salesforce", "hubspot", "pipedrive"},
				},
				"timeout": {
					Type:        "integer",
					Description: "Request timeout in seconds",
					Default:     30,
					Minimum:     &[]float64{1}[0],
					Maximum:     &[]float64{300}[0],
				},
				"rate_limit": {
					Type:        "integer",
					Description: "Rate limit (requests per minute)",
					Default:     100,
					Minimum:     &[]float64{1}[0],
					Maximum:     &[]float64{1000}[0],
				},
			},
			Required: []string{"api_url", "api_key", "crm_type"},
		},
		Tags: []string{"crm", "user-data", "demographics"},
	}
}

// Private methods

func (c *CRMPlugin) parseConfig(config map[string]interface{}) (*CRMConfig, error) {
	crmConfig := &CRMConfig{}

	if apiURL, ok := config["api_url"].(string); ok {
		crmConfig.APIURL = apiURL
	} else {
		return nil, fmt.Errorf("api_url is required")
	}

	if apiKey, ok := config["api_key"].(string); ok {
		crmConfig.APIKey = apiKey
	} else {
		return nil, fmt.Errorf("api_key is required")
	}

	if crmType, ok := config["crm_type"].(string); ok {
		crmConfig.CRMType = crmType
	} else {
		return nil, fmt.Errorf("crm_type is required")
	}

	if clientID, ok := config["client_id"].(string); ok {
		crmConfig.ClientID = clientID
	}

	if clientSecret, ok := config["client_secret"].(string); ok {
		crmConfig.ClientSecret = clientSecret
	}

	if timeout, ok := config["timeout"]; ok {
		if timeoutInt, ok := timeout.(int); ok {
			crmConfig.Timeout = timeoutInt
		} else if timeoutFloat, ok := timeout.(float64); ok {
			crmConfig.Timeout = int(timeoutFloat)
		}
	}

	if rateLimit, ok := config["rate_limit"]; ok {
		if rateLimitInt, ok := rateLimit.(int); ok {
			crmConfig.RateLimit = rateLimitInt
		} else if rateLimitFloat, ok := rateLimit.(float64); ok {
			crmConfig.RateLimit = int(rateLimitFloat)
		}
	}

	return crmConfig, nil
}

func (c *CRMPlugin) testConnection() error {
	// Perform a simple API call to test connectivity
	endpoint := c.getHealthCheckEndpoint()

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return err
	}

	c.addAuthHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("CRM API returned status %d", resp.StatusCode)
	}

	return nil
}

func (c *CRMPlugin) healthCheck() error {
	// Lightweight health check
	return c.testConnection()
}

func (c *CRMPlugin) fetchUserData(userID string) (*CRMUserData, error) {
	endpoint := c.getUserEndpoint(userID)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	c.addAuthHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, &PluginError{
			Plugin:    c.name,
			Operation: "fetch_user",
			Message:   "HTTP request failed",
			Cause:     err,
			Code:      ErrorCodeConnection,
			Retryable: true,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, &PluginError{
			Plugin:    c.name,
			Operation: "fetch_user",
			Message:   "user not found in CRM",
			Code:      ErrorCodeNotFound,
			Retryable: false,
		}
	}

	if resp.StatusCode >= 400 {
		return nil, &PluginError{
			Plugin:    c.name,
			Operation: "fetch_user",
			Message:   fmt.Sprintf("CRM API error: %d", resp.StatusCode),
			Code:      ErrorCodeConnection,
			Retryable: resp.StatusCode >= 500,
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse response based on CRM type
	return c.parseUserResponse(body)
}

func (c *CRMPlugin) convertToEnrichment(crmData *CRMUserData) *UserEnrichment {
	enrichment := &UserEnrichment{
		Source:    c.name,
		Timestamp: time.Now(),
		TTL:       1 * time.Hour, // Cache for 1 hour
	}

	// Demographics
	if crmData != nil {
		enrichment.Demographics = &Demographics{
			Location:   crmData.Company,
			Occupation: crmData.JobTitle,
		}

		// Behavior patterns
		enrichment.BehaviorPatterns = &BehaviorPatterns{
			PurchaseHistory: c.convertPurchaseHistory(crmData.PurchaseHistory),
		}

		// Calculate confidence based on data completeness
		confidence := 0.0
		dataPoints := 0

		if crmData.Email != "" {
			confidence += 0.2
			dataPoints++
		}
		if crmData.Company != "" {
			confidence += 0.2
			dataPoints++
		}
		if crmData.JobTitle != "" {
			confidence += 0.2
			dataPoints++
		}
		if crmData.LeadScore > 0 {
			confidence += 0.2
			dataPoints++
		}
		if len(crmData.PurchaseHistory) > 0 {
			confidence += 0.2
			dataPoints++
		}

		enrichment.Confidence = confidence

		// Add contextual data
		enrichment.ContextualData = map[string]interface{}{
			"lead_score":      crmData.LeadScore,
			"lifecycle_stage": crmData.LifecycleStage,
			"total_revenue":   crmData.TotalRevenue,
			"last_activity":   crmData.LastActivity,
			"industry":        crmData.Industry,
		}
	}

	return enrichment
}

func (c *CRMPlugin) convertPurchaseHistory(crmPurchases []CRMPurchase) []PurchaseEvent {
	purchases := make([]PurchaseEvent, len(crmPurchases))

	for i, purchase := range crmPurchases {
		purchases[i] = PurchaseEvent{
			ProductID: purchase.Product,
			Category:  purchase.Category,
			Amount:    purchase.Amount,
			Currency:  purchase.Currency,
			Timestamp: purchase.Date,
			Channel:   "crm",
		}
	}

	return purchases
}

func (c *CRMPlugin) getHealthCheckEndpoint() string {
	switch c.crmType {
	case "salesforce":
		return c.apiURL + "/services/data/v52.0/"
	case "hubspot":
		return c.apiURL + "/crm/v3/objects/contacts"
	case "pipedrive":
		return c.apiURL + "/v1/users"
	default:
		return c.apiURL + "/health"
	}
}

func (c *CRMPlugin) getUserEndpoint(userID string) string {
	switch c.crmType {
	case "salesforce":
		return fmt.Sprintf("%s/services/data/v52.0/sobjects/Contact/%s", c.apiURL, userID)
	case "hubspot":
		return fmt.Sprintf("%s/crm/v3/objects/contacts/%s", c.apiURL, userID)
	case "pipedrive":
		return fmt.Sprintf("%s/v1/persons/%s", c.apiURL, userID)
	default:
		return fmt.Sprintf("%s/users/%s", c.apiURL, userID)
	}
}

func (c *CRMPlugin) addAuthHeaders(req *http.Request) {
	switch c.crmType {
	case "salesforce":
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	case "hubspot":
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	case "pipedrive":
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	default:
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "RecommendationEngine-CRMPlugin/"+c.version)
}

func (c *CRMPlugin) parseUserResponse(body []byte) (*CRMUserData, error) {
	var crmData CRMUserData

	// This would be implemented differently for each CRM type
	// For now, assume a generic JSON response
	if err := json.Unmarshal(body, &crmData); err != nil {
		return nil, fmt.Errorf("failed to parse CRM response: %w", err)
	}

	return &crmData, nil
}

// RateLimiter implements a simple rate limiter
type RateLimiter struct {
	limit    int
	window   time.Duration
	requests []time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limit:    limit,
		window:   window,
		requests: make([]time.Time, 0),
	}
}

// Allow checks if a request is allowed under the rate limit
func (rl *RateLimiter) Allow() bool {
	now := time.Now()

	// Remove old requests outside the window
	cutoff := now.Add(-rl.window)
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
