# ML Models Directory

This directory contains the ONNX model files used by the recommendation engine.

## Required Models

### Text Embedding Model
- **File**: `all-MiniLM-L6-v2.onnx`
- **Source**: Sentence Transformers (all-MiniLM-L6-v2)
- **Dimensions**: 384
- **Description**: Lightweight text embedding model for semantic similarity

### Image Embedding Model
- **File**: `clip-vit-base-patch32.onnx`
- **Source**: OpenAI CLIP (ViT-B/32)
- **Dimensions**: 512
- **Description**: Vision transformer for image embeddings

## Model Download Instructions

### Option 1: Download Pre-converted ONNX Models

```bash
# Create models directory
mkdir -p models

# Download text embedding model (example URLs - replace with actual)
wget -O models/all-MiniLM-L6-v2.onnx \
  "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/model.onnx"

# Download image embedding model
wget -O models/clip-vit-base-patch32.onnx \
  "https://huggingface.co/openai/clip-vit-base-patch32/resolve/main/model.onnx"
```

### Option 2: Convert Models from PyTorch

```python
# Convert text model
from sentence_transformers import SentenceTransformer
import torch

model = SentenceTransformer('all-MiniLM-L6-v2')
model.save('./models/all-MiniLM-L6-v2')

# Convert to ONNX (requires additional conversion steps)
```

### Option 3: Use Optimum for Conversion

```bash
pip install optimum[onnxruntime]

# Convert text model
optimum-cli export onnx --model sentence-transformers/all-MiniLM-L6-v2 models/all-MiniLM-L6-v2/

# Convert image model
optimum-cli export onnx --model openai/clip-vit-base-patch32 models/clip-vit-base-patch32/
```

## Model Configuration

The models are configured in the application config:

```yaml
models:
  text-embedding:
    name: "all-MiniLM-L6-v2"
    path: "./models/all-MiniLM-L6-v2.onnx"
    type: "text"
    dimensions: 384
    version: "1.0.0"
    config:
      max_sequence_length: 512
      do_lower_case: true

  image-embedding:
    name: "clip-vit-base-patch32"
    path: "./models/clip-vit-base-patch32.onnx"
    type: "image"
    dimensions: 512
    version: "1.0.0"
    config:
      image_size: 224
      patch_size: 32
      num_channels: 3
```

## Model Versioning

Models are versioned using:
- File path
- Configuration parameters
- Content hash

This ensures cache invalidation when models are updated.

## Performance Considerations

### Memory Usage
- Text model: ~90MB
- Image model: ~350MB
- Total: ~440MB for both models

### Inference Speed
- Text embedding: ~10-50ms per text (depending on length)
- Image embedding: ~100-200ms per image
- Batch processing significantly improves throughput

### Optimization Tips
1. Use model pooling for concurrent requests
2. Batch similar requests together
3. Cache embeddings aggressively
4. Consider quantized models for production

## Troubleshooting

### Common Issues

1. **Model not found**: Ensure models are downloaded to correct paths
2. **ONNX runtime errors**: Check ONNX runtime installation
3. **Memory issues**: Monitor model loading and consider lazy loading
4. **Performance issues**: Use batch processing and caching

### Validation

Test model loading:

```bash
go test -v ./internal/ml -run TestModelRegistry
```

Test embedding generation:

```bash
go test -v ./internal/ml -run TestTextEmbeddingService
```

## Future Enhancements

1. **Model Hot-swapping**: Update models without service restart
2. **A/B Testing**: Compare different model versions
3. **Quantization**: Reduce model size and improve speed
4. **GPU Support**: Leverage GPU acceleration for inference
5. **Custom Models**: Support for domain-specific fine-tuned models