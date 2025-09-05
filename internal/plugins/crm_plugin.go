package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// CRMPlugin integrates with CRM systems to enrich user data
type CRMPlugin struct {
	name    string
	apiURL  string
	apiKey  string
	client  *http.Client
	timeout time.Duration
}

// NewCRMPlugin creates a new CRM plugin
func NewCRMPlugin() *CRMPlugin {
	return &CRMPlugin{
		name: "crm_plugin",
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		timeout: 5 * time.Second,
	}
}

// Name returns the plugin name
func (p *CRMPlugin) Name() string {
	return p.name
}

// Initialize initializes the plugin with configuration
func (p *CRMPlugin) Initialize(config map[string]interface{}) error {
	if apiURL, ok := config["api_url"].(string); ok {
		p.apiURL = apiURL
	} else {
		return fmt.Errorf("api_url is required for CRM plugin")
	}

	if apiKey, ok := config["api_key"].(string); ok {
		p.apiKey = apiKey
	} else {
		return fmt.Errorf("api_key is required for CRM plugin")
	}

	if timeout, ok := config["timeout"].(string); ok {
		if d, err := time.ParseDuration(timeout); err == nil {
			p.timeout = d
			p.client.Timeout = d
		}
	}

	return nil
}

// Process enriches user data from CRM system
func (p *CRMPlugin) Process(ctx context.Context, userID uuid.UUID) (*UserEnrichment, error) {
	// Create request to CRM API
	url := fmt.Sprintf("%s/users/%s", p.apiURL, userID.String())
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication header
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	// Make request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// User not found in CRM, return empty enrichment
		return &UserEnrichment{
			UserID:     userID,
			Source:     p.name,
			Confidence: 0.0,
			Timestamp:  time.Now(),
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CRM API returned status %d", resp.StatusCode)
	}

	// Parse response
	var crmData struct {
		UserID       string `json:"user_id"`
		Demographics struct {
			Age        int    `json:"age"`
			Gender     string `json:"gender"`
			Location   string `json:"location"`
			Occupation string `json:"occupation"`
			Income     int    `json:"income"`
		} `json:"demographics"`
		Interests []string `json:"interests"`
		Segment   string   `json:"segment"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&crmData); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to enrichment format
	enrichment := &UserEnrichment{
		UserID: userID,
		Demographics: &Demographics{
			Gender:     crmData.Demographics.Gender,
			Location:   crmData.Demographics.Location,
			Occupation: crmData.Demographics.Occupation,
		},
		Interests: crmData.Interests,
		ExternalData: map[string]interface{}{
			"segment": crmData.Segment,
			"age":     crmData.Demographics.Age,
			"income":  crmData.Demographics.Income,
		},
		Source:     p.name,
		Confidence: 0.9, // High confidence for CRM data
		Timestamp:  time.Now(),
	}

	// Set age range
	if crmData.Demographics.Age > 0 {
		enrichment.Demographics.AgeRange = getAgeRange(crmData.Demographics.Age)
	}

	// Set income range
	if crmData.Demographics.Income > 0 {
		enrichment.Demographics.IncomeRange = getIncomeRange(crmData.Demographics.Income)
	}

	return enrichment, nil
}

// Cleanup performs cleanup operations
func (p *CRMPlugin) Cleanup() error {
	// Close HTTP client if needed
	return nil
}

// HealthCheck checks if the plugin is healthy
func (p *CRMPlugin) HealthCheck() error {
	// Simple health check - ping the API
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/health", p.apiURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

// Helper functions

func getAgeRange(age int) string {
	switch {
	case age < 18:
		return "under_18"
	case age < 25:
		return "18_24"
	case age < 35:
		return "25_34"
	case age < 45:
		return "35_44"
	case age < 55:
		return "45_54"
	case age < 65:
		return "55_64"
	default:
		return "65_plus"
	}
}

func getIncomeRange(income int) string {
	switch {
	case income < 25000:
		return "under_25k"
	case income < 50000:
		return "25k_50k"
	case income < 75000:
		return "50k_75k"
	case income < 100000:
		return "75k_100k"
	case income < 150000:
		return "100k_150k"
	default:
		return "150k_plus"
	}
}
