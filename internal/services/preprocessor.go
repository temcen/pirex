package services

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/unicode/norm"

	"github.com/temcen/pirex/pkg/models"
)

type DataPreprocessor struct {
	logger           *logrus.Logger
	httpClient       *http.Client
	categoryTaxonomy map[string][]string
	stopWords        map[string]bool
}

type ProcessingResult struct {
	ProcessedContent *models.ContentItem
	QualityScore     float64
	ProcessingHints  map[string]interface{}
	Errors           []string
}

type ImageMetadata struct {
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Format      string `json:"format"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
	Valid       bool   `json:"valid"`
}

func NewDataPreprocessor(logger *logrus.Logger) *DataPreprocessor {
	return &DataPreprocessor{
		logger: logger,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		categoryTaxonomy: initializeCategoryTaxonomy(),
		stopWords:        initializeStopWords(),
	}
}

func (dp *DataPreprocessor) ProcessContent(ctx context.Context, jobID uuid.UUID, content models.ContentIngestionRequest) (*ProcessingResult, error) {
	dp.logger.WithFields(logrus.Fields{
		"job_id":       jobID,
		"content_type": content.Type,
		"title":        content.Title,
	}).Info("Starting content preprocessing")

	result := &ProcessingResult{
		ProcessedContent: &models.ContentItem{
			ID:        uuid.New(),
			Type:      content.Type,
			Active:    true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		ProcessingHints: make(map[string]interface{}),
		Errors:          []string{},
	}

	// Text processing
	if err := dp.processText(content, result); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("text processing error: %v", err))
	}

	// Image processing
	if err := dp.processImages(ctx, content, result); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("image processing error: %v", err))
	}

	// Feature extraction
	if err := dp.extractFeatures(content, result); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("feature extraction error: %v", err))
	}

	// Category normalization
	if err := dp.normalizeCategories(content, result); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("category normalization error: %v", err))
	}

	// Metadata validation
	if err := dp.validateMetadata(content, result); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("metadata validation error: %v", err))
	}

	// Quality scoring
	result.QualityScore = dp.calculateQualityScore(result.ProcessedContent)
	result.ProcessedContent.QualityScore = result.QualityScore

	dp.logger.WithFields(logrus.Fields{
		"job_id":        jobID,
		"quality_score": result.QualityScore,
		"errors_count":  len(result.Errors),
	}).Info("Content preprocessing completed")

	return result, nil
}

func (dp *DataPreprocessor) processText(content models.ContentIngestionRequest, result *ProcessingResult) error {
	// Clean and normalize title
	cleanTitle := dp.cleanText(content.Title)
	if cleanTitle == "" {
		return fmt.Errorf("title cannot be empty after cleaning")
	}
	result.ProcessedContent.Title = cleanTitle

	// Clean and normalize description
	if content.Description != nil && *content.Description != "" {
		cleanDesc := dp.cleanText(*content.Description)
		result.ProcessedContent.Description = &cleanDesc
	}

	// Extract keywords using TF-IDF (simplified)
	keywords := dp.extractKeywords(cleanTitle, content.Description)
	result.ProcessingHints["keywords"] = keywords

	// Detect language (simplified)
	language := dp.detectLanguage(cleanTitle)
	result.ProcessingHints["language"] = language

	// Extract entities using regex patterns
	entities := dp.extractEntities(cleanTitle, content.Description)
	result.ProcessingHints["entities"] = entities

	return nil
}

func (dp *DataPreprocessor) processImages(ctx context.Context, content models.ContentIngestionRequest, result *ProcessingResult) error {
	if len(content.ImageURLs) == 0 {
		result.ProcessedContent.ImageURLs = []string{}
		return nil
	}

	validImages := []string{}
	imageMetadata := []ImageMetadata{}

	for _, imageURL := range content.ImageURLs {
		metadata, err := dp.validateImageURL(ctx, imageURL)
		if err != nil {
			dp.logger.WithError(err).WithField("image_url", imageURL).Warn("Image validation failed")
			continue
		}

		if metadata.Valid {
			validImages = append(validImages, imageURL)
		}
		imageMetadata = append(imageMetadata, *metadata)
	}

	result.ProcessedContent.ImageURLs = validImages
	result.ProcessingHints["image_metadata"] = imageMetadata

	return nil
}

func (dp *DataPreprocessor) extractFeatures(content models.ContentIngestionRequest, result *ProcessingResult) error {
	// Generate content fingerprint
	fingerprint := dp.generateFingerprint(content.Title, content.Description)
	result.ProcessingHints["fingerprint"] = fingerprint

	// Extract content features
	features := map[string]interface{}{
		"title_length":    len(content.Title),
		"has_description": content.Description != nil && *content.Description != "",
		"image_count":     len(content.ImageURLs),
		"category_count":  len(content.Categories),
		"metadata_keys":   len(content.Metadata),
	}

	if content.Description != nil {
		features["description_length"] = len(*content.Description)
	}

	result.ProcessingHints["content_features"] = features

	return nil
}

func (dp *DataPreprocessor) normalizeCategories(content models.ContentIngestionRequest, result *ProcessingResult) error {
	normalizedCategories := []string{}

	for _, category := range content.Categories {
		normalized := dp.normalizeCategoryName(category)
		if normalized != "" {
			normalizedCategories = append(normalizedCategories, normalized)
		}
	}

	// Remove duplicates
	categorySet := make(map[string]bool)
	uniqueCategories := []string{}
	for _, cat := range normalizedCategories {
		if !categorySet[cat] {
			categorySet[cat] = true
			uniqueCategories = append(uniqueCategories, cat)
		}
	}

	result.ProcessedContent.Categories = uniqueCategories
	return nil
}

func (dp *DataPreprocessor) validateMetadata(content models.ContentIngestionRequest, result *ProcessingResult) error {
	validatedMetadata := make(map[string]interface{})

	for key, value := range content.Metadata {
		// Validate metadata based on content type
		if dp.isValidMetadataField(content.Type, key, value) {
			validatedMetadata[key] = value
		} else {
			dp.logger.WithFields(logrus.Fields{
				"content_type": content.Type,
				"key":          key,
				"value":        value,
			}).Warn("Invalid metadata field")
		}
	}

	result.ProcessedContent.Metadata = validatedMetadata
	return nil
}

func (dp *DataPreprocessor) calculateQualityScore(content *models.ContentItem) float64 {
	score := 0.0
	maxScore := 10.0

	// Title quality (2 points)
	if len(content.Title) >= 10 && len(content.Title) <= 100 {
		score += 2.0
	} else if len(content.Title) >= 5 {
		score += 1.0
	}

	// Description quality (3 points)
	if content.Description != nil && *content.Description != "" {
		descLen := len(*content.Description)
		if descLen >= 50 && descLen <= 500 {
			score += 3.0
		} else if descLen >= 20 {
			score += 2.0
		} else if descLen >= 10 {
			score += 1.0
		}
	}

	// Image availability (2 points)
	imageCount := len(content.ImageURLs)
	if imageCount >= 3 {
		score += 2.0
	} else if imageCount >= 1 {
		score += 1.0
	}

	// Categories (2 points)
	categoryCount := len(content.Categories)
	if categoryCount >= 2 && categoryCount <= 5 {
		score += 2.0
	} else if categoryCount >= 1 {
		score += 1.0
	}

	// Metadata completeness (1 point)
	if len(content.Metadata) >= 3 {
		score += 1.0
	} else if len(content.Metadata) >= 1 {
		score += 0.5
	}

	return score / maxScore
}

// Helper functions

func (dp *DataPreprocessor) cleanText(text string) string {
	// Remove HTML tags and replace with space
	htmlTagRegex := regexp.MustCompile(`<[^>]*>`)
	cleaned := htmlTagRegex.ReplaceAllString(text, " ")

	// Unescape HTML entities
	cleaned = html.UnescapeString(cleaned)

	// Normalize Unicode
	cleaned = norm.NFC.String(cleaned)

	// Remove special characters (keep alphanumeric, spaces, and basic punctuation)
	specialCharRegex := regexp.MustCompile(`[^\p{L}\p{N}\s\-.,!?()'"$:&]+`)
	cleaned = specialCharRegex.ReplaceAllString(cleaned, " ")

	// Normalize whitespace
	whitespaceRegex := regexp.MustCompile(`\s+`)
	cleaned = whitespaceRegex.ReplaceAllString(cleaned, " ")

	return strings.TrimSpace(cleaned)
}

func (dp *DataPreprocessor) extractKeywords(title string, description *string) []string {
	text := title
	if description != nil {
		text += " " + *description
	}

	// Simple keyword extraction (TF-IDF would be more sophisticated)
	words := strings.Fields(strings.ToLower(text))
	wordCount := make(map[string]int)

	for _, word := range words {
		// Remove punctuation
		word = regexp.MustCompile(`[^\p{L}\p{N}]`).ReplaceAllString(word, "")
		if len(word) >= 3 && !dp.stopWords[word] {
			wordCount[word]++
		}
	}

	// Get top keywords
	keywords := []string{}
	for word, count := range wordCount {
		if count >= 2 || len(keywords) < 10 {
			keywords = append(keywords, word)
		}
		if len(keywords) >= 10 {
			break
		}
	}

	return keywords
}

func (dp *DataPreprocessor) detectLanguage(text string) string {
	// Simplified language detection
	// In a real implementation, you'd use a proper language detection library

	// Count character frequencies for basic detection
	latinCount := 0
	totalCount := 0

	for _, r := range text {
		if unicode.IsLetter(r) {
			totalCount++
			if r <= 255 {
				latinCount++
			}
		}
	}

	if totalCount == 0 {
		return "unknown"
	}

	latinRatio := float64(latinCount) / float64(totalCount)
	if latinRatio > 0.8 {
		return "en" // Assume English for Latin script
	}

	return "unknown"
}

func (dp *DataPreprocessor) extractEntities(title string, description *string) map[string][]string {
	text := title
	if description != nil {
		text += " " + *description
	}

	entities := make(map[string][]string)

	// Extract email addresses
	emailRegex := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	emails := emailRegex.FindAllString(text, -1)
	if len(emails) > 0 {
		entities["emails"] = emails
	}

	// Extract URLs
	urlRegex := regexp.MustCompile(`https?://[^\s]+`)
	urls := urlRegex.FindAllString(text, -1)
	if len(urls) > 0 {
		entities["urls"] = urls
	}

	// Extract numbers (prices, quantities, etc.)
	numberRegex := regexp.MustCompile(`\$?\d+(?:\.\d{2})?`)
	numbers := numberRegex.FindAllString(text, -1)
	if len(numbers) > 0 {
		entities["numbers"] = numbers
	}

	return entities
}

func (dp *DataPreprocessor) validateImageURL(ctx context.Context, imageURL string) (*ImageMetadata, error) {
	metadata := &ImageMetadata{
		Valid: false,
	}

	req, err := http.NewRequestWithContext(ctx, "HEAD", imageURL, nil)
	if err != nil {
		return metadata, fmt.Errorf("invalid URL: %w", err)
	}

	resp, err := dp.httpClient.Do(req)
	if err != nil {
		return metadata, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return metadata, fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		return metadata, fmt.Errorf("not an image: %s", contentType)
	}

	// Check content length
	contentLengthStr := resp.Header.Get("Content-Length")
	if contentLengthStr != "" {
		contentLength, err := strconv.ParseInt(contentLengthStr, 10, 64)
		if err == nil {
			if contentLength > 10*1024*1024 { // 10MB limit
				return metadata, fmt.Errorf("image too large: %d bytes", contentLength)
			}
			metadata.Size = contentLength
		}
	}

	metadata.ContentType = contentType
	metadata.Format = strings.TrimPrefix(contentType, "image/")
	metadata.Valid = true

	return metadata, nil
}

func (dp *DataPreprocessor) generateFingerprint(title string, description *string) string {
	text := title
	if description != nil {
		text += *description
	}

	// Simple hash-based fingerprint
	// In production, you might use a more sophisticated approach
	hash := 0
	for _, char := range text {
		hash = hash*31 + int(char)
	}

	return fmt.Sprintf("%x", hash)
}

func (dp *DataPreprocessor) normalizeCategoryName(category string) string {
	// Convert to lowercase and trim
	normalized := strings.ToLower(strings.TrimSpace(category))

	// Check against taxonomy
	for standardCategory, aliases := range dp.categoryTaxonomy {
		if normalized == strings.ToLower(standardCategory) {
			return standardCategory
		}
		for _, alias := range aliases {
			if normalized == strings.ToLower(alias) {
				return standardCategory
			}
		}
	}

	// If not in taxonomy, return cleaned version
	if len(normalized) > 0 {
		return normalized
	}

	return ""
}

func (dp *DataPreprocessor) isValidMetadataField(contentType, key string, value interface{}) bool {
	// Type-specific validation rules
	switch contentType {
	case "product":
		return dp.isValidProductMetadata(key, value)
	case "video":
		return dp.isValidVideoMetadata(key, value)
	case "article":
		return dp.isValidArticleMetadata(key, value)
	default:
		return true // Allow all for unknown types
	}
}

func (dp *DataPreprocessor) isValidProductMetadata(key string, value interface{}) bool {
	validKeys := map[string]bool{
		"price":        true,
		"currency":     true,
		"brand":        true,
		"model":        true,
		"sku":          true,
		"availability": true,
		"rating":       true,
		"reviews":      true,
	}
	return validKeys[key]
}

func (dp *DataPreprocessor) isValidVideoMetadata(key string, value interface{}) bool {
	validKeys := map[string]bool{
		"duration":   true,
		"resolution": true,
		"format":     true,
		"views":      true,
		"likes":      true,
		"channel":    true,
		"published":  true,
	}
	return validKeys[key]
}

func (dp *DataPreprocessor) isValidArticleMetadata(key string, value interface{}) bool {
	validKeys := map[string]bool{
		"author":       true,
		"published":    true,
		"word_count":   true,
		"reading_time": true,
		"tags":         true,
		"source":       true,
	}
	return validKeys[key]
}

func initializeCategoryTaxonomy() map[string][]string {
	return map[string][]string{
		"Electronics":   {"electronic", "tech", "technology", "gadget", "gadgets"},
		"Clothing":      {"clothes", "apparel", "fashion", "wear"},
		"Books":         {"book", "literature", "reading"},
		"Home":          {"house", "household", "domestic"},
		"Sports":        {"sport", "fitness", "exercise", "athletic"},
		"Food":          {"foods", "cuisine", "cooking", "recipe"},
		"Travel":        {"tourism", "vacation", "trip", "journey"},
		"Health":        {"medical", "wellness", "healthcare"},
		"Education":     {"learning", "academic", "school", "university"},
		"Entertainment": {"fun", "games", "movies", "music"},
	}
}

func initializeStopWords() map[string]bool {
	stopWords := []string{
		"a", "an", "and", "are", "as", "at", "be", "by", "for", "from",
		"has", "he", "in", "is", "it", "its", "of", "on", "that", "the",
		"to", "was", "will", "with", "the", "this", "but", "they", "have",
		"had", "what", "said", "each", "which", "she", "do", "how", "their",
		"if", "up", "out", "many", "then", "them", "these", "so", "some",
	}

	stopWordMap := make(map[string]bool)
	for _, word := range stopWords {
		stopWordMap[word] = true
	}
	return stopWordMap
}
