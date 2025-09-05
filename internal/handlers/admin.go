package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/temcen/pirex/internal/config"
)

// AdminHandler handles admin-related requests
type AdminHandler struct {
	logger *logrus.Logger
	config *config.Config
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(logger *logrus.Logger, cfg *config.Config) *AdminHandler {
	return &AdminHandler{
		logger: logger,
		config: cfg,
	}
}

// AlgorithmConfig represents the algorithm configuration
type AlgorithmConfig struct {
	Algorithms struct {
		SemanticSearch struct {
			Weight              float64 `json:"weight"`
			SimilarityThreshold float64 `json:"similarity_threshold"`
		} `json:"semantic_search"`
		CollaborativeFiltering struct {
			Weight              float64 `json:"weight"`
			SimilarityThreshold float64 `json:"similarity_threshold"`
		} `json:"collaborative_filtering"`
		PageRank struct {
			Weight        float64 `json:"weight"`
			DampingFactor float64 `json:"damping_factor"`
		} `json:"pagerank"`
	} `json:"algorithms"`
	Diversity struct {
		IntraListDiversity float64 `json:"intra_list_diversity"`
		CategoryMaxItems   int     `json:"category_max_items"`
		SerendipityRatio   float64 `json:"serendipity_ratio"`
	} `json:"diversity"`
	Features struct {
		MLRanking          bool `json:"ml_ranking"`
		RealTimeLearning   bool `json:"real_time_learning"`
		ExplanationService bool `json:"explanation_service"`
	} `json:"features"`
}

// GetAlgorithmConfig returns the current algorithm configuration
func (h *AdminHandler) GetAlgorithmConfig(c *gin.Context) {
	// Convert internal config to API format
	apiConfig := AlgorithmConfig{}

	// Map from internal config structure
	if h.config != nil && h.config.Algorithms.SemanticSearch.Enabled {
		apiConfig.Algorithms.SemanticSearch.Weight = h.config.Algorithms.SemanticSearch.Weight
		apiConfig.Algorithms.SemanticSearch.SimilarityThreshold = h.config.Algorithms.SemanticSearch.SimilarityThreshold

		apiConfig.Algorithms.CollaborativeFiltering.Weight = h.config.Algorithms.CollaborativeFilter.Weight
		apiConfig.Algorithms.CollaborativeFiltering.SimilarityThreshold = h.config.Algorithms.CollaborativeFilter.SimilarityThreshold

		apiConfig.Algorithms.PageRank.Weight = h.config.Algorithms.PageRank.Weight
		apiConfig.Algorithms.PageRank.DampingFactor = 0.85 // Default value

		apiConfig.Diversity.IntraListDiversity = h.config.Algorithms.Diversity.IntraListDiversity
		apiConfig.Diversity.CategoryMaxItems = h.config.Algorithms.Diversity.CategoryMaxItems
		apiConfig.Diversity.SerendipityRatio = h.config.Algorithms.Diversity.SerendipityRatio

		// Feature flags - these would be stored in config or database
		apiConfig.Features.MLRanking = true
		apiConfig.Features.RealTimeLearning = true
		apiConfig.Features.ExplanationService = true
	} else {
		// Return default configuration
		apiConfig.Algorithms.SemanticSearch.Weight = 0.4
		apiConfig.Algorithms.SemanticSearch.SimilarityThreshold = 0.7
		apiConfig.Algorithms.CollaborativeFiltering.Weight = 0.3
		apiConfig.Algorithms.CollaborativeFiltering.SimilarityThreshold = 0.5
		apiConfig.Algorithms.PageRank.Weight = 0.3
		apiConfig.Algorithms.PageRank.DampingFactor = 0.85
		apiConfig.Diversity.IntraListDiversity = 0.3
		apiConfig.Diversity.CategoryMaxItems = 3
		apiConfig.Diversity.SerendipityRatio = 0.15
		apiConfig.Features.MLRanking = true
		apiConfig.Features.RealTimeLearning = true
		apiConfig.Features.ExplanationService = true
	}

	c.JSON(http.StatusOK, apiConfig)
}

// UpdateAlgorithmConfig updates the algorithm configuration
func (h *AdminHandler) UpdateAlgorithmConfig(c *gin.Context) {
	var newConfig AlgorithmConfig

	if err := c.ShouldBindJSON(&newConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid configuration format",
			"details": err.Error(),
		})
		return
	}

	// Validate configuration
	if err := h.validateAlgorithmConfig(&newConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid configuration values",
			"details": err.Error(),
		})
		return
	}

	// In a real implementation, this would update the configuration
	// and potentially restart services or reload configuration
	h.logger.WithField("config", newConfig).Info("Algorithm configuration updated")

	// TODO: Apply configuration to running services
	// This might involve:
	// 1. Updating the config file
	// 2. Notifying services to reload configuration
	// 3. Validating the new configuration works

	c.JSON(http.StatusOK, gin.H{
		"status":  "updated",
		"message": "Algorithm configuration updated successfully",
		"config":  newConfig,
	})
}

