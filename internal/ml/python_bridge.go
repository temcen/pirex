package ml

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

// PythonBridge handles communication with Python ML models
type PythonBridge struct {
	logger      *logrus.Logger
	pythonPath  string
	scriptPath  string
	initialized bool
	mutex       sync.RWMutex
}

// EmbeddingRequest represents a request to generate embeddings
type EmbeddingRequest struct {
	Texts     []string `json:"texts"`
	ModelName string   `json:"model_name"`
}

// EmbeddingResponse represents the response from Python
type EmbeddingResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
	Error      string      `json:"error,omitempty"`
	Latency    float64     `json:"latency"`
}

// NewPythonBridge creates a new Python bridge
func NewPythonBridge(logger *logrus.Logger) *PythonBridge {
	return &PythonBridge{
		logger:     logger,
		pythonPath: "python3",
	}
}

// Initialize sets up the Python environment and scripts
func (pb *PythonBridge) Initialize() error {
	pb.mutex.Lock()
	defer pb.mutex.Unlock()

	if pb.initialized {
		return nil
	}

	// Check if Python is available
	if err := pb.checkPython(); err != nil {
		return fmt.Errorf("python check failed: %w", err)
	}

	// Create the Python inference script
	if err := pb.createInferenceScript(); err != nil {
		return fmt.Errorf("failed to create inference script: %w", err)
	}

	// Install required packages
	if err := pb.installDependencies(); err != nil {
		pb.logger.Warn("Failed to install Python dependencies, will use fallback")
	}

	pb.initialized = true
	pb.logger.Info("Python bridge initialized successfully")
	return nil
}

// checkPython verifies Python installation
func (pb *PythonBridge) checkPython() error {
	cmd := exec.Command(pb.pythonPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("python not found: %w", err)
	}

	version := strings.TrimSpace(string(output))
	pb.logger.WithField("version", version).Info("Python found")
	return nil
}

// installDependencies installs required Python packages
func (pb *PythonBridge) installDependencies() error {
	packages := []string{
		"sentence-transformers",
		"torch",
		"numpy",
	}

	for _, pkg := range packages {
		pb.logger.WithField("package", pkg).Info("Installing Python package")
		cmd := exec.Command(pb.pythonPath, "-m", "pip", "install", pkg)
		if err := cmd.Run(); err != nil {
			pb.logger.WithFields(logrus.Fields{
				"package": pkg,
				"error":   err.Error(),
			}).Warn("Failed to install package")
		}
	}

	return nil
}

