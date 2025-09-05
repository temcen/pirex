#!/bin/bash

# ML Setup Script
# Complete setup for local ONNX-based ML inference

set -e

echo "🚀 Setting up ML inference with local ONNX models"
echo "================================================="

# Check prerequisites
echo "🔍 Checking prerequisites..."

# Check Go installation
if ! command -v go >/dev/null 2>&1; then
    echo "❌ Go is not installed. Please install Go 1.21+ first."
    exit 1
fi

GO_VERSION=$(go version | grep -o 'go[0-9]\+\.[0-9]\+' | sed 's/go//')
echo "✅ Go version: $GO_VERSION"

# Check Redis (optional)
if command -v redis-server >/dev/null 2>&1; then
    echo "✅ Redis available (caching will be enabled)"
    REDIS_AVAILABLE=true
else
    echo "⚠️  Redis not found (will work without caching)"
    REDIS_AVAILABLE=false
fi

# Create directories
echo ""
echo "📁 Creating directories..."
mkdir -p models
mkdir -p logs
mkdir -p config
echo "✅ Directories created"

# Download dependencies
echo ""
echo "📦 Installing Go dependencies..."
go mod tidy
echo "✅ Dependencies installed"

# Download models
echo ""
echo "🤖 Downloading ONNX models..."
if [ -f "scripts/download_onnx_models.sh" ]; then
    chmod +x scripts/download_onnx_models.sh
    ./scripts/download_onnx_models.sh
else
    echo "⚠️  Model download script not found. You'll need to download models manually."
    echo "   See docs/ONNX_SETUP.md for instructions"
fi

# Test setup
echo ""
echo "🧪 Testing setup..."

# Test model registry
echo "Testing model registry..."
if go test -v ./internal/ml -run TestModelRegistry >/dev/null 2>&1; then
    echo "✅ Model registry tests passed"
else
    echo "⚠️  Model registry tests failed (may need actual model files)"
fi

# Test text embedding service
echo "Testing text embedding service..."
if go test -v ./internal/ml -run TestTextEmbeddingService >/dev/null 2>&1; then
    echo "✅ Text embedding tests passed"
else
    echo "⚠️  Text embedding tests failed"
fi

# Start Redis if available
if [ "$REDIS_AVAILABLE" = true ]; then
    echo ""
    echo "🔄 Starting Redis server..."
    if pgrep redis-server >/dev/null; then
        echo "✅ Redis already running"
    else
        echo "Starting Redis in background..."
        redis-server --daemonize yes --port 6379
        sleep 2
        if pgrep redis-server >/dev/null; then
            echo "✅ Redis started successfully"
        else
            echo "⚠️  Failed to start Redis"
        fi
    fi
fi

# Run demo
echo ""
echo "🎯 Running ML demo..."
echo "This will demonstrate local ONNX inference capabilities"
echo ""

if go run examples/ml_demo.go; then
    echo ""
    echo "🎉 Setup completed successfully!"
else
    echo ""
    echo "⚠️  Demo failed - check model files and dependencies"
fi

echo ""
echo "📚 Next Steps:"
echo "=============="
echo ""
echo "1. 📖 Read the setup guide:"
echo "   cat docs/ONNX_SETUP.md"
echo ""
echo "2. 🧪 Run tests:"
echo "   go test -v ./internal/ml"
echo ""
echo "3. 🚀 Run the demo:"
echo "   go run examples/ml_demo.go"
echo ""
echo "4. 📊 Run benchmarks:"
echo "   go test -bench=. ./internal/ml"
echo ""
echo "5. 🔧 Customize configuration:"
echo "   edit config/ml.yaml"
echo ""
echo "🎯 Key Benefits of This Setup:"
echo "• No OpenAI API key required"
echo "• No external service dependencies"  
echo "• Complete privacy - data never leaves your system"
echo "• Fast local inference with caching"
echo "• Cost-effective - no per-request charges"
echo "• Offline capable"
echo ""
echo "Happy coding! 🚀"