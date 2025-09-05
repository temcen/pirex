package models

import (
	"time"

	"github.com/google/uuid"
)

type Recommendation struct {
	ItemID      uuid.UUID    `json:"item_id"`
	Score       float64      `json:"score"`
	Algorithm   string       `json:"algorithm"`
	Explanation *string      `json:"explanation,omitempty"`
	Confidence  float64      `json:"confidence"`
	Position    int          `json:"position"`
	Item        *ContentItem `json:"item,omitempty"`
}

type RecommendationRequest struct {
	UserID  uuid.UUID `json:"user_id" validate:"required"`
	Count   int       `json:"count" validate:"min=1,max=100"`
	Context string    `json:"context,omitempty" validate:"omitempty,oneof=home search category product"`
	Explain bool      `json:"explain"`
}

type RecommendationResponse struct {
	UserID          uuid.UUID        `json:"user_id"`
	Recommendations []Recommendation `json:"recommendations"`
	Context         string           `json:"context"`
	GeneratedAt     time.Time        `json:"generated_at"`
	CacheHit        bool             `json:"cache_hit"`
}

type BatchRecommendationRequest struct {
	Requests []RecommendationRequest `json:"requests" validate:"required,min=1,max=50"`
}

type BatchRecommendationResponse struct {
	Responses []RecommendationResponse `json:"responses"`
}

type ScoredItem struct {
	ItemID     uuid.UUID `json:"item_id"`
	Score      float64   `json:"score"`
	Algorithm  string    `json:"algorithm"`
	Confidence float64   `json:"confidence"`
}

type SimilarItemResponse struct {
	UserID          uuid.UUID        `json:"user_id"`
	SeedItemID      uuid.UUID        `json:"seed_item_id"`
	Recommendations []Recommendation `json:"recommendations"`
	GeneratedAt     time.Time        `json:"generated_at"`
	CacheHit        bool             `json:"cache_hit"`
}

type RecommendationFeedback struct {
	UserID           uuid.UUID `json:"user_id" validate:"required"`
	RecommendationID uuid.UUID `json:"recommendation_id" validate:"required"`
	ItemID           uuid.UUID `json:"item_id" validate:"required"`
	FeedbackType     string    `json:"feedback_type" validate:"required,oneof=positive negative not_interested not_relevant inappropriate"`
	Comment          *string   `json:"comment,omitempty"`
	Timestamp        time.Time `json:"timestamp"`
}

// Pagination support
type PaginationRequest struct {
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit" validate:"min=1,max=100"`
}

type PaginationResponse struct {
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
	Total      *int   `json:"total,omitempty"`
}
