package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/temcen/pirex/internal/database"
	"github.com/temcen/pirex/internal/messaging"
	"github.com/temcen/pirex/pkg/models"
)

type PipelineOrchestrator struct {
	db           *database.Database
	messageBus   *messaging.MessageBus
	preprocessor *DataPreprocessor
	jobManager   *JobManager
	logger       *logrus.Logger

	// Worker pool configuration
	workerCount int
	workerPool  chan chan messaging.KafkaMessage
	jobQueue    chan messaging.KafkaMessage
	workers     []*Worker
	quit        chan bool
	wg          sync.WaitGroup
}

type Worker struct {
	id           int
	orchestrator *PipelineOrchestrator
	jobChannel   chan messaging.KafkaMessage
	quit         chan bool
	logger       *logrus.Logger
}

type ProcessingStage string

const (
	StageValidate          ProcessingStage = "validate"
	StagePreprocess        ProcessingStage = "preprocess"
	StageGenerateEmbedding ProcessingStage = "generate_embedding"
	StageStore             ProcessingStage = "store"
	StageUpdateCache       ProcessingStage = "update_cache"
)

type ProcessingContext struct {
	JobID            uuid.UUID
	Message          messaging.KafkaMessage
	ProcessedContent *models.ContentItem
	ProcessingResult *ProcessingResult
	CurrentStage     ProcessingStage
	StartTime        time.Time
	StageTimings     map[ProcessingStage]time.Duration
	Errors           []error
}

func NewPipelineOrchestrator(
	db *database.Database,
	messageBus *messaging.MessageBus,
	preprocessor *DataPreprocessor,
	jobManager *JobManager,
	logger *logrus.Logger,
) *PipelineOrchestrator {
	workerCount := 5 // As specified in requirements

	po := &PipelineOrchestrator{
		db:           db,
		messageBus:   messageBus,
		preprocessor: preprocessor,
		jobManager:   jobManager,
		logger:       logger,
		workerCount:  workerCount,
		workerPool:   make(chan chan messaging.KafkaMessage, workerCount),
		jobQueue:     make(chan messaging.KafkaMessage, 100),
		quit:         make(chan bool),
	}

	// Initialize workers
	po.workers = make([]*Worker, workerCount)
	for i := 0; i < workerCount; i++ {
		po.workers[i] = &Worker{
			id:           i + 1,
			orchestrator: po,
			jobChannel:   make(chan messaging.KafkaMessage),
			quit:         make(chan bool),
			logger:       logger,
		}
	}

	return po
}

func (po *PipelineOrchestrator) Start(ctx context.Context) error {
	po.logger.Info("Starting pipeline orchestrator")

	// Start workers
	for _, worker := range po.workers {
		po.wg.Add(1)
		go worker.start(&po.wg)
	}

	// Start dispatcher
	po.wg.Add(1)
	go po.dispatch(&po.wg)

	// Start Kafka consumer
	po.wg.Add(1)
	go func() {
		defer po.wg.Done()
		if err := po.messageBus.ConsumeMessages(ctx, po.handleMessage); err != nil {
			po.logger.WithError(err).Error("Kafka consumer stopped")
		}
	}()

	po.logger.WithField("worker_count", po.workerCount).Info("Pipeline orchestrator started")
	return nil
}

func (po *PipelineOrchestrator) Stop() error {
	po.logger.Info("Stopping pipeline orchestrator")

	// Signal all workers to quit
	close(po.quit)
	for _, worker := range po.workers {
		worker.quit <- true
	}

	// Wait for all workers to finish
	po.wg.Wait()

	po.logger.Info("Pipeline orchestrator stopped")
	return nil
}

func (po *PipelineOrchestrator) handleMessage(message messaging.KafkaMessage) error {
	// Add message to job queue
	select {
	case po.jobQueue <- message:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("job queue full, message dropped")
	}
}

func (po *PipelineOrchestrator) dispatch(wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case job := <-po.jobQueue:
			// Get available worker
			select {
			case jobChannel := <-po.workerPool:
				// Dispatch job to worker
				jobChannel <- job
			case <-po.quit:
				return
			}
		case <-po.quit:
			return
		}
	}
}

func (w *Worker) start(wg *sync.WaitGroup) {
	defer wg.Done()

	w.logger.Info("Worker started")

	for {
		// Add worker to pool
		w.orchestrator.workerPool <- w.jobChannel

		select {
		case job := <-w.jobChannel:
			w.processJob(job)
		case <-w.quit:
			w.logger.Info("Worker stopped")
			return
		}
	}
}

