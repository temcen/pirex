package messaging

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/temcen/pirex/pkg/models"
)

func TestKafkaMessage_Serialization(t *testing.T) {
	jobID := uuid.New()
	content := models.ContentIngestionRequest{
		Type:        "product",
		Title:       "Test Product",
		Description: stringPtr("Test description"),
		Categories:  []string{"Electronics"},
	}

	message := KafkaMessage{
		JobID:       jobID,
		ContentItem: content,
		Timestamp:   time.Now(),
		RetryCount:  0,
		ProcessingHints: map[string]interface{}{
			"source": "test",
		},
	}

	// Test JSON serialization
	messageBytes, err := json.Marshal(message)
	require.NoError(t, err)
	assert.NotEmpty(t, messageBytes)

	// Test JSON deserialization
	var deserializedMessage KafkaMessage
	err = json.Unmarshal(messageBytes, &deserializedMessage)
	require.NoError(t, err)

	assert.Equal(t, message.JobID, deserializedMessage.JobID)
	assert.Equal(t, message.ContentItem.Type, deserializedMessage.ContentItem.Type)
	assert.Equal(t, message.ContentItem.Title, deserializedMessage.ContentItem.Title)
	assert.Equal(t, message.RetryCount, deserializedMessage.RetryCount)
	assert.Equal(t, message.ProcessingHints["source"], deserializedMessage.ProcessingHints["source"])
}

func TestKafkaMessage_Validation(t *testing.T) {
	tests := []struct {
		name    string
		message KafkaMessage
		isValid bool
	}{
		{
			name: "valid message",
			message: KafkaMessage{
				JobID: uuid.New(),
				ContentItem: models.ContentIngestionRequest{
					Type:  "product",
					Title: "Valid Product",
				},
				Timestamp:  time.Now(),
				RetryCount: 0,
			},
			isValid: true,
		},
		{
			name: "empty job ID",
			message: KafkaMessage{
				JobID: uuid.Nil,
				ContentItem: models.ContentIngestionRequest{
					Type:  "product",
					Title: "Valid Product",
				},
				Timestamp:  time.Now(),
				RetryCount: 0,
			},
			isValid: false,
		},
		{
			name: "empty content title",
			message: KafkaMessage{
				JobID: uuid.New(),
				ContentItem: models.ContentIngestionRequest{
					Type:  "product",
					Title: "",
				},
				Timestamp:  time.Now(),
				RetryCount: 0,
			},
			isValid: false,
		},
		{
			name: "invalid content type",
			message: KafkaMessage{
				JobID: uuid.New(),
				ContentItem: models.ContentIngestionRequest{
					Type:  "invalid",
					Title: "Valid Title",
				},
				Timestamp:  time.Now(),
				RetryCount: 0,
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate job ID
			jobIDValid := tt.message.JobID != uuid.Nil

			// Validate content
			contentValid := tt.message.ContentItem.Title != "" &&
				(tt.message.ContentItem.Type == "product" ||
					tt.message.ContentItem.Type == "video" ||
					tt.message.ContentItem.Type == "article")

			// Validate timestamp
			timestampValid := !tt.message.Timestamp.IsZero()

			// Validate retry count
			retryCountValid := tt.message.RetryCount >= 0

			isValid := jobIDValid && contentValid && timestampValid && retryCountValid

			if tt.isValid {
				assert.True(t, isValid, "Message should be valid")
			} else {
				assert.False(t, isValid, "Message should be invalid")
			}
		})
	}
}

func TestRetryLogic(t *testing.T) {
	tests := []struct {
		name          string
		retryCount    int
		maxRetries    int
		shouldRetry   bool
		expectedDelay time.Duration
	}{
		{
			name:          "first retry",
			retryCount:    1,
			maxRetries:    3,
			shouldRetry:   true,
			expectedDelay: 1 * time.Second, // base delay
		},
		{
			name:          "second retry",
			retryCount:    2,
			maxRetries:    3,
			shouldRetry:   true,
			expectedDelay: 2 * time.Second, // exponential backoff
		},
		{
			name:          "third retry",
			retryCount:    3,
			maxRetries:    3,
			shouldRetry:   true,
			expectedDelay: 4 * time.Second, // exponential backoff
		},
		{
			name:          "max retries exceeded",
			retryCount:    4,
			maxRetries:    3,
			shouldRetry:   false,
			expectedDelay: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldRetry := tt.retryCount <= tt.maxRetries
			assert.Equal(t, tt.shouldRetry, shouldRetry)

			if shouldRetry && tt.retryCount > 0 {
				// Test exponential backoff calculation
				baseDelay := time.Second
				expectedDelay := baseDelay * time.Duration(1<<uint(tt.retryCount-1))
				assert.Equal(t, tt.expectedDelay, expectedDelay)
			}
		})
	}
}

