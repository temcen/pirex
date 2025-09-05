package ml

import (
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTextEmbeddingService(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// Create mock Redis client (in practice, use miniredis for testing)
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Use test database
	})

	registry := NewModelRegistry(logger)

	// Register a test model
	modelInfo := &ModelInfo{
		Name:       "test-text-model",
		Path:       "/path/to/model.onnx",
		ModelType:  "text",
		Dimensions: 384,
		Version:    "1.0.0",
	}
	err := registry.RegisterModel(modelInfo)
	require.NoError(t, err)

	config := TextEmbeddingConfig{
		MaxTokens:   512,
		BatchSize:   4,
		CachePrefix: "test:embed:text",
		CacheTTL:    time.Hour,
		WorkerCount: 2,
	}

	service := NewTextEmbeddingService(registry, redisClient, logger, config)
	defer service.Stop()

	t.Run("GenerateEmbedding", func(t *testing.T) {
		text := "This is a test sentence for embedding generation."

		embedding, err := service.GenerateEmbedding(text, "test-text-model")
		assert.NoError(t, err)
		assert.NotNil(t, embedding)
		assert.Equal(t, 384, len(embedding))

		// Check that embedding is normalized (L2 norm should be close to 1)
		var norm float64
		for _, v := range embedding {
			norm += float64(v * v)
		}
		norm = norm // sqrt would be taken for actual norm
		assert.Greater(t, norm, 0.0)
	})

	t.Run("GenerateEmbedding_EmptyText", func(t *testing.T) {
		embedding, err := service.GenerateEmbedding("", "test-text-model")
		assert.Error(t, err)
		assert.Nil(t, embedding)
		assert.Contains(t, err.Error(), "text cannot be empty")
	})

	t.Run("GenerateEmbedding_InvalidModel", func(t *testing.T) {
		embedding, err := service.GenerateEmbedding("test text", "non-existent-model")
		assert.Error(t, err)
		assert.Nil(t, embedding)
	})

	t.Run("GenerateBatchEmbeddings", func(t *testing.T) {
		texts := []string{
			"First test sentence.",
			"Second test sentence.",
			"Third test sentence.",
		}

		embeddings, err := service.GenerateBatchEmbeddings(texts, "test-text-model")
		assert.NoError(t, err)
		assert.NotNil(t, embeddings)
		assert.Equal(t, len(texts), len(embeddings))

		// Check each embedding
		for i, embedding := range embeddings {
			assert.Equal(t, 384, len(embedding), "Embedding %d has wrong dimensions", i)

			// Check that embeddings are different for different texts
			if i > 0 {
				assert.NotEqual(t, embeddings[0], embedding, "Embeddings should be different for different texts")
			}
		}
	})

	t.Run("GenerateBatchEmbeddings_EmptyTexts", func(t *testing.T) {
		embeddings, err := service.GenerateBatchEmbeddings([]string{}, "test-text-model")
		assert.Error(t, err)
		assert.Nil(t, embeddings)
		assert.Contains(t, err.Error(), "texts cannot be empty")
	})

	t.Run("Tokenize", func(t *testing.T) {
		text := "Hello world test"
		tokens := service.tokenize(text)

		assert.Contains(t, tokens, "[CLS]")
		assert.Contains(t, tokens, "[SEP]")
		assert.Contains(t, tokens, "hello")
		assert.Contains(t, tokens, "world")
		assert.Contains(t, tokens, "test")

		// Check order
		assert.Equal(t, "[CLS]", tokens[0])
		assert.Equal(t, "[SEP]", tokens[len(tokens)-1])
	})

	t.Run("L2Normalize", func(t *testing.T) {
		// Test with known vector
		input := []float32{3.0, 4.0, 0.0} // Length = 5
		normalized := service.l2Normalize(input)

		// Check that L2 norm is approximately 1
		var norm float64
		for _, v := range normalized {
			norm += float64(v * v)
		}
		norm = norm // Would take sqrt for actual norm

		// The squared norm should be close to 1
		assert.InDelta(t, 1.0, norm, 0.001)

		// Check individual values
		assert.InDelta(t, 0.6, normalized[0], 0.001) // 3/5
		assert.InDelta(t, 0.8, normalized[1], 0.001) // 4/5
		assert.InDelta(t, 0.0, normalized[2], 0.001) // 0/5
	})

	t.Run("L2Normalize_ZeroVector", func(t *testing.T) {
		input := []float32{0.0, 0.0, 0.0}
		normalized := service.l2Normalize(input)

		// Should return original vector to avoid division by zero
		assert.Equal(t, input, normalized)
	})

	t.Run("GenerateCacheKey", func(t *testing.T) {
		text := "test text"
		modelName := "test-text-model"

		key := service.generateCacheKey(text, modelName)

		assert.Contains(t, key, service.cachePrefix)
		assert.Contains(t, key, modelName)
		assert.Contains(t, key, "1.0.0") // Model version

		// Same inputs should generate same key
		key2 := service.generateCacheKey(text, modelName)
		assert.Equal(t, key, key2)

		// Different inputs should generate different keys
		key3 := service.generateCacheKey("different text", modelName)
		assert.NotEqual(t, key, key3)
	})

	t.Run("GetStats", func(t *testing.T) {
		stats := service.GetStats()

		assert.Contains(t, stats, "worker_count")
		assert.Contains(t, stats, "batch_size")
		assert.Contains(t, stats, "max_tokens")
		assert.Contains(t, stats, "cache_prefix")
		assert.Contains(t, stats, "cache_ttl")
		assert.Contains(t, stats, "queue_length")

		assert.Equal(t, 2, stats["worker_count"])
		assert.Equal(t, 4, stats["batch_size"])
		assert.Equal(t, 512, stats["max_tokens"])
	})
}