func (w *Worker) processJob(message messaging.KafkaMessage) {
	ctx := context.Background()

	processingCtx := &ProcessingContext{
		JobID:        message.JobID,
		Message:      message,
		CurrentStage: StageValidate,
		StartTime:    time.Now(),
		StageTimings: make(map[ProcessingStage]time.Duration),
		Errors:       []error{},
	}

	w.logger.WithFields(logrus.Fields{
		"job_id":       message.JobID,
		"content_type": message.ContentItem.Type,
		"retry_count":  message.RetryCount,
	}).Info("Processing job")

	// Update job status to processing
	if err := w.orchestrator.jobManager.UpdateJobProgress(
		ctx, message.JobID, 0, 0, JobStatusProcessing, nil,
	); err != nil {
		w.logger.WithError(err).Warn("Failed to update job status to processing")
	}

	// Execute processing pipeline
	success := w.executePipeline(ctx, processingCtx)

	// Update final job status
	if success {
		if err := w.orchestrator.jobManager.UpdateJobProgress(
			ctx, message.JobID, 1, 0, JobStatusCompleted, nil,
		); err != nil {
			w.logger.WithError(err).Warn("Failed to update job status to completed")
		}
		w.logger.WithField("job_id", message.JobID).Info("Job completed successfully")
	} else {
		errorMsg := "Processing failed"
		if len(processingCtx.Errors) > 0 {
			errorMsg = processingCtx.Errors[0].Error()
		}

		if err := w.orchestrator.jobManager.UpdateJobProgress(
			ctx, message.JobID, 0, 1, JobStatusFailed, &errorMsg,
		); err != nil {
			w.logger.WithError(err).Warn("Failed to update job status to failed")
		}
		w.logger.WithField("job_id", message.JobID).Error("Job failed")
	}

	// Log processing metrics
	totalTime := time.Since(processingCtx.StartTime)
	w.logger.WithFields(logrus.Fields{
		"job_id":      message.JobID,
		"total_time":  totalTime,
		"stage_times": processingCtx.StageTimings,
		"success":     success,
		"error_count": len(processingCtx.Errors),
	}).Info("Job processing completed")
}

func (w *Worker) executePipeline(ctx context.Context, processingCtx *ProcessingContext) bool {
	stages := []ProcessingStage{
		StageValidate,
		StagePreprocess,
		StageGenerateEmbedding,
		StageStore,
		StageUpdateCache,
	}

	for _, stage := range stages {
		processingCtx.CurrentStage = stage
		stageStart := time.Now()

		success := w.executeStage(ctx, processingCtx, stage)

		processingCtx.StageTimings[stage] = time.Since(stageStart)

		if !success {
			w.logger.WithFields(logrus.Fields{
				"job_id": processingCtx.JobID,
				"stage":  stage,
				"errors": len(processingCtx.Errors),
			}).Error("Stage failed")
			return false
		}

		w.logger.WithFields(logrus.Fields{
			"job_id":     processingCtx.JobID,
			"stage":      stage,
			"stage_time": processingCtx.StageTimings[stage],
		}).Debug("Stage completed")
	}

	return true
}

func (w *Worker) executeStage(ctx context.Context, processingCtx *ProcessingContext, stage ProcessingStage) bool {
	switch stage {
	case StageValidate:
		return w.validateContent(processingCtx)
	case StagePreprocess:
		return w.preprocessContent(ctx, processingCtx)
	case StageGenerateEmbedding:
		return w.generateEmbedding(ctx, processingCtx)
	case StageStore:
		return w.storeContent(ctx, processingCtx)
	case StageUpdateCache:
		return w.updateCache(ctx, processingCtx)
	default:
		processingCtx.Errors = append(processingCtx.Errors, fmt.Errorf("unknown stage: %s", stage))
		return false
	}
}

func (w *Worker) validateContent(processingCtx *ProcessingContext) bool {
	content := processingCtx.Message.ContentItem

	// Basic validation
	if content.Title == "" {
		processingCtx.Errors = append(processingCtx.Errors, fmt.Errorf("title is required"))
		return false
	}

	if content.Type == "" {
		processingCtx.Errors = append(processingCtx.Errors, fmt.Errorf("type is required"))
		return false
	}

	validTypes := map[string]bool{"product": true, "video": true, "article": true}
	if !validTypes[content.Type] {
		processingCtx.Errors = append(processingCtx.Errors, fmt.Errorf("invalid content type: %s", content.Type))
		return false
	}

	return true
}

