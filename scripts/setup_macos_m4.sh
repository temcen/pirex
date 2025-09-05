#!/bin/bash

# macOS M4 ML Setup Script
# Optimized setup for Apple Silicon with real model inference

set -e

echo "🍎 Setting up ML inference for macOS M4"
echo "======================================="

# Check system
echo "🔍 Checking system..."
if [[ $(uname -m) != "arm64" ]]; then
    echo "⚠️  This script is optimized for Apple Silicon (M4), but will work on other systems too"
fi

echo "✅ System: $(uname -s) $(uname -m)"

# Check prerequisites
echo ""
echo "📋 Checking prerequisites..."

# Check Go
if ! command -v go >/dev/null 2>&1; then
    echo "❌ Go is not installed. Please install Go 1.21+ first."
    echo "   Download from: https://golang.org/dl/"
    exit 1
fi

GO_VERSION=$(go version | grep -o 'go[0-9]\+\.[0-9]\+' | sed 's/go//')
echo "✅ Go version: $GO_VERSION"

# Check Python
PYTHON_CMD=""
if command -v python3 >/dev/null 2>&1; then
    PYTHON_CMD="python3"
elif command -v python >/dev/null 2>&1; then
    PYTHON_CMD="python"
else
    echo "❌ Python is not installed. Please install Python 3.8+ first."
    echo "   Install via Homebrew: brew install python"
    exit 1
fi

PYTHON_VERSION=$($PYTHON_CMD --version 2>&1 | grep -o '[0-9]\+\.[0-9]\+')
echo "✅ Python version: $PYTHON_VERSION ($PYTHON_CMD)"

# Check pip
if ! $PYTHON_CMD -m pip --version >/dev/null 2>&1; then
    echo "❌ pip is not available. Please install pip first."
    exit 1
fi

echo "✅ pip available"

# Check Redis (optional)
if command -v redis-server >/dev/null 2>&1; then
    echo "✅ Redis available (caching will be enabled)"
    REDIS_AVAILABLE=true
else
    echo "⚠️  Redis not found (install with: brew install redis)"
    echo "   Will work without caching"
    REDIS_AVAILABLE=false
fi

# Install Python dependencies
echo ""
echo "🐍 Installing Python ML dependencies..."

# Create virtual environment (optional but recommended)
if [ ! -d "venv" ]; then
    echo "Creating Python virtual environment..."
    $PYTHON_CMD -m venv venv
fi

# Activate virtual environment
source venv/bin/activate 2>/dev/null || echo "Virtual environment not activated (continuing anyway)"

# Install packages optimized for Apple Silicon
echo "Installing sentence-transformers (optimized for Apple Silicon)..."
pip install --upgrade pip

# Install PyTorch with Apple Silicon optimization
pip install torch torchvision torchaudio

# Install sentence-transformers
pip install sentence-transformers

# Install additional dependencies
pip install numpy transformers

echo "✅ Python dependencies installed"

# Setup Go dependencies
echo ""
echo "📦 Installing Go dependencies..."
go mod tidy
echo "✅ Go dependencies installed"

# Create directories
echo ""
echo "📁 Creating directories..."
mkdir -p models
mkdir -p logs
mkdir -p scripts
echo "✅ Directories created"

# Test Python bridge
echo ""
echo "🧪 Testing Python ML bridge..."

# Create test script
cat > test_python_bridge.py << 'EOF'
#!/usr/bin/env python3
import json
import sys

try:
    from sentence_transformers import SentenceTransformer
    print(json.dumps({"status": "success", "message": "sentence-transformers available"}))
except ImportError as e:
    print(json.dumps({"status": "error", "message": f"Import failed: {str(e)}"}))
except Exception as e:
    print(json.dumps({"status": "error", "message": f"Error: {str(e)}"}))
EOF

PYTHON_TEST_RESULT=$($PYTHON_CMD test_python_bridge.py)
rm test_python_bridge.py

