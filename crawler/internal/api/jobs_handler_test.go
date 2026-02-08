// crawler/internal/api/jobs_handler_test.go
package api_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/crawler/internal/api"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

// errMockNoData is returned by mock methods that return nil values (not implemented in test).
var errMockNoData = errors.New("mock: no data")

// mockJobRepo implements the job repository interface for testing.
type mockJobRepo struct {
	createOrUpdateFunc func(ctx context.Context, job *domain.Job) (bool, error)
}

func (m *mockJobRepo) Create(ctx context.Context, job *domain.Job) error {
	return nil
}

func (m *mockJobRepo) CreateOrUpdate(ctx context.Context, job *domain.Job) (bool, error) {
	if m.createOrUpdateFunc != nil {
		return m.createOrUpdateFunc(ctx, job)
	}
	return true, nil
}

func (m *mockJobRepo) GetByID(ctx context.Context, id string) (*domain.Job, error) {
	return nil, errMockNoData
}

func (m *mockJobRepo) List(ctx context.Context, params database.ListJobsParams) ([]*domain.Job, error) {
	return nil, errMockNoData
}

func (m *mockJobRepo) Update(ctx context.Context, job *domain.Job) error {
	return nil
}

func (m *mockJobRepo) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockJobRepo) Count(ctx context.Context, params database.CountJobsParams) (int, error) {
	return 0, nil
}

func (m *mockJobRepo) GetJobsReadyToRun(ctx context.Context) ([]*domain.Job, error) {
	return nil, errMockNoData
}

func (m *mockJobRepo) AcquireLock(ctx context.Context, jobID string, token uuid.UUID, now time.Time, duration time.Duration) (bool, error) {
	return false, nil
}

func (m *mockJobRepo) ReleaseLock(ctx context.Context, jobID string) error {
	return nil
}

func (m *mockJobRepo) ClearStaleLocks(ctx context.Context, cutoff time.Time) (int, error) {
	return 0, nil
}

func (m *mockJobRepo) GetScheduledJobs(ctx context.Context) ([]*domain.Job, error) {
	return nil, errMockNoData
}

func (m *mockJobRepo) PauseJob(ctx context.Context, jobID string) error {
	return nil
}

func (m *mockJobRepo) ResumeJob(ctx context.Context, jobID string) error {
	return nil
}

func (m *mockJobRepo) CancelJob(ctx context.Context, jobID string) error {
	return nil
}

func (m *mockJobRepo) CountByStatus(ctx context.Context) (map[string]int, error) {
	return map[string]int{}, nil
}

// mockExecutionRepo implements database.ExecutionRepositoryInterface for testing.
type mockExecutionRepo struct{}

func (m *mockExecutionRepo) Create(ctx context.Context, execution *domain.JobExecution) error {
	return nil
}

func (m *mockExecutionRepo) GetByID(ctx context.Context, id string) (*domain.JobExecution, error) {
	return nil, errMockNoData
}

func (m *mockExecutionRepo) Update(ctx context.Context, execution *domain.JobExecution) error {
	return nil
}

func (m *mockExecutionRepo) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockExecutionRepo) ListByJobID(ctx context.Context, jobID string, limit, offset int) ([]*domain.JobExecution, error) {
	return nil, errMockNoData
}

func (m *mockExecutionRepo) CountByJobID(ctx context.Context, jobID string) (int, error) {
	return 0, nil
}

func (m *mockExecutionRepo) GetLatestByJobID(ctx context.Context, jobID string) (*domain.JobExecution, error) {
	return nil, errMockNoData
}

func (m *mockExecutionRepo) GetJobStats(ctx context.Context, jobID string) (*domain.JobStats, error) {
	return nil, errMockNoData
}

func (m *mockExecutionRepo) GetAggregateStats(ctx context.Context) (*domain.AggregateStats, error) {
	return nil, errMockNoData
}

func (m *mockExecutionRepo) GetTodayStats(ctx context.Context) (crawledToday, indexedToday int64, err error) {
	return 0, 0, nil
}

func (m *mockExecutionRepo) GetFailureRate(ctx context.Context, window time.Duration) (float64, error) {
	return 0, nil
}

func (m *mockExecutionRepo) GetStuckJobs(ctx context.Context, threshold time.Duration) ([]*domain.Job, error) {
	return nil, errMockNoData
}

func (m *mockExecutionRepo) CleanupOldExecutions(ctx context.Context) (int, error) {
	return 0, nil
}

func TestJobsHandler_CreateJob_Insert(t *testing.T) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	jobRepo := &mockJobRepo{
		createOrUpdateFunc: func(ctx context.Context, job *domain.Job) (bool, error) {
			job.ID = "job-123"
			job.CreatedAt = time.Now()
			job.UpdatedAt = time.Now()
			return true, nil // wasInserted
		},
	}

	handler := api.NewJobsHandler(jobRepo, &mockExecutionRepo{})
	router.POST("/api/v1/jobs", handler.CreateJob)

	body := `{"source_id":"80729e12-5127-48f5-9f5c-dcc2647c6fe6","source_name":"Calgary Herald",` +
		`"url":"https://calgaryherald.com","schedule_enabled":false}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestJobsHandler_CreateJob_UpdateExisting(t *testing.T) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	existingJobID := "existing-job-456"
	jobRepo := &mockJobRepo{
		createOrUpdateFunc: func(ctx context.Context, job *domain.Job) (bool, error) {
			job.ID = existingJobID
			job.CreatedAt = time.Now().Add(-24 * time.Hour)
			job.UpdatedAt = time.Now()
			return false, nil // wasInserted = false (update)
		},
	}

	handler := api.NewJobsHandler(jobRepo, &mockExecutionRepo{})
	router.POST("/api/v1/jobs", handler.CreateJob)

	body := `{"source_id":"80729e12-5127-48f5-9f5c-dcc2647c6fe6","source_name":"Calgary Herald",` +
		`"url":"https://calgaryherald.com","schedule_enabled":false}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201 when updating existing job, got %d: %s", w.Code, w.Body.String())
	}
}

func TestJobsHandler_CreateJob_InvalidRequest(t *testing.T) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	handler := api.NewJobsHandler(&mockJobRepo{}, &mockExecutionRepo{})
	router.POST("/api/v1/jobs", handler.CreateJob)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid request, got %d", w.Code)
	}
}
