package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"github.com/temcen/pirex/internal/database"
)

type JobManager struct {
	db     *database.Database
	logger *logrus.Logger
}

type JobProgress struct {
	JobID          uuid.UUID              `json:"job_id"`
	Status         string                 `json:"status"`
	Progress       int                    `json:"progress"`
	TotalItems     int                    `json:"total_items"`
	ProcessedItems int                    `json:"processed_items"`
	FailedItems    int                    `json:"failed_items"`
	EstimatedTime  *int                   `json:"estimated_time,omitempty"`
	ErrorMessage   *string                `json:"error_message,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
	Details        map[string]interface{} `json:"details,omitempty"`
}

const (
	JobStatusQueued     = "queued"
	JobStatusProcessing = "processing"
	JobStatusCompleted  = "completed"
	JobStatusFailed     = "failed"
	JobStatusCancelled  = "cancelled"
)

func NewJobManager(db *database.Database, logger *logrus.Logger) *JobManager {
	return &JobManager{
		db:     db,
		logger: logger,
	}
}

func (jm *JobManager) CreateJob(ctx context.Context, totalItems int, jobType string) (*JobProgress, error) {
	jobID := uuid.New()
	now := time.Now()

	job := &JobProgress{
		JobID:          jobID,
		Status:         JobStatusQueued,
		Progress:       0,
		TotalItems:     totalItems,
		ProcessedItems: 0,
		FailedItems:    0,
		CreatedAt:      now,
		UpdatedAt:      now,
		Details: map[string]interface{}{
			"job_type": jobType,
		},
	}

	// Estimate processing time (rough estimate: 2 seconds per item)
	estimatedSeconds := totalItems * 2
	job.EstimatedTime = &estimatedSeconds

	// Store in Redis for fast access
	if err := jm.storeJobInRedis(ctx, job); err != nil {
		return nil, fmt.Errorf("failed to store job in Redis: %w", err)
	}

	// Store in PostgreSQL for persistence
	if err := jm.storeJobInPostgreSQL(ctx, job); err != nil {
		jm.logger.WithError(err).WithField("job_id", jobID).Warn("Failed to store job in PostgreSQL")
		// Continue anyway, Redis is primary for job tracking
	}

	jm.logger.WithFields(logrus.Fields{
		"job_id":      jobID,
		"total_items": totalItems,
		"job_type":    jobType,
	}).Info("Job created")

	return job, nil
}

func (jm *JobManager) GetJob(ctx context.Context, jobID uuid.UUID) (*JobProgress, error) {
	// Try Redis first for fast access
	job, err := jm.getJobFromRedis(ctx, jobID)
	if err == nil {
		return job, nil
	}

	if err != redis.Nil {
		jm.logger.WithError(err).WithField("job_id", jobID).Warn("Failed to get job from Redis")
	}

	// Fallback to PostgreSQL
	job, err = jm.getJobFromPostgreSQL(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("job not found: %w", err)
	}

	// Restore to Redis for future fast access
	if redisErr := jm.storeJobInRedis(ctx, job); redisErr != nil {
		jm.logger.WithError(redisErr).WithField("job_id", jobID).Warn("Failed to restore job to Redis")
	}

	return job, nil
}

func (jm *JobManager) UpdateJobProgress(ctx context.Context, jobID uuid.UUID, processedItems, failedItems int, status string, errorMessage *string) error {
	job, err := jm.GetJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	// Update job progress
	job.ProcessedItems = processedItems
	job.FailedItems = failedItems
	job.Status = status
	job.UpdatedAt = time.Now()

	if errorMessage != nil {
		job.ErrorMessage = errorMessage
	}

	// Calculate progress percentage
	if job.TotalItems > 0 {
		job.Progress = int((float64(processedItems+failedItems) / float64(job.TotalItems)) * 100)
	}

	// Update estimated time remaining
	if status == JobStatusProcessing && processedItems > 0 {
		elapsed := time.Since(job.CreatedAt).Seconds()
		avgTimePerItem := elapsed / float64(processedItems)
		remainingItems := job.TotalItems - processedItems - failedItems
		estimatedRemaining := int(avgTimePerItem * float64(remainingItems))
		job.EstimatedTime = &estimatedRemaining
	}

	// Store updates
	if err := jm.storeJobInRedis(ctx, job); err != nil {
		jm.logger.WithError(err).WithField("job_id", jobID).Warn("Failed to update job in Redis")
	}

	if err := jm.updateJobInPostgreSQL(ctx, job); err != nil {
		jm.logger.WithError(err).WithField("job_id", jobID).Warn("Failed to update job in PostgreSQL")
	}

	jm.logger.WithFields(logrus.Fields{
		"job_id":          jobID,
		"status":          status,
		"progress":        job.Progress,
		"processed_items": processedItems,
		"failed_items":    failedItems,
	}).Debug("Job progress updated")

	return nil
}

func (jm *JobManager) CompleteJob(ctx context.Context, jobID uuid.UUID, successCount, failureCount int) error {
	status := JobStatusCompleted
	if failureCount > 0 && successCount == 0 {
		status = JobStatusFailed
	}

	return jm.UpdateJobProgress(ctx, jobID, successCount, failureCount, status, nil)
}

func (jm *JobManager) FailJob(ctx context.Context, jobID uuid.UUID, errorMessage string) error {
	return jm.UpdateJobProgress(ctx, jobID, 0, 0, JobStatusFailed, &errorMessage)
}

func (jm *JobManager) ListActiveJobs(ctx context.Context, limit int) ([]*JobProgress, error) {
	// Get active jobs from Redis
	pattern := "job:*"
	keys, err := jm.db.Redis.Warm.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get job keys: %w", err)
	}

	jobs := []*JobProgress{}
	for i, key := range keys {
		if i >= limit {
			break
		}

		jobData, err := jm.db.Redis.Warm.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var job JobProgress
		if err := json.Unmarshal([]byte(jobData), &job); err != nil {
			continue
		}

		// Only include active jobs
		if job.Status == JobStatusQueued || job.Status == JobStatusProcessing {
			jobs = append(jobs, &job)
		}
	}

	return jobs, nil
}

func (jm *JobManager) CleanupCompletedJobs(ctx context.Context, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)

	// Clean up from Redis
	pattern := "job:*"
	keys, err := jm.db.Redis.Warm.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("failed to get job keys: %w", err)
	}

	cleanedCount := 0
	for _, key := range keys {
		jobData, err := jm.db.Redis.Warm.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var job JobProgress
		if err := json.Unmarshal([]byte(jobData), &job); err != nil {
			continue
		}

		// Remove completed jobs older than cutoff
		if (job.Status == JobStatusCompleted || job.Status == JobStatusFailed) && job.UpdatedAt.Before(cutoff) {
			if err := jm.db.Redis.Warm.Del(ctx, key).Err(); err != nil {
				jm.logger.WithError(err).WithField("job_id", job.JobID).Warn("Failed to delete job from Redis")
			} else {
				cleanedCount++
			}
		}
	}

	jm.logger.WithFields(logrus.Fields{
		"cleaned_count": cleanedCount,
		"cutoff":        cutoff,
	}).Info("Completed job cleanup")

	return nil
}

// Redis operations

func (jm *JobManager) storeJobInRedis(ctx context.Context, job *JobProgress) error {
	key := fmt.Sprintf("job:%s", job.JobID.String())

	jobData, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	// Set with TTL of 24 hours for completed jobs, no TTL for active jobs
	ttl := time.Duration(0)
	if job.Status == JobStatusCompleted || job.Status == JobStatusFailed {
		ttl = 24 * time.Hour
	}

	if err := jm.db.Redis.Warm.Set(ctx, key, jobData, ttl).Err(); err != nil {
		return fmt.Errorf("failed to store job in Redis: %w", err)
	}

	return nil
}

func (jm *JobManager) getJobFromRedis(ctx context.Context, jobID uuid.UUID) (*JobProgress, error) {
	key := fmt.Sprintf("job:%s", jobID.String())

	jobData, err := jm.db.Redis.Warm.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var job JobProgress
	if err := json.Unmarshal([]byte(jobData), &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	return &job, nil
}

// PostgreSQL operations

func (jm *JobManager) storeJobInPostgreSQL(ctx context.Context, job *JobProgress) error {
	query := `
		INSERT INTO content_jobs (
			id, status, progress, total_items, processed_items, failed_items,
			estimated_time, error_message, created_at, updated_at, details
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	detailsJSON, err := json.Marshal(job.Details)
	if err != nil {
		return fmt.Errorf("failed to marshal job details: %w", err)
	}

	_, err = jm.db.PG.Exec(ctx, query,
		job.JobID, job.Status, job.Progress, job.TotalItems, job.ProcessedItems,
		job.FailedItems, job.EstimatedTime, job.ErrorMessage, job.CreatedAt,
		job.UpdatedAt, detailsJSON,
	)

	if err != nil {
		return fmt.Errorf("failed to insert job: %w", err)
	}

	return nil
}

func (jm *JobManager) getJobFromPostgreSQL(ctx context.Context, jobID uuid.UUID) (*JobProgress, error) {
	query := `
		SELECT id, status, progress, total_items, processed_items, failed_items,
			   estimated_time, error_message, created_at, updated_at, details
		FROM content_jobs WHERE id = $1
	`

	var job JobProgress
	var detailsJSON []byte

	err := jm.db.PG.QueryRow(ctx, query, jobID).Scan(
		&job.JobID, &job.Status, &job.Progress, &job.TotalItems, &job.ProcessedItems,
		&job.FailedItems, &job.EstimatedTime, &job.ErrorMessage, &job.CreatedAt,
		&job.UpdatedAt, &detailsJSON,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	if err := json.Unmarshal(detailsJSON, &job.Details); err != nil {
		jm.logger.WithError(err).WithField("job_id", jobID).Warn("Failed to unmarshal job details")
		job.Details = make(map[string]interface{})
	}

	return &job, nil
}

func (jm *JobManager) updateJobInPostgreSQL(ctx context.Context, job *JobProgress) error {
	query := `
		UPDATE content_jobs SET
			status = $2, progress = $3, processed_items = $4, failed_items = $5,
			estimated_time = $6, error_message = $7, updated_at = $8, details = $9
		WHERE id = $1
	`

	detailsJSON, err := json.Marshal(job.Details)
	if err != nil {
		return fmt.Errorf("failed to marshal job details: %w", err)
	}

	_, err = jm.db.PG.Exec(ctx, query,
		job.JobID, job.Status, job.Progress, job.ProcessedItems, job.FailedItems,
		job.EstimatedTime, job.ErrorMessage, job.UpdatedAt, detailsJSON,
	)

	if err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	return nil
}
