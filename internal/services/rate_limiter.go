package services

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter implements sliding window rate limiting using Redis
type RateLimiter struct {
	redisClient *redis.Client
	ctx         context.Context
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(redisClient *redis.Client) *RateLimiter {
	return &RateLimiter{
		redisClient: redisClient,
		ctx:         context.Background(),
	}
}

// Allow checks if the request is allowed based on rate limiting rules
func (rl *RateLimiter) Allow(userID, action string, limit int, window time.Duration) bool {
	key := fmt.Sprintf("rate_limit:%s:%s", userID, action)
	now := time.Now().Unix()
	windowStart := now - int64(window.Seconds())

	pipe := rl.redisClient.Pipeline()

	// Remove expired entries
	pipe.ZRemRangeByScore(rl.ctx, key, "0", strconv.FormatInt(windowStart, 10))

	// Count current requests in window
	countCmd := pipe.ZCard(rl.ctx, key)

	// Add current request
	pipe.ZAdd(rl.ctx, key, redis.Z{
		Score:  float64(now),
		Member: fmt.Sprintf("%d", now),
	})

	// Set expiration
	pipe.Expire(rl.ctx, key, window)

	_, err := pipe.Exec(rl.ctx)
	if err != nil {
		// On error, allow the request (fail open)
		return true
	}

	count := countCmd.Val()
	return count < int64(limit)
}

// GetCurrentCount returns the current count for a user/action combination
func (rl *RateLimiter) GetCurrentCount(userID, action string, window time.Duration) int64 {
	key := fmt.Sprintf("rate_limit:%s:%s", userID, action)
	now := time.Now().Unix()
	windowStart := now - int64(window.Seconds())

	count, err := rl.redisClient.ZCount(rl.ctx, key,
		strconv.FormatInt(windowStart, 10),
		strconv.FormatInt(now, 10)).Result()
	if err != nil {
		return 0
	}

	return count
}

// Reset resets the rate limit for a user/action combination
func (rl *RateLimiter) Reset(userID, action string) error {
	key := fmt.Sprintf("rate_limit:%s:%s", userID, action)
	return rl.redisClient.Del(rl.ctx, key).Err()
}
