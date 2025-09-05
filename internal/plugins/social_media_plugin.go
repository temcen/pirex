package plugins

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// SocialMediaPlugin implements ExternalSystemPlugin for social media integration
type SocialMediaPlugin struct {
	name         string
	version      string
	platforms    map[string]*PlatformConfig
	client       *http.Client
	logger       *logrus.Logger
	rateLimiters map[string]*RateLimiter
	lastError    error
	connected    bool
}

// PlatformConfig represents configuration for a social media platform
type PlatformConfig struct {
	Name         string `json:"name"`
	APIURL       string `json:"api_url"`
	APIKey       string `json:"api_key"`
	APISecret    string `json:"api_secret"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Enabled      bool   `json:"enabled"`
	RateLimit    int    `json:"rate_limit"`
}

// SocialUserData represents user data from social media platforms
type SocialUserData struct {
	UserID       string                 `json:"user_id"`
	Platform     string                 `json:"platform"`
	Username     string                 `json:"username"`
	DisplayName  string                 `json:"display_name"`
	Bio          string                 `json:"bio"`
	Location     string                 `json:"location"`
	Followers    int                    `json:"followers"`
	Following    int                    `json:"following"`
	Posts        int                    `json:"posts"`
	Verified     bool                   `json:"verified"`
	Interests    []string               `json:"interests"`
	Hashtags     []string               `json:"hashtags"`
	Mentions     []string               `json:"mentions"`
	Engagement   *EngagementMetrics     `json:"engagement"`
	Demographics *SocialDemographics    `json:"demographics"`
	Activity     *ActivityPatterns      `json:"activity"`
	Network      *NetworkData           `json:"network"`
	CustomData   map[string]interface{} `json:"custom_data"`
}

// EngagementMetrics represents user engagement metrics
type EngagementMetrics struct {
	LikesGiven       int     `json:"likes_given"`
	LikesReceived    int     `json:"likes_received"`
	CommentsGiven    int     `json:"comments_given"`
	CommentsReceived int     `json:"comments_received"`
	SharesGiven      int     `json:"shares_given"`
	SharesReceived   int     `json:"shares_received"`
	EngagementRate   float64 `json:"engagement_rate"`
	Reach            int     `json:"reach"`
	Impressions      int     `json:"impressions"`
}

// SocialDemographics represents demographic data from social platforms
type SocialDemographics struct {
	Age          *int   `json:"age,omitempty"`
	Gender       string `json:"gender,omitempty"`
	Location     string `json:"location,omitempty"`
	Language     string `json:"language,omitempty"`
	Timezone     string `json:"timezone,omitempty"`
	Relationship string `json:"relationship,omitempty"`
	Education    string `json:"education,omitempty"`
	Work         string `json:"work,omitempty"`
}

// NewSocialMediaPlugin creates a new social media plugin instance
func NewSocialMediaPlugin(logger *logrus.Logger) *SocialMediaPlugin {
	if logger == nil {
		logger = logrus.New()
	}

	return &SocialMediaPlugin{
		name:      "social-media-plugin",
		version:   "1.0.0",
		platforms: make(map[string]*PlatformConfig),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger:       logger,
		rateLimiters: make(map[string]*RateLimiter),
	}
}

// Name returns the plugin name
func (s *SocialMediaPlugin) Name() string {
	return s.name
}

// Connect establishes connections to social media platforms
func (s *SocialMediaPlugin) Connect(config map[string]interface{}) error {
	// Parse configuration
	socialConfig, err := s.parseConfig(config)
	if err != nil {
		return fmt.Errorf("invalid social media configuration: %w", err)
	}

	// Set timeout
	if socialConfig.Timeout > 0 {
		s.client.Timeout = time.Duration(socialConfig.Timeout) * time.Second
	}

	// Configure platforms
	connectedPlatforms := 0
	for _, platformConfig := range socialConfig.Platforms {
		if !platformConfig.Enabled {
			continue
		}

		// Store platform config
		s.platforms[platformConfig.Name] = &platformConfig

		// Set up rate limiter
		rateLimit := platformConfig.RateLimit
		if rateLimit == 0 {
			rateLimit = 100 // Default rate limit
		}
		s.rateLimiters[platformConfig.Name] = NewRateLimiter(rateLimit, time.Minute)

		// Test platform connection
		if err := s.testPlatformConnection(&platformConfig); err != nil {
			s.logger.WithFields(logrus.Fields{
				"platform": platformConfig.Name,
				"error":    err,
			}).Warn("Failed to connect to social media platform")
			continue
		}

		connectedPlatforms++
		s.logger.WithField("platform", platformConfig.Name).Info("Connected to social media platform")
	}

	if connectedPlatforms == 0 {
		s.connected = false
		return fmt.Errorf("no social media platforms connected successfully")
	}

	s.connected = true
	s.lastError = nil

	s.logger.WithField("platforms", connectedPlatforms).Info("Social media plugin connected successfully")

	return nil
}

// EnrichUserProfile enriches user profile with social media data
func (s *SocialMediaPlugin) EnrichUserProfile(userID string) (*UserEnrichment, error) {
	if !s.connected {
		return nil, &PluginError{
			Plugin:    s.name,
			Operation: "enrich",
			Message:   "social media plugin not connected",
			Code:      ErrorCodeConnection,
			Retryable: false,
		}
	}

	// Collect data from all connected platforms
	platformData := make(map[string]*SocialUserData)

	for platformName, platformConfig := range s.platforms {
		if !platformConfig.Enabled {
			continue
		}

		// Check rate limit
		if rateLimiter, exists := s.rateLimiters[platformName]; exists && !rateLimiter.Allow() {
			s.logger.WithField("platform", platformName).Warn("Rate limit exceeded for platform")
			continue
		}

		// Fetch user data from platform
		data, err := s.fetchUserDataFromPlatform(platformConfig, userID)
		if err != nil {
			s.logger.WithFields(logrus.Fields{
				"platform": platformName,
				"error":    err,
			}).Warn("Failed to fetch user data from platform")
			continue
		}

		if data != nil {
			platformData[platformName] = data
		}
	}

	if len(platformData) == 0 {
		return nil, &PluginError{
			Plugin:    s.name,
			Operation: "enrich",
			Message:   "no data available from any platform",
			Code:      ErrorCodeNotFound,
			Retryable: true,
		}
	}

	// Convert to UserEnrichment
	enrichment := s.convertToEnrichment(platformData)

	s.logger.WithFields(logrus.Fields{
		"user_id":    userID,
		"platforms":  len(platformData),
		"confidence": enrichment.Confidence,
	}).Debug("Social media user profile enriched")

	return enrichment, nil
}

// IsHealthy checks if the social media connections are healthy
func (s *SocialMediaPlugin) IsHealthy() bool {
	if !s.connected {
		return false
	}

	// Check at least one platform is healthy
	healthyPlatforms := 0

	for _, platformConfig := range s.platforms {
		if !platformConfig.Enabled {
			continue
		}

		if err := s.testPlatformConnection(platformConfig); err == nil {
			healthyPlatforms++
		}
	}

	return healthyPlatforms > 0
}

// Cleanup performs cleanup when the plugin is shut down
func (s *SocialMediaPlugin) Cleanup() error {
	s.connected = false
	s.logger.Info("Social media plugin cleaned up")
	return nil
}

// GetMetadata returns plugin metadata
func (s *SocialMediaPlugin) GetMetadata() *PluginMetadata {
	return &PluginMetadata{
		Name:        s.name,
		Version:     s.version,
		Description: "Social media integration plugin for user profile enrichment",
		Author:      "Recommendation Engine Team",
		License:     "MIT",
		Capabilities: []string{
			"social_demographics",
			"interests_extraction",
			"social_connections",
			"engagement_metrics",
			"activity_patterns",
			"network_analysis",
		},
		Config: &ConfigSchema{
			Properties: map[string]*ConfigProperty{
				"platforms": {
					Type:        "array",
					Description: "List of social media platforms to connect",
					Required:    true,
				},
				"timeout": {
					Type:        "integer",
					Description: "Request timeout in seconds",
					Default:     30,
					Minimum:     &[]float64{1}[0],
					Maximum:     &[]float64{300}[0],
				},
			},
			Required: []string{"platforms"},
		},
		Tags: []string{"social-media", "user-data", "interests", "demographics"},
	}
}

// SocialMediaConfig represents the overall social media plugin configuration
type SocialMediaConfig struct {
	Platforms []PlatformConfig `json:"platforms"`
	Timeout   int              `json:"timeout"`
}

// Private methods

func (s *SocialMediaPlugin) parseConfig(config map[string]interface{}) (*SocialMediaConfig, error) {
	socialConfig := &SocialMediaConfig{}

	// Parse platforms
	if platformsData, ok := config["platforms"]; ok {
		platformsJSON, err := json.Marshal(platformsData)
		if err != nil {
			return nil, fmt.Errorf("invalid platforms configuration: %w", err)
		}

		if err := json.Unmarshal(platformsJSON, &socialConfig.Platforms); err != nil {
			return nil, fmt.Errorf("failed to parse platforms: %w", err)
		}
	} else {
		return nil, fmt.Errorf("platforms configuration is required")
	}

	// Parse timeout
	if timeout, ok := config["timeout"]; ok {
		if timeoutInt, ok := timeout.(int); ok {
			socialConfig.Timeout = timeoutInt
		} else if timeoutFloat, ok := timeout.(float64); ok {
			socialConfig.Timeout = int(timeoutFloat)
		}
	}

	return socialConfig, nil
}

func (s *SocialMediaPlugin) testPlatformConnection(config *PlatformConfig) error {
	endpoint := s.getHealthCheckEndpoint(config)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return err
	}

	s.addPlatformAuthHeaders(req, config)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("platform API returned status %d", resp.StatusCode)
	}

	return nil
}

func (s *SocialMediaPlugin) fetchUserDataFromPlatform(config *PlatformConfig, userID string) (*SocialUserData, error) {
	endpoint := s.getUserEndpoint(config, userID)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	s.addPlatformAuthHeaders(req, config)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, &PluginError{
			Plugin:    s.name,
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
			Plugin:    s.name,
			Operation: "fetch_user",
			Message:   "user not found on platform",
			Code:      ErrorCodeNotFound,
			Retryable: false,
		}
	}

	if resp.StatusCode >= 400 {
		return nil, &PluginError{
			Plugin:    s.name,
			Operation: "fetch_user",
			Message:   fmt.Sprintf("platform API error: %d", resp.StatusCode),
			Code:      ErrorCodeConnection,
			Retryable: resp.StatusCode >= 500,
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse response based on platform
	return s.parseUserResponse(config, body)
}

func (s *SocialMediaPlugin) convertToEnrichment(platformData map[string]*SocialUserData) *UserEnrichment {
	enrichment := &UserEnrichment{
		Source:    s.name,
		Timestamp: time.Now(),
		TTL:       2 * time.Hour, // Cache for 2 hours
	}

	// Aggregate interests from all platforms
	interestsMap := make(map[string]int)
	var allHashtags []string
	var totalFollowers, totalFollowing int
	var locations []string
	var demographics []*SocialDemographics

	for platform, data := range platformData {
		// Collect interests
		for _, interest := range data.Interests {
			interestsMap[interest]++
		}

		// Collect hashtags
		allHashtags = append(allHashtags, data.Hashtags...)

		// Aggregate social metrics
		totalFollowers += data.Followers
		totalFollowing += data.Following

		// Collect locations
		if data.Location != "" {
			locations = append(locations, data.Location)
		}

		// Collect demographics
		if data.Demographics != nil {
			demographics = append(demographics, data.Demographics)
		}

		s.logger.WithFields(logrus.Fields{
			"platform":  platform,
			"interests": len(data.Interests),
			"followers": data.Followers,
		}).Debug("Processed platform data")
	}

	// Convert interests map to slice
	interests := make([]string, 0, len(interestsMap))
	for interest := range interestsMap {
		interests = append(interests, interest)
	}

	enrichment.Interests = interests

	// Set demographics from most complete source
	if len(demographics) > 0 {
		enrichment.Demographics = s.mergeDemographics(demographics)
	}

	// Set social connections
	enrichment.SocialConnections = &SocialConnections{
		Platforms:   s.getPlatformNames(),
		Connections: totalFollowers + totalFollowing,
		Influence:   s.calculateInfluence(platformData),
		Communities: s.extractCommunities(platformData),
	}

	// Calculate confidence based on data richness
	confidence := s.calculateConfidence(platformData)
	enrichment.Confidence = confidence

	// Add contextual data
	enrichment.ContextualData = map[string]interface{}{
		"total_followers":    totalFollowers,
		"total_following":    totalFollowing,
		"platform_count":     len(platformData),
		"hashtags":           allHashtags,
		"locations":          locations,
		"engagement_metrics": s.aggregateEngagement(platformData),
	}

	return enrichment
}

func (s *SocialMediaPlugin) mergeDemographics(demographics []*SocialDemographics) *Demographics {
	merged := &Demographics{}

	// Use the most complete demographic data
	for _, demo := range demographics {
		if demo.Age != nil && merged.Age == nil {
			merged.Age = demo.Age
		}
		if demo.Gender != "" && merged.Gender == "" {
			merged.Gender = demo.Gender
		}
		if demo.Location != "" && merged.Location == "" {
			merged.Location = demo.Location
		}
		if demo.Education != "" && merged.Education == "" {
			merged.Education = demo.Education
		}
		if demo.Work != "" && merged.Occupation == "" {
			merged.Occupation = demo.Work
		}
	}

	return merged
}

func (s *SocialMediaPlugin) calculateInfluence(platformData map[string]*SocialUserData) float64 {
	var totalInfluence float64
	count := 0

	for _, data := range platformData {
		if data.Engagement != nil {
			// Simple influence calculation based on engagement rate and followers
			influence := float64(data.Followers) * data.Engagement.EngagementRate
			totalInfluence += influence
			count++
		}
	}

	if count == 0 {
		return 0.0
	}

	return totalInfluence / float64(count)
}

func (s *SocialMediaPlugin) extractCommunities(platformData map[string]*SocialUserData) []string {
	communitiesMap := make(map[string]bool)

	for _, data := range platformData {
		if data.Network != nil {
			for _, community := range data.Network.Communities {
				communitiesMap[community] = true
			}
		}
	}

	communities := make([]string, 0, len(communitiesMap))
	for community := range communitiesMap {
		communities = append(communities, community)
	}

	return communities
}

func (s *SocialMediaPlugin) calculateConfidence(platformData map[string]*SocialUserData) float64 {
	var totalConfidence float64
	count := 0

	for _, data := range platformData {
		confidence := 0.0

		// Base confidence from platform connection
		confidence += 0.2

		// Additional confidence from data completeness
		if len(data.Interests) > 0 {
			confidence += 0.2
		}
		if data.Demographics != nil {
			confidence += 0.2
		}
		if data.Engagement != nil {
			confidence += 0.2
		}
		if data.Network != nil {
			confidence += 0.2
		}

		totalConfidence += confidence
		count++
	}

	if count == 0 {
		return 0.0
	}

	return totalConfidence / float64(count)
}

func (s *SocialMediaPlugin) aggregateEngagement(platformData map[string]*SocialUserData) map[string]interface{} {
	engagement := make(map[string]interface{})

	var totalLikes, totalComments, totalShares int
	var totalEngagementRate float64
	count := 0

	for platform, data := range platformData {
		if data.Engagement != nil {
			totalLikes += data.Engagement.LikesReceived
			totalComments += data.Engagement.CommentsReceived
			totalShares += data.Engagement.SharesReceived
			totalEngagementRate += data.Engagement.EngagementRate
			count++

			engagement[platform+"_engagement"] = data.Engagement
		}
	}

	if count > 0 {
		engagement["total_likes"] = totalLikes
		engagement["total_comments"] = totalComments
		engagement["total_shares"] = totalShares
		engagement["average_engagement_rate"] = totalEngagementRate / float64(count)
	}

	return engagement
}

func (s *SocialMediaPlugin) getPlatformNames() []string {
	names := make([]string, 0, len(s.platforms))
	for name, config := range s.platforms {
		if config.Enabled {
			names = append(names, name)
		}
	}
	return names
}

func (s *SocialMediaPlugin) getHealthCheckEndpoint(config *PlatformConfig) string {
	switch strings.ToLower(config.Name) {
	case "facebook":
		return config.APIURL + "/me"
	case "twitter":
		return config.APIURL + "/2/users/me"
	case "instagram":
		return config.APIURL + "/me"
	case "linkedin":
		return config.APIURL + "/v2/people/~"
	case "tiktok":
		return config.APIURL + "/user/info/"
	default:
		return config.APIURL + "/health"
	}
}

func (s *SocialMediaPlugin) getUserEndpoint(config *PlatformConfig, userID string) string {
	switch strings.ToLower(config.Name) {
	case "facebook":
		return fmt.Sprintf("%s/%s?fields=id,name,location,interests", config.APIURL, userID)
	case "twitter":
		return fmt.Sprintf("%s/2/users/%s?user.fields=public_metrics,description,location", config.APIURL, userID)
	case "instagram":
		return fmt.Sprintf("%s/%s?fields=id,username,media_count,followers_count", config.APIURL, userID)
	case "linkedin":
		return fmt.Sprintf("%s/v2/people/id=%s", config.APIURL, userID)
	case "tiktok":
		return fmt.Sprintf("%s/user/info/?username=%s", config.APIURL, userID)
	default:
		return fmt.Sprintf("%s/users/%s", config.APIURL, userID)
	}
}

func (s *SocialMediaPlugin) addPlatformAuthHeaders(req *http.Request, config *PlatformConfig) {
	switch strings.ToLower(config.Name) {
	case "facebook", "instagram":
		req.Header.Set("Authorization", "Bearer "+config.AccessToken)
	case "twitter":
		req.Header.Set("Authorization", "Bearer "+config.AccessToken)
	case "linkedin":
		req.Header.Set("Authorization", "Bearer "+config.AccessToken)
	case "tiktok":
		req.Header.Set("Authorization", "Bearer "+config.AccessToken)
	default:
		req.Header.Set("Authorization", "Bearer "+config.AccessToken)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "RecommendationEngine-SocialPlugin/"+s.version)
}

func (s *SocialMediaPlugin) parseUserResponse(config *PlatformConfig, body []byte) (*SocialUserData, error) {
	var socialData SocialUserData

	// This would be implemented differently for each platform
	// For now, assume a generic JSON response
	if err := json.Unmarshal(body, &socialData); err != nil {
		return nil, fmt.Errorf("failed to parse social media response: %w", err)
	}

	socialData.Platform = config.Name

	return &socialData, nil
}

// NetworkData represents social network information
type NetworkData struct {
	Connections     []string               `json:"connections"`
	Communities     []string               `json:"communities"`
	Influencers     []string               `json:"influencers"`
	Brands          []string               `json:"brands"`
	NetworkSize     int                    `json:"network_size"`
	NetworkQuality  float64                `json:"network_quality"`
	ClusteringCoeff float64                `json:"clustering_coefficient"`
	Centrality      float64                `json:"centrality"`
	NetworkData     map[string]interface{} `json:"network_data"`
}

// ActivityPatterns represents user activity patterns
type ActivityPatterns struct {
	PostFrequency  float64   `json:"post_frequency"` // posts per day
	ActiveHours    []int     `json:"active_hours"`   // hours of day when most active
	ActiveDays     []string  `json:"active_days"`    // days of week when most active
	LastActive     time.Time `json:"last_active"`
	AverageSession int       `json:"average_session"` // minutes
	ContentTypes   []string  `json:"content_types"`   // photo, video, text, etc.
	PopularTopics  []string  `json:"popular_topics"`
}
