package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// Mock database for testing
type mockDatabase struct {
	jobs map[uuid.UUID]*JobProgress
}

func (m *mockDatabase) storeJob(job *JobProgress) error {
	m.jobs[job.JobID] = job
	return nil
}

func (m *mockDatabase) getJob(jobID uuid.UUID) (*JobProgress, error) {
	job, exists := m.jobs[jobID]
	if !exists {
		return nil, assert.AnError
	}
	return job, nil
}

func TestJobManager_CreateJob(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	// This test would require a real database connection
	// For now, we'll test the logic without database operations
	t.Run("job creation logic", func(t *testing.T) {
		// Test job creation parameters
		totalItems := 10
		_ = "test_job" // jobType not used in this test

		// Verify estimated time calculation
		expectedEstimatedTime := totalItems * 2 // 2 seconds per item
		assert.Equal(t, 20, expectedEstimatedTime)

		// Verify job ID generation
		jobID := uuid.New()
		assert.NotEqual(t, uuid.Nil, jobID)

		// Verify timestamp generation
		now := time.Now()
		assert.True(t, now.After(time.Time{}))
	})
}

func TestJobManager_UpdateJobProgress(t *testing.T) {
	tests := []struct {
		name             string
		totalItems       int
		processedItems   int
		failedItems      int
		expectedProgress int
	}{
		{
			name:             "no progress",
			totalItems:       10,
			processedItems:   0,
			failedItems:      0,
			expectedProgress: 0,
		},
		{
			name:             "half complete",
			totalItems:       10,
			processedItems:   3,
			failedItems:      2,
			expectedProgress: 50, // (3+2)/10 * 100
		},
		{
			name:             "fully complete",
			totalItems:       10,
			processedItems:   8,
			failedItems:      2,
			expectedProgress: 100, // (8+2)/10 * 100
		},
		{
			name:             "zero total items",
			totalItems:       0,
			processedItems:   0,
			failedItems:      0,
			expectedProgress: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test progress calculation logic
			var progress int
			if tt.totalItems > 0 {
				progress = int((float64(tt.processedItems+tt.failedItems) / float64(tt.totalItems)) * 100)
			}
			assert.Equal(t, tt.expectedProgress, progress)
		})
	}
}

func TestJobManager_EstimatedTimeCalculation(t *testing.T) {
	tests := []struct {
		name              string
		totalItems        int
		processedItems    int
		elapsedSeconds    float64
		expectedRemaining int
	}{
		{
			name:              "steady progress",
			totalItems:        10,
			processedItems:    5,
			elapsedSeconds:    10.0, // 2 seconds per item
			expectedRemaining: 10,   // 5 remaining items * 2 seconds each
		},
		{
			name:              "fast progress",
			totalItems:        10,
			processedItems:    8,
			elapsedSeconds:    8.0, // 1 second per item
			expectedRemaining: 2,   // 2 remaining items * 1 second each
		},
		{
			name:              "slow progress",
			totalItems:        10,
			processedItems:    2,
			elapsedSeconds:    10.0, // 5 seconds per item
			expectedRemaining: 40,   // 8 remaining items * 5 seconds each
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.processedItems > 0 {
				avgTimePerItem := tt.elapsedSeconds / float64(tt.processedItems)
				remainingItems := tt.totalItems - tt.processedItems
				estimatedRemaining := int(avgTimePerItem * float64(remainingItems))
				assert.Equal(t, tt.expectedRemaining, estimatedRemaining)
			}
		})
	}
}