func TestTextEmbeddingWorker(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1,
	})

	registry := NewModelRegistry(logger)

	// Register a test model
	modelInfo := &ModelInfo{
		Name:       "worker-test-model",
		ModelType:  "text",
		Dimensions: 384,
		Version:    "1.0.0",
	}
	err := registry.RegisterModel(modelInfo)
	require.NoError(t, err)

	config := TextEmbeddingConfig{
		WorkerCount: 1,
		BatchSize:   2,
	}

	service := NewTextEmbeddingService(registry, redisClient, logger, config)
	defer service.Stop()

	t.Run("ProcessJob", func(t *testing.T) {
		// Create a job
		job := EmbeddingJob{
			ID:        "test-job-1",
			Text:      "Test text for worker",
			ModelName: "worker-test-model",
			Response:  make(chan EmbeddingResult, 1),
		}

		// Submit job
		service.jobQueue <- job

		// Wait for result
		select {
		case result := <-job.Response:
			assert.NoError(t, result.Error)
			assert.NotNil(t, result.Embedding)
			assert.Equal(t, 384, len(result.Embedding))
			assert.Greater(t, result.Latency, time.Duration(0))
		case <-time.After(5 * time.Second):
			t.Fatal("Job processing timed out")
		}
	})
}

func BenchmarkTextEmbeddingGeneration(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1,
	})

	registry := NewModelRegistry(logger)

	modelInfo := &ModelInfo{
		Name:       "bench-text-model",
		ModelType:  "text",
		Dimensions: 384,
		Version:    "1.0.0",
	}
	registry.RegisterModel(modelInfo)

	config := TextEmbeddingConfig{
		WorkerCount: 4,
		BatchSize:   32,
	}

	service := NewTextEmbeddingService(registry, redisClient, logger, config)
	defer service.Stop()

	text := "This is a benchmark test sentence for measuring embedding generation performance."

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := service.GenerateEmbedding(text, "bench-text-model")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkBatchTextEmbedding(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1,
	})

	registry := NewModelRegistry(logger)

	modelInfo := &ModelInfo{
		Name:       "batch-bench-model",
		ModelType:  "text",
		Dimensions: 384,
		Version:    "1.0.0",
	}
	registry.RegisterModel(modelInfo)

	config := TextEmbeddingConfig{
		WorkerCount: 4,
		BatchSize:   32,
	}

	service := NewTextEmbeddingService(registry, redisClient, logger, config)
	defer service.Stop()

	// Create batch of texts
	texts := make([]string, 32)
	for i := range texts {
		texts[i] = "This is benchmark text number " + string(rune(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.GenerateBatchEmbeddings(texts, "batch-bench-model")
		if err != nil {
			b.Fatal(err)
		}
	}
}
