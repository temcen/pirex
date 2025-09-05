package models

import "github.com/google/uuid"

// Add missing UUID import for other model files
func init() {
	// This ensures the uuid package is imported
	_ = uuid.New()
}
