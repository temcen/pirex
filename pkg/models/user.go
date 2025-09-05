package models

import (
	"time"

	"github.com/google/uuid"
)

type UserProfile struct {
	UserID           uuid.UUID              `json:"user_id" db:"user_id"`
	PreferenceVector []float32              `json:"-" db:"preference_vector"`
	ExplicitPrefs    map[string]interface{} `json:"explicit_preferences" db:"explicit_preferences"`
	BehaviorPatterns map[string]interface{} `json:"behavior_patterns" db:"behavior_patterns"`
	Demographics     map[string]interface{} `json:"demographics,omitempty" db:"demographics"`
	InteractionCount int                    `json:"interaction_count" db:"interaction_count"`
	LastInteraction  *time.Time             `json:"last_interaction,omitempty" db:"last_interaction"`
	CreatedAt        time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at" db:"updated_at"`
}

type UserInteraction struct {
	ID              uuid.UUID              `json:"id" db:"id"`
	UserID          uuid.UUID              `json:"user_id" db:"user_id" validate:"required"`
	ItemID          *uuid.UUID             `json:"item_id,omitempty" db:"item_id"`
	InteractionType string                 `json:"interaction_type" db:"interaction_type" validate:"required"`
	Value           *float64               `json:"value,omitempty" db:"value"`
	Duration        *int                   `json:"duration,omitempty" db:"duration"` // seconds
	Query           *string                `json:"query,omitempty" db:"query"`
	SessionID       uuid.UUID              `json:"session_id" db:"session_id" validate:"required"`
	Context         map[string]interface{} `json:"context,omitempty" db:"context"`
	Timestamp       time.Time              `json:"timestamp" db:"timestamp"`
}

type ExplicitInteractionRequest struct {
	UserID    uuid.UUID `json:"user_id" validate:"required"`
	ItemID    uuid.UUID `json:"item_id" validate:"required"`
	Type      string    `json:"type" validate:"required,oneof=rating like dislike share"`
	Value     *float64  `json:"value,omitempty" validate:"omitempty,min=1,max=5"`
	SessionID uuid.UUID `json:"session_id" validate:"required"`
}

type ImplicitInteractionRequest struct {
	UserID    uuid.UUID              `json:"user_id" validate:"required"`
	ItemID    *uuid.UUID             `json:"item_id,omitempty"`
	Type      string                 `json:"type" validate:"required,oneof=click view search browse"`
	Duration  *int                   `json:"duration,omitempty"`
	Query     *string                `json:"query,omitempty"`
	SessionID uuid.UUID              `json:"session_id" validate:"required"`
	Context   map[string]interface{} `json:"context,omitempty"`
}

type InteractionBatchRequest struct {
	ExplicitInteractions []ExplicitInteractionRequest `json:"explicit_interactions,omitempty"`
	ImplicitInteractions []ImplicitInteractionRequest `json:"implicit_interactions,omitempty"`
}

type SimilarUser struct {
	UserID          uuid.UUID `json:"user_id"`
	SimilarityScore float64   `json:"similarity_score"`
	Basis           string    `json:"basis"`
	SharedItems     int       `json:"shared_items"`
}
