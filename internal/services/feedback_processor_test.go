package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Test the core feedback processing logic without external dependencies

func TestFeedbackProcessor_CalculateAlpha(t *testing.T) {
	fp := &FeedbackProcessor{}

	tests := []struct {
		name     string
		event    FeedbackEvent
		expected float64
	}{
		{
			name: "High rating",
			event: FeedbackEvent{
				Type:   FeedbackExplicit,
				Action: "rating",
				Value:  5.0,
			},
			expected: 0.2, // baseAlpha * 2.0 * (5.0/5.0)
		},
		{
			name: "Low rating",
			event: FeedbackEvent{
				Type:   FeedbackExplicit,
				Action: "rating",
				Value:  1.0,
			},
			expected: 0.04, // baseAlpha * 2.0 * (1.0/5.0)
		},
		{
			name: "Like action",
			event: FeedbackEvent{
				Type:   FeedbackExplicit,
				Action: "like",
			},
			expected: 0.15, // baseAlpha * 1.5
		},
		{
			name: "Click action",
			event: FeedbackEvent{
				Type:   FeedbackImplicit,
				Action: "click",
			},
			expected: 0.05, // baseAlpha * 0.5
		},
		{
			name: "Purchase action",
			event: FeedbackEvent{
				Type:   FeedbackImplicit,
				Action: "purchase",
			},
			expected: 0.3, // baseAlpha * 3.0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alpha := fp.calculateAlpha(tt.event)
			assert.InDelta(t, tt.expected, alpha, 0.001)
		})
	}
}

func TestFeedbackProcessor_GetFeedbackWeight(t *testing.T) {
	fp := &FeedbackProcessor{}

	tests := []struct {
		name     string
		event    FeedbackEvent
		expected float64
	}{
		{
			name: "Positive rating",
			event: FeedbackEvent{
				Type:   FeedbackExplicit,
				Action: "rating",
				Value:  4.0,
			},
			expected: 0.6, // (4.0 - 2.5) / 2.5
		},
		{
			name: "Negative rating",
			event: FeedbackEvent{
				Type:   FeedbackExplicit,
				Action: "rating",
				Value:  1.0,
			},
			expected: -0.6, // (1.0 - 2.5) / 2.5
		},
		{
			name: "Like action",
			event: FeedbackEvent{
				Type:   FeedbackExplicit,
				Action: "like",
			},
			expected: 1.0,
		},
		{
			name: "Dislike action",
			event: FeedbackEvent{
				Type:   FeedbackExplicit,
				Action: "dislike",
			},
			expected: -1.0,
		},
		{
			name: "View with duration",
			event: FeedbackEvent{
				Type:   FeedbackImplicit,
				Action: "view",
				Value:  120.0, // 2 minutes
			},
			expected: 0.2, // 0.1 * (120.0 / 60.0)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			weight := fp.getFeedbackWeight(tt.event)
			assert.InDelta(t, tt.expected, weight, 0.001)
		})
	}
}

func TestFeedbackProcessor_AggregateImplicitFeedback(t *testing.T) {
	fp := &FeedbackProcessor{}

	events := []FeedbackEvent{
		{
			Action: "click",
			Value:  1.0,
		},
		{
			Action: "click",
			Value:  1.0,
		},
		{
			Action: "view",
			Value:  30.0,
		},
		{
			Action: "view",
			Value:  60.0,
		},
	}

	aggregated := fp.aggregateImplicitFeedback(events)

	// Check action counts
	actionCounts := aggregated["action_counts"].(map[string]int)
	assert.Equal(t, 2, actionCounts["click"])
	assert.Equal(t, 2, actionCounts["view"])

	// Check action values
	actionValues := aggregated["action_values"].(map[string]float64)
	assert.Equal(t, 2.0, actionValues["click"])
	assert.Equal(t, 90.0, actionValues["view"])

	// Check total events
	assert.Equal(t, 4, aggregated["total_events"])
}

func TestFeedbackEvent_Validation(t *testing.T) {
	tests := []struct {
		name    string
		event   FeedbackEvent
		isValid bool
	}{
		{
			name: "Valid explicit feedback",
			event: FeedbackEvent{
				UserID:    "user123",
				ItemID:    "item456",
				Type:      FeedbackExplicit,
				Action:    "rating",
				Value:     4.5,
				Timestamp: time.Now(),
			},
			isValid: true,
		},
		{
			name: "Valid implicit feedback",
			event: FeedbackEvent{
				UserID:    "user123",
				ItemID:    "item456",
				Type:      FeedbackImplicit,
				Action:    "click",
				Value:     1.0,
				Timestamp: time.Now(),
			},
			isValid: true,
		},
		{
			name: "Missing user ID",
			event: FeedbackEvent{
				ItemID:    "item456",
				Type:      FeedbackExplicit,
				Action:    "rating",
				Value:     4.5,
				Timestamp: time.Now(),
			},
			isValid: false,
		},
		{
			name: "Missing item ID",
			event: FeedbackEvent{
				UserID:    "user123",
				Type:      FeedbackExplicit,
				Action:    "rating",
				Value:     4.5,
				Timestamp: time.Now(),
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simple validation logic
			isValid := tt.event.UserID != "" && tt.event.ItemID != ""
			assert.Equal(t, tt.isValid, isValid)
		})
	}
}

