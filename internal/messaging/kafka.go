package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"

	"github.com/temcen/pirex/internal/config"
	"github.com/temcen/pirex/pkg/models"
)

const (
	ContentIngestionTopic    = "content-ingestion"
	ContentIngestionDLQTopic = "content-ingestion-dlq"
	ConsumerGroup            = "content-processors"
)

type KafkaMessage struct {
	JobID           uuid.UUID                      `json:"job_id"`
	ContentItem     models.ContentIngestionRequest `json:"content_item"`
	Timestamp       time.Time                      `json:"timestamp"`
	RetryCount      int                            `json:"retry_count"`
	ProcessingHints map[string]interface{}         `json:"processing_hints,omitempty"`
}

type KafkaProducer struct {
	writer *kafka.Writer
	logger *logrus.Logger
}

type KafkaConsumer struct {
	reader *kafka.Reader
	logger *logrus.Logger
}

type MessageBus struct {
	producer  *KafkaProducer
	consumer  *KafkaConsumer
	dlqWriter *kafka.Writer
	logger    *logrus.Logger
}

func NewMessageBus(cfg *config.Config, logger *logrus.Logger) (*MessageBus, error) {
	// Create producer
	producer := &KafkaProducer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(cfg.Kafka.Brokers...),
			Topic:        ContentIngestionTopic,
			Balancer:     &kafka.Hash{}, // Key by content type for load balancing
			RequiredAcks: kafka.RequireOne,
			Async:        false,
			BatchTimeout: 10 * time.Millisecond,
			BatchSize:    100,
		},
		logger: logger,
	}

	// Create consumer
	consumer := &KafkaConsumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:        cfg.Kafka.Brokers,
			Topic:          ContentIngestionTopic,
			GroupID:        ConsumerGroup,
			MinBytes:       10e3, // 10KB
			MaxBytes:       10e6, // 10MB
			CommitInterval: time.Second,
			StartOffset:    kafka.LastOffset,
		}),
		logger: logger,
	}

	// Create DLQ writer
	dlqWriter := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Kafka.Brokers...),
		Topic:        ContentIngestionDLQTopic,
		RequiredAcks: kafka.RequireOne,
		Async:        false,
	}

	return &MessageBus{
		producer:  producer,
		consumer:  consumer,
		dlqWriter: dlqWriter,
		logger:    logger,
	}, nil
}

func (mb *MessageBus) PublishContentIngestion(jobID uuid.UUID, content models.ContentIngestionRequest, hints map[string]interface{}) error {
	message := KafkaMessage{
		JobID:           jobID,
		ContentItem:     content,
		Timestamp:       time.Now(),
		RetryCount:      0,
		ProcessingHints: hints,
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	kafkaMessage := kafka.Message{
		Key:   []byte(content.Type), // Key by content type for load balancing
		Value: messageBytes,
		Headers: []kafka.Header{
			{Key: "job_id", Value: []byte(jobID.String())},
			{Key: "content_type", Value: []byte(content.Type)},
			{Key: "timestamp", Value: []byte(message.Timestamp.Format(time.RFC3339))},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := mb.producer.writer.WriteMessages(ctx, kafkaMessage); err != nil {
		mb.logger.WithError(err).WithField("job_id", jobID).Error("Failed to publish message to Kafka")
		return fmt.Errorf("failed to write message to Kafka: %w", err)
	}

	mb.logger.WithFields(logrus.Fields{
		"job_id":       jobID,
		"content_type": content.Type,
		"topic":        ContentIngestionTopic,
	}).Info("Message published to Kafka")

	return nil
}

func (mb *MessageBus) ConsumeMessages(ctx context.Context, handler func(KafkaMessage) error) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			message, err := mb.consumer.reader.ReadMessage(ctx)
			if err != nil {
				mb.logger.WithError(err).Error("Failed to read message from Kafka")
				continue
			}

			var kafkaMessage KafkaMessage
			if err := json.Unmarshal(message.Value, &kafkaMessage); err != nil {
				mb.logger.WithError(err).Error("Failed to unmarshal Kafka message")
				continue
			}

			// Process message with retry logic
			if err := mb.processWithRetry(ctx, kafkaMessage, handler); err != nil {
				mb.logger.WithError(err).WithField("job_id", kafkaMessage.JobID).Error("Failed to process message after retries")

				// Send to DLQ after max retries
				if kafkaMessage.RetryCount >= 3 {
					if dlqErr := mb.sendToDLQ(ctx, kafkaMessage, err); dlqErr != nil {
						mb.logger.WithError(dlqErr).Error("Failed to send message to DLQ")
					}
				}
			}
		}
	}
}

