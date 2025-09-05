package ml

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ModelInfo contains metadata about a loaded model
type ModelInfo struct {
	Name        string                 `json:"name"`
	Version     string                 `json:"version"`
	Path        string                 `json:"path"`
	Dimensions  int                    `json:"dimensions"`
	ModelType   string                 `json:"model_type"` // "text", "image", "multimodal"
	LoadedAt    time.Time              `json:"loaded_at"`
	Performance ModelMetrics           `json:"performance"`
	Config      map[string]interface{} `json:"config"`
}

// ModelMetrics tracks performance metrics for a model
type ModelMetrics struct {
	InferenceLatencyMs float64   `json:"inference_latency_ms"`
	ThroughputPerSec   float64   `json:"throughput_per_sec"`
	MemoryUsageMB      float64   `json:"memory_usage_mb"`
	ErrorRate          float64   `json:"error_rate"`
	LastUpdated        time.Time `json:"last_updated"`
}

// ModelSession represents a loaded model session
// Note: In production, this would contain the actual ONNX runtime session
type ModelSession struct {
	Info       *ModelInfo
	Session    interface{} // ONNX Runtime session (placeholder for now)
	LoadedAt   time.Time
	UsageCount int64
}

// ModelRegistry manages model loading, caching, and versioning
type ModelRegistry struct {
	models map[string]*ModelInfo
	cache  *sync.Map // Thread-safe model cache
	pool   sync.Pool // Model instance pooling
	mutex  sync.RWMutex
	logger *logrus.Logger
}

// NewModelRegistry creates a new model registry
func NewModelRegistry(logger *logrus.Logger) *ModelRegistry {
	return &ModelRegistry{
		models: make(map[string]*ModelInfo),
		cache:  &sync.Map{},
		pool: sync.Pool{
			New: func() interface{} {
				return &ModelSession{}
			},
		},
		logger: logger,
	}
}

// RegisterModel registers a new model with the registry
func (mr *ModelRegistry) RegisterModel(info *ModelInfo) error {
	mr.mutex.Lock()
	defer mr.mutex.Unlock()

	if info.Name == "" {
		return fmt.Errorf("model name cannot be empty")
	}

	// Validate model type
	validTypes := map[string]bool{
		"text":       true,
		"image":      true,
		"multimodal": true,
	}
	if !validTypes[info.ModelType] {
		return fmt.Errorf("invalid model type: %s", info.ModelType)
	}

	mr.models[info.Name] = info
	mr.logger.WithFields(logrus.Fields{
		"model_name": info.Name,
		"model_type": info.ModelType,
		"dimensions": info.Dimensions,
	}).Info("Model registered successfully")

	return nil
}

// LoadModel loads a model with lazy loading and caching
func (mr *ModelRegistry) LoadModel(name string) (*ModelSession, error) {
	// Check cache first
	if cached, ok := mr.cache.Load(name); ok {
		session := cached.(*ModelSession)
		session.UsageCount++
		return session, nil
	}

	mr.mutex.RLock()
	modelInfo, exists := mr.models[name]
	mr.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("model not found: %s", name)
	}

	// TODO: Load actual ONNX model here
	// For production use, integrate with ONNX Runtime:
	// onnxSession, err := onnxruntime.NewSession(modelInfo.Path, options)
	// if err != nil {
	//     return nil, fmt.Errorf("failed to load ONNX model: %w", err)
	// }

	// Create new session (placeholder implementation)
	session := &ModelSession{
		Info:       modelInfo,
		Session:    nil, // Will be actual ONNX session in production
		LoadedAt:   time.Now(),
		UsageCount: 1,
	}

	// Cache the session
	mr.cache.Store(name, session)

	mr.logger.WithFields(logrus.Fields{
		"model_name": name,
		"model_type": modelInfo.ModelType,
	}).Info("Model loaded successfully")

	return session, nil
}

// GetModelInfo returns information about a registered model
func (mr *ModelRegistry) GetModelInfo(name string) (*ModelInfo, error) {
	mr.mutex.RLock()
	defer mr.mutex.RUnlock()

	info, exists := mr.models[name]
	if !exists {
		return nil, fmt.Errorf("model not found: %s", name)
	}

	return info, nil
}

// ListModels returns all registered models
func (mr *ModelRegistry) ListModels() map[string]*ModelInfo {
	mr.mutex.RLock()
	defer mr.mutex.RUnlock()

	result := make(map[string]*ModelInfo)
	for name, info := range mr.models {
		result[name] = info
	}

	return result
}

// UnloadModel removes a model from cache
func (mr *ModelRegistry) UnloadModel(name string) error {
	mr.cache.Delete(name)
	mr.logger.WithField("model_name", name).Info("Model unloaded from cache")
	return nil
}

// UpdateMetrics updates performance metrics for a model
func (mr *ModelRegistry) UpdateMetrics(name string, metrics ModelMetrics) error {
	mr.mutex.Lock()
	defer mr.mutex.Unlock()

	info, exists := mr.models[name]
	if !exists {
		return fmt.Errorf("model not found: %s", name)
	}

	metrics.LastUpdated = time.Now()
	info.Performance = metrics

	return nil
}

// GetModelSession retrieves a model session from the pool
func (mr *ModelRegistry) GetModelSession() *ModelSession {
	return mr.pool.Get().(*ModelSession)
}

// PutModelSession returns a model session to the pool
func (mr *ModelRegistry) PutModelSession(session *ModelSession) {
	// Reset session for reuse
	session.Info = nil
	session.Session = nil
	session.LoadedAt = time.Time{}
	session.UsageCount = 0

	mr.pool.Put(session)
}

// GenerateModelHash creates a hash for model versioning
func GenerateModelHash(modelPath string, config map[string]interface{}) string {
	hasher := sha256.New()
	hasher.Write([]byte(modelPath))

	// Add config to hash for versioning
	for key, value := range config {
		hasher.Write([]byte(fmt.Sprintf("%s:%v", key, value)))
	}

	return fmt.Sprintf("%x", hasher.Sum(nil))[:16]
}
