package services

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/temcen/pirex/pkg/models"
)

func TestDataPreprocessor_ProcessContent(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel) // Reduce noise in tests

	preprocessor := NewDataPreprocessor(logger)
	ctx := context.Background()
	jobID := uuid.New()

	tests := []struct {
		name            string
		content         models.ContentIngestionRequest
		expectError     bool
		expectedQuality float64
		checkTitle      string
		checkCategories []string
	}{
		{
			name: "valid product content",
			content: models.ContentIngestionRequest{
				Type:        "product",
				Title:       "High Quality Smartphone",
				Description: stringPtr("Latest smartphone with advanced features and excellent camera quality"),
				ImageURLs:   []string{"https://example.com/image1.jpg"},
				Categories:  []string{"Electronics", "smartphones", "tech"},
				Metadata: map[string]interface{}{
					"price":  999.99,
					"brand":  "TechCorp",
					"rating": 4.5,
				},
			},
			expectError:     false,
			expectedQuality: 0.8, // Should be high quality
			checkTitle:      "High Quality Smartphone",
			checkCategories: []string{"Electronics", "smartphones"},
		},
		{
			name: "content with HTML in title and description",
			content: models.ContentIngestionRequest{
				Type:        "article",
				Title:       "<h1>Breaking News</h1> &amp; Updates",
				Description: stringPtr("<p>This is an <strong>important</strong> article with HTML tags.</p>"),
				Categories:  []string{"News"},
			},
			expectError: false,
			checkTitle:  "Breaking News & Updates",
		},
		{
			name: "minimal valid content",
			content: models.ContentIngestionRequest{
				Type:  "video",
				Title: "Short Video",
			},
			expectError:     false,
			expectedQuality: 0.2, // Should be low quality due to minimal content
			checkTitle:      "Short Video",
		},
		{
			name: "content with invalid metadata",
			content: models.ContentIngestionRequest{
				Type:  "product",
				Title: "Test Product",
				Metadata: map[string]interface{}{
					"price":         999.99,
					"invalid_field": "should be filtered out",
				},
			},
			expectError: false,
			checkTitle:  "Test Product",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := preprocessor.ProcessContent(ctx, jobID, tt.content)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, result.ProcessedContent)

			// Check basic fields
			assert.Equal(t, tt.content.Type, result.ProcessedContent.Type)
			assert.Equal(t, tt.checkTitle, result.ProcessedContent.Title)
			assert.True(t, result.ProcessedContent.Active)
			assert.NotZero(t, result.ProcessedContent.CreatedAt)
			assert.NotZero(t, result.ProcessedContent.UpdatedAt)

			// Check quality score if specified
			if tt.expectedQuality > 0 {
				assert.InDelta(t, tt.expectedQuality, result.QualityScore, 0.2)
			}

			// Check categories if specified
			if len(tt.checkCategories) > 0 {
				assert.ElementsMatch(t, tt.checkCategories, result.ProcessedContent.Categories)
			}

			// Check processing hints
			assert.NotNil(t, result.ProcessingHints)
			assert.Contains(t, result.ProcessingHints, "keywords")
			assert.Contains(t, result.ProcessingHints, "language")
			assert.Contains(t, result.ProcessingHints, "content_features")
		})
	}
}

func TestDataPreprocessor_CleanText(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	preprocessor := NewDataPreprocessor(logger)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "HTML tags removal",
			input:    "<h1>Title</h1><p>Content with <strong>bold</strong> text</p>",
			expected: "Title Content with bold text",
		},
		{
			name:     "HTML entities",
			input:    "Price: $100 &amp; free shipping",
			expected: "Price: $100 & free shipping",
		},
		{
			name:     "Special characters",
			input:    "Product™ with ® symbol and © copyright",
			expected: "Product with symbol and copyright",
		},
		{
			name:     "Multiple whitespace",
			input:    "Text   with    multiple     spaces",
			expected: "Text with multiple spaces",
		},
		{
			name:     "Empty after cleaning",
			input:    "   ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := preprocessor.cleanText(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDataPreprocessor_ExtractKeywords(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	preprocessor := NewDataPreprocessor(logger)

	title := "High Quality Smartphone with Advanced Camera"
	description := stringPtr("This smartphone features advanced camera technology and high quality display")

	keywords := preprocessor.extractKeywords(title, description)

	assert.NotEmpty(t, keywords)
	assert.Contains(t, keywords, "smartphone")
	assert.Contains(t, keywords, "camera")
	assert.Contains(t, keywords, "quality")
	assert.Contains(t, keywords, "advanced")

	// Should not contain stop words
	assert.NotContains(t, keywords, "with")
	assert.NotContains(t, keywords, "and")
	assert.NotContains(t, keywords, "this")
}

