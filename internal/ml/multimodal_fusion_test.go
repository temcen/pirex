package ml

import (
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultiModalFusionService(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1,
	})

	registry := NewModelRegistry(logger)

	// Register test models
	textModel := &ModelInfo{
		Name:       "fusion-text-model",
		ModelType:  "text",
		Dimensions: 384,
		Version:    "1.0.0",
	}
	imageModel := &ModelInfo{
		Name:       "fusion-image-model",
		ModelType:  "image",
		Dimensions: 512,
		Version:    "1.0.0",
	}

	err := registry.RegisterModel(textModel)
	require.NoError(t, err)
	err = registry.RegisterModel(imageModel)
	require.NoError(t, err)

	// Create services
	textConfig := TextEmbeddingConfig{WorkerCount: 1}
	imageConfig := ImageEmbeddingConfig{}
	fusionConfig := MultiModalFusionConfig{
		TextDimensions:  384,
		ImageDimensions: 512,
		FinalDimensions: 768,
		TextWeight:      0.6,
		ImageWeight:     0.4,
	}

	textService := NewTextEmbeddingService(registry, redisClient, logger, textConfig)
	imageService := NewImageEmbeddingService(registry, redisClient, logger, imageConfig)
	fusionService := NewMultiModalFusionService(textService, imageService, logger, fusionConfig)

	defer textService.Stop()

	t.Run("FuseEmbeddings", func(t *testing.T) {
		// Create mock embeddings
		textEmbedding := make([]float32, 384)
		imageEmbedding := make([]float32, 512)

		// Fill with test values
		for i := range textEmbedding {
			textEmbedding[i] = float32(i) / 384.0
		}
		for i := range imageEmbedding {
			imageEmbedding[i] = float32(i) / 512.0
		}

		result, err := fusionService.fuseEmbeddings(textEmbedding, imageEmbedding)
		assert.NoError(t, err)
		assert.NotNil(t, result)

		// Check dimensions
		assert.Equal(t, 384, len(result.TextEmbedding))
		assert.Equal(t, 512, len(result.ImageEmbedding))
		assert.Equal(t, 896, len(result.FusedEmbedding)) // 384 + 512
		assert.Equal(t, 768, len(result.FinalEmbedding))

		// Check fusion method
		assert.Equal(t, "late_fusion_with_projection", result.FusionMethod)

		// Check weights
		assert.Equal(t, float32(0.6), result.TextWeight)
		assert.Equal(t, float32(0.4), result.ImageWeight)
	})

	t.Run("FuseEmbeddings_WrongDimensions", func(t *testing.T) {
		textEmbedding := make([]float32, 100) // Wrong dimension
		imageEmbedding := make([]float32, 512)

		result, err := fusionService.fuseEmbeddings(textEmbedding, imageEmbedding)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "text embedding dimension mismatch")
	})

	t.Run("LateFusion", func(t *testing.T) {
		textEmbedding := []float32{1.0, 2.0, 3.0}
		imageEmbedding := []float32{4.0, 5.0}

		fused := fusionService.lateFusion(textEmbedding, imageEmbedding)

		expected := []float32{1.0, 2.0, 3.0, 4.0, 5.0}
		assert.Equal(t, expected, fused)
	})

	t.Run("EarlyFusion", func(t *testing.T) {
		textEmbedding := []float32{1.0, 2.0, 3.0}
		imageEmbedding := []float32{4.0, 5.0}

		fused := fusionService.earlyFusion(textEmbedding, imageEmbedding)

		// Should pad to max dimension (max of textDimensions=384, imageDimensions=512) = 512
		assert.Equal(t, 512, len(fused))

		// Check weighted combination for first few elements: 0.6 * text + 0.4 * image
		expectedFirst := 0.6*1.0 + 0.4*4.0  // 0.6 + 1.6 = 2.2
		expectedSecond := 0.6*2.0 + 0.4*5.0 // 1.2 + 2.0 = 3.2
		expectedThird := 0.6*3.0 + 0.4*0.0  // 1.8 + 0.0 = 1.8 (image padded with 0 after index 1)

		assert.InDelta(t, expectedFirst, fused[0], 0.001)
		assert.InDelta(t, expectedSecond, fused[1], 0.001)
		assert.InDelta(t, expectedThird, fused[2], 0.001)
	})

	t.Run("PadEmbedding", func(t *testing.T) {
		embedding := []float32{1.0, 2.0, 3.0}

		// Pad to larger dimension
		padded := fusionService.padEmbedding(embedding, 5)
		expected := []float32{1.0, 2.0, 3.0, 0.0, 0.0}
		assert.Equal(t, expected, padded)

		// Truncate to smaller dimension
		truncated := fusionService.padEmbedding(embedding, 2)
		expected = []float32{1.0, 2.0}
		assert.Equal(t, expected, truncated)

		// Same dimension
		same := fusionService.padEmbedding(embedding, 3)
		assert.Equal(t, embedding, same)
	})

	t.Run("L2Normalize", func(t *testing.T) {
		embedding := []float32{3.0, 4.0, 0.0} // Length = 5
		normalized := fusionService.l2Normalize(embedding)

		// Check that L2 norm is approximately 1
		var norm float64
		for _, v := range normalized {
			norm += float64(v * v)
		}

		assert.InDelta(t, 1.0, norm, 0.001)

		// Check individual values
		assert.InDelta(t, 0.6, normalized[0], 0.001) // 3/5
		assert.InDelta(t, 0.8, normalized[1], 0.001) // 4/5
		assert.InDelta(t, 0.0, normalized[2], 0.001) // 0/5
	})

	t.Run("L2Normalize_ZeroVector", func(t *testing.T) {
		embedding := []float32{0.0, 0.0, 0.0}
		normalized := fusionService.l2Normalize(embedding)

		// Should return original vector to avoid division by zero
		assert.Equal(t, embedding, normalized)
	})

	t.Run("ApplyProjection", func(t *testing.T) {
		// Create a fused embedding (896 dimensions)
		fusedEmbedding := make([]float32, 896)
		for i := range fusedEmbedding {
			fusedEmbedding[i] = float32(i) / 896.0
		}

		finalEmbedding := fusionService.applyProjection(fusedEmbedding)

		// Check dimensions
		assert.Equal(t, 768, len(finalEmbedding))

		// Check that it's normalized (L2 norm should be close to 1)
		var norm float64
		for _, v := range finalEmbedding {
			norm += float64(v * v)
		}
		assert.InDelta(t, 1.0, norm, 0.001)

		// Check that ReLU was applied (no negative values)
		for _, v := range finalEmbedding {
			assert.GreaterOrEqual(t, v, float32(0.0))
		}
	})

	t.Run("UpdateProjectionWeights", func(t *testing.T) {
		// Create new weights
		weights := make([][]float64, 768)
		for i := range weights {
			weights[i] = make([]float64, 896)
			for j := range weights[i] {
				weights[i][j] = 0.01 // Small values
			}
		}

		bias := make([]float32, 768)
		for i := range bias {
			bias[i] = 0.1
		}

		err := fusionService.UpdateProjectionWeights(weights, bias)
		assert.NoError(t, err)
	})

	t.Run("UpdateProjectionWeights_WrongDimensions", func(t *testing.T) {
		// Wrong weight dimensions
		weights := make([][]float64, 100) // Should be 768
		bias := make([]float32, 768)

		err := fusionService.UpdateProjectionWeights(weights, bias)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "weight matrix dimension mismatch")

		// Wrong bias dimensions
		weights = make([][]float64, 768)
		for i := range weights {
			weights[i] = make([]float64, 896)
		}
		bias = make([]float32, 100) // Should be 768

		err = fusionService.UpdateProjectionWeights(weights, bias)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "bias vector dimension mismatch")
	})

	t.Run("GetFusionStats", func(t *testing.T) {
		stats := fusionService.GetFusionStats()

		assert.Contains(t, stats, "text_dimensions")
		assert.Contains(t, stats, "image_dimensions")
		assert.Contains(t, stats, "fused_dimensions")
		assert.Contains(t, stats, "final_dimensions")
		assert.Contains(t, stats, "text_weight")
		assert.Contains(t, stats, "image_weight")
		assert.Contains(t, stats, "fusion_method")

		assert.Equal(t, 384, stats["text_dimensions"])
		assert.Equal(t, 512, stats["image_dimensions"])
		assert.Equal(t, 896, stats["fused_dimensions"])
		assert.Equal(t, 768, stats["final_dimensions"])
		assert.Equal(t, float32(0.6), stats["text_weight"])
		assert.Equal(t, float32(0.4), stats["image_weight"])
		assert.Equal(t, "late_fusion_with_projection", stats["fusion_method"])
	})
}

