package services

import (
	"context"
	"crypto/md5"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// SpamDetector detects spam feedback using various heuristics
type SpamDetector struct {
	redisClient *redis.Client
	ctx         context.Context
}

// NewSpamDetector creates a new spam detector
func NewSpamDetector(redisClient *redis.Client) *SpamDetector {
	return &SpamDetector{
		redisClient: redisClient,
		ctx:         context.Background(),
	}
}

// IsSpam checks if a feedback event is likely spam
func (sd *SpamDetector) IsSpam(event FeedbackEvent) bool {
	// Check multiple spam indicators
	checks := []func(FeedbackEvent) bool{
		sd.checkDuplicateContent,
		sd.checkRapidFeedback,
		sd.checkSuspiciousPatterns,
		sd.checkUserReliability,
	}

	spamScore := 0
	for _, check := range checks {
		if check(event) {
			spamScore++
		}
	}

	// Consider spam if multiple indicators are present
	return spamScore >= 2
}

// checkDuplicateContent checks for duplicate feedback content
func (sd *SpamDetector) checkDuplicateContent(event FeedbackEvent) bool {
	// Create content hash
	contentHash := sd.createContentHash(event)
	key := fmt.Sprintf("spam:content:%s", contentHash)

	// Check if we've seen this exact content recently
	count, err := sd.redisClient.Incr(sd.ctx, key).Result()
	if err != nil {
		return false
	}

	// Set expiration on first occurrence
	if count == 1 {
		sd.redisClient.Expire(sd.ctx, key, 1*time.Hour)
	}

	// Flag as spam if seen more than 5 times in an hour
	return count > 5
}

// checkRapidFeedback checks for unusually rapid feedback submission
func (sd *SpamDetector) checkRapidFeedback(event FeedbackEvent) bool {
	key := fmt.Sprintf("spam:rapid:%s", event.UserID)

	// Count feedback in last minute
	now := time.Now().Unix()
	windowStart := now - 60 // 1 minute window

	count, err := sd.redisClient.ZCount(sd.ctx, key,
		strconv.FormatInt(windowStart, 10),
		strconv.FormatInt(now, 10)).Result()
	if err != nil {
		return false
	}

	// Add current event
	sd.redisClient.ZAdd(sd.ctx, key, redis.Z{
		Score:  float64(now),
		Member: fmt.Sprintf("%d", now),
	})
	sd.redisClient.Expire(sd.ctx, key, 5*time.Minute)

	// Flag as spam if more than 10 feedback events per minute
	return count > 10
}

// checkSuspiciousPatterns checks for suspicious feedback patterns
func (sd *SpamDetector) checkSuspiciousPatterns(event FeedbackEvent) bool {
	// Check for patterns like:
	// - All ratings are the same value
	// - Feedback only on specific items
	// - Unusual timing patterns

	key := fmt.Sprintf("spam:pattern:%s", event.UserID)

	// Store recent feedback patterns
	patternData := map[string]interface{}{
		"action": event.Action,
		"value":  event.Value,
		"item":   event.ItemID,
		"time":   event.Timestamp.Unix(),
	}

	// Get recent patterns
	patterns, err := sd.redisClient.LRange(sd.ctx, key, 0, 9).Result()
	if err != nil {
		return false
	}

	// Add current pattern
	sd.redisClient.LPush(sd.ctx, key, fmt.Sprintf("%v", patternData))
	sd.redisClient.LTrim(sd.ctx, key, 0, 9) // Keep last 10
	sd.redisClient.Expire(sd.ctx, key, 1*time.Hour)

	// Analyze patterns for suspicious behavior
	if len(patterns) >= 5 {
		return sd.analyzeSuspiciousPatterns(patterns, event)
	}

	return false
}

// checkUserReliability checks the user's historical reliability
func (sd *SpamDetector) checkUserReliability(event FeedbackEvent) bool {
	key := fmt.Sprintf("spam:reliability:%s", event.UserID)

	// Get user reliability score (0-100, higher is better)
	reliabilityStr, err := sd.redisClient.Get(sd.ctx, key).Result()
	if err != nil {
		// New user, assume neutral reliability
		return false
	}

	reliability, err := strconv.Atoi(reliabilityStr)
	if err != nil {
		return false
	}

	// Flag as spam if reliability is very low
	return reliability < 20
}

// createContentHash creates a hash of the feedback content
func (sd *SpamDetector) createContentHash(event FeedbackEvent) string {
	content := fmt.Sprintf("%s:%s:%s:%.2f",
		event.UserID, event.ItemID, event.Action, event.Value)
	hash := md5.Sum([]byte(content))
	return fmt.Sprintf("%x", hash)
}

// analyzeSuspiciousPatterns analyzes patterns for suspicious behavior
func (sd *SpamDetector) analyzeSuspiciousPatterns(patterns []string, event FeedbackEvent) bool {
	// Simple pattern analysis - in production, this would be more sophisticated

	// Check if all recent ratings are the same
	if event.Type == FeedbackExplicit && event.Action == "rating" {
		sameValueCount := 0
		for _, pattern := range patterns {
			// This is a simplified check - in practice, you'd parse the pattern properly
			if fmt.Sprintf("value:%v", event.Value) == pattern {
				sameValueCount++
			}
		}

		// Flag if more than 80% of recent ratings are the same value
		return float64(sameValueCount)/float64(len(patterns)) > 0.8
	}

	return false
}

// UpdateUserReliability updates a user's reliability score based on feedback quality
func (sd *SpamDetector) UpdateUserReliability(userID string, delta int) error {
	key := fmt.Sprintf("spam:reliability:%s", userID)

	// Get current reliability
	currentStr, err := sd.redisClient.Get(sd.ctx, key).Result()
	current := 50 // Default neutral score
	if err == nil {
		if val, parseErr := strconv.Atoi(currentStr); parseErr == nil {
			current = val
		}
	}

	// Update reliability (clamp between 0 and 100)
	newReliability := current + delta
	if newReliability < 0 {
		newReliability = 0
	} else if newReliability > 100 {
		newReliability = 100
	}

	// Store updated reliability
	return sd.redisClient.Set(sd.ctx, key, newReliability, 30*24*time.Hour).Err()
}

// GetUserReliability gets a user's current reliability score
func (sd *SpamDetector) GetUserReliability(userID string) int {
	key := fmt.Sprintf("spam:reliability:%s", userID)

	reliabilityStr, err := sd.redisClient.Get(sd.ctx, key).Result()
	if err != nil {
		return 50 // Default neutral score
	}

	reliability, err := strconv.Atoi(reliabilityStr)
	if err != nil {
		return 50
	}

	return reliability
}
