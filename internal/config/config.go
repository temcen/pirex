package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Redis      RedisConfig      `mapstructure:"redis"`
	Neo4j      Neo4jConfig      `mapstructure:"neo4j"`
	Kafka      KafkaConfig      `mapstructure:"kafka"`
	Auth       AuthConfig       `mapstructure:"auth"`
	Logging    LoggingConfig    `mapstructure:"logging"`
	Algorithms AlgorithmConfig  `mapstructure:"recommendation"`
	Models     ModelConfig      `mapstructure:"models"`
	Monitoring MonitoringConfig `mapstructure:"monitoring"`
	Security   SecurityConfig   `mapstructure:"security"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type DatabaseConfig struct {
	URL            string        `mapstructure:"url"`
	MaxConnections int           `mapstructure:"max_connections"`
	MaxIdleTime    time.Duration `mapstructure:"max_idle_time"`
	MaxLifetime    time.Duration `mapstructure:"max_lifetime"`
	ConnectTimeout time.Duration `mapstructure:"connect_timeout"`
}

type RedisConfig struct {
	Hot  RedisInstanceConfig `mapstructure:"hot"`
	Warm RedisInstanceConfig `mapstructure:"warm"`
	Cold RedisInstanceConfig `mapstructure:"cold"`
}

type RedisInstanceConfig struct {
	URL        string        `mapstructure:"url"`
	MaxRetries int           `mapstructure:"max_retries"`
	PoolSize   int           `mapstructure:"pool_size"`
	Timeout    time.Duration `mapstructure:"timeout"`
}

type Neo4jConfig struct {
	URL      string `mapstructure:"url"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type KafkaConfig struct {
	Brokers []string `mapstructure:"brokers"`
	Topics  struct {
		ContentIngestion string `mapstructure:"content_ingestion"`
		UserInteractions string `mapstructure:"user_interactions"`
	} `mapstructure:"topics"`
}

type AuthConfig struct {
	JWTSecret string          `mapstructure:"jwt_secret"`
	TokenTTL  time.Duration   `mapstructure:"token_ttl"`
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`
}

type RateLimitConfig struct {
	Default int           `mapstructure:"default"`
	Premium int           `mapstructure:"premium"`
	Window  time.Duration `mapstructure:"window"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type AlgorithmConfig struct {
	SemanticSearch      AlgorithmWeightConfig `mapstructure:"semantic_search"`
	CollaborativeFilter AlgorithmWeightConfig `mapstructure:"collaborative_filtering"`
	PageRank            AlgorithmWeightConfig `mapstructure:"pagerank"`
	Diversity           DiversityConfig       `mapstructure:"diversity"`
	Caching             CachingConfig         `mapstructure:"caching"`
}

type AlgorithmWeightConfig struct {
	Enabled             bool    `mapstructure:"enabled"`
	Weight              float64 `mapstructure:"weight"`
	SimilarityThreshold float64 `mapstructure:"similarity_threshold"`
}

type DiversityConfig struct {
	IntraListDiversity     float64 `mapstructure:"intra_list_diversity"`
	CategoryMaxItems       int     `mapstructure:"category_max_items"`
	SerendipityRatio       float64 `mapstructure:"serendipity_ratio"`
	MaxSimilarityThreshold float64 `mapstructure:"max_similarity_threshold"`
	TemporalDecayFactor    float64 `mapstructure:"temporal_decay_factor"`
	MaxRecentSimilarItems  int     `mapstructure:"max_recent_similar_items"`
}

type CachingConfig struct {
	EmbeddingsTTL      time.Duration `mapstructure:"embeddings_ttl"`
	RecommendationsTTL time.Duration `mapstructure:"recommendations_ttl"`
	MetadataTTL        time.Duration `mapstructure:"metadata_ttl"`
	GraphResultsTTL    time.Duration `mapstructure:"graph_results_ttl"`
}

type ModelConfig struct {
	TextEmbedding  ModelInstanceConfig `mapstructure:"text_embedding"`
	ImageEmbedding ModelInstanceConfig `mapstructure:"image_embedding"`
}

type ModelInstanceConfig struct {
	ModelPath  string `mapstructure:"model_path"`
	Dimensions int    `mapstructure:"dimensions"`
}

type MonitoringConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	Port        string `mapstructure:"port"`
	MetricsPath string `mapstructure:"metrics_path"`
}

