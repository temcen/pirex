#!/usr/bin/env python3
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
