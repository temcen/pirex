# ONNX Models Setup Guide

This guide explains how to set up and use ONNX models locally without any external API dependencies.

## What You Get

✅ **No OpenAI API needed** - Everything runs locally  
✅ **No external service calls** - Complete privacy  
✅ **Fast inference** - Direct model execution  
✅ **Cost-effective** - No per-request charges  
✅ **Offline capable** - Works without internet  

## Prerequisites

### 1. Install ONNX Runtime

The Go application uses ONNX Runtime through Go bindings. The runtime will be automatically downloaded when you build the project.

### 2. System Requirements

- **Memory**: 2GB+ RAM (models use ~500MB)
- **Storage**: 1GB+ free space for models
- **CPU**: Any modern CPU (ARM64/AMD64 supported)
- **OS**: Linux, macOS, Windows

## Quick Start

### 1. Download Models

```bash
# Make scripts executable
chmod +x scripts/download_onnx_models.sh

# Download optimized ONNX models
./scripts/download_onnx_models.sh
```

This downloads:
- **Text Model**: `all-MiniLM-L6-v2.onnx` (~90MB, 384 dimensions)
- **Image Model**: `clip-vit-base-patch32.onnx` (~350MB, 512 dimensions)

### 2. Verify Setup

```bash
# Test model loading
go test -v ./internal/ml -run TestModelRegistry

# Test text embeddings
go test -v ./internal/ml -run TestTextEmbeddingService

# Run performance benchmarks
go test -bench=BenchmarkTextEmbeddingGeneration ./internal/ml
```

### 3. Use in Your Application

```go
package main

import (
    "log"
    "github.com/temcen/pirex/internal/ml"
    "github.com/redis/go-redis/v9"
    "github.com/sirupsen/logrus"
)

func main() {
    // Setup
    logger := logrus.New()
    redisClient := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })
    
    // Create ML service with default config
    config := ml.DefaultMLConfig()
    mlService, err := ml.NewMLService(redisClient, logger, config)
    if err != nil {
        log.Fatal(err)
    }
    defer mlService.Stop()
    
    // Generate text embedding
    text := "This is a sample text for embedding"
    embedding, err := mlService.GenerateTextEmbedding(text, "all-MiniLM-L6-v2")
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Generated embedding with %d dimensions", len(embedding))
    
    // Generate image embedding
    imageURL := "https://example.com/image.jpg"
    imageEmbedding, metadata, err := mlService.GenerateImageEmbedding(imageURL, "clip-vit-base-patch32")
    if err != nil {
        log.Printf("Image embedding failed: %v", err)
    } else {
        log.Printf("Generated image embedding: %d dimensions, image: %dx%d", 
            len(imageEmbedding), metadata.Width, metadata.Height)
    }
    
    // Generate multi-modal embedding
    result, err := mlService.GenerateMultiModalEmbedding(
        text, imageURL, 
        "all-MiniLM-L6-v2", "clip-vit-base-patch32")
    if err != nil {
        log.Printf("Multi-modal embedding failed: %v", err)
    } else {
        log.Printf("Fused embedding: %d dimensions", len(result.FinalEmbedding))
    }
}
```

## Model Details

### Text Embedding Model (all-MiniLM-L6-v2)

- **Purpose**: Convert text to 384-dimensional vectors
- **Use Cases**: Semantic search, text similarity, clustering
- **Input**: Text strings (up to 512 tokens)
- **Output**: 384-dimensional float32 vector
- **Performance**: ~10-50ms per text on modern CPU

**Example Usage:**
```go
embedding, err := mlService.GenerateTextEmbedding("Hello world", "all-MiniLM-L6-v2")
// embedding is []float32 with 384 elements
```

### Image Embedding Model (CLIP ViT-B/32)

- **Purpose**: Convert images to 512-dimensional vectors
- **Use Cases**: Image search, visual similarity, multi-modal tasks
- **Input**: Images (JPEG, PNG, WebP) up to 10MB
- **Output**: 512-dimensional float32 vector
- **Performance**: ~100-200ms per image on modern CPU

**Example Usage:**
```go
embedding, metadata, err := mlService.GenerateImageEmbedding(
    "https://example.com/image.jpg", 
    "clip-vit-base-patch32")
// embedding is []float32 with 512 elements
```

### Multi-Modal Fusion

- **Purpose**: Combine text and image embeddings
- **Method**: Late fusion with learned projection
- **Input**: Text + Image embeddings
- **Output**: 768-dimensional unified vector
- **Use Cases**: Multi-modal search, content recommendation

## Performance Optimization

### 1. Caching Strategy