func TestDataPreprocessor_DetectLanguage(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	preprocessor := NewDataPreprocessor(logger)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "English text",
			input:    "This is an English text with normal characters",
			expected: "en",
		},
		{
			name:     "Empty text",
			input:    "",
			expected: "unknown",
		},
		{
			name:     "Numbers only",
			input:    "12345",
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := preprocessor.detectLanguage(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDataPreprocessor_ExtractEntities(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	preprocessor := NewDataPreprocessor(logger)

	title := "Contact us at support@example.com or visit https://example.com"
	description := stringPtr("Price: $99.99 for premium package")

	entities := preprocessor.extractEntities(title, description)

	assert.Contains(t, entities, "emails")
	assert.Contains(t, entities["emails"], "support@example.com")

	assert.Contains(t, entities, "urls")
	assert.Contains(t, entities["urls"], "https://example.com")

	assert.Contains(t, entities, "numbers")
	assert.Contains(t, entities["numbers"], "$99.99")
}

func TestDataPreprocessor_NormalizeCategoryName(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	preprocessor := NewDataPreprocessor(logger)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "exact match",
			input:    "Electronics",
			expected: "Electronics",
		},
		{
			name:     "case insensitive",
			input:    "electronics",
			expected: "Electronics",
		},
		{
			name:     "alias match",
			input:    "tech",
			expected: "Electronics",
		},
		{
			name:     "unknown category",
			input:    "unknown_category",
			expected: "unknown_category",
		},
		{
			name:     "empty category",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := preprocessor.normalizeCategoryName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDataPreprocessor_CalculateQualityScore(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	preprocessor := NewDataPreprocessor(logger)

	tests := []struct {
		name     string
		content  *models.ContentItem
		expected float64
	}{
		{
			name: "high quality content",
			content: &models.ContentItem{
				Title:       "High Quality Product with Great Features",
				Description: stringPtr("This is a comprehensive description with sufficient detail about the product features and benefits"),
				ImageURLs:   []string{"image1.jpg", "image2.jpg", "image3.jpg"},
				Categories:  []string{"Electronics", "Gadgets"},
				Metadata: map[string]interface{}{
					"price":  99.99,
					"brand":  "TestBrand",
					"rating": 4.5,
				},
			},
			expected: 1.0, // Perfect score
		},
		{
			name: "minimal content",
			content: &models.ContentItem{
				Title: "Test",
			},
			expected: 0.1, // Very low score
		},
		{
			name: "medium quality content",
			content: &models.ContentItem{
				Title:       "Medium Quality Product",
				Description: stringPtr("Basic description"),
				ImageURLs:   []string{"image1.jpg"},
				Categories:  []string{"Electronics"},
			},
			expected: 0.6, // Medium score
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := preprocessor.calculateQualityScore(tt.content)
			assert.InDelta(t, tt.expected, result, 0.1)
		})
	}
}

func TestDataPreprocessor_ValidateMetadata(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)
	preprocessor := NewDataPreprocessor(logger)

	tests := []struct {
		name          string
		contentType   string
		metadata      map[string]interface{}
		expectValid   []string
		expectInvalid []string
	}{
		{
			name:        "product metadata",
			contentType: "product",
			metadata: map[string]interface{}{
				"price":         99.99,
				"brand":         "TestBrand",
				"invalid_field": "should be filtered",
			},
			expectValid:   []string{"price", "brand"},
			expectInvalid: []string{"invalid_field"},
		},
		{
			name:        "video metadata",
			contentType: "video",
			metadata: map[string]interface{}{
				"duration":      "10:30",
				"views":         1000,
				"invalid_field": "should be filtered",
			},
			expectValid:   []string{"duration", "views"},
			expectInvalid: []string{"invalid_field"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := models.ContentIngestionRequest{
				Type:     tt.contentType,
				Title:    "Test",
				Metadata: tt.metadata,
			}

			result := &ProcessingResult{
				ProcessedContent: &models.ContentItem{},
			}

			err := preprocessor.validateMetadata(content, result)
			assert.NoError(t, err)

			for _, validKey := range tt.expectValid {
				assert.Contains(t, result.ProcessedContent.Metadata, validKey)
			}

			for _, invalidKey := range tt.expectInvalid {
				assert.NotContains(t, result.ProcessedContent.Metadata, invalidKey)
			}
		})
	}
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}
