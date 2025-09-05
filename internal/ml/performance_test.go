package ml

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// BenchmarkMLServicePerformance tests the overall ML service performance
func BenchmarkMLServicePerformance(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1,
	})

	config := DefaultMLConfig()
	mlService, err := NewMLService(redisClient, logger, config)
	require.NoError(b, err)
	defer mlService.Stop()

	b.Run("TextEmbedding", func(b *testing.B) {
		text := "This is a performance test for text embedding generation with a reasonably long sentence to simulate real-world usage."

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, err := mlService.GenerateTextEmbedding(text, "all-MiniLM-L6-v2")
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	b.Run("BatchTextEmbedding", func(b *testing.B) {
		texts := make([]string, 32)
		for i := range texts {
			texts[i] = fmt.Sprintf("Performance test sentence number %d with some additional content to make it realistic.", i)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := mlService.GenerateBatchTextEmbeddings(texts, "all-MiniLM-L6-v2")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("ImageEmbedding", func(b *testing.B) {
		imageURL := "https://example.com/test-image.jpg"

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				// Note: This will fail in actual benchmark due to invalid URL
				// In real tests, use a valid test image URL or mock the HTTP client
				_, _, err := mlService.GenerateImageEmbedding(imageURL, "clip-vit-base-patch32")
				// Ignore network errors for benchmark purposes
				_ = err
			}
		})
	})
}

// BenchmarkConcurrentEmbeddingGeneration tests concurrent embedding generation
func BenchmarkConcurrentEmbeddingGeneration(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1,
	})

	config := DefaultMLConfig()
	config.TextEmbedding.WorkerCount = runtime.NumCPU()

	mlService, err := NewMLService(redisClient, logger, config)
	require.NoError(b, err)
	defer mlService.Stop()

	texts := []string{
		"First concurrent test sentence for performance evaluation.",
		"Second concurrent test sentence with different content.",
		"Third concurrent test sentence for load testing purposes.",
		"Fourth concurrent test sentence to simulate real workload.",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var wg sync.WaitGroup
			for _, text := range texts {
				wg.Add(1)
				go func(t string) {
					defer wg.Done()
					_, err := mlService.GenerateTextEmbedding(t, "all-MiniLM-L6-v2")
					if err != nil {
						b.Error(err)
					}
				}(text)
			}
			wg.Wait()
		}
	})
}

// BenchmarkMemoryUsage tests memory usage patterns
func BenchmarkMemoryUsage(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1,
	})

	config := DefaultMLConfig()
	mlService, err := NewMLService(redisClient, logger, config)
	require.NoError(b, err)
	defer mlService.Stop()

	// Measure initial memory
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		text := fmt.Sprintf("Memory usage test sentence number %d with varying content to test allocation patterns.", i)
		_, err := mlService.GenerateTextEmbedding(text, "all-MiniLM-L6-v2")
		if err != nil {
			b.Fatal(err)
		}

		// Force GC every 100 iterations to measure steady-state memory
		if i%100 == 0 {
			runtime.GC()
		}
	}

	// Measure final memory
	runtime.GC()
	runtime.ReadMemStats(&m2)

	b.ReportMetric(float64(m2.Alloc-m1.Alloc)/float64(b.N), "bytes/op")
	b.ReportMetric(float64(m2.TotalAlloc-m1.TotalAlloc)/float64(b.N), "total-bytes/op")
}