func TestMultiModalFusionConfig(t *testing.T) {
	t.Run("DefaultConfig", func(t *testing.T) {
		logger := logrus.New()
		logger.SetLevel(logrus.ErrorLevel)

		redisClient := redis.NewClient(&redis.Options{
			Addr: "localhost:6379",
			DB:   1,
		})

		registry := NewModelRegistry(logger)
		textService := NewTextEmbeddingService(registry, redisClient, logger, TextEmbeddingConfig{})
		imageService := NewImageEmbeddingService(registry, redisClient, logger, ImageEmbeddingConfig{})

		// Use empty config to test defaults
		config := MultiModalFusionConfig{}

		fusionService := NewMultiModalFusionService(textService, imageService, logger, config)

		stats := fusionService.GetFusionStats()

		// Check default values
		assert.Equal(t, 384, stats["text_dimensions"])
		assert.Equal(t, 512, stats["image_dimensions"])
		assert.Equal(t, 768, stats["final_dimensions"])
		assert.Equal(t, float32(0.6), stats["text_weight"])
		assert.Equal(t, float32(0.4), stats["image_weight"])
	})
}

func BenchmarkMultiModalFusion(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1,
	})

	registry := NewModelRegistry(logger)
	textService := NewTextEmbeddingService(registry, redisClient, logger, TextEmbeddingConfig{})
	imageService := NewImageEmbeddingService(registry, redisClient, logger, ImageEmbeddingConfig{})

	config := MultiModalFusionConfig{
		TextDimensions:  384,
		ImageDimensions: 512,
		FinalDimensions: 768,
	}

	fusionService := NewMultiModalFusionService(textService, imageService, logger, config)

	// Create test embeddings
	textEmbedding := make([]float32, 384)
	imageEmbedding := make([]float32, 512)

	for i := range textEmbedding {
		textEmbedding[i] = float32(i) / 384.0
	}
	for i := range imageEmbedding {
		imageEmbedding[i] = float32(i) / 512.0
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := fusionService.fuseEmbeddings(textEmbedding, imageEmbedding)
		if err != nil {
			b.Fatal(err)
		}
	}
}
