package ml

import (
	"fmt"
	"math"

	"github.com/sirupsen/logrus"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
)

// MultiModalFusionService handles fusion of text and image embeddings
type MultiModalFusionService struct {
	textService  *TextEmbeddingService
	imageService *ImageEmbeddingService
	logger       *logrus.Logger

	// Configuration
	textDimensions  int
	imageDimensions int
	fusedDimensions int
	finalDimensions int

	// Projection layer for dimensionality reduction
	projectionMatrix *mat.Dense
	projectionBias   []float32

	// Fusion weights
	textWeight  float32
	imageWeight float32
}

// MultiModalFusionConfig contains configuration for the fusion service
type MultiModalFusionConfig struct {
	TextDimensions  int     `json:"text_dimensions"`
	ImageDimensions int     `json:"image_dimensions"`
	FinalDimensions int     `json:"final_dimensions"`
	TextWeight      float32 `json:"text_weight"`
	ImageWeight     float32 `json:"image_weight"`
}

// FusionResult contains the result of multi-modal fusion
type FusionResult struct {
	TextEmbedding  []float32 `json:"text_embedding"`
	ImageEmbedding []float32 `json:"image_embedding"`
	FusedEmbedding []float32 `json:"fused_embedding"`
	FinalEmbedding []float32 `json:"final_embedding"`
	FusionMethod   string    `json:"fusion_method"`
	TextWeight     float32   `json:"text_weight"`
	ImageWeight    float32   `json:"image_weight"`
}

// NewMultiModalFusionService creates a new multi-modal fusion service
func NewMultiModalFusionService(
	textService *TextEmbeddingService,
	imageService *ImageEmbeddingService,
	logger *logrus.Logger,
	config MultiModalFusionConfig,
) *MultiModalFusionService {
	if config.TextDimensions == 0 {
		config.TextDimensions = 384 // all-MiniLM-L6-v2 dimensions
	}
	if config.ImageDimensions == 0 {
		config.ImageDimensions = 512 // CLIP dimensions
	}
	if config.FinalDimensions == 0 {
		config.FinalDimensions = 768 // Standard final dimensions
	}
	if config.TextWeight == 0 {
		config.TextWeight = 0.6
	}
	if config.ImageWeight == 0 {
		config.ImageWeight = 0.4
	}

	service := &MultiModalFusionService{
		textService:     textService,
		imageService:    imageService,
		logger:          logger,
		textDimensions:  config.TextDimensions,
		imageDimensions: config.ImageDimensions,
		fusedDimensions: config.TextDimensions + config.ImageDimensions, // 896 for late fusion
		finalDimensions: config.FinalDimensions,
		textWeight:      config.TextWeight,
		imageWeight:     config.ImageWeight,
	}

	// Initialize projection layer
	service.initializeProjectionLayer()

	return service
}

// initializeProjectionLayer initializes the learned projection layer
func (mmfs *MultiModalFusionService) initializeProjectionLayer() {
	// Initialize projection matrix with Xavier initialization
	projectionMatrix := mat.NewDense(mmfs.finalDimensions, mmfs.fusedDimensions, nil)

	// Xavier initialization: weights ~ N(0, sqrt(2/(fan_in + fan_out)))
	fanIn := float64(mmfs.fusedDimensions)
	fanOut := float64(mmfs.finalDimensions)
	stddev := math.Sqrt(2.0 / (fanIn + fanOut))

	// Fill with random values (simplified - in practice would use proper random initialization)
	for i := 0; i < mmfs.finalDimensions; i++ {
		for j := 0; j < mmfs.fusedDimensions; j++ {
			// Use deterministic initialization for reproducibility
			value := math.Sin(float64(i*mmfs.fusedDimensions+j)) * stddev
			projectionMatrix.Set(i, j, value)
		}
	}

	mmfs.projectionMatrix = projectionMatrix

	// Initialize bias to zero
	mmfs.projectionBias = make([]float32, mmfs.finalDimensions)
}