type SecurityConfig struct {
	CORS CORSConfig `mapstructure:"cors"`
}

type CORSConfig struct {
	AllowedOrigins []string `mapstructure:"allowed_origins"`
	AllowedMethods []string `mapstructure:"allowed_methods"`
	AllowedHeaders []string `mapstructure:"allowed_headers"`
}

func Load() (*Config, error) {
	viper.SetConfigName("app")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath(".")

	// Set defaults
	setDefaults()

	// Environment variable overrides
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil {
		// Config file is optional, continue with env vars and defaults
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

func setDefaults() {
	// Server defaults
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.mode", "development")

	// Database defaults
	viper.SetDefault("database.max_connections", 25)
	viper.SetDefault("database.max_idle_time", "15m")
	viper.SetDefault("database.max_lifetime", "1h")
	viper.SetDefault("database.connect_timeout", "10s")

	// Redis defaults
	viper.SetDefault("redis.hot.max_retries", 3)
	viper.SetDefault("redis.hot.pool_size", 10)
	viper.SetDefault("redis.hot.timeout", "5s")
	viper.SetDefault("redis.warm.max_retries", 3)
	viper.SetDefault("redis.warm.pool_size", 5)
	viper.SetDefault("redis.warm.timeout", "10s")
	viper.SetDefault("redis.cold.max_retries", 3)
	viper.SetDefault("redis.cold.pool_size", 5)
	viper.SetDefault("redis.cold.timeout", "15s")

	// Auth defaults
	viper.SetDefault("auth.token_ttl", "24h")
	viper.SetDefault("auth.rate_limit.default", 1000)
	viper.SetDefault("auth.rate_limit.premium", 10000)
	viper.SetDefault("auth.rate_limit.window", "1h")

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "text")

	// Algorithm defaults
	viper.SetDefault("recommendation.semantic_search.enabled", true)
	viper.SetDefault("recommendation.semantic_search.weight", 0.4)
	viper.SetDefault("recommendation.semantic_search.similarity_threshold", 0.7)
	viper.SetDefault("recommendation.collaborative_filtering.enabled", true)
	viper.SetDefault("recommendation.collaborative_filtering.weight", 0.3)
	viper.SetDefault("recommendation.collaborative_filtering.similarity_threshold", 0.5)
	viper.SetDefault("recommendation.pagerank.enabled", true)
	viper.SetDefault("recommendation.pagerank.weight", 0.3)
	viper.SetDefault("recommendation.pagerank.similarity_threshold", 0.0)

	// Diversity defaults
	viper.SetDefault("recommendation.diversity.intra_list_diversity", 0.3)
	viper.SetDefault("recommendation.diversity.category_max_items", 3)
	viper.SetDefault("recommendation.diversity.serendipity_ratio", 0.15)
	viper.SetDefault("recommendation.diversity.max_similarity_threshold", 0.8)
	viper.SetDefault("recommendation.diversity.temporal_decay_factor", 7.0)
	viper.SetDefault("recommendation.diversity.max_recent_similar_items", 2)

	// Caching defaults
	viper.SetDefault("recommendation.caching.embeddings_ttl", "24h")
	viper.SetDefault("recommendation.caching.recommendations_ttl", "15m")
	viper.SetDefault("recommendation.caching.metadata_ttl", "1h")
	viper.SetDefault("recommendation.caching.graph_results_ttl", "30m")

	// Model defaults
	viper.SetDefault("models.text_embedding.model_path", "./models/all-MiniLM-L6-v2.onnx")
	viper.SetDefault("models.text_embedding.dimensions", 384)
	viper.SetDefault("models.image_embedding.model_path", "./models/clip-vit-base-patch32.onnx")
	viper.SetDefault("models.image_embedding.dimensions", 512)

	// Monitoring defaults
	viper.SetDefault("monitoring.enabled", true)
	viper.SetDefault("monitoring.port", "9090")
	viper.SetDefault("monitoring.metrics_path", "/metrics")

	// Security defaults
	viper.SetDefault("security.cors.allowed_origins", []string{"*"})
	viper.SetDefault("security.cors.allowed_methods", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"})
	viper.SetDefault("security.cors.allowed_headers", []string{"*"})
}
