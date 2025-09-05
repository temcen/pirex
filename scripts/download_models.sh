#!/bin/bash

# Download ML Models Script
# This script downloads the required ONNX models for the recommendation engine

set -e

MODELS_DIR="./models"
mkdir -p "$MODELS_DIR"

echo "🚀 Downloading ML models for recommendation engine..."

# Function to download with progress
download_with_progress() {
    local url=$1
    local output=$2
    local description=$3
    
    echo "📥 Downloading $description..."
    if command -v wget >/dev/null 2>&1; then
        wget --progress=bar:force:noscroll -O "$output" "$url"
    elif command -v curl >/dev/null 2>&1; then
        curl -L --progress-bar -o "$output" "$url"
    else
        echo "❌ Error: Neither wget nor curl is available"
        exit 1
    fi
    echo "✅ Downloaded $description"
}

# Download text embedding model (all-MiniLM-L6-v2)
TEXT_MODEL_URL="https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/pytorch_model.bin"
TEXT_MODEL_CONFIG_URL="https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/config.json"
TEXT_TOKENIZER_URL="https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/tokenizer.json"

# Note: These are PyTorch models. For ONNX, we need to convert them or find pre-converted versions
echo "📝 Note: Downloading PyTorch models. ONNX conversion required."

# Create text model directory
mkdir -p "$MODELS_DIR/all-MiniLM-L6-v2"

# Download text model files
download_with_progress "$TEXT_MODEL_CONFIG_URL" "$MODELS_DIR/all-MiniLM-L6-v2/config.json" "Text model config"
download_with_progress "$TEXT_TOKENIZER_URL" "$MODELS_DIR/all-MiniLM-L6-v2/tokenizer.json" "Text tokenizer"

# Download image embedding model (CLIP)
CLIP_MODEL_URL="https://huggingface.co/openai/clip-vit-base-patch32/resolve/main/pytorch_model.bin"
CLIP_CONFIG_URL="https://huggingface.co/openai/clip-vit-base-patch32/resolve/main/config.json"

# Create image model directory
mkdir -p "$MODELS_DIR/clip-vit-base-patch32"

# Download CLIP model files
download_with_progress "$CLIP_CONFIG_URL" "$MODELS_DIR/clip-vit-base-patch32/config.json" "CLIP model config"

echo ""
echo "🔄 Model Conversion Required"
echo "The downloaded models are in PyTorch format. To use them with ONNX Runtime:"
echo ""
echo "Option 1: Use pre-converted ONNX models from Hugging Face"
echo "Option 2: Convert using Python (recommended)"
echo ""
echo "To convert to ONNX format, run:"
echo "  python scripts/convert_to_onnx.py"
echo ""
echo "Or use the optimized pre-converted models:"
echo "  bash scripts/download_onnx_models.sh"
echo ""