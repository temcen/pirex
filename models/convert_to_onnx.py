#!/usr/bin/env python3
"""
Convert PyTorch models to ONNX format
Requires: pip install torch transformers sentence-transformers onnx
"""

import torch
from sentence_transformers import SentenceTransformer
from transformers import CLIPModel
import os

def convert_text_model():
    """Convert sentence transformer to ONNX"""
    try:
        print("🔄 Converting text model to ONNX...")
        model = SentenceTransformer('sentence-transformers/all-MiniLM-L6-v2')
        
        # Export to ONNX
        model.save('./all-MiniLM-L6-v2-pytorch')
        
        # Manual ONNX export would go here
        print("✅ Text model ready (manual ONNX conversion needed)")
        print("   Use: python -m transformers.onnx --model=./all-MiniLM-L6-v2-pytorch onnx/")
        
    except Exception as e:
        print(f"❌ Text model conversion failed: {e}")

def convert_image_model():
    """Convert CLIP model to ONNX"""
    try:
        print("🔄 Converting image model to ONNX...")
        model = CLIPModel.from_pretrained('openai/clip-vit-base-patch32')
        
        print("✅ Image model ready (manual ONNX conversion needed)")
        print("   Use: python -m transformers.onnx --model=openai/clip-vit-base-patch32 onnx/")
        
    except Exception as e:
        print(f"❌ Image model conversion failed: {e}")

if __name__ == "__main__":
    print("🚀 ONNX Model Conversion Script")
    print("===============================")
    
    convert_text_model()
    convert_image_model()
    
    print("\n📚 Next Steps:")
    print("1. Install dependencies: pip install torch transformers sentence-transformers onnx")
    print("2. Run conversion: python convert_to_onnx.py")
    print("3. Or use Optimum: pip install optimum[onnxruntime]")
    print("4. Convert with Optimum: optimum-cli export onnx --model sentence-transformers/all-MiniLM-L6-v2 ./onnx/")