func (mb *MessageBus) processWithRetry(ctx context.Context, message KafkaMessage, handler func(KafkaMessage) error) error {
	maxRetries := 3
	baseDelay := time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			delay := baseDelay * time.Duration(1<<uint(attempt-1))
			mb.logger.WithFields(logrus.Fields{
				"job_id":  message.JobID,
				"attempt": attempt,
				"delay":   delay,
			}).Info("Retrying message processing")

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		message.RetryCount = attempt
		if err := handler(message); err != nil {
			mb.logger.WithError(err).WithFields(logrus.Fields{
				"job_id":  message.JobID,
				"attempt": attempt,
			}).Warn("Message processing failed")

			if attempt == maxRetries {
				return fmt.Errorf("max retries exceeded: %w", err)
			}
			continue
		}

		// Success
		mb.logger.WithFields(logrus.Fields{
			"job_id":  message.JobID,
			"attempt": attempt,
		}).Info("Message processed successfully")
		return nil
	}

	return fmt.Errorf("unexpected retry loop exit")
}

func (mb *MessageBus) sendToDLQ(ctx context.Context, message KafkaMessage, originalError error) error {
	dlqMessage := map[string]interface{}{
		"original_message": message,
		"error":            originalError.Error(),
		"dlq_timestamp":    time.Now(),
	}

	dlqBytes, err := json.Marshal(dlqMessage)
	if err != nil {
		return fmt.Errorf("failed to marshal DLQ message: %w", err)
	}

	kafkaMessage := kafka.Message{
		Key:   []byte(message.JobID.String()),
		Value: dlqBytes,
		Headers: []kafka.Header{
			{Key: "job_id", Value: []byte(message.JobID.String())},
			{Key: "original_topic", Value: []byte(ContentIngestionTopic)},
			{Key: "error", Value: []byte(originalError.Error())},
		},
	}

	if err := mb.dlqWriter.WriteMessages(ctx, kafkaMessage); err != nil {
		return fmt.Errorf("failed to write message to DLQ: %w", err)
	}

	mb.logger.WithFields(logrus.Fields{
		"job_id": message.JobID,
		"error":  originalError.Error(),
	}).Warn("Message sent to DLQ")

	return nil
}

func (mb *MessageBus) Close() error {
	var errors []error

	if err := mb.producer.writer.Close(); err != nil {
		errors = append(errors, fmt.Errorf("failed to close producer: %w", err))
	}

	if err := mb.consumer.reader.Close(); err != nil {
		errors = append(errors, fmt.Errorf("failed to close consumer: %w", err))
	}

	if err := mb.dlqWriter.Close(); err != nil {
		errors = append(errors, fmt.Errorf("failed to close DLQ writer: %w", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors closing message bus: %v", errors)
	}

	return nil
}

// GetMetrics returns Kafka metrics for monitoring
func (mb *MessageBus) GetMetrics() map[string]interface{} {
	stats := mb.consumer.reader.Stats()
	return map[string]interface{}{
		"consumer_lag":    stats.Lag,
		"consumer_offset": stats.Offset,
		"messages_read":   stats.Messages,
		"bytes_read":      stats.Bytes,
		"rebalances":      stats.Rebalances,
		"timeouts":        stats.Timeouts,
		"errors":          stats.Errors,
	}
}
