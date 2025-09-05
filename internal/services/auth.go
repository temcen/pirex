package services

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"github.com/temcen/pirex/internal/config"
	"github.com/temcen/pirex/pkg/models"
)

type AuthService struct {
	config      *config.Config
	logger      *logrus.Logger
	redisClient *redis.Client
	jwtSecret   []byte
}

func NewAuthService(cfg *config.Config, logger *logrus.Logger, redisClient *redis.Client) *AuthService {
	return &AuthService{
		config:      cfg,
		logger:      logger,
		redisClient: redisClient,
		jwtSecret:   []byte(cfg.Auth.JWTSecret),
	}
}

func (s *AuthService) GenerateToken(userID uuid.UUID, apiKey, userTier string) (string, error) {
	now := time.Now()
	claims := &models.JWTClaims{
		UserID:   userID,
		APIKey:   apiKey,
		UserTier: userTier,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.config.Auth.TokenTTL)),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "github.com/temcen/pirex",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	// Store token in Redis for session management
	sessionKey := fmt.Sprintf("session:%s", userID.String())
	err = s.redisClient.Set(context.Background(), sessionKey, tokenString, s.config.Auth.TokenTTL).Err()
	if err != nil {
		s.logger.WithError(err).Warn("Failed to store session in Redis")
		// Don't fail token generation if Redis is down
	}

	return tokenString, nil
}

func (s *AuthService) ValidateToken(tokenString string) (*models.JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &models.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*models.JWTClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Check if session exists in Redis
	sessionKey := fmt.Sprintf("session:%s", claims.UserID.String())
	exists, err := s.redisClient.Exists(context.Background(), sessionKey).Result()
	if err != nil {
		s.logger.WithError(err).Warn("Failed to check session in Redis")
		// Continue validation even if Redis is down
	} else if exists == 0 {
		return nil, fmt.Errorf("session not found or expired")
	}

	return claims, nil
}

func (s *AuthService) RevokeToken(userID uuid.UUID) error {
	sessionKey := fmt.Sprintf("session:%s", userID.String())
	err := s.redisClient.Del(context.Background(), sessionKey).Err()
	if err != nil {
		return fmt.Errorf("failed to revoke session: %w", err)
	}
	return nil
}

func (s *AuthService) ValidateAPIKey(apiKey string) (string, error) {
	// In a real implementation, this would check against a database
	// For now, we'll use a simple mapping
	apiKeyToTier := map[string]string{
		"demo-free-key":       "free",
		"demo-premium-key":    "premium",
		"demo-enterprise-key": "enterprise",
	}

	if tier, exists := apiKeyToTier[apiKey]; exists {
		return tier, nil
	}

	return "", fmt.Errorf("invalid API key")
}
