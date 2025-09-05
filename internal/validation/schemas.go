package validation

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/xeipuuv/gojsonschema"
)

// SchemaValidator handles JSON schema validation for API requests and responses
type SchemaValidator struct {
	schemas map[string]*gojsonschema.Schema
}

// NewSchemaValidator creates a new schema validator instance
func NewSchemaValidator() *SchemaValidator {
	return &SchemaValidator{
		schemas: make(map[string]*gojsonschema.Schema),
	}
}

// LoadSchemas loads all JSON schemas from the specified directory
func (sv *SchemaValidator) LoadSchemas(schemaDir string) error {
	schemaFiles := map[string]string{
		"content-item":     "content-item.json",
		"user-interaction": "user-interaction.json",
		"recommendation":   "recommendation.json",
		"user-profile":     "user-profile.json",
		"error-response":   "error-response.json",
	}

	for name, filename := range schemaFiles {
		schemaPath := filepath.Join(schemaDir, filename)

		// Load schema file
		schemaLoader := gojsonschema.NewReferenceLoader("file://" + schemaPath)
		schema, err := gojsonschema.NewSchema(schemaLoader)
		if err != nil {
			return fmt.Errorf("failed to load schema %s: %w", name, err)
		}

		sv.schemas[name] = schema
	}

	return nil
}

// ValidateContentItem validates a content item against its JSON schema
func (sv *SchemaValidator) ValidateContentItem(data interface{}) *ValidationResult {
	return sv.validate("content-item", data)
}

// ValidateUserInteraction validates a user interaction against its JSON schema
func (sv *SchemaValidator) ValidateUserInteraction(data interface{}) *ValidationResult {
	return sv.validate("user-interaction", data)
}

// ValidateRecommendation validates a recommendation against its JSON schema
func (sv *SchemaValidator) ValidateRecommendation(data interface{}) *ValidationResult {
	return sv.validate("recommendation", data)
}

// ValidateUserProfile validates a user profile against its JSON schema
func (sv *SchemaValidator) ValidateUserProfile(data interface{}) *ValidationResult {
	return sv.validate("user-profile", data)
}

// ValidateErrorResponse validates an error response against its JSON schema
func (sv *SchemaValidator) ValidateErrorResponse(data interface{}) *ValidationResult {
	return sv.validate("error-response", data)
}

// validate performs the actual validation against a named schema
func (sv *SchemaValidator) validate(schemaName string, data interface{}) *ValidationResult {
	schema, exists := sv.schemas[schemaName]
	if !exists {
		return &ValidationResult{
			Valid: false,
			Errors: []ValidationError{{
				Field:   "schema",
				Message: fmt.Sprintf("Schema '%s' not found", schemaName),
				Code:    "SCHEMA_NOT_FOUND",
			}},
		}
	}

	// Convert data to JSON for validation
	var documentLoader gojsonschema.JSONLoader
	switch v := data.(type) {
	case string:
		documentLoader = gojsonschema.NewStringLoader(v)
	case []byte:
		documentLoader = gojsonschema.NewBytesLoader(v)
	default:
		// Convert to JSON bytes
		jsonBytes, err := json.Marshal(data)
		if err != nil {
			return &ValidationResult{
				Valid: false,
				Errors: []ValidationError{{
					Field:   "data",
					Message: fmt.Sprintf("Failed to marshal data to JSON: %v", err),
					Code:    "JSON_MARSHAL_ERROR",
				}},
			}
		}
		documentLoader = gojsonschema.NewBytesLoader(jsonBytes)
	}

	// Perform validation
	result, err := schema.Validate(documentLoader)
	if err != nil {
		return &ValidationResult{
			Valid: false,
			Errors: []ValidationError{{
				Field:   "validation",
				Message: fmt.Sprintf("Validation error: %v", err),
				Code:    "VALIDATION_ERROR",
			}},
		}
	}

	// Convert results
	validationResult := &ValidationResult{
		Valid:  result.Valid(),
		Errors: make([]ValidationError, 0),
	}

	if !result.Valid() {
		for _, err := range result.Errors() {
			validationResult.Errors = append(validationResult.Errors, ValidationError{
				Field:   err.Field(),
				Message: err.Description(),
				Code:    "VALIDATION_ERROR",
				Value:   err.Value(),
				Context: err.Context().String(),
			})
		}
	}

	return validationResult
}

// ValidationResult represents the result of a validation operation
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// ValidationError represents a single validation error
type ValidationError struct {
	Field   string      `json:"field"`
	Message string      `json:"message"`
	Code    string      `json:"code"`
	Value   interface{} `json:"value,omitempty"`
	Context string      `json:"context,omitempty"`
}

// Error implements the error interface
func (ve ValidationError) Error() string {
	return fmt.Sprintf("validation error in field '%s': %s", ve.Field, ve.Message)
}

// ToAPIError converts validation errors to API error format
func (vr *ValidationResult) ToAPIError() map[string]interface{} {
	if vr.Valid {
		return nil
	}

	errorDetails := make(map[string]interface{})
	errorDetails["validationErrors"] = vr.Errors

	// Extract field-specific errors for easier client handling
	fieldErrors := make(map[string][]string)
	for _, err := range vr.Errors {
		if err.Field != "" {
			fieldErrors[err.Field] = append(fieldErrors[err.Field], err.Message)
		}
	}

	if len(fieldErrors) > 0 {
		errorDetails["fieldErrors"] = fieldErrors
	}

	return map[string]interface{}{
		"error": map[string]interface{}{
			"code":    "VALIDATION_ERROR",
			"message": "Request validation failed",
			"details": errorDetails,
		},
	}
}

// ValidateJSONString validates a JSON string against a schema
func (sv *SchemaValidator) ValidateJSONString(schemaName, jsonString string) *ValidationResult {
	return sv.validate(schemaName, jsonString)
}

// ValidateStruct validates a Go struct against a schema
func (sv *SchemaValidator) ValidateStruct(schemaName string, data interface{}) *ValidationResult {
	return sv.validate(schemaName, data)
}

// GetAvailableSchemas returns a list of loaded schema names
func (sv *SchemaValidator) GetAvailableSchemas() []string {
	schemas := make([]string, 0, len(sv.schemas))
	for name := range sv.schemas {
		schemas = append(schemas, name)
	}
	return schemas
}

// SchemaExists checks if a schema with the given name is loaded
func (sv *SchemaValidator) SchemaExists(name string) bool {
	_, exists := sv.schemas[name]
	return exists
}

// LoadSchemaFromFS loads schemas from an embedded filesystem
func (sv *SchemaValidator) LoadSchemaFromFS(fsys fs.FS, schemaDir string) error {
	schemaFiles := map[string]string{
		"content-item":     "content-item.json",
		"user-interaction": "user-interaction.json",
		"recommendation":   "recommendation.json",
		"user-profile":     "user-profile.json",
		"error-response":   "error-response.json",
	}

	for name, filename := range schemaFiles {
		schemaPath := filepath.Join(schemaDir, filename)

		// Read schema file from embedded FS
		schemaBytes, err := fs.ReadFile(fsys, schemaPath)
		if err != nil {
			return fmt.Errorf("failed to read schema file %s: %w", schemaPath, err)
		}

		// Load schema from bytes
		schemaLoader := gojsonschema.NewBytesLoader(schemaBytes)
		schema, err := gojsonschema.NewSchema(schemaLoader)
		if err != nil {
			return fmt.Errorf("failed to load schema %s: %w", name, err)
		}

		sv.schemas[name] = schema
	}

	return nil
}