// GenerateMultiModalEmbedding generates a fused embedding from text and image
func (mmfs *MultiModalFusionService) GenerateMultiModalEmbedding(
	text string,
	imageURL string,
	textModelName string,
	imageModelName string,
) (*FusionResult, error) {

	// Generate text embedding
	textEmbedding, err := mmfs.textService.GenerateEmbedding(text, textModelName)
	if err != nil {
		return nil, fmt.Errorf("failed to generate text embedding: %w", err)
	}

	// Generate image embedding
	imageEmbedding, _, err := mmfs.imageService.GenerateEmbeddingFromURL(imageURL, imageModelName)
	if err != nil {
		return nil, fmt.Errorf("failed to generate image embedding: %w", err)
	}

	return mmfs.fuseEmbeddings(textEmbedding, imageEmbedding)
}

// GenerateMultiModalEmbeddingFromData generates a fused embedding from text and image data
func (mmfs *MultiModalFusionService) GenerateMultiModalEmbeddingFromData(
	text string,
	imageData []byte,
	textModelName string,
	imageModelName string,
) (*FusionResult, error) {

	// Generate text embedding
	textEmbedding, err := mmfs.textService.GenerateEmbedding(text, textModelName)
	if err != nil {
		return nil, fmt.Errorf("failed to generate text embedding: %w", err)
	}

	// Generate image embedding
	imageEmbedding, _, err := mmfs.imageService.GenerateEmbeddingFromData(imageData, imageModelName)
	if err != nil {
		return nil, fmt.Errorf("failed to generate image embedding: %w", err)
	}

	return mmfs.fuseEmbeddings(textEmbedding, imageEmbedding)
}

// fuseEmbeddings performs the actual fusion of text and image embeddings
func (mmfs *MultiModalFusionService) fuseEmbeddings(textEmbedding, imageEmbedding []float32) (*FusionResult, error) {
	// Validate dimensions
	if len(textEmbedding) != mmfs.textDimensions {
		return nil, fmt.Errorf("text embedding dimension mismatch: expected %d, got %d",
			mmfs.textDimensions, len(textEmbedding))
	}
	if len(imageEmbedding) != mmfs.imageDimensions {
		return nil, fmt.Errorf("image embedding dimension mismatch: expected %d, got %d",
			mmfs.imageDimensions, len(imageEmbedding))
	}

	// Normalize embeddings
	normalizedText := mmfs.l2Normalize(textEmbedding)
	normalizedImage := mmfs.l2Normalize(imageEmbedding)

	// Late fusion: concatenate normalized embeddings
	fusedEmbedding := mmfs.lateFusion(normalizedText, normalizedImage)

	// Apply learned projection to final dimensions
	finalEmbedding := mmfs.applyProjection(fusedEmbedding)

	return &FusionResult{
		TextEmbedding:  normalizedText,
		ImageEmbedding: normalizedImage,
		FusedEmbedding: fusedEmbedding,
		FinalEmbedding: finalEmbedding,
		FusionMethod:   "late_fusion_with_projection",
		TextWeight:     mmfs.textWeight,
		ImageWeight:    mmfs.imageWeight,
	}, nil
}

// lateFusion performs late fusion by concatenating embeddings
func (mmfs *MultiModalFusionService) lateFusion(textEmbedding, imageEmbedding []float32) []float32 {
	// Simple concatenation for late fusion
	fused := make([]float32, len(textEmbedding)+len(imageEmbedding))

	// Copy text embedding
	copy(fused[:len(textEmbedding)], textEmbedding)

	// Copy image embedding
	copy(fused[len(textEmbedding):], imageEmbedding)

	return fused
}