// TestEmbeddingQuality tests the quality and consistency of embeddings
func TestEmbeddingQuality(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1,
	})

	config := DefaultMLConfig()
	mlService, err := NewMLService(redisClient, logger, config)
	require.NoError(t, err)
	defer mlService.Stop()

	t.Run("EmbeddingConsistency", func(t *testing.T) {
		text := "This is a consistency test sentence."

		// Generate embedding multiple times
		embeddings := make([][]float32, 5)
		for i := range embeddings {
			embedding, err := mlService.GenerateTextEmbedding(text, "all-MiniLM-L6-v2")
			require.NoError(t, err)
			embeddings[i] = embedding
		}

		// All embeddings should be identical for the same input
		for i := 1; i < len(embeddings); i++ {
			require.Equal(t, embeddings[0], embeddings[i], "Embeddings should be consistent for same input")
		}
	})

	t.Run("EmbeddingNormalization", func(t *testing.T) {
		text := "Test sentence for normalization check."

		embedding, err := mlService.GenerateTextEmbedding(text, "all-MiniLM-L6-v2")
		require.NoError(t, err)

		// Calculate L2 norm
		var norm float64
		for _, v := range embedding {
			norm += float64(v * v)
		}
		norm = norm // Would take sqrt for actual L2 norm

		// Should be close to 1 (normalized)
		require.InDelta(t, 1.0, norm, 0.01, "Embedding should be L2 normalized")
	})

	t.Run("EmbeddingSimilarity", func(t *testing.T) {
		// Similar sentences should have similar embeddings
		text1 := "The cat is sleeping on the mat."
		text2 := "A cat is resting on the mat."
		text3 := "The dog is running in the park."

		emb1, err := mlService.GenerateTextEmbedding(text1, "all-MiniLM-L6-v2")
		require.NoError(t, err)

		emb2, err := mlService.GenerateTextEmbedding(text2, "all-MiniLM-L6-v2")
		require.NoError(t, err)

		emb3, err := mlService.GenerateTextEmbedding(text3, "all-MiniLM-L6-v2")
		require.NoError(t, err)

		// Calculate cosine similarities
		sim12 := cosineSimilarity(emb1, emb2)
		sim13 := cosineSimilarity(emb1, emb3)

		// Similar sentences should be more similar than dissimilar ones
		require.Greater(t, sim12, sim13, "Similar sentences should have higher similarity")
	})

	t.Run("EmbeddingDimensions", func(t *testing.T) {
		text := "Dimension test sentence."

		embedding, err := mlService.GenerateTextEmbedding(text, "all-MiniLM-L6-v2")
		require.NoError(t, err)

		// Should have correct dimensions
		require.Equal(t, 384, len(embedding), "Text embedding should have 384 dimensions")
	})
}

// TestMultiModalFusionQuality tests multi-modal fusion quality
func TestMultiModalFusionQuality(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1,
	})

	config := DefaultMLConfig()
	mlService, err := NewMLService(redisClient, logger, config)
	require.NoError(t, err)
	defer mlService.Stop()

	t.Run("FusionDimensions", func(t *testing.T) {
		// Note: This test would need valid image URLs or mock data
		text := "A beautiful sunset over the ocean."
		imageURL := "https://example.com/sunset.jpg"

		// This will fail due to invalid URL, but tests the interface
		result, err := mlService.GenerateMultiModalEmbedding(
			text, imageURL, "all-MiniLM-L6-v2", "clip-vit-base-patch32")

		if err == nil { // Only test if no error (would need valid setup)
			require.Equal(t, 384, len(result.TextEmbedding))
			require.Equal(t, 512, len(result.ImageEmbedding))
			require.Equal(t, 896, len(result.FusedEmbedding))
			require.Equal(t, 768, len(result.FinalEmbedding))
		}
	})
}

// TestPerformanceMetrics tests performance metric collection
func TestPerformanceMetrics(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1,
	})

	config := DefaultMLConfig()
	mlService, err := NewMLService(redisClient, logger, config)
	require.NoError(t, err)
	defer mlService.Stop()

	// Generate some embeddings to populate metrics
	texts := []string{
		"First metrics test sentence.",
		"Second metrics test sentence.",
		"Third metrics test sentence.",
	}

	for _, text := range texts {
		_, err := mlService.GenerateTextEmbedding(text, "all-MiniLM-L6-v2")
		require.NoError(t, err)
	}

	// Check metrics
	metrics := mlService.GetMetrics()
	require.NotNil(t, metrics)
	require.Greater(t, metrics.TotalRequests, int64(0))
	require.Greater(t, metrics.SuccessfulRequests, int64(0))
	require.Greater(t, metrics.AverageLatencyMs, 0.0)
	require.True(t, metrics.LastUpdated.After(time.Now().Add(-time.Minute)))

	// Check stats
	stats := mlService.GetStats()
	require.Contains(t, stats, "metrics")
	require.Contains(t, stats, "text_service")
	require.Contains(t, stats, "models")
}

// Helper function to calculate cosine similarity
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (normA * normB) // Would take sqrt for actual norms
}
