package ml

import (
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

// MLService orchestrates all ML components
type MLService struct {
	registry      *ModelRegistry
	textService   *TextEmbeddingService
	imageService  *ImageEmbeddingService
	fusionService *MultiModalFusionService
	logger        *logrus.Logger

	// Performance optimization
	sessionPool sync.Pool

	// Configuration
	config *MLConfig

	// Metrics
	metrics *MLMetrics
	mutex   sync.RWMutex
}

// MLConfig contains configuration for the ML service
type MLConfig struct {
	Models         map[string]ModelConfig `json:"models"`
	TextEmbedding  TextEmbeddingConfig    `json:"text_embedding"`
	ImageEmbedding ImageEmbeddingConfig   `json:"image_embedding"`
	Fusion         MultiModalFusionConfig `json:"fusion"`
}

// ModelConfig contains configuration for individual models
type ModelConfig struct {
	Name       string                 `json:"name"`
	Path       string                 `json:"path"`
	Type       string                 `json:"type"`
	Dimensions int                    `json:"dimensions"`
	Version    string                 `json:"version"`
	Config     map[string]interface{} `json:"config"`
}

// MLMetrics tracks ML service performance metrics
type MLMetrics struct {
	TotalRequests      int64     `json:"total_requests"`
	SuccessfulRequests int64     `json:"successful_requests"`
	FailedRequests     int64     `json:"failed_requests"`
	AverageLatencyMs   float64   `json:"average_latency_ms"`
	CacheHitRate       float64   `json:"cache_hit_rate"`
	ModelsLoaded       int       `json:"models_loaded"`
	LastUpdated        time.Time `json:"last_updated"`
}

// NewMLService creates a new ML service
func NewMLService(redisClient *redis.Client, logger *logrus.Logger, config *MLConfig) (*MLService, error) {
	// Create model registry
	registry := NewModelRegistry(logger)

	// Register models from config
	for _, modelConfig := range config.Models {
		modelInfo := &ModelInfo{
			Name:       modelConfig.Name,
			Path:       modelConfig.Path,
			ModelType:  modelConfig.Type,
			Dimensions: modelConfig.Dimensions,
			Version:    modelConfig.Version,
			Config:     modelConfig.Config,
			LoadedAt:   time.Now(),
		}

		if err := registry.RegisterModel(modelInfo); err != nil {
			return nil, fmt.Errorf("failed to register model %s: %w", modelConfig.Name, err)
		}
	}

	// Create text embedding service
	textService := NewTextEmbeddingService(registry, redisClient, logger, config.TextEmbedding)

	// Create image embedding service
	imageService := NewImageEmbeddingService(registry, redisClient, logger, config.ImageEmbedding)

	// Create fusion service
	fusionService := NewMultiModalFusionService(textService, imageService, logger, config.Fusion)

	service := &MLService{
		registry:      registry,
		textService:   textService,
		imageService:  imageService,
		fusionService: fusionService,
		logger:        logger,
		config:        config,
		metrics:       &MLMetrics{},
		sessionPool: sync.Pool{
			New: func() interface{} {
				return &MLSession{}
			},
		},
	}

	logger.Info("ML service initialized successfully")
	return service, nil
}

// MLSession represents a session for ML operations
type MLSession struct {
	ID        string
	StartTime time.Time
	Metrics   map[string]interface{}
}

// GetSession retrieves a session from the pool
func (mls *MLService) GetSession() *MLSession {
	session := mls.sessionPool.Get().(*MLSession)
	session.ID = fmt.Sprintf("ml_%d", time.Now().UnixNano())
	session.StartTime = time.Now()
	session.Metrics = make(map[string]interface{})
	return session
}

// PutSession returns a session to the pool
func (mls *MLService) PutSession(session *MLSession) {
	// Reset session
	session.ID = ""
	session.StartTime = time.Time{}
	session.Metrics = nil

	mls.sessionPool.Put(session)
}

// GenerateTextEmbedding generates a text embedding
func (mls *MLService) GenerateTextEmbedding(text string, modelName string) ([]float32, error) {
	startTime := time.Now()

	embedding, err := mls.textService.GenerateEmbedding(text, modelName)

	// Update metrics
	mls.updateMetrics(time.Since(startTime), err == nil)

	if err != nil {
		mls.logger.WithFields(logrus.Fields{
			"error": err.Error(),
			"model": modelName,
		}).Error("Failed to generate text embedding")
		return nil, err
	}

	return embedding, nil
}

// GenerateImageEmbedding generates an image embedding from URL
func (mls *MLService) GenerateImageEmbedding(imageURL string, modelName string) ([]float32, *ImageMetadata, error) {
	startTime := time.Now()

	embedding, metadata, err := mls.imageService.GenerateEmbeddingFromURL(imageURL, modelName)

	// Update metrics
	mls.updateMetrics(time.Since(startTime), err == nil)

	if err != nil {
		mls.logger.WithFields(logrus.Fields{
			"error":     err.Error(),
			"model":     modelName,
			"image_url": imageURL,
		}).Error("Failed to generate image embedding")
		return nil, nil, err
	}

	return embedding, metadata, nil
}

// GenerateMultiModalEmbedding generates a fused multi-modal embedding
func (mls *MLService) GenerateMultiModalEmbedding(
	text string,
	imageURL string,
	textModelName string,
	imageModelName string,
) (*FusionResult, error) {
	startTime := time.Now()

	result, err := mls.fusionService.GenerateMultiModalEmbedding(text, imageURL, textModelName, imageModelName)

	// Update metrics
	mls.updateMetrics(time.Since(startTime), err == nil)

	if err != nil {
		mls.logger.WithFields(logrus.Fields{
			"error":       err.Error(),
			"text_model":  textModelName,
			"image_model": imageModelName,
			"image_url":   imageURL,
		}).Error("Failed to generate multi-modal embedding")
		return nil, err
	}

	return result, nil
}

// GenerateBatchTextEmbeddings generates embeddings for multiple texts
func (mls *MLService) GenerateBatchTextEmbeddings(texts []string, modelName string) ([][]float32, error) {
	startTime := time.Now()

	embeddings, err := mls.textService.GenerateBatchEmbeddings(texts, modelName)

	// Update metrics
	mls.updateMetrics(time.Since(startTime), err == nil)

	if err != nil {
		mls.logger.WithFields(logrus.Fields{
			"error":      err.Error(),
			"model":      modelName,
			"batch_size": len(texts),
		}).Error("Failed to generate batch text embeddings")
		return nil, err
	}

	return embeddings, nil
}

// LoadModel loads a specific model
func (mls *MLService) LoadModel(modelName string) error {
	_, err := mls.registry.LoadModel(modelName)
	if err != nil {
		mls.logger.WithFields(logrus.Fields{
			"error": err.Error(),
			"model": modelName,
		}).Error("Failed to load model")
		return err
	}

	mls.logger.WithField("model", modelName).Info("Model loaded successfully")
	return nil
}

// UnloadModel unloads a specific model
func (mls *MLService) UnloadModel(modelName string) error {
	err := mls.registry.UnloadModel(modelName)
	if err != nil {
		mls.logger.WithFields(logrus.Fields{
			"error": err.Error(),
			"model": modelName,
		}).Error("Failed to unload model")
		return err
	}

	mls.logger.WithField("model", modelName).Info("Model unloaded successfully")
	return nil
}

// GetModelInfo returns information about a model
func (mls *MLService) GetModelInfo(modelName string) (*ModelInfo, error) {
	return mls.registry.GetModelInfo(modelName)
}

// ListModels returns all available models
func (mls *MLService) ListModels() map[string]*ModelInfo {
	return mls.registry.ListModels()
}

// updateMetrics updates service metrics
func (mls *MLService) updateMetrics(latency time.Duration, success bool) {
	mls.mutex.Lock()
	defer mls.mutex.Unlock()

	mls.metrics.TotalRequests++
	if success {
		mls.metrics.SuccessfulRequests++
	} else {
		mls.metrics.FailedRequests++
	}

	// Update average latency (exponential moving average)
	alpha := 0.1
	newLatencyMs := float64(latency.Nanoseconds()) / 1e6
	if mls.metrics.AverageLatencyMs == 0 {
		mls.metrics.AverageLatencyMs = newLatencyMs
	} else {
		mls.metrics.AverageLatencyMs = alpha*newLatencyMs + (1-alpha)*mls.metrics.AverageLatencyMs
	}

	mls.metrics.LastUpdated = time.Now()
}

// GetMetrics returns current service metrics
func (mls *MLService) GetMetrics() *MLMetrics {
	mls.mutex.RLock()
	defer mls.mutex.RUnlock()

	// Create a copy to avoid race conditions
	metrics := *mls.metrics
	metrics.ModelsLoaded = len(mls.registry.ListModels())

	// Calculate cache hit rate (simplified)
	if metrics.TotalRequests > 0 {
		// This would be calculated from actual cache statistics
		metrics.CacheHitRate = 0.75 // Placeholder
	}

	return &metrics
}

// GetStats returns comprehensive service statistics
func (mls *MLService) GetStats() map[string]interface{} {
	metrics := mls.GetMetrics()

	return map[string]interface{}{
		"metrics":        metrics,
		"text_service":   mls.textService.GetStats(),
		"image_service":  mls.imageService.GetStats(),
		"fusion_service": mls.fusionService.GetFusionStats(),
		"models":         mls.registry.ListModels(),
	}
}

// Stop gracefully shuts down the ML service
func (mls *MLService) Stop() {
	mls.textService.Stop()
	mls.logger.Info("ML service stopped")
}

// DefaultMLConfig returns a default ML configuration
func DefaultMLConfig() *MLConfig {
	return &MLConfig{
		Models: map[string]ModelConfig{
			"text-embedding": {
				Name:       "all-MiniLM-L6-v2",
				Path:       "./models/all-MiniLM-L6-v2.onnx",
				Type:       "text",
				Dimensions: 384,
				Version:    "1.0.0",
				Config: map[string]interface{}{
					"max_sequence_length": 512,
					"do_lower_case":       true,
				},
			},
			"image-embedding": {
				Name:       "clip-vit-base-patch32",
				Path:       "./models/clip-vit-base-patch32.onnx",
				Type:       "image",
				Dimensions: 512,
				Version:    "1.0.0",
				Config: map[string]interface{}{
					"image_size":   224,
					"patch_size":   32,
					"num_channels": 3,
				},
			},
		},
		TextEmbedding: TextEmbeddingConfig{
			MaxTokens:   512,
			BatchSize:   32,
			CachePrefix: "embed:text",
			CacheTTL:    24 * time.Hour,
			WorkerCount: 4,
		},
		ImageEmbedding: ImageEmbeddingConfig{
			TargetWidth:  224,
			TargetHeight: 224,
			MaxFileSize:  10 * 1024 * 1024, // 10MB
			Timeout:      30 * time.Second,
			CachePrefix:  "embed:image",
			CacheTTL:     24 * time.Hour,
		},
		Fusion: MultiModalFusionConfig{
			TextDimensions:  384,
			ImageDimensions: 512,
			FinalDimensions: 768,
			TextWeight:      0.6,
			ImageWeight:     0.4,
		},
	}
}