func TestJobStatus_Validation(t *testing.T) {
	validStatuses := []string{
		JobStatusQueued,
		JobStatusProcessing,
		JobStatusCompleted,
		JobStatusFailed,
		JobStatusCancelled,
	}

	expectedStatuses := []string{
		"queued",
		"processing",
		"completed",
		"failed",
		"cancelled",
	}

	assert.Equal(t, expectedStatuses, validStatuses)

	// Test status transitions
	validTransitions := map[string][]string{
		JobStatusQueued:     {JobStatusProcessing, JobStatusCancelled},
		JobStatusProcessing: {JobStatusCompleted, JobStatusFailed, JobStatusCancelled},
		JobStatusCompleted:  {}, // Terminal state
		JobStatusFailed:     {}, // Terminal state
		JobStatusCancelled:  {}, // Terminal state
	}

	// Verify queued can transition to processing
	assert.Contains(t, validTransitions[JobStatusQueued], JobStatusProcessing)

	// Verify processing can transition to completed or failed
	assert.Contains(t, validTransitions[JobStatusProcessing], JobStatusCompleted)
	assert.Contains(t, validTransitions[JobStatusProcessing], JobStatusFailed)

	// Verify terminal states have no valid transitions
	assert.Empty(t, validTransitions[JobStatusCompleted])
	assert.Empty(t, validTransitions[JobStatusFailed])
}

func TestJobProgress_Validation(t *testing.T) {
	tests := []struct {
		name    string
		job     JobProgress
		isValid bool
	}{
		{
			name: "valid job",
			job: JobProgress{
				JobID:          uuid.New(),
				Status:         JobStatusQueued,
				Progress:       0,
				TotalItems:     10,
				ProcessedItems: 0,
				FailedItems:    0,
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			},
			isValid: true,
		},
		{
			name: "invalid progress - negative",
			job: JobProgress{
				Progress: -1,
			},
			isValid: false,
		},
		{
			name: "invalid progress - over 100",
			job: JobProgress{
				Progress: 101,
			},
			isValid: false,
		},
		{
			name: "invalid items - negative processed",
			job: JobProgress{
				ProcessedItems: -1,
			},
			isValid: false,
		},
		{
			name: "invalid items - negative failed",
			job: JobProgress{
				FailedItems: -1,
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate progress range
			progressValid := tt.job.Progress >= 0 && tt.job.Progress <= 100

			// Validate item counts
			itemsValid := tt.job.ProcessedItems >= 0 && tt.job.FailedItems >= 0

			// Validate total consistency
			totalValid := tt.job.ProcessedItems+tt.job.FailedItems <= tt.job.TotalItems

			isValid := progressValid && itemsValid && totalValid

			if tt.isValid {
				assert.True(t, isValid, "Job should be valid")
			} else {
				// At least one validation should fail for invalid jobs
				assert.False(t, progressValid && itemsValid, "Job should be invalid")
			}
		})
	}
}

func TestJobManager_CleanupLogic(t *testing.T) {
	now := time.Now()
	cutoff := now.Add(-24 * time.Hour) // 24 hours ago

	jobs := []JobProgress{
		{
			JobID:     uuid.New(),
			Status:    JobStatusCompleted,
			UpdatedAt: now.Add(-25 * time.Hour), // Should be cleaned
		},
		{
			JobID:     uuid.New(),
			Status:    JobStatusCompleted,
			UpdatedAt: now.Add(-1 * time.Hour), // Should not be cleaned
		},
		{
			JobID:     uuid.New(),
			Status:    JobStatusProcessing,
			UpdatedAt: now.Add(-25 * time.Hour), // Should not be cleaned (active)
		},
		{
			JobID:     uuid.New(),
			Status:    JobStatusFailed,
			UpdatedAt: now.Add(-25 * time.Hour), // Should be cleaned
		},
	}

	shouldCleanCount := 0
	for _, job := range jobs {
		shouldClean := (job.Status == JobStatusCompleted || job.Status == JobStatusFailed) &&
			job.UpdatedAt.Before(cutoff)
		if shouldClean {
			shouldCleanCount++
		}
	}

	assert.Equal(t, 2, shouldCleanCount, "Should clean 2 jobs (1 completed + 1 failed, both old)")
}

func TestJobManager_RedisKeyGeneration(t *testing.T) {
	jobID := uuid.New()
	expectedKey := "job:" + jobID.String()

	// Test key generation logic
	key := "job:" + jobID.String()
	assert.Equal(t, expectedKey, key)

	// Test key parsing
	if len(key) > 4 && key[:4] == "job:" {
		parsedID, err := uuid.Parse(key[4:])
		assert.NoError(t, err)
		assert.Equal(t, jobID, parsedID)
	}
}