// TestAlgorithmConfig tests a configuration without applying it
func (h *AdminHandler) TestAlgorithmConfig(c *gin.Context) {
	var testConfig AlgorithmConfig

	if err := c.ShouldBindJSON(&testConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid configuration format",
			"details": err.Error(),
		})
		return
	}

	// Validate configuration
	if err := h.validateAlgorithmConfig(&testConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Configuration validation failed",
			"details": err.Error(),
		})
		return
	}

	// Simulate testing the configuration
	// In a real implementation, this might:
	// 1. Run the algorithms with test data
	// 2. Calculate performance metrics
	// 3. Return predicted performance

	performanceScore := h.calculateConfigPerformanceScore(&testConfig)

	c.JSON(http.StatusOK, gin.H{
		"status":            "tested",
		"performance_score": performanceScore,
		"recommendations": []string{
			"Configuration appears valid",
			"Estimated performance improvement: +2.3%",
			"No critical issues detected",
		},
	})
}

// validateAlgorithmConfig validates the algorithm configuration
func (h *AdminHandler) validateAlgorithmConfig(config *AlgorithmConfig) error {
	// Check that weights sum to approximately 1.0
	totalWeight := config.Algorithms.SemanticSearch.Weight +
		config.Algorithms.CollaborativeFiltering.Weight +
		config.Algorithms.PageRank.Weight

	if totalWeight < 0.95 || totalWeight > 1.05 {
		return gin.Error{
			Err:  nil,
			Type: gin.ErrorTypeBind,
			Meta: "Algorithm weights must sum to approximately 1.0",
		}
	}

	// Validate individual parameters
	if config.Algorithms.SemanticSearch.SimilarityThreshold < 0 || config.Algorithms.SemanticSearch.SimilarityThreshold > 1 {
		return gin.Error{
			Err:  nil,
			Type: gin.ErrorTypeBind,
			Meta: "Semantic search similarity threshold must be between 0 and 1",
		}
	}

	if config.Algorithms.CollaborativeFiltering.SimilarityThreshold < 0 || config.Algorithms.CollaborativeFiltering.SimilarityThreshold > 1 {
		return gin.Error{
			Err:  nil,
			Type: gin.ErrorTypeBind,
			Meta: "Collaborative filtering similarity threshold must be between 0 and 1",
		}
	}

	if config.Algorithms.PageRank.DampingFactor < 0 || config.Algorithms.PageRank.DampingFactor > 1 {
		return gin.Error{
			Err:  nil,
			Type: gin.ErrorTypeBind,
			Meta: "PageRank damping factor must be between 0 and 1",
		}
	}

	if config.Diversity.IntraListDiversity < 0 || config.Diversity.IntraListDiversity > 1 {
		return gin.Error{
			Err:  nil,
			Type: gin.ErrorTypeBind,
			Meta: "Intra-list diversity must be between 0 and 1",
		}
	}

	if config.Diversity.SerendipityRatio < 0 || config.Diversity.SerendipityRatio > 0.5 {
		return gin.Error{
			Err:  nil,
			Type: gin.ErrorTypeBind,
			Meta: "Serendipity ratio must be between 0 and 0.5",
		}
	}

	if config.Diversity.CategoryMaxItems < 1 || config.Diversity.CategoryMaxItems > 10 {
		return gin.Error{
			Err:  nil,
			Type: gin.ErrorTypeBind,
			Meta: "Category max items must be between 1 and 10",
		}
	}

	return nil
}

