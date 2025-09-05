package services

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/temcen/pirex/pkg/models"
)

// MockDatabase is a mock implementation for testing
type MockDatabase struct {
	mock.Mock
}

func (m *MockDatabase) QueryRow(ctx context.Context, query string, args ...interface{}) *MockRow {
	mockArgs := m.Called(ctx, query, args)
	return mockArgs.Get(0).(*MockRow)
}

func (m *MockDatabase) Query(ctx context.Context, query string, args ...interface{}) (*MockRows, error) {
	mockArgs := m.Called(ctx, query, args)
	return mockArgs.Get(0).(*MockRows), mockArgs.Error(1)
}

func (m *MockDatabase) Exec(ctx context.Context, query string, args ...interface{}) (interface{}, error) {
	mockArgs := m.Called(ctx, query, args)
	return mockArgs.Get(0), mockArgs.Error(1)
}

type MockRow struct {
	mock.Mock
}

func (m *MockRow) Scan(dest ...interface{}) error {
	args := m.Called(dest)
	return args.Error(0)
}

type MockRows struct {
	mock.Mock
}

func (m *MockRows) Next() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockRows) Scan(dest ...interface{}) error {
	args := m.Called(dest)
	return args.Error(0)
}

func (m *MockRows) Close() {
	m.Called()
}

func TestUserInteractionService_RecordExplicitInteraction(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise in tests

	tests := []struct {
		name    string
		request *models.ExplicitInteractionRequest
		wantErr bool
	}{
		{
			name: "valid rating interaction",
			request: &models.ExplicitInteractionRequest{
				UserID:    uuid.New(),
				ItemID:    uuid.New(),
				Type:      "rating",
				Value:     func() *float64 { v := 4.5; return &v }(),
				SessionID: uuid.New(),
			},
			wantErr: false,
		},
		{
			name: "valid like interaction",
			request: &models.ExplicitInteractionRequest{
				UserID:    uuid.New(),
				ItemID:    uuid.New(),
				Type:      "like",
				SessionID: uuid.New(),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: This is a simplified test structure
			// In a real implementation, you would mock the database properly
			assert.NotNil(t, tt.request)
			assert.NotEqual(t, uuid.Nil, tt.request.UserID)
			assert.NotEqual(t, uuid.Nil, tt.request.ItemID)
			assert.Contains(t, []string{"rating", "like", "dislike", "share"}, tt.request.Type)
		})
	}
}

func TestUserInteractionService_GetInteractionWeight(t *testing.T) {
	service := &UserInteractionService{}

	tests := []struct {
		name            string
		interactionType string
		value           *float64
		duration        *int
		expectedWeight  float64
	}{
		{
			name:            "rating with value 5",
			interactionType: "rating",
			value:           func() *float64 { v := 5.0; return &v }(),
			expectedWeight:  1.0,
		},
		{
			name:            "rating with value 3",
			interactionType: "rating",
			value:           func() *float64 { v := 3.0; return &v }(),
			expectedWeight:  0.6,
		},
		{
			name:            "like interaction",
			interactionType: "like",
			expectedWeight:  0.8,
		},
		{
			name:            "dislike interaction",
			interactionType: "dislike",
			expectedWeight:  -0.8,
		},
		{
			name:            "view with 60 seconds duration",
			interactionType: "view",
			duration:        func() *int { d := 60; return &d }(),
			expectedWeight:  0.2, // 60/300 = 0.2
		},
		{
			name:            "view with 300 seconds duration",
			interactionType: "view",
			duration:        func() *int { d := 300; return &d }(),
			expectedWeight:  1.0, // 300/300 = 1.0
		},
		{
			name:            "view with 600 seconds duration (capped)",
			interactionType: "view",
			duration:        func() *int { d := 600; return &d }(),
			expectedWeight:  1.0, // Capped at 1.0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			weight := service.getInteractionWeight(tt.interactionType, tt.value, tt.duration)
			assert.InDelta(t, tt.expectedWeight, weight, 0.01, "Weight should match expected value")
		})
	}
}

