package ml

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

// ImageEmbeddingService handles image embedding generation
type ImageEmbeddingService struct {
	registry    *ModelRegistry
	redisClient *redis.Client
	logger      *logrus.Logger

	// Configuration
	targetWidth  int
	targetHeight int
	maxFileSize  int64
	timeout      time.Duration
	cachePrefix  string
	cacheTTL     time.Duration

	// HTTP client for fetching images
	httpClient *http.Client
}

// ImageEmbeddingConfig contains configuration for the image embedding service
type ImageEmbeddingConfig struct {
	TargetWidth  int           `json:"target_width"`
	TargetHeight int           `json:"target_height"`
	MaxFileSize  int64         `json:"max_file_size"`
	Timeout      time.Duration `json:"timeout"`
	CachePrefix  string        `json:"cache_prefix"`
	CacheTTL     time.Duration `json:"cache_ttl"`
}

// ImageMetadata contains metadata about a processed image
type ImageMetadata struct {
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Format      string `json:"format"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
}

// NewImageEmbeddingService creates a new image embedding service
func NewImageEmbeddingService(registry *ModelRegistry, redisClient *redis.Client, logger *logrus.Logger, config ImageEmbeddingConfig) *ImageEmbeddingService {
	if config.TargetWidth == 0 {
		config.TargetWidth = 224
	}
	if config.TargetHeight == 0 {
		config.TargetHeight = 224
	}
	if config.MaxFileSize == 0 {
		config.MaxFileSize = 10 * 1024 * 1024 // 10MB
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.CachePrefix == "" {
		config.CachePrefix = "embed:image"
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = 24 * time.Hour
	}

	return &ImageEmbeddingService{
		registry:     registry,
		redisClient:  redisClient,
		logger:       logger,
		targetWidth:  config.TargetWidth,
		targetHeight: config.TargetHeight,
		maxFileSize:  config.MaxFileSize,
		timeout:      config.Timeout,
		cachePrefix:  config.CachePrefix,
		cacheTTL:     config.CacheTTL,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// GenerateEmbeddingFromURL generates an embedding from an image URL
func (ies *ImageEmbeddingService) GenerateEmbeddingFromURL(imageURL string, modelName string) ([]float32, *ImageMetadata, error) {
	if imageURL == "" {
		return nil, nil, fmt.Errorf("image URL cannot be empty")
	}

	// Check cache first
	if embedding, metadata, found := ies.getCachedEmbedding(imageURL, modelName); found {
		return embedding, metadata, nil
	}

	// Fetch image
	imageData, metadata, err := ies.fetchImage(imageURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch image: %w", err)
	}

	// Generate embedding
	embedding, err := ies.generateEmbedding(imageData, modelName)
	if err != nil {
		return nil, metadata, fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Cache the result
	ies.cacheEmbedding(imageURL, modelName, embedding, metadata)

	return embedding, metadata, nil
}

// GenerateEmbeddingFromData generates an embedding from image data
func (ies *ImageEmbeddingService) GenerateEmbeddingFromData(imageData []byte, modelName string) ([]float32, *ImageMetadata, error) {
	if len(imageData) == 0 {
		return nil, nil, fmt.Errorf("image data cannot be empty")
	}

	// Validate image data and extract metadata
	metadata, err := ies.validateAndExtractMetadata(imageData)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid image data: %w", err)
	}

	// Generate embedding
	embedding, err := ies.generateEmbedding(imageData, modelName)
	if err != nil {
		return nil, metadata, fmt.Errorf("failed to generate embedding: %w", err)
	}

	return embedding, metadata, nil
}

// fetchImage downloads an image from a URL with validation
func (ies *ImageEmbeddingService) fetchImage(imageURL string) ([]byte, *ImageMetadata, error) {
	// Validate URL
	if !strings.HasPrefix(imageURL, "http://") && !strings.HasPrefix(imageURL, "https://") {
		return nil, nil, fmt.Errorf("invalid image URL scheme")
	}

	// Create request
	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("User-Agent", "RecommendationEngine/1.0")
	req.Header.Set("Accept", "image/jpeg,image/png,image/webp,image/*")

	// Make request
	resp, err := ies.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch image: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if !ies.isValidImageContentType(contentType) {
		return nil, nil, fmt.Errorf("invalid content type: %s", contentType)
	}

	// Check content length
	if resp.ContentLength > ies.maxFileSize {
		return nil, nil, fmt.Errorf("image too large: %d bytes", resp.ContentLength)
	}

	// Read image data with size limit
	limitedReader := io.LimitReader(resp.Body, ies.maxFileSize)
	imageData, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read image data: %w", err)
	}

	// Validate and extract metadata
	metadata, err := ies.validateAndExtractMetadata(imageData)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid image data: %w", err)
	}

	metadata.ContentType = contentType
	metadata.Size = int64(len(imageData))

	return imageData, metadata, nil
}

// validateAndExtractMetadata validates image data and extracts metadata
func (ies *ImageEmbeddingService) validateAndExtractMetadata(imageData []byte) (*ImageMetadata, error) {
	// Decode image to get metadata
	img, format, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	metadata := &ImageMetadata{
		Width:  bounds.Dx(),
		Height: bounds.Dy(),
		Format: format,
		Size:   int64(len(imageData)),
	}

	return metadata, nil
}

// isValidImageContentType checks if the content type is a valid image type
func (ies *ImageEmbeddingService) isValidImageContentType(contentType string) bool {
	validTypes := map[string]bool{
		"image/jpeg": true,
		"image/jpg":  true,
		"image/png":  true,
		"image/webp": true,
		"image/gif":  true,
	}

	return validTypes[strings.ToLower(contentType)]
}

// preprocessImage resizes and normalizes the image for model input
func (ies *ImageEmbeddingService) preprocessImage(imageData []byte) ([]float32, error) {
	// Decode image
	img, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Resize image to target dimensions using bilinear interpolation
	resizedImg := ies.resizeImage(img, ies.targetWidth, ies.targetHeight)

	// Convert to tensor format with CLIP-style normalization
	tensorSize := ies.targetWidth * ies.targetHeight * 3 // RGB channels
	tensor := make([]float32, tensorSize)

	// CLIP normalization values
	mean := []float32{0.48145466, 0.4578275, 0.40821073}
	std := []float32{0.26862954, 0.26130258, 0.27577711}

	// Fill tensor with normalized pixel values
	for y := 0; y < ies.targetHeight; y++ {
		for x := 0; x < ies.targetWidth; x++ {
			r, g, b, _ := resizedImg.At(x, y).RGBA()

			// Convert from uint32 to [0, 1] range
			rNorm := float32(r) / 65535.0
			gNorm := float32(g) / 65535.0
			bNorm := float32(b) / 65535.0

			// Apply CLIP normalization: (pixel - mean) / std
			idx := (y*ies.targetWidth + x) * 3
			tensor[idx] = (rNorm - mean[0]) / std[0]   // R
			tensor[idx+1] = (gNorm - mean[1]) / std[1] // G
			tensor[idx+2] = (bNorm - mean[2]) / std[2] // B
		}
	}

	return tensor, nil
}

// resizeImage resizes an image using bilinear interpolation
func (ies *ImageEmbeddingService) resizeImage(src image.Image, width, height int) image.Image {
	srcBounds := src.Bounds()
	srcW, srcH := srcBounds.Dx(), srcBounds.Dy()

	// Create new image
	dst := image.NewRGBA(image.Rect(0, 0, width, height))

	// Calculate scaling factors
	xScale := float64(srcW) / float64(width)
	yScale := float64(srcH) / float64(height)

	// Bilinear interpolation
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Map destination coordinates to source coordinates
			srcX := float64(x) * xScale
			srcY := float64(y) * yScale

			// Get the four surrounding pixels
			x1, y1 := int(srcX), int(srcY)
			x2, y2 := x1+1, y1+1

			// Clamp coordinates
			if x2 >= srcW {
				x2 = srcW - 1
			}
			if y2 >= srcH {
				y2 = srcH - 1
			}

			// Get colors of the four pixels
			c1 := color.RGBAModel.Convert(src.At(x1+srcBounds.Min.X, y1+srcBounds.Min.Y)).(color.RGBA)
			c2 := color.RGBAModel.Convert(src.At(x2+srcBounds.Min.X, y1+srcBounds.Min.Y)).(color.RGBA)
			c3 := color.RGBAModel.Convert(src.At(x1+srcBounds.Min.X, y2+srcBounds.Min.Y)).(color.RGBA)
			c4 := color.RGBAModel.Convert(src.At(x2+srcBounds.Min.X, y2+srcBounds.Min.Y)).(color.RGBA)

			// Calculate interpolation weights
			wx := srcX - float64(x1)
			wy := srcY - float64(y1)

			// Interpolate
			r := ies.interpolate(float64(c1.R), float64(c2.R), float64(c3.R), float64(c4.R), wx, wy)
			g := ies.interpolate(float64(c1.G), float64(c2.G), float64(c3.G), float64(c4.G), wx, wy)
			b := ies.interpolate(float64(c1.B), float64(c2.B), float64(c3.B), float64(c4.B), wx, wy)

			dst.Set(x, y, color.RGBA{
				R: uint8(math.Max(0, math.Min(255, r))),
				G: uint8(math.Max(0, math.Min(255, g))),
				B: uint8(math.Max(0, math.Min(255, b))),
				A: 255,
			})
		}
	}

	return dst
}

// interpolate performs bilinear interpolation
func (ies *ImageEmbeddingService) interpolate(c1, c2, c3, c4, wx, wy float64) float64 {
	// Bilinear interpolation formula
	top := c1*(1-wx) + c2*wx
	bottom := c3*(1-wx) + c4*wx
	return top*(1-wy) + bottom*wy
}

// generateEmbedding performs the actual embedding generation (placeholder implementation)
func (ies *ImageEmbeddingService) generateEmbedding(imageData []byte, modelName string) ([]float32, error) {
	// Load model
	session, err := ies.registry.LoadModel(modelName)
	if err != nil {
		return nil, fmt.Errorf("failed to load model %s: %w", modelName, err)
	}

	// Preprocess image
	preprocessed, err := ies.preprocessImage(imageData)
	if err != nil {
		return nil, fmt.Errorf("failed to preprocess image: %w", err)
	}

	// TODO: Replace with actual ONNX inference when integrated
	// For now, generate a mock embedding based on preprocessed data
	embedding := ies.generateMockEmbedding(preprocessed, session.Info.Dimensions)

	return embedding, nil
}

// generateMockEmbedding creates a mock embedding for testing (placeholder)
func (ies *ImageEmbeddingService) generateMockEmbedding(preprocessed []float32, dimensions int) []float32 {
	// Generate deterministic embedding based on image data
	hasher := sha256.New()

	// Hash the preprocessed image data
	for _, pixel := range preprocessed {
		fmt.Fprintf(hasher, "%.6f", pixel)
	}
	hash := hasher.Sum(nil)

	embedding := make([]float32, dimensions)
	for i := 0; i < dimensions; i++ {
		// Use hash bytes to generate pseudo-random values
		byteIndex := i % len(hash)
		embedding[i] = float32(hash[byteIndex])/255.0 - 0.5 // Range [-0.5, 0.5]
	}

	return embedding
}

// CachedImageEmbedding represents a cached image embedding with metadata
type CachedImageEmbedding struct {
	Embedding []float32      `json:"embedding"`
	Metadata  *ImageMetadata `json:"metadata"`
}

// getCachedEmbedding retrieves an embedding from cache
func (ies *ImageEmbeddingService) getCachedEmbedding(imageURL string, modelName string) ([]float32, *ImageMetadata, bool) {
	key := ies.generateCacheKey(imageURL, modelName)

	ctx := context.Background()
	result, err := ies.redisClient.Get(ctx, key).Result()
	if err != nil {
		return nil, nil, false
	}

	// Deserialize cached embedding and metadata from JSON
	var cached CachedImageEmbedding
	if err := json.Unmarshal([]byte(result), &cached); err != nil {
		ies.logger.WithFields(logrus.Fields{
			"error": err.Error(),
			"key":   key,
		}).Warn("Failed to deserialize cached image embedding")
		return nil, nil, false
	}

	return cached.Embedding, cached.Metadata, true
}

// cacheEmbedding stores an embedding in cache
func (ies *ImageEmbeddingService) cacheEmbedding(imageURL string, modelName string, embedding []float32, metadata *ImageMetadata) {
	key := ies.generateCacheKey(imageURL, modelName)

	// Serialize embedding and metadata to JSON
	cached := CachedImageEmbedding{
		Embedding: embedding,
		Metadata:  metadata,
	}

	data, err := json.Marshal(cached)
	if err != nil {
		ies.logger.WithFields(logrus.Fields{
			"error": err.Error(),
			"key":   key,
		}).Warn("Failed to serialize image embedding for caching")
		return
	}

	ctx := context.Background()
	if err := ies.redisClient.Set(ctx, key, data, ies.cacheTTL).Err(); err != nil {
		ies.logger.WithFields(logrus.Fields{
			"error": err.Error(),
			"key":   key,
		}).Warn("Failed to cache image embedding")
	}
}

// generateCacheKey creates a content-based cache key
func (ies *ImageEmbeddingService) generateCacheKey(imageURL string, modelName string) string {
	// Get model info for version
	modelInfo, err := ies.registry.GetModelInfo(modelName)
	if err != nil {
		modelInfo = &ModelInfo{Version: "unknown"}
	}

	// Generate content hash from URL
	hasher := sha256.New()
	hasher.Write([]byte(imageURL))
	contentHash := fmt.Sprintf("%x", hasher.Sum(nil))[:16]

	return fmt.Sprintf("%s:%s:%s:%s", ies.cachePrefix, modelName, modelInfo.Version, contentHash)
}

// GetStats returns service statistics
func (ies *ImageEmbeddingService) GetStats() map[string]any {
	return map[string]interface{}{
		"target_width":  ies.targetWidth,
		"target_height": ies.targetHeight,
		"max_file_size": ies.maxFileSize,
		"timeout":       ies.timeout.String(),
		"cache_prefix":  ies.cachePrefix,
		"cache_ttl":     ies.cacheTTL.String(),
	}
}