The system uses Redis for intelligent caching:

```yaml
# Hot cache (frequently accessed)
redis_hot:
  addr: "localhost:6379"
  ttl: "1h"

# Warm cache (moderately accessed)  
redis_warm:
  addr: "localhost:6380"
  ttl: "24h"

# Cold cache (long-term storage)
redis_cold:
  addr: "localhost:6381" 
  ttl: "7d"
```

### 2. Batch Processing

Process multiple texts efficiently:

```go
texts := []string{
    "First text to embed",
    "Second text to embed", 
    "Third text to embed",
}

embeddings, err := mlService.GenerateBatchTextEmbeddings(texts, "all-MiniLM-L6-v2")
// embeddings is [][]float32 - one embedding per text
```

### 3. Concurrent Processing

The service uses worker pools for concurrent processing:

```yaml
text_embedding:
  worker_count: 4      # Number of worker goroutines
  batch_size: 32       # Max batch size
  max_tokens: 512      # Max tokens per text
```

### 4. Memory Management

- **Model Pooling**: Reuse model instances across requests
- **Session Caching**: Keep loaded models in memory
- **Garbage Collection**: Automatic cleanup of unused sessions

## Troubleshooting

### Common Issues

1. **"Model not found" error**
   ```bash
   # Ensure models are downloaded
   ls -la models/
   # Should show .onnx files
   ```

2. **"ONNX Runtime error"**
   ```bash
   # Check ONNX Runtime installation
   go mod tidy
   # Rebuild the application
   go build
   ```

3. **Memory issues**
   ```yaml
   # Reduce concurrent workers
   text_embedding:
     worker_count: 2
   # Or increase system memory
   ```

4. **Slow performance**
   ```yaml
   # Enable batch processing
   text_embedding:
     batch_size: 32
   # Use caching
   redis:
     addr: "localhost:6379"
   ```

### Performance Benchmarks

Expected performance on modern hardware:

| Operation | Latency | Throughput |
|-----------|---------|------------|
| Text Embedding | 10-50ms | 20-100 req/sec |
| Image Embedding | 100-200ms | 5-10 req/sec |
| Multi-Modal Fusion | 150-300ms | 3-7 req/sec |
| Batch Text (32) | 200-500ms | 64-160 req/sec |

### Monitoring

Check service health:

```go
// Get performance metrics
metrics := mlService.GetMetrics()
log.Printf("Total requests: %d", metrics.TotalRequests)
log.Printf("Average latency: %.2fms", metrics.AverageLatencyMs)
log.Printf("Cache hit rate: %.2f%%", metrics.CacheHitRate*100)

// Get detailed stats
stats := mlService.GetStats()
log.Printf("Models loaded: %d", len(stats["models"].(map[string]*ml.ModelInfo)))
```

## Advanced Configuration

### Custom Model Paths

```yaml
models:
  custom-text-model:
    name: "custom-model"
    path: "./custom_models/my-model.onnx"
    type: "text"
    dimensions: 768
    version: "2.0.0"
```

### Performance Tuning

```yaml
performance:
  model_pool_size: 10           # Model instance pool
  max_concurrent_requests: 100  # Request concurrency
  batch_timeout: "100ms"        # Batch collection timeout
  gc_percent: 100              # Go GC tuning
```

### Development Mode

```yaml
development:
  mock_mode: true              # Use mock embeddings for testing
  deterministic_embeddings: true # Reproducible results
  debug_logging: true          # Verbose logging
```

## Production Deployment

### Docker Setup

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o recommendation-engine

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/recommendation-engine .
COPY --from=builder /app/models ./models
COPY --from=builder /app/config ./config
CMD ["./recommendation-engine"]
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: recommendation-engine
spec:
  replicas: 3
  selector:
    matchLabels:
      app: recommendation-engine
  template:
    metadata:
      labels:
        app: recommendation-engine
    spec:
      containers:
      - name: recommendation-engine
        image: recommendation-engine:latest
        resources:
          requests:
            memory: "1Gi"
            cpu: "500m"
          limits:
            memory: "2Gi" 
            cpu: "1000m"
        volumeMounts:
        - name: models
          mountPath: /app/models
      volumes:
      - name: models
        persistentVolumeClaim:
          claimName: models-pvc
```

## Summary

This setup gives you:

- **Complete local ML inference** without external dependencies
- **High-performance embedding generation** with caching and batching  
- **Multi-modal capabilities** combining text and images
- **Production-ready architecture** with monitoring and scaling
- **Cost-effective solution** with no per-request charges

The ONNX models run entirely on your infrastructure, providing privacy, performance, and cost benefits over external API services.