// earlyFusion performs early fusion with weighted combination (hook for future enhancement)
func (mmfs *MultiModalFusionService) earlyFusion(textEmbedding, imageEmbedding []float32) []float32 {
	// This is a hook for future early fusion implementation
	// For now, pad shorter embedding to match dimensions and combine

	maxDim := max(mmfs.textDimensions, mmfs.imageDimensions)

	// Pad embeddings to same dimension
	paddedText := mmfs.padEmbedding(textEmbedding, maxDim)
	paddedImage := mmfs.padEmbedding(imageEmbedding, maxDim)

	// Weighted combination
	fused := make([]float32, maxDim)
	for i := 0; i < maxDim; i++ {
		fused[i] = mmfs.textWeight*paddedText[i] + mmfs.imageWeight*paddedImage[i]
	}

	return fused
}

// padEmbedding pads an embedding to target dimension
func (mmfs *MultiModalFusionService) padEmbedding(embedding []float32, targetDim int) []float32 {
	if len(embedding) >= targetDim {
		return embedding[:targetDim]
	}

	padded := make([]float32, targetDim)
	copy(padded, embedding)
	// Remaining elements are zero-padded

	return padded
}

// applyProjection applies the learned projection layer
func (mmfs *MultiModalFusionService) applyProjection(fusedEmbedding []float32) []float32 {
	// Convert to matrix for multiplication
	input := mat.NewDense(1, len(fusedEmbedding), nil)
	for i, v := range fusedEmbedding {
		input.Set(0, i, float64(v))
	}

	// Matrix multiplication: output = input * projection^T
	var output mat.Dense
	output.Mul(input, mmfs.projectionMatrix.T())

	// Convert back to float32 slice and add bias
	result := make([]float32, mmfs.finalDimensions)
	for i := 0; i < mmfs.finalDimensions; i++ {
		result[i] = float32(output.At(0, i)) + mmfs.projectionBias[i]
	}

	// Apply activation function (ReLU)
	for i := range result {
		if result[i] < 0 {
			result[i] = 0
		}
	}

	// L2 normalize final embedding
	return mmfs.l2Normalize(result)
}

// l2Normalize performs L2 normalization on an embedding vector
func (mmfs *MultiModalFusionService) l2Normalize(embedding []float32) []float32 {
	// Convert to float64 for precision
	vec := make([]float64, len(embedding))
	for i, v := range embedding {
		vec[i] = float64(v)
	}

	// Calculate L2 norm
	norm := floats.Norm(vec, 2)
	if norm == 0 {
		return embedding // Avoid division by zero
	}

	// Normalize
	normalized := make([]float32, len(embedding))
	for i, v := range vec {
		normalized[i] = float32(v / norm)
	}

	return normalized
}

// UpdateProjectionWeights updates the projection layer weights (hook for training)
func (mmfs *MultiModalFusionService) UpdateProjectionWeights(weights [][]float64, bias []float32) error {
	if len(weights) != mmfs.finalDimensions || len(weights[0]) != mmfs.fusedDimensions {
		return fmt.Errorf("weight matrix dimension mismatch")
	}
	if len(bias) != mmfs.finalDimensions {
		return fmt.Errorf("bias vector dimension mismatch")
	}

	// Update projection matrix
	for i := 0; i < mmfs.finalDimensions; i++ {
		for j := 0; j < mmfs.fusedDimensions; j++ {
			mmfs.projectionMatrix.Set(i, j, weights[i][j])
		}
	}

	// Update bias
	copy(mmfs.projectionBias, bias)

	mmfs.logger.Info("Projection layer weights updated")
	return nil
}

// GetFusionStats returns fusion service statistics
func (mmfs *MultiModalFusionService) GetFusionStats() map[string]any {
	return map[string]interface{}{
		"text_dimensions":  mmfs.textDimensions,
		"image_dimensions": mmfs.imageDimensions,
		"fused_dimensions": mmfs.fusedDimensions,
		"final_dimensions": mmfs.finalDimensions,
		"text_weight":      mmfs.textWeight,
		"image_weight":     mmfs.imageWeight,
		"fusion_method":    "late_fusion_with_projection",
	}
}
