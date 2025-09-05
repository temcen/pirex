package models

import (
	"time"

	"github.com/google/uuid"
)

type ContentItem struct {
	ID           uuid.UUID              `json:"id" db:"id"`
	Type         string                 `json:"type" db:"type" validate:"required,oneof=product video article"`
	Title        string                 `json:"title" db:"title" validate:"required,min=1,max=255"`
	Description  *string                `json:"description,omitempty" db:"description"`
	ImageURLs    []string               `json:"image_urls,omitempty" db:"image_urls"`
	Metadata     map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
	Categories   []string               `json:"categories,omitempty" db:"categories"`
	Embedding    []float32              `json:"-" db:"embedding"`
	QualityScore float64                `json:"quality_score" db:"quality_score"`
	Active       bool                   `json:"active" db:"active"`
	CreatedAt    time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at" db:"updated_at"`
}

type ContentIngestionRequest struct {
	Type        string                 `json:"type" validate:"required,oneof=product video article"`
	Title       string                 `json:"title" validate:"required,min=1,max=255"`
	Description *string                `json:"description,omitempty"`
	ImageURLs   []string               `json:"image_urls,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Categories  []string               `json:"categories,omitempty"`
}

type ContentBatchRequest struct {
	Items []ContentIngestionRequest `json:"items" validate:"required,min=1,max=100"`
}

type ContentJobStatus struct {
	JobID          uuid.UUID `json:"job_id"`
	Status         string    `json:"status"`   // queued, processing, completed, failed
	Progress       int       `json:"progress"` // 0-100
	TotalItems     int       `json:"total_items"`
	ProcessedItems int       `json:"processed_items"`
	FailedItems    int       `json:"failed_items"`
	EstimatedTime  *int      `json:"estimated_time,omitempty"` // seconds
	ErrorMessage   *string   `json:"error_message,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