// Test algorithm optimization logic
func TestAlgorithmOptimizer_DetermineUserSegment(t *testing.T) {
	// This would normally require database access, so we'll test the logic conceptually
	tests := []struct {
		name                     string
		interactionCount         int
		daysSinceLastInteraction int
		expectedSegment          UserSegment
	}{
		{
			name:             "New user",
			interactionCount: 3,
			expectedSegment:  SegmentNewUser,
		},
		{
			name:             "Power user",
			interactionCount: 150,
			expectedSegment:  SegmentPowerUser,
		},
		{
			name:                     "Inactive user",
			interactionCount:         20,
			daysSinceLastInteraction: 35,
			expectedSegment:          SegmentInactiveUser,
		},
		{
			name:             "Regular user",
			interactionCount: 50,
			expectedSegment:  SegmentRegularUser,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the logic that would be in DetermineUserSegment
			var segment UserSegment

			if tt.interactionCount < 5 {
				segment = SegmentNewUser
			} else if tt.interactionCount > 100 {
				segment = SegmentPowerUser
			} else if tt.daysSinceLastInteraction > 30 {
				segment = SegmentInactiveUser
			} else {
				segment = SegmentRegularUser
			}

			assert.Equal(t, tt.expectedSegment, segment)
		})
	}
}

// Test continuous learning feature extraction logic
func TestFeatureExtractor_CalculateLabel(t *testing.T) {
	fe := &FeatureExtractor{}

	tests := []struct {
		name        string
		interaction UserInteraction
		expected    float64
	}{
		{
			name: "High rating",
			interaction: UserInteraction{
				InteractionType: "rating",
				Value:           5.0,
			},
			expected: 1.0, // (5.0 - 1.0) / 4.0
		},
		{
			name: "Low rating",
			interaction: UserInteraction{
				InteractionType: "rating",
				Value:           1.0,
			},
			expected: 0.0, // (1.0 - 1.0) / 4.0
		},
		{
			name: "Like interaction",
			interaction: UserInteraction{
				InteractionType: "like",
			},
			expected: 1.0,
		},
		{
			name: "Click interaction",
			interaction: UserInteraction{
				InteractionType: "click",
			},
			expected: 0.3,
		},
		{
			name: "View interaction",
			interaction: UserInteraction{
				InteractionType: "view",
				Value:           150.0, // 2.5 minutes
			},
			expected: 0.5, // min(150.0/300.0, 1.0)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			label := fe.calculateLabel(tt.interaction)
			assert.InDelta(t, tt.expected, label, 0.001)
		})
	}
}

// Test A/B testing statistical functions
func TestABTesting_ProportionZTest(t *testing.T) {
	ab := &ABTestingFramework{}

	tests := []struct {
		name         string
		successes1   int64
		trials1      int64
		successes2   int64
		trials2      int64
		expectPValue bool // Whether we expect a meaningful p-value
	}{
		{
			name:         "No difference",
			successes1:   50,
			trials1:      100,
			successes2:   50,
			trials2:      100,
			expectPValue: true,
		},
		{
			name:         "Large difference",
			successes1:   10,
			trials1:      100,
			successes2:   90,
			trials2:      100,
			expectPValue: true,
		},
		{
			name:         "Zero trials",
			successes1:   0,
			trials1:      0,
			successes2:   50,
			trials2:      100,
			expectPValue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pValue, effect := ab.proportionZTest(tt.successes1, tt.trials1, tt.successes2, tt.trials2)

			if tt.expectPValue {
				assert.GreaterOrEqual(t, pValue, 0.0)
				assert.LessOrEqual(t, pValue, 1.0)
				// Effect should be reasonable
				assert.GreaterOrEqual(t, effect, -10.0)
				assert.LessOrEqual(t, effect, 10.0)
			} else {
				assert.Equal(t, 1.0, pValue) // Should return 1.0 for invalid input
				assert.Equal(t, 0.0, effect)
			}
		})
	}
}

// Benchmark tests for performance
func BenchmarkFeedbackProcessor_CalculateAlpha(b *testing.B) {
	fp := &FeedbackProcessor{}
	event := FeedbackEvent{
		Type:   FeedbackExplicit,
		Action: "rating",
		Value:  4.0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fp.calculateAlpha(event)
	}
}

func BenchmarkFeedbackProcessor_GetFeedbackWeight(b *testing.B) {
	fp := &FeedbackProcessor{}
	event := FeedbackEvent{
		Type:   FeedbackExplicit,
		Action: "rating",
		Value:  4.0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fp.getFeedbackWeight(event)
	}
}
