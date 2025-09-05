package ml

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelRegistry(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise in tests

	registry := NewModelRegistry(logger)

	t.Run("RegisterModel", func(t *testing.T) {
		modelInfo := &ModelInfo{
			Name:       "test-model",
			Path:       "/path/to/model.onnx",
			ModelType:  "text",
			Dimensions: 384,
			Version:    "1.0.0",
			Config:     map[string]interface{}{"test": "value"},
		}

		err := registry.RegisterModel(modelInfo)
		assert.NoError(t, err)

		// Test duplicate registration
		err = registry.RegisterModel(modelInfo)
		assert.NoError(t, err) // Should allow overwrite
	})

	t.Run("RegisterModel_InvalidType", func(t *testing.T) {
		modelInfo := &ModelInfo{
			Name:      "invalid-model",
			ModelType: "invalid",
		}

		err := registry.RegisterModel(modelInfo)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid model type")
	})

	t.Run("RegisterModel_EmptyName", func(t *testing.T) {
		modelInfo := &ModelInfo{
			Name:      "",
			ModelType: "text",
		}

		err := registry.RegisterModel(modelInfo)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "model name cannot be empty")
	})

	t.Run("LoadModel", func(t *testing.T) {
		// First register a model
		modelInfo := &ModelInfo{
			Name:       "load-test-model",
			Path:       "/path/to/model.onnx",
			ModelType:  "text",
			Dimensions: 384,
			Version:    "1.0.0",
		}

		err := registry.RegisterModel(modelInfo)
		require.NoError(t, err)

		// Load the model
		session, err := registry.LoadModel("load-test-model")
		assert.NoError(t, err)
		assert.NotNil(t, session)
		assert.Equal(t, "load-test-model", session.Info.Name)
		assert.Equal(t, int64(1), session.UsageCount)

		// Load again (should come from cache)
		session2, err := registry.LoadModel("load-test-model")
		assert.NoError(t, err)
		assert.NotNil(t, session2)
		assert.Equal(t, int64(2), session2.UsageCount)
	})

	t.Run("LoadModel_NotFound", func(t *testing.T) {
		session, err := registry.LoadModel("non-existent-model")
		assert.Error(t, err)
		assert.Nil(t, session)
		assert.Contains(t, err.Error(), "model not found")
	})

	t.Run("GetModelInfo", func(t *testing.T) {
		// Register a model first
		modelInfo := &ModelInfo{
			Name:       "info-test-model",
			Path:       "/path/to/model.onnx",
			ModelType:  "image",
			Dimensions: 512,
			Version:    "2.0.0",
		}

		err := registry.RegisterModel(modelInfo)
		require.NoError(t, err)

		// Get model info
		info, err := registry.GetModelInfo("info-test-model")
		assert.NoError(t, err)
		assert.NotNil(t, info)
		assert.Equal(t, "info-test-model", info.Name)
		assert.Equal(t, "image", info.ModelType)
		assert.Equal(t, 512, info.Dimensions)
	})

	t.Run("ListModels", func(t *testing.T) {
		// Register multiple models
		models := []*ModelInfo{
			{Name: "model1", ModelType: "text", Dimensions: 384},
			{Name: "model2", ModelType: "image", Dimensions: 512},
			{Name: "model3", ModelType: "multimodal", Dimensions: 768},
		}

		for _, model := range models {
			err := registry.RegisterModel(model)
			require.NoError(t, err)
		}

		// List all models
		allModels := registry.ListModels()
		assert.GreaterOrEqual(t, len(allModels), 3) // At least the 3 we just added

		// Check specific models exist
		assert.Contains(t, allModels, "model1")
		assert.Contains(t, allModels, "model2")
		assert.Contains(t, allModels, "model3")
	})

	t.Run("UnloadModel", func(t *testing.T) {
		// Register and load a model
		modelInfo := &ModelInfo{
			Name:      "unload-test-model",
			ModelType: "text",
		}

		err := registry.RegisterModel(modelInfo)
		require.NoError(t, err)

		_, err = registry.LoadModel("unload-test-model")
		require.NoError(t, err)

		// Unload the model
		err = registry.UnloadModel("unload-test-model")
		assert.NoError(t, err)

		// Loading again should create a new session
		session, err := registry.LoadModel("unload-test-model")
		assert.NoError(t, err)
		assert.Equal(t, int64(1), session.UsageCount) // Should be 1, not 2
	})

	t.Run("UpdateMetrics", func(t *testing.T) {
		// Register a model
		modelInfo := &ModelInfo{
			Name:      "metrics-test-model",
			ModelType: "text",
		}

		err := registry.RegisterModel(modelInfo)
		require.NoError(t, err)

		// Update metrics
		metrics := ModelMetrics{
			InferenceLatencyMs: 150.5,
			ThroughputPerSec:   100.0,
			MemoryUsageMB:      256.0,
			ErrorRate:          0.01,
		}

		err = registry.UpdateMetrics("metrics-test-model", metrics)
		assert.NoError(t, err)

		// Verify metrics were updated
		info, err := registry.GetModelInfo("metrics-test-model")
		require.NoError(t, err)
		assert.Equal(t, 150.5, info.Performance.InferenceLatencyMs)
		assert.Equal(t, 100.0, info.Performance.ThroughputPerSec)
		assert.True(t, info.Performance.LastUpdated.After(time.Now().Add(-time.Second)))
	})

	t.Run("SessionPool", func(t *testing.T) {
		// Test session pool operations
		session1 := registry.GetModelSession()
		assert.NotNil(t, session1)

		session2 := registry.GetModelSession()
		assert.NotNil(t, session2)

		// Return sessions to pool
		registry.PutModelSession(session1)
		registry.PutModelSession(session2)

		// Get session again (should be reused)
		session3 := registry.GetModelSession()
		assert.NotNil(t, session3)

		// Session should be reset
		assert.Empty(t, session3.Info)
		assert.Nil(t, session3.Session)
		assert.Zero(t, session3.UsageCount)
	})
}

func TestGenerateModelHash(t *testing.T) {
	t.Run("SameInputsSameHash", func(t *testing.T) {
		config := map[string]interface{}{
			"param1": "value1",
			"param2": 42,
		}

		hash1 := GenerateModelHash("/path/to/model", config)
		hash2 := GenerateModelHash("/path/to/model", config)

		assert.Equal(t, hash1, hash2)
		assert.Len(t, hash1, 16) // Should be 16 characters
	})

	t.Run("DifferentInputsDifferentHash", func(t *testing.T) {
		config1 := map[string]interface{}{"param": "value1"}
		config2 := map[string]interface{}{"param": "value2"}

		hash1 := GenerateModelHash("/path/to/model", config1)
		hash2 := GenerateModelHash("/path/to/model", config2)

		assert.NotEqual(t, hash1, hash2)
	})

	t.Run("EmptyConfig", func(t *testing.T) {
		config := map[string]interface{}{}

		hash := GenerateModelHash("/path/to/model", config)
		assert.Len(t, hash, 16)
		assert.NotEmpty(t, hash)
	})
}
