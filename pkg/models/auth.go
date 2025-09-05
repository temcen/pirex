package models

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JWTClaims struct {
	UserID   uuid.UUID `json:"user_id"`
	APIKey   string    `json:"api_key,omitempty"`
	UserTier string    `json:"user_tier"` // free, premium, enterprise
	jwt.RegisteredClaims
}

type AuthRequest struct {
	APIKey string `json:"api_key" validate:"required"`
	UserID string `json:"user_id,omitempty"`
}

type AuthResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	UserTier  string    `json:"user_tier"`
}

type RateLimitInfo struct {
	Limit     int   `json:"limit"`
	Remaining int   `json:"remaining"`
	ResetTime int64 `json:"reset_time"`
}
