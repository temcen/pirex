package plugins

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SocialMediaPlugin integrates with social media platforms
type SocialMediaPlugin struct {
	name      string
	platforms map[string]*PlatformClient
}

// PlatformClient represents a social media platform client
type PlatformClient struct {
	Name    string
	APIKey  string
	BaseURL string
	Enabled bool
}

// NewSocialMediaPlugin creates a new social media plugin
func NewSocialMediaPlugin() *SocialMediaPlugin {
	return &SocialMediaPlugin{
		name:      "social_media_plugin",
		platforms: make(map[string]*PlatformClient),
	}
}

// Name returns the plugin name
func (p *SocialMediaPlugin) Name() string {
	return p.name
}

// Initialize initializes the plugin with configuration
func (p *SocialMediaPlugin) Initialize(config map[string]interface{}) error {
	if platforms, ok := config["platforms"].(map[string]interface{}); ok {
		for platformName, platformConfig := range platforms {
			if pc, ok := platformConfig.(map[string]interface{}); ok {
				client := &PlatformClient{
					Name:    platformName,
					Enabled: true,
				}

				if apiKey, ok := pc["api_key"].(string); ok {
					client.APIKey = apiKey
				}

				if baseURL, ok := pc["base_url"].(string); ok {
					client.BaseURL = baseURL
				}

				if enabled, ok := pc["enabled"].(bool); ok {
					client.Enabled = enabled
				}

				if client.Enabled && client.APIKey != "" && client.BaseURL != "" {
					p.platforms[platformName] = client
				}
			}
		}
	}

	if len(p.platforms) == 0 {
		return fmt.Errorf("no valid social media platforms configured")
	}

	return nil
}

// Process enriches user data from social media platforms
func (p *SocialMediaPlugin) Process(ctx context.Context, userID uuid.UUID) (*UserEnrichment, error) {
	enrichment := &UserEnrichment{
		UserID:            userID,
		Interests:         []string{},
		SocialConnections: []SocialConnection{},
		ExternalData:      make(map[string]interface{}),
		Source:            p.name,
		Confidence:        0.0,
		Timestamp:         time.Now(),
	}

	totalPlatforms := len(p.platforms)
	successfulPlatforms := 0

	// Process each platform
	for platformName, client := range p.platforms {
		if !client.Enabled {
			continue
		}

		platformData, err := p.fetchPlatformData(ctx, client, userID)
		if err != nil {
			// Log error but continue with other platforms
			continue
		}

		// Merge platform data
		if platformData != nil {
			enrichment.Interests = append(enrichment.Interests, platformData.Interests...)
			enrichment.SocialConnections = append(enrichment.SocialConnections, platformData.SocialConnections...)

			// Add platform-specific data
			enrichment.ExternalData[platformName] = platformData.ExternalData
			successfulPlatforms++
		}
	}

	// Calculate confidence based on successful platforms
	if totalPlatforms > 0 {
		enrichment.Confidence = float64(successfulPlatforms) / float64(totalPlatforms)
	}

	// Deduplicate interests
	enrichment.Interests = deduplicateStrings(enrichment.Interests)

	return enrichment, nil
}

// fetchPlatformData fetches data from a specific platform
func (p *SocialMediaPlugin) fetchPlatformData(ctx context.Context, client *PlatformClient, userID uuid.UUID) (*UserEnrichment, error) {
	// This is a mock implementation
	// In a real implementation, this would make actual API calls to social media platforms

	switch client.Name {
	case "twitter":
		return p.fetchTwitterData(ctx, client, userID)
	case "facebook":
		return p.fetchFacebookData(ctx, client, userID)
	case "linkedin":
		return p.fetchLinkedInData(ctx, client, userID)
	default:
		return nil, fmt.Errorf("unsupported platform: %s", client.Name)
	}
}

// fetchTwitterData fetches data from Twitter (mock implementation)
func (p *SocialMediaPlugin) fetchTwitterData(ctx context.Context, client *PlatformClient, userID uuid.UUID) (*UserEnrichment, error) {
	// Mock Twitter data
	return &UserEnrichment{
		UserID:    userID,
		Interests: []string{"technology", "programming", "artificial_intelligence"},
		SocialConnections: []SocialConnection{
			{
				Platform: "twitter",
				UserID:   userID.String(),
				Username: "user_" + userID.String()[:8],
			},
		},
		ExternalData: map[string]interface{}{
			"followers_count": 150,
			"following_count": 300,
			"tweet_count":     450,
		},
		Source:     client.Name,
		Confidence: 0.7,
		Timestamp:  time.Now(),
	}, nil
}

// fetchFacebookData fetches data from Facebook (mock implementation)
func (p *SocialMediaPlugin) fetchFacebookData(ctx context.Context, client *PlatformClient, userID uuid.UUID) (*UserEnrichment, error) {
	// Mock Facebook data
	return &UserEnrichment{
		UserID:    userID,
		Interests: []string{"travel", "photography", "food"},
		SocialConnections: []SocialConnection{
			{
				Platform: "facebook",
				UserID:   userID.String(),
			},
		},
		ExternalData: map[string]interface{}{
			"friends_count": 250,
			"likes_count":   80,
		},
		Source:     client.Name,
		Confidence: 0.6,
		Timestamp:  time.Now(),
	}, nil
}

// fetchLinkedInData fetches data from LinkedIn (mock implementation)
func (p *SocialMediaPlugin) fetchLinkedInData(ctx context.Context, client *PlatformClient, userID uuid.UUID) (*UserEnrichment, error) {
	// Mock LinkedIn data
	return &UserEnrichment{
		UserID:    userID,
		Interests: []string{"business", "leadership", "marketing"},
		SocialConnections: []SocialConnection{
			{
				Platform: "linkedin",
				UserID:   userID.String(),
			},
		},
		ExternalData: map[string]interface{}{
			"connections_count": 500,
			"industry":          "Technology",
			"experience_years":  5,
		},
		Source:     client.Name,
		Confidence: 0.8,
		Timestamp:  time.Now(),
	}, nil
}

// Cleanup performs cleanup operations
func (p *SocialMediaPlugin) Cleanup() error {
	// Clean up any resources
	return nil
}

// HealthCheck checks if the plugin is healthy
func (p *SocialMediaPlugin) HealthCheck() error {
	// Check if at least one platform is configured and enabled
	enabledCount := 0
	for _, client := range p.platforms {
		if client.Enabled {
			enabledCount++
		}
	}

	if enabledCount == 0 {
		return fmt.Errorf("no enabled social media platforms")
	}

	return nil
}

// Helper functions

func deduplicateStrings(slice []string) []string {
	seen := make(map[string]bool)
	result := []string{}

	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}