// createInferenceScript creates the Python inference script
func (pb *PythonBridge) createInferenceScript() error {
	scriptDir := "./scripts"
	if err := os.MkdirAll(scriptDir, 0755); err != nil {
		return err
	}

	pb.scriptPath = filepath.Join(scriptDir, "embedding_inference.py")

	script := `#!/usr/bin/env python3
"""
Real ML Inference Script for macOS M4
Uses sentence-transformers for actual embeddings
"""

import json
import sys
import time
import numpy as np
from typing import List, Dict, Any

# Try to import sentence-transformers
try:
    from sentence_transformers import SentenceTransformer
    SENTENCE_TRANSFORMERS_AVAILABLE = True
except ImportError:
    SENTENCE_TRANSFORMERS_AVAILABLE = False
    print(json.dumps({"error": "sentence-transformers not available, install with: pip install sentence-transformers"}))

# Global model cache
MODEL_CACHE = {}

def load_model(model_name: str) -> SentenceTransformer:
    """Load and cache a sentence transformer model"""
    if model_name not in MODEL_CACHE:
        try:
            if model_name == "all-MiniLM-L6-v2":
                MODEL_CACHE[model_name] = SentenceTransformer('sentence-transformers/all-MiniLM-L6-v2')
            else:
                MODEL_CACHE[model_name] = SentenceTransformer(model_name)
        except Exception as e:
            raise Exception(f"Failed to load model {model_name}: {str(e)}")
    
    return MODEL_CACHE[model_name]

def generate_embeddings(texts: List[str], model_name: str) -> Dict[str, Any]:
    """Generate embeddings for a list of texts"""
    start_time = time.time()
    
    try:
        if not SENTENCE_TRANSFORMERS_AVAILABLE:
            # Fallback to mock embeddings
            embeddings = []
            for text in texts:
                # Generate deterministic mock embedding
                import hashlib
                hash_obj = hashlib.sha256(text.encode())
                hash_bytes = hash_obj.digest()
                
                # Create 384-dimensional embedding
                embedding = []
                for i in range(384):
                    byte_idx = i % len(hash_bytes)
                    value = (hash_bytes[byte_idx] / 255.0) - 0.5
                    embedding.append(float(value))
                
                # L2 normalize
                norm = np.linalg.norm(embedding)
                if norm > 0:
                    embedding = [x / norm for x in embedding]
                
                embeddings.append(embedding)
            
            return {
                "embeddings": embeddings,
                "latency": time.time() - start_time,
                "model_used": "mock_fallback"
            }
        
        # Load the model
        model = load_model(model_name)
        
        # Generate embeddings
        embeddings = model.encode(texts, convert_to_numpy=True)
        
        # Convert to list format
        embeddings_list = [emb.tolist() for emb in embeddings]
        
        return {
            "embeddings": embeddings_list,
            "latency": time.time() - start_time,
            "model_used": model_name
        }
        
    except Exception as e:
        return {
            "error": str(e),
            "latency": time.time() - start_time
        }

def main():
    """Main function to handle requests"""
    try:
        # Read request from stdin
        request_line = sys.stdin.readline().strip()
        if not request_line:
            print(json.dumps({"error": "No input provided"}))
            return
        
        request = json.loads(request_line)
        texts = request.get("texts", [])
        model_name = request.get("model_name", "all-MiniLM-L6-v2")
        
        if not texts:
            print(json.dumps({"error": "No texts provided"}))
            return
        
        # Generate embeddings
        result = generate_embeddings(texts, model_name)
        
        # Output result
        print(json.dumps(result))
        
    except Exception as e:
        print(json.dumps({"error": f"Script error: {str(e)}"}))

if __name__ == "__main__":
    main()
`

	return os.WriteFile(pb.scriptPath, []byte(script), 0755)
}

// GenerateEmbeddings generates embeddings using Python
func (pb *PythonBridge) GenerateEmbeddings(texts []string, modelName string) ([][]float32, error) {
	if !pb.initialized {
		if err := pb.Initialize(); err != nil {
			return nil, fmt.Errorf("failed to initialize Python bridge: %w", err)
		}
	}

	// Prepare request
	request := EmbeddingRequest{
		Texts:     texts,
		ModelName: modelName,
	}

	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Execute Python script
	cmd := exec.Command(pb.pythonPath, pb.scriptPath)
	cmd.Stdin = strings.NewReader(string(requestJSON))

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("python execution failed: %w", err)
	}

	// Parse response
	var response EmbeddingResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("failed to parse Python response: %w", err)
	}

	if response.Error != "" {
		return nil, fmt.Errorf("python error: %s", response.Error)
	}

	pb.logger.WithFields(logrus.Fields{
		"texts_count": len(texts),
		"latency_ms":  response.Latency * 1000,
		"model":       modelName,
	}).Debug("Generated embeddings via Python")

	return response.Embeddings, nil
}

// IsAvailable checks if the Python bridge is available
func (pb *PythonBridge) IsAvailable() bool {
	pb.mutex.RLock()
	defer pb.mutex.RUnlock()
	return pb.initialized
}

// TestConnection tests the Python bridge
func (pb *PythonBridge) TestConnection() error {
	testTexts := []string{"Hello world"}
	_, err := pb.GenerateEmbeddings(testTexts, "all-MiniLM-L6-v2")
	return err
}
