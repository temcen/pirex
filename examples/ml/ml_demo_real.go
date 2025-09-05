package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/temcen/pirex/internal/ml"
)

func main() {
	fmt.Println("🚀 Real ML Inference Demo - macOS M4 Optimized")
	fmt.Println("==============================================")

	// Setup logging
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// Setup Redis (optional)
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   0,
	})

	// Test Redis connection
	ctx := context.Background()
	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		fmt.Printf("⚠️  Redis not available: %v\n", err)
		fmt.Println("   Continuing without caching...")
	} else {
		fmt.Println("✅ Redis connected - caching enabled")
	}

	// Create ML service with default configuration
	config := ml.DefaultMLConfig()
	mlService, err := ml.NewMLService(redisClient, logger, config)
	if err != nil {
		log.Fatalf("❌ Failed to create ML service: %v", err)
	}
	defer mlService.Stop()

	fmt.Println("\n📊 Available Models:")
	models := mlService.ListModels()
	for name, info := range models {
		fmt.Printf("   • %s (%s, %d dims)\n", name, info.ModelType, info.Dimensions)
	}

	// Demo 1: Real vs Mock Embedding Comparison
	fmt.Println("\n🔬 Real vs Mock Embedding Comparison")
	fmt.Println("------------------------------------")

	testTexts := []string{
		"The quick brown fox jumps over the lazy dog",
		"A fast brown fox leaps over a sleepy dog",
		"Machine learning is revolutionizing technology",
		"Artificial intelligence transforms our world",
	}

	fmt.Println("Generating embeddings with real models (if available)...")

	var realEmbeddings [][]float32

	start := time.Now()
	for i, text := range testTexts {
		embedding, err := mlService.GenerateTextEmbedding(text, "all-MiniLM-L6-v2")
		if err != nil {
			fmt.Printf("❌ Error generating embedding for text %d: %v\n", i+1, err)
			continue
		}

		realEmbeddings = append(realEmbeddings, embedding)
		fmt.Printf("   Text %d: %d dimensions (first 5: [%.3f, %.3f, %.3f, %.3f, %.3f])\n",
			i+1, len(embedding), embedding[0], embedding[1], embedding[2], embedding[3], embedding[4])
	}

	realTime := time.Since(start)
	fmt.Printf("⏱️  Real embedding time: %v\n", realTime)

	// Demo 2: Semantic Similarity Analysis
	fmt.Println("\n🧠 Semantic Similarity Analysis")
	fmt.Println("-------------------------------")

	if len(realEmbeddings) >= 4 {
		// Calculate similarities
		sim12 := cosineSimilarity(realEmbeddings[0], realEmbeddings[1])
		sim34 := cosineSimilarity(realEmbeddings[2], realEmbeddings[3])
		sim13 := cosineSimilarity(realEmbeddings[0], realEmbeddings[2])
		sim24 := cosineSimilarity(realEmbeddings[1], realEmbeddings[3])

		fmt.Printf("📈 Similar texts (fox sentences): %.3f\n", sim12)
		fmt.Printf("📈 Similar texts (AI sentences): %.3f\n", sim34)
		fmt.Printf("📉 Different topics (fox vs AI): %.3f\n", sim13)
		fmt.Printf("📉 Different topics (fox vs AI): %.3f\n", sim24)

		// Analyze quality
		avgSimilarSimilarity := (sim12 + sim34) / 2
		avgDifferentSimilarity := (sim13 + sim24) / 2

		fmt.Printf("\n📊 Quality Analysis:\n")
		fmt.Printf("   Average similar text similarity: %.3f\n", avgSimilarSimilarity)
		fmt.Printf("   Average different topic similarity: %.3f\n", avgDifferentSimilarity)
		fmt.Printf("   Semantic separation: %.3f\n", avgSimilarSimilarity-avgDifferentSimilarity)

		if avgSimilarSimilarity > avgDifferentSimilarity+0.1 {
			fmt.Println("✅ Good semantic understanding detected!")
		} else {
			fmt.Println("⚠️  Limited semantic separation (may be using mock embeddings)")
		}
	}

	// Demo 3: Batch Processing Performance
	fmt.Println("\n📦 Batch Processing Performance")
	fmt.Println("-------------------------------")

	batchTexts := []string{
		"Natural language processing enables computers to understand human language",
		"Deep learning models can recognize patterns in complex data",
		"Computer vision allows machines to interpret visual information",
		"Reinforcement learning helps AI systems learn through trial and error",
		"Neural networks are inspired by the structure of the human brain",
		"Transformer models have revolutionized natural language understanding",
		"Convolutional neural networks excel at image recognition tasks",
		"Recurrent neural networks are effective for sequential data processing",
	}

	start = time.Now()
	batchEmbeddings, err := mlService.GenerateBatchTextEmbeddings(batchTexts, "all-MiniLM-L6-v2")
	batchTime := time.Since(start)

	if err != nil {
		fmt.Printf("❌ Batch processing error: %v\n", err)
	} else {
		fmt.Printf("✅ Generated %d embeddings in batch\n", len(batchEmbeddings))
		fmt.Printf("⏱️  Batch processing time: %v\n", batchTime)
		fmt.Printf("📊 Average time per embedding: %v\n", batchTime/time.Duration(len(batchTexts)))

		// Calculate batch similarity matrix
		fmt.Println("\n🔗 Similarity Matrix (showing related concepts):")
		for i := 0; i < len(batchEmbeddings) && i < 4; i++ {
			for j := i + 1; j < len(batchEmbeddings) && j < 4; j++ {
				sim := cosineSimilarity(batchEmbeddings[i], batchEmbeddings[j])
				fmt.Printf("   Text %d ↔ Text %d: %.3f\n", i+1, j+1, sim)
			}
		}
	}

	// Demo 4: Caching Performance
	fmt.Println("\n💾 Caching Performance Test")
	fmt.Println("---------------------------")

	cacheTestText := "This is a test sentence for caching performance evaluation"

	// First call (no cache)
	start = time.Now()
	_, err = mlService.GenerateTextEmbedding(cacheTestText, "all-MiniLM-L6-v2")
	firstCallTime := time.Since(start)

	// Second call (should be cached)
	start = time.Now()
	_, err = mlService.GenerateTextEmbedding(cacheTestText, "all-MiniLM-L6-v2")
	secondCallTime := time.Since(start)

	if err != nil {
		fmt.Printf("❌ Caching test error: %v\n", err)
	} else {
		fmt.Printf("🔄 First call (no cache): %v\n", firstCallTime)
		fmt.Printf("⚡ Second call (cached): %v\n", secondCallTime)

		if secondCallTime < firstCallTime/2 {
			fmt.Printf("✅ Caching working! Speedup: %.1fx\n", float64(firstCallTime)/float64(secondCallTime))
		} else {
			fmt.Println("⚠️  Caching may not be working optimally")
		}
	}

	// Demo 5: Model Information and Statistics
	fmt.Println("\n📊 Model Information & Statistics")
	fmt.Println("---------------------------------")

	metrics := mlService.GetMetrics()
	fmt.Printf("Total requests: %d\n", metrics.TotalRequests)
	fmt.Printf("Successful requests: %d\n", metrics.SuccessfulRequests)
	fmt.Printf("Failed requests: %d\n", metrics.FailedRequests)
	fmt.Printf("Average latency: %.2f ms\n", metrics.AverageLatencyMs)
	fmt.Printf("Cache hit rate: %.1f%%\n", metrics.CacheHitRate*100)
	fmt.Printf("Models loaded: %d\n", metrics.ModelsLoaded)

	// Demo 6: Python Bridge Status
	fmt.Println("\n🐍 Python Bridge Status")
	fmt.Println("-----------------------")

	stats := mlService.GetStats()
	if textStats, ok := stats["text_service"].(map[string]interface{}); ok {
		fmt.Printf("Worker count: %v\n", textStats["worker_count"])
		fmt.Printf("Batch size: %v\n", textStats["batch_size"])
		fmt.Printf("Max tokens: %v\n", textStats["max_tokens"])
		fmt.Printf("Queue length: %v\n", textStats["queue_length"])
	}

	fmt.Println("\n🎉 Demo completed!")
	fmt.Println("\n🔍 What This Demo Shows:")
	fmt.Println("✅ Real model inference via Python bridge (if available)")
	fmt.Println("✅ Fallback to enhanced mock embeddings")
	fmt.Println("✅ Semantic similarity analysis")
	fmt.Println("✅ Batch processing optimization")
	fmt.Println("✅ Redis caching performance")
	fmt.Println("✅ Comprehensive metrics and monitoring")

	fmt.Println("\n🚀 Key Benefits:")
	fmt.Println("• Real sentence-transformers models on macOS M4")
	fmt.Println("• Automatic fallback to mock embeddings")
	fmt.Println("• No external API dependencies")
	fmt.Println("• Privacy-preserving local inference")
	fmt.Println("• Production-ready performance optimization")

	fmt.Println("\n🔧 To Enable Real Models:")
	fmt.Println("1. Install Python dependencies: pip install sentence-transformers torch")
	fmt.Println("2. Models will be downloaded automatically on first use")
	fmt.Println("3. Restart the demo to see real model performance")
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

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
