package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/temcen/pirex/internal/ml"
)

func _main() {
	fmt.Println("🚀 ML Embedding Demo - Local ONNX Inference")
	fmt.Println("==========================================")

	// Setup logging
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// Setup Redis (optional - will work without Redis too)
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

	// Demo 1: Text Embedding
	fmt.Println("\n🔤 Text Embedding Demo")
	fmt.Println("----------------------")

	texts := []string{
		"The quick brown fox jumps over the lazy dog",
		"A fast brown fox leaps over a sleepy dog",
		"The weather is sunny and warm today",
		"Machine learning is transforming technology",
	}

	fmt.Println("Generating embeddings for sample texts...")
	start := time.Now()

	for i, text := range texts {
		embedding, err := mlService.GenerateTextEmbedding(text, "all-MiniLM-L6-v2")
		if err != nil {
			fmt.Printf("❌ Error generating embedding for text %d: %v\n", i+1, err)
			continue
		}

		fmt.Printf("   Text %d: %d dimensions (first 5: [%.3f, %.3f, %.3f, %.3f, %.3f])\n",
			i+1, len(embedding), embedding[0], embedding[1], embedding[2], embedding[3], embedding[4])
	}

	fmt.Printf("⏱️  Text embedding time: %v\n", time.Since(start))

	// Demo 2: Batch Processing
	fmt.Println("\n📦 Batch Processing Demo")
	fmt.Println("------------------------")

	start = time.Now()
	batchEmbeddings, err := mlService.GenerateBatchTextEmbeddings(texts, "all-MiniLM-L6-v2")
	if err != nil {
		fmt.Printf("❌ Batch processing error: %v\n", err)
	} else {
		fmt.Printf("✅ Generated %d embeddings in batch\n", len(batchEmbeddings))
		fmt.Printf("⏱️  Batch processing time: %v\n", time.Since(start))

		// Calculate similarity between first two texts (should be high)
		if len(batchEmbeddings) >= 2 {
			similarity := cosineSimilarity(batchEmbeddings[0], batchEmbeddings[1])
			fmt.Printf("📈 Similarity between text 1 & 2: %.3f (should be high - similar meaning)\n", similarity)

			similarity2 := cosineSimilarity(batchEmbeddings[0], batchEmbeddings[2])
			fmt.Printf("📉 Similarity between text 1 & 3: %.3f (should be lower - different topics)\n", similarity2)
		}
	}

	// Demo 3: Image Embedding (will fail without valid image, but shows the interface)
	fmt.Println("\n🖼️  Image Embedding Demo")
	fmt.Println("------------------------")

	imageURL := "https://httpbin.org/image/jpeg" // Test image URL
	fmt.Printf("Attempting to generate embedding for: %s\n", imageURL)

	start = time.Now()
	imageEmbedding, metadata, err := mlService.GenerateImageEmbedding(imageURL, "clip-vit-base-patch32")
	if err != nil {
		fmt.Printf("⚠️  Image embedding failed (expected without valid image): %v\n", err)
		fmt.Println("   To test image embeddings, provide a valid image URL")
	} else {
		fmt.Printf("✅ Generated image embedding: %d dimensions\n", len(imageEmbedding))
		fmt.Printf("   Image metadata: %dx%d, format: %s, size: %d bytes\n",
			metadata.Width, metadata.Height, metadata.Format, metadata.Size)
		fmt.Printf("⏱️  Image embedding time: %v\n", time.Since(start))
	}

	// Demo 4: Multi-Modal Fusion (will use mock data if image fails)
	fmt.Println("\n🔗 Multi-Modal Fusion Demo")
	fmt.Println("--------------------------")

	text := "A beautiful sunset over the ocean with orange and pink colors"
	fmt.Printf("Text: %s\n", text)
	fmt.Printf("Image: %s\n", imageURL)

	start = time.Now()
	fusionResult, err := mlService.GenerateMultiModalEmbedding(
		text, imageURL,
		"all-MiniLM-L6-v2", "clip-vit-base-patch32")

	if err != nil {
		fmt.Printf("⚠️  Multi-modal fusion failed: %v\n", err)
		fmt.Println("   This is expected without a valid image URL")
	} else {
		fmt.Printf("✅ Multi-modal fusion successful!\n")
		fmt.Printf("   Text embedding: %d dims\n", len(fusionResult.TextEmbedding))
		fmt.Printf("   Image embedding: %d dims\n", len(fusionResult.ImageEmbedding))
		fmt.Printf("   Fused embedding: %d dims\n", len(fusionResult.FusedEmbedding))
		fmt.Printf("   Final embedding: %d dims\n", len(fusionResult.FinalEmbedding))
		fmt.Printf("   Fusion method: %s\n", fusionResult.FusionMethod)
		fmt.Printf("⏱️  Fusion time: %v\n", time.Since(start))
	}

	// Demo 5: Performance Metrics
	fmt.Println("\n📊 Performance Metrics")
	fmt.Println("----------------------")

	metrics := mlService.GetMetrics()
	fmt.Printf("Total requests: %d\n", metrics.TotalRequests)
	fmt.Printf("Successful requests: %d\n", metrics.SuccessfulRequests)
	fmt.Printf("Failed requests: %d\n", metrics.FailedRequests)
	fmt.Printf("Average latency: %.2f ms\n", metrics.AverageLatencyMs)
	fmt.Printf("Cache hit rate: %.1f%%\n", metrics.CacheHitRate*100)
	fmt.Printf("Models loaded: %d\n", metrics.ModelsLoaded)

	fmt.Println("\n🎉 Demo completed!")
	fmt.Println("\nKey Benefits:")
	fmt.Println("✅ No external API calls - everything runs locally")
	fmt.Println("✅ No OpenAI API key required")
	fmt.Println("✅ Fast inference with caching")
	fmt.Println("✅ Privacy-preserving - data never leaves your system")
	fmt.Println("✅ Cost-effective - no per-request charges")
	fmt.Println("\nTo run with real models:")
	fmt.Println("1. Download models: ./scripts/download_onnx_models.sh")
	fmt.Println("2. Start Redis: redis-server")
	fmt.Println("3. Run demo: go run examples/ml_demo.go")
}