if echo "$PYTHON_TEST_RESULT" | grep -q '"status": "success"'; then
    echo "✅ Python bridge test passed - real models available"
    REAL_MODELS_AVAILABLE=true
else
    echo "⚠️  Python bridge test failed - will use mock embeddings"
    echo "   Error: $PYTHON_TEST_RESULT"
    REAL_MODELS_AVAILABLE=false
fi

# Start Redis if available
if [ "$REDIS_AVAILABLE" = true ]; then
    echo ""
    echo "🔄 Starting Redis server..."
    if pgrep redis-server >/dev/null; then
        echo "✅ Redis already running"
    else
        echo "Starting Redis in background..."
        redis-server --daemonize yes --port 6379 2>/dev/null || echo "⚠️  Failed to start Redis"
        sleep 2
        if pgrep redis-server >/dev/null; then
            echo "✅ Redis started successfully"
        else
            echo "⚠️  Redis failed to start"
        fi
    fi
fi

# Run tests
echo ""
echo "🧪 Running ML tests..."

echo "Testing model registry..."
if go test -v ./internal/ml -run TestModelRegistry >/dev/null 2>&1; then
    echo "✅ Model registry tests passed"
else
    echo "⚠️  Model registry tests failed"
fi

echo "Testing text embedding service..."
if go test -v ./internal/ml -run TestTextEmbeddingService >/dev/null 2>&1; then
    echo "✅ Text embedding tests passed"
else
    echo "⚠️  Text embedding tests failed"
fi

# Run the real demo
echo ""
echo "🎯 Running Real ML Demo..."
echo "=========================="

if go run examples/ml_demo_real.go; then
    echo ""
    echo "🎉 Setup completed successfully!"
else
    echo ""
    echo "⚠️  Demo failed - check dependencies"
fi

echo ""
echo "📚 Summary"
echo "=========="
echo ""
echo "✅ System: macOS $(uname -m)"
echo "✅ Go: $GO_VERSION"
echo "✅ Python: $PYTHON_VERSION"

if [ "$REAL_MODELS_AVAILABLE" = true ]; then
    echo "✅ Real ML models: Available (sentence-transformers)"
else
    echo "⚠️  Real ML models: Not available (using mock embeddings)"
fi

if [ "$REDIS_AVAILABLE" = true ]; then
    echo "✅ Redis caching: Available"
else
    echo "⚠️  Redis caching: Not available"
fi

echo ""
echo "🚀 Next Steps:"
echo "=============="
echo ""
echo "1. 🎮 Run the real demo:"
echo "   go run examples/ml_demo_real.go"
echo ""
echo "2. 🧪 Run all tests:"
echo "   go test -v ./internal/ml"
echo ""
echo "3. 📊 Run benchmarks:"
echo "   go test -bench=. ./internal/ml"
echo ""
echo "4. 🔧 Customize configuration:"
echo "   edit config/ml.yaml"
echo ""

if [ "$REAL_MODELS_AVAILABLE" = true ]; then
    echo "🎯 Real Model Features:"
    echo "• Actual sentence-transformers inference"
    echo "• Semantic similarity with real understanding"
    echo "• 384-dimensional embeddings from all-MiniLM-L6-v2"
    echo "• Optimized for Apple Silicon M4"
    echo "• Automatic model downloading on first use"
else
    echo "🔧 To Enable Real Models:"
    echo "• Fix Python dependencies: pip install sentence-transformers torch"
    echo "• Ensure Python 3.8+ is available"
    echo "• Run this script again"
fi

echo ""
echo "💡 Key Benefits:"
echo "• No OpenAI API key required"
echo "• Complete privacy - data never leaves your system"
echo "• Optimized for Apple Silicon performance"
echo "• Automatic fallback to mock embeddings"
echo "• Production-ready architecture"
echo ""
echo "Happy coding! 🚀"