func (w *Worker) preprocessContent(ctx context.Context, processingCtx *ProcessingContext) bool {
	result, err := w.orchestrator.preprocessor.ProcessContent(
		ctx, processingCtx.JobID, processingCtx.Message.ContentItem,
	)
	if err != nil {
		processingCtx.Errors = append(processingCtx.Errors, fmt.Errorf("preprocessing failed: %w", err))
		return false
	}

	processingCtx.ProcessingResult = result
	processingCtx.ProcessedContent = result.ProcessedContent

	// Check if preprocessing had critical errors
	if len(result.Errors) > 0 {
		w.logger.WithFields(logrus.Fields{
			"job_id":      processingCtx.JobID,
			"error_count": len(result.Errors),
		}).Warn("Preprocessing completed with errors")
	}

	return true
}

func (w *Worker) generateEmbedding(ctx context.Context, processingCtx *ProcessingContext) bool {
	// TODO: This will be implemented in task 3 (ML Models and Embedding Generation)
	// For now, we'll create a placeholder embedding

	w.logger.WithField("job_id", processingCtx.JobID).Info("Embedding generation placeholder - will be implemented in task 3")

	// Create placeholder embedding (768 dimensions as specified in design)
	embedding := make([]float32, 768)
	for i := range embedding {
		embedding[i] = 0.1 // Placeholder value
	}

	processingCtx.ProcessedContent.Embedding = embedding
	return true
}

func (w *Worker) storeContent(ctx context.Context, processingCtx *ProcessingContext) bool {
	content := processingCtx.ProcessedContent

	// Store in PostgreSQL
	query := `
		INSERT INTO content_items (
			id, type, title, description, image_urls, metadata, categories,
			embedding, quality_score, active, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (id) DO UPDATE SET
			type = EXCLUDED.type,
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			image_urls = EXCLUDED.image_urls,
			metadata = EXCLUDED.metadata,
			categories = EXCLUDED.categories,
			embedding = EXCLUDED.embedding,
			quality_score = EXCLUDED.quality_score,
			active = EXCLUDED.active,
			updated_at = EXCLUDED.updated_at
	`

	_, err := w.orchestrator.db.PG.Exec(ctx, query,
		content.ID, content.Type, content.Title, content.Description,
		content.ImageURLs, content.Metadata, content.Categories,
		content.Embedding, content.QualityScore, content.Active,
		content.CreatedAt, content.UpdatedAt,
	)

	if err != nil {
		processingCtx.Errors = append(processingCtx.Errors, fmt.Errorf("failed to store content: %w", err))
		return false
	}

	w.logger.WithFields(logrus.Fields{
		"job_id":        processingCtx.JobID,
		"content_id":    content.ID,
		"quality_score": content.QualityScore,
	}).Info("Content stored in PostgreSQL")

	return true
}

func (w *Worker) updateCache(ctx context.Context, processingCtx *ProcessingContext) bool {
	content := processingCtx.ProcessedContent

	// Cache content metadata in Redis warm cache
	cacheKey := fmt.Sprintf("content:%s", content.ID.String())

	// Create cache-friendly version (without embedding for size)
	cacheContent := map[string]interface{}{
		"id":            content.ID,
		"type":          content.Type,
		"title":         content.Title,
		"description":   content.Description,
		"image_urls":    content.ImageURLs,
		"categories":    content.Categories,
		"quality_score": content.QualityScore,
		"active":        content.Active,
		"updated_at":    content.UpdatedAt,
	}

	if err := w.orchestrator.db.Redis.Warm.HSet(ctx, cacheKey, cacheContent).Err(); err != nil {
		w.logger.WithError(err).WithField("job_id", processingCtx.JobID).Warn("Failed to cache content metadata")
		// Don't fail the entire pipeline for cache errors
	}

	// Set cache TTL (1 hour as specified in design)
	if err := w.orchestrator.db.Redis.Warm.Expire(ctx, cacheKey, time.Hour).Err(); err != nil {
		w.logger.WithError(err).WithField("job_id", processingCtx.JobID).Warn("Failed to set cache TTL")
	}

	// Cache embedding in cold cache (longer TTL)
	embeddingKey := fmt.Sprintf("embedding:%s", content.ID.String())
	if err := w.orchestrator.db.Redis.Cold.Set(ctx, embeddingKey, content.Embedding, 24*time.Hour).Err(); err != nil {
		w.logger.WithError(err).WithField("job_id", processingCtx.JobID).Warn("Failed to cache embedding")
	}

	w.logger.WithField("job_id", processingCtx.JobID).Debug("Content cached successfully")
	return true
}

// GetMetrics returns pipeline processing metrics
func (po *PipelineOrchestrator) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"worker_count":      po.workerCount,
		"queue_length":      len(po.jobQueue),
		"available_workers": len(po.workerPool),
	}
}