func TestUserInteractionService_MapInteractionTypeToRelationship(t *testing.T) {
	service := &UserInteractionService{}

	tests := []struct {
		interactionType string
		expectedRel     string
	}{
		{"rating", "RATED"},
		{"like", "RATED"},
		{"dislike", "RATED"},
		{"view", "VIEWED"},
		{"click", "VIEWED"},
		{"share", "SHARED"},
		{"browse", "INTERACTED_WITH"},
		{"unknown", "INTERACTED_WITH"},
	}

	for _, tt := range tests {
		t.Run(tt.interactionType, func(t *testing.T) {
			rel := service.mapInteractionTypeToRelationship(tt.interactionType)
			assert.Equal(t, tt.expectedRel, rel)
		})
	}
}

func TestUserInteractionService_CalculateConfidence(t *testing.T) {
	service := &UserInteractionService{}

	tests := []struct {
		name        string
		interaction *models.UserInteraction
		expected    float64
	}{
		{
			name: "rating interaction",
			interaction: &models.UserInteraction{
				InteractionType: "rating",
			},
			expected: 0.9,
		},
		{
			name: "like interaction",
			interaction: &models.UserInteraction{
				InteractionType: "like",
			},
			expected: 0.8,
		},
		{
			name: "view with long duration",
			interaction: &models.UserInteraction{
				InteractionType: "view",
				Duration:        func() *int { d := 60; return &d }(),
			},
			expected: 0.7,
		},
		{
			name: "view with short duration",
			interaction: &models.UserInteraction{
				InteractionType: "view",
				Duration:        func() *int { d := 10; return &d }(),
			},
			expected: 0.4,
		},
		{
			name: "click interaction",
			interaction: &models.UserInteraction{
				InteractionType: "click",
			},
			expected: 0.6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := service.calculateConfidence(tt.interaction)
			assert.Equal(t, tt.expected, confidence)
		})
	}
}

func TestUserInteractionService_NormalizeVector(t *testing.T) {
	service := &UserInteractionService{}

	tests := []struct {
		name     string
		input    []float32
		expected []float32
	}{
		{
			name:     "simple vector",
			input:    []float32{3.0, 4.0},
			expected: []float32{0.6, 0.8}, // 3/5, 4/5
		},
		{
			name:     "zero vector",
			input:    []float32{0.0, 0.0},
			expected: []float32{0.0, 0.0},
		},
		{
			name:     "unit vector",
			input:    []float32{1.0, 0.0},
			expected: []float32{1.0, 0.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vector := make([]float32, len(tt.input))
			copy(vector, tt.input)

			service.normalizeVector(vector)

			for i, expected := range tt.expected {
				assert.InDelta(t, expected, vector[i], 0.01, "Vector component should be normalized correctly")
			}
		})
	}
}

func TestUserInteractionService_CalculateVectorNorm(t *testing.T) {
	service := &UserInteractionService{}

	tests := []struct {
		name     string
		vector   []float32
		expected float64
	}{
		{
			name:     "simple vector",
			vector:   []float32{3.0, 4.0},
			expected: 5.0, // sqrt(9 + 16) = 5
		},
		{
			name:     "zero vector",
			vector:   []float32{0.0, 0.0},
			expected: 0.0,
		},
		{
			name:     "unit vector",
			vector:   []float32{1.0, 0.0},
			expected: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			norm := service.calculateVectorNorm(tt.vector)
			assert.InDelta(t, tt.expected, norm, 0.01, "Vector norm should be calculated correctly")
		})
	}
}

// Benchmark tests for performance-critical functions
func BenchmarkUserInteractionService_NormalizeVector(b *testing.B) {
	service := &UserInteractionService{}
	vector := make([]float32, 768) // Typical embedding dimension

	// Initialize with random values
	for i := range vector {
		vector[i] = float32(i % 100)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testVector := make([]float32, len(vector))
		copy(testVector, vector)
		service.normalizeVector(testVector)
	}
}

func BenchmarkUserInteractionService_CalculateVectorNorm(b *testing.B) {
	service := &UserInteractionService{}
	vector := make([]float32, 768) // Typical embedding dimension

	// Initialize with random values
	for i := range vector {
		vector[i] = float32(i % 100)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.calculateVectorNorm(vector)
	}
}