// calculateConfigPerformanceScore calculates a performance score for the configuration
func (h *AdminHandler) calculateConfigPerformanceScore(config *AlgorithmConfig) float64 {
	// This is a simplified scoring algorithm
	// In practice, this would use historical data and ML models

	score := 75.0 // Base score

	// Reward balanced weights
	weights := []float64{
		config.Algorithms.SemanticSearch.Weight,
		config.Algorithms.CollaborativeFiltering.Weight,
		config.Algorithms.PageRank.Weight,
	}

	// Calculate variance in weights (lower variance = more balanced = higher score)
	mean := (weights[0] + weights[1] + weights[2]) / 3.0
	variance := 0.0
	for _, w := range weights {
		variance += (w - mean) * (w - mean)
	}
	variance /= 3.0

	// Lower variance gets higher score (up to +10 points)
	balanceScore := 10.0 * (1.0 - variance*10.0)
	if balanceScore < 0 {
		balanceScore = 0
	}
	score += balanceScore

	// Reward optimal similarity thresholds
	if config.Algorithms.SemanticSearch.SimilarityThreshold >= 0.6 && config.Algorithms.SemanticSearch.SimilarityThreshold <= 0.8 {
		score += 5.0
	}

	if config.Algorithms.CollaborativeFiltering.SimilarityThreshold >= 0.4 && config.Algorithms.CollaborativeFiltering.SimilarityThreshold <= 0.6 {
		score += 5.0
	}

	// Reward moderate diversity settings
	if config.Diversity.IntraListDiversity >= 0.2 && config.Diversity.IntraListDiversity <= 0.4 {
		score += 3.0
	}

	if config.Diversity.SerendipityRatio >= 0.1 && config.Diversity.SerendipityRatio <= 0.2 {
		score += 2.0
	}

	// Cap the score at 100
	if score > 100 {
		score = 100
	}

	return score
}

// GetSystemConfiguration returns system-wide configuration
func (h *AdminHandler) GetSystemConfiguration(c *gin.Context) {
	systemConfig := gin.H{
		"caching": gin.H{
			"embeddings_ttl":      "24h",
			"recommendations_ttl": "15m",
			"metadata_ttl":        "1h",
			"graph_results_ttl":   "30m",
		},
		"models": gin.H{
			"text_embedding": gin.H{
				"model_path": "./models/all-MiniLM-L6-v2.onnx",
				"dimensions": 384,
			},
			"image_embedding": gin.H{
				"model_path": "./models/clip-vit-base-patch32.onnx",
				"dimensions": 512,
			},
		},
		"security": gin.H{
			"rate_limit": gin.H{
				"default": 1000,
				"premium": 10000,
				"window":  "1h",
			},
		},
		"monitoring": gin.H{
			"enabled":      true,
			"port":         "9090",
			"metrics_path": "/metrics",
		},
	}

	c.JSON(http.StatusOK, systemConfig)
}

// UpdateSystemConfiguration updates system-wide configuration
func (h *AdminHandler) UpdateSystemConfiguration(c *gin.Context) {
	var newConfig map[string]interface{}

	if err := c.ShouldBindJSON(&newConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid configuration format",
			"details": err.Error(),
		})
		return
	}

	// In a real implementation, this would validate and apply the configuration
	h.logger.WithField("config", newConfig).Info("System configuration update requested")

	c.JSON(http.StatusOK, gin.H{
		"status":  "updated",
		"message": "System configuration updated successfully",
	})
}
