package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// CrawlerClient is a client for the crawler API
type CrawlerClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewCrawlerClient creates a new crawler client
func NewCrawlerClient(baseURL string) *CrawlerClient {
	return &CrawlerClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}
}

// Job represents a crawl job
type Job struct {
	ID              string    `json:"id"`
	SourceID        string    `json:"source_id"`
	URL             string    `json:"url"`
	Status          string    `json:"status"`
	ScheduleEnabled bool      `json:"schedule_enabled"`
	IntervalMinutes int       `json:"interval_minutes,omitempty"`
	IntervalType    string    `json:"interval_type,omitempty"`
	NextRunAt       time.Time `json:"next_run_at,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// JobExecution represents a job execution record
type JobExecution struct {
	ID              string    `json:"id"`
	JobID           string    `json:"job_id"`
	Status          string    `json:"status"`
	StartedAt       time.Time `json:"started_at"`
	CompletedAt     time.Time `json:"completed_at,omitempty"`
	DurationSeconds int       `json:"duration_seconds"`
	ItemsCrawled    int       `json:"items_crawled"`
	ItemsIndexed    int       `json:"items_indexed"`
	ErrorMessage    string    `json:"error_message,omitempty"`
}

// JobStats represents job statistics
type JobStats struct {
	TotalExecutions int     `json:"total_executions"`
	SuccessCount    int     `json:"success_count"`
	FailureCount    int     `json:"failure_count"`
	AvgDuration     float64 `json:"avg_duration"`
	SuccessRate     float64 `json:"success_rate"`
}

// SchedulerMetrics represents scheduler metrics
type SchedulerMetrics struct {
	TotalJobs       int       `json:"total_jobs"`
	PendingJobs     int       `json:"pending_jobs"`
	ScheduledJobs   int       `json:"scheduled_jobs"`
	RunningJobs     int       `json:"running_jobs"`
	CompletedJobs   int       `json:"completed_jobs"`
	FailedJobs      int       `json:"failed_jobs"`
	PausedJobs      int       `json:"paused_jobs"`
	CancelledJobs   int       `json:"cancelled_jobs"`
	LastUpdated     time.Time `json:"last_updated"`
	AvgDuration     float64   `json:"avg_duration"`
	TotalExecutions int       `json:"total_executions"`
}

// CreateJobRequest represents a request to create a job
type CreateJobRequest struct {
	SourceID        string `json:"source_id"`
	URL             string `json:"url"`
	ScheduleEnabled bool   `json:"schedule_enabled"`
	IntervalMinutes int    `json:"interval_minutes,omitempty"`
	IntervalType    string `json:"interval_type,omitempty"`
}

// CreateJob creates a new crawl job
func (c *CrawlerClient) CreateJob(req CreateJobRequest) (*Job, error) {
	endpoint := fmt.Sprintf("%s/api/v1/jobs", c.baseURL)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error string `json:"error"`
		}
		if jsonErr := json.Unmarshal(respBody, &errorResp); jsonErr == nil && errorResp.Error != "" {
			return nil, fmt.Errorf("crawler error: %s", errorResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var job Job
	if err = json.Unmarshal(respBody, &job); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &job, nil
}

// ListJobs lists all crawl jobs with optional status filter
func (c *CrawlerClient) ListJobs(status string) ([]Job, error) {
	endpoint := fmt.Sprintf("%s/api/v1/jobs", c.baseURL)

	if status != "" {
		params := url.Values{}
		params.Add("status", status)
		endpoint = fmt.Sprintf("%s?%s", endpoint, params.Encode())
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var jobs []Job
	if err = json.Unmarshal(body, &jobs); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return jobs, nil
}

// PauseJob pauses a crawl job
func (c *CrawlerClient) PauseJob(jobID string) (*Job, error) {
	endpoint := fmt.Sprintf("%s/api/v1/jobs/%s/pause", c.baseURL, jobID)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error string `json:"error"`
		}
		if jsonErr := json.Unmarshal(body, &errorResp); jsonErr == nil && errorResp.Error != "" {
			return nil, fmt.Errorf("crawler error: %s", errorResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var job Job
	if err = json.Unmarshal(body, &job); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &job, nil
}

// ResumeJob resumes a paused job
func (c *CrawlerClient) ResumeJob(jobID string) (*Job, error) {
	endpoint := fmt.Sprintf("%s/api/v1/jobs/%s/resume", c.baseURL, jobID)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error string `json:"error"`
		}
		if jsonErr := json.Unmarshal(body, &errorResp); jsonErr == nil && errorResp.Error != "" {
			return nil, fmt.Errorf("crawler error: %s", errorResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var job Job
	if err = json.Unmarshal(body, &job); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &job, nil
}

// CancelJob cancels a job
func (c *CrawlerClient) CancelJob(jobID string) (*Job, error) {
	endpoint := fmt.Sprintf("%s/api/v1/jobs/%s/cancel", c.baseURL, jobID)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error string `json:"error"`
		}
		if jsonErr := json.Unmarshal(body, &errorResp); jsonErr == nil && errorResp.Error != "" {
			return nil, fmt.Errorf("crawler error: %s", errorResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var job Job
	if err = json.Unmarshal(body, &job); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &job, nil
}

// GetJobStats gets statistics for a job
func (c *CrawlerClient) GetJobStats(jobID string) (*JobStats, error) {
	endpoint := fmt.Sprintf("%s/api/v1/jobs/%s/stats", c.baseURL, jobID)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var stats JobStats
	if err = json.Unmarshal(body, &stats); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &stats, nil
}

// GetSchedulerMetrics gets scheduler-wide metrics
func (c *CrawlerClient) GetSchedulerMetrics() (*SchedulerMetrics, error) {
	endpoint := fmt.Sprintf("%s/api/v1/scheduler/metrics", c.baseURL)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var metrics SchedulerMetrics
	if err = json.Unmarshal(body, &metrics); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &metrics, nil
}

// ListJobExecutions lists executions for a job
func (c *CrawlerClient) ListJobExecutions(jobID string) ([]JobExecution, error) {
	endpoint := fmt.Sprintf("%s/api/v1/jobs/%s/executions", c.baseURL, jobID)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var executions []JobExecution
	if err = json.Unmarshal(body, &executions); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return executions, nil
}