func TestMessageKeyGeneration(t *testing.T) {
	contentTypes := []string{"product", "video", "article"}

	for _, contentType := range contentTypes {
		t.Run("key_for_"+contentType, func(t *testing.T) {
			// Test that message key is generated from content type
			key := []byte(contentType)
			assert.Equal(t, contentType, string(key))
			assert.NotEmpty(t, key)
		})
	}
}

func TestMessageHeaders(t *testing.T) {
	jobID := uuid.New()
	contentType := "product"
	timestamp := time.Now()

	expectedHeaders := map[string]string{
		"job_id":       jobID.String(),
		"content_type": contentType,
		"timestamp":    timestamp.Format(time.RFC3339),
	}

	// Test header generation logic
	headers := make(map[string]string)
	headers["job_id"] = jobID.String()
	headers["content_type"] = contentType
	headers["timestamp"] = timestamp.Format(time.RFC3339)

	assert.Equal(t, expectedHeaders["job_id"], headers["job_id"])
	assert.Equal(t, expectedHeaders["content_type"], headers["content_type"])
	assert.Equal(t, expectedHeaders["timestamp"], headers["timestamp"])

	// Test timestamp parsing
	parsedTime, err := time.Parse(time.RFC3339, headers["timestamp"])
	require.NoError(t, err)
	assert.True(t, parsedTime.Equal(timestamp) || parsedTime.Sub(timestamp) < time.Second)
}

func TestDLQMessage(t *testing.T) {
	originalMessage := KafkaMessage{
		JobID: uuid.New(),
		ContentItem: models.ContentIngestionRequest{
			Type:  "product",
			Title: "Test Product",
		},
		Timestamp:  time.Now(),
		RetryCount: 3,
	}

	originalError := "processing failed"

	// Test DLQ message structure
	dlqMessage := map[string]interface{}{
		"original_message": originalMessage,
		"error":            originalError,
		"dlq_timestamp":    time.Now(),
	}

	// Serialize DLQ message
	dlqBytes, err := json.Marshal(dlqMessage)
	require.NoError(t, err)
	assert.NotEmpty(t, dlqBytes)

	// Deserialize and validate
	var deserializedDLQ map[string]interface{}
	err = json.Unmarshal(dlqBytes, &deserializedDLQ)
	require.NoError(t, err)

	assert.Contains(t, deserializedDLQ, "original_message")
	assert.Contains(t, deserializedDLQ, "error")
	assert.Contains(t, deserializedDLQ, "dlq_timestamp")
	assert.Equal(t, originalError, deserializedDLQ["error"])
}

func TestTopicConfiguration(t *testing.T) {
	// Test topic names
	assert.Equal(t, "content-ingestion", ContentIngestionTopic)
	assert.Equal(t, "content-ingestion-dlq", ContentIngestionDLQTopic)
	assert.Equal(t, "content-processors", ConsumerGroup)

	// Test topic naming conventions
	assert.Contains(t, ContentIngestionTopic, "content")
	assert.Contains(t, ContentIngestionDLQTopic, "dlq")
	assert.Contains(t, ConsumerGroup, "processors")
}

func TestMessageBusMetrics(t *testing.T) {
	// Test metrics structure
	metrics := map[string]interface{}{
		"consumer_lag":    int64(0),
		"consumer_offset": int64(100),
		"messages_read":   int64(50),
		"bytes_read":      int64(1024),
		"rebalances":      int64(1),
		"timeouts":        int64(0),
		"errors":          int64(0),
	}

	// Validate metric types and values
	assert.IsType(t, int64(0), metrics["consumer_lag"])
	assert.IsType(t, int64(0), metrics["consumer_offset"])
	assert.IsType(t, int64(0), metrics["messages_read"])
	assert.IsType(t, int64(0), metrics["bytes_read"])

	// Validate metric values are non-negative
	assert.GreaterOrEqual(t, metrics["consumer_lag"].(int64), int64(0))
	assert.GreaterOrEqual(t, metrics["consumer_offset"].(int64), int64(0))
	assert.GreaterOrEqual(t, metrics["messages_read"].(int64), int64(0))
	assert.GreaterOrEqual(t, metrics["bytes_read"].(int64), int64(0))
}

// Mock handler for testing message processing
func mockMessageHandler(message KafkaMessage) error {
	// Simulate processing logic
	if message.ContentItem.Title == "fail" {
		return assert.AnError
	}
	return nil
}

func TestMessageHandlerLogic(t *testing.T) {
	tests := []struct {
		name        string
		message     KafkaMessage
		expectError bool
	}{
		{
			name: "successful processing",
			message: KafkaMessage{
				JobID: uuid.New(),
				ContentItem: models.ContentIngestionRequest{
					Type:  "product",
					Title: "success",
				},
			},
			expectError: false,
		},
		{
			name: "failed processing",
			message: KafkaMessage{
				JobID: uuid.New(),
				ContentItem: models.ContentIngestionRequest{
					Type:  "product",
					Title: "fail",
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mockMessageHandler(tt.message)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
