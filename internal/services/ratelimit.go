package services

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"github.com/temcen/pirex/internal/config"
	"github.com/temcen/pirex/pkg/models"
)

type RateLimitService struct {
	config      *config.Config
	logger      *logrus.Logger
	redisClient *redis.Client
}

func NewRateLimitService(cfg *config.Config, logger *logrus.Logger, redisClient *redis.Client) *RateLimitService {
	return &RateLimitService{
		config:      cfg,
		logger:      logger,
		redisClient: redisClient,
	}
}

func (s *RateLimitService) CheckLimit(userID, userTier string) (*models.RateLimitInfo, error) {
	limit := s.getLimitForTier(userTier)
	window := s.config.Auth.RateLimit.Window

	key := fmt.Sprintf("rate_limit:user:%s", userID)

	// Use sliding window rate limiting
	now := time.Now()
	windowStart := now.Add(-window)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Redis pipeline for atomic operations
	pipe := s.redisClient.Pipeline()

	// Remove expired entries
	pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStart.Unix(), 10))

	// Count current requests in window
	countCmd := pipe.ZCard(ctx, key)

	// Add current request
	pipe.ZAdd(ctx, key, redis.Z{
		Score:  float64(now.Unix()),
		Member: fmt.Sprintf("%d", now.UnixNano()),
	})

	// Set expiration
	pipe.Expire(ctx, key, window)

	_, err := pipe.Exec(ctx)
	if err != nil {
		s.logger.WithError(err).Error("Failed to execute rate limit pipeline")
		// Return permissive result if Redis is down
		return &models.RateLimitInfo{
			Limit:     limit,
			Remaining: limit - 1,
			ResetTime: now.Add(window).Unix(),
		}, nil
	}

	currentCount := int(countCmd.Val())
	remaining := limit - currentCount
	if remaining < 0 {
		remaining = 0
	}

	resetTime := now.Add(window).Unix()

	return &models.RateLimitInfo{
		Limit:     limit,
		Remaining: remaining,
		ResetTime: resetTime,
	}, nil
}

func (s *RateLimitService) IsAllowed(userID, userTier string) (bool, *models.RateLimitInfo, error) {
	info, err := s.CheckLimit(userID, userTier)
	if err != nil {
		return false, nil, err
	}

	allowed := info.Remaining > 0
	return allowed, info, nil
}

func (s *RateLimitService) getLimitForTier(userTier string) int {
	switch userTier {
	case "premium":
		return s.config.Auth.RateLimit.Premium
	case "enterprise":
		return s.config.Auth.RateLimit.Premium * 10 // 10x premium limit
	default:
		return s.config.Auth.RateLimit.Default
	}
}
