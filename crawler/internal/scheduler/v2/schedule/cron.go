// Package schedule provides scheduling strategies for the V2 scheduler.
package schedule

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/reugn/go-quartz/quartz"
)

var (
	// ErrInvalidCronExpression is returned when a cron expression is invalid.
	ErrInvalidCronExpression = errors.New("invalid cron expression")

	// ErrSchedulerNotRunning is returned when the scheduler is not running.
	ErrSchedulerNotRunning = errors.New("cron scheduler not running")

	// ErrSchedulerInitFailed is returned when the scheduler fails to initialize.
	ErrSchedulerInitFailed = errors.New("cron scheduler initialization failed")
)

// CronJobFunc is a function that processes a scheduled job.
type CronJobFunc func(ctx context.Context, jobID string) error

// cronJob wraps a job function to implement quartz.Job.
type cronJob struct {
	jobID   string
	handler CronJobFunc
	ctx     context.Context
}

func (j *cronJob) Execute(ctx context.Context) error {
	return j.handler(ctx, j.jobID)
}

func (j *cronJob) Description() string {
	return fmt.Sprintf("CronJob[%s]", j.jobID)
}

// CronScheduler manages cron-scheduled jobs using go-quartz.
type CronScheduler struct {
	scheduler quartz.Scheduler
	mu        sync.RWMutex
	jobKeys   map[string]*quartz.JobKey // jobID -> JobKey mapping
	running   bool
}

// NewCronScheduler creates a new cron scheduler.
func NewCronScheduler() *CronScheduler {
	scheduler, err := quartz.NewStdScheduler()
	if err != nil {
		// Return a scheduler that will fail on Start
		return &CronScheduler{
			scheduler: nil,
			jobKeys:   make(map[string]*quartz.JobKey),
		}
	}

	return &CronScheduler{
		scheduler: scheduler,
		jobKeys:   make(map[string]*quartz.JobKey),
	}
}

// Start starts the cron scheduler.
func (s *CronScheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.scheduler == nil {
		return ErrSchedulerInitFailed
	}

	if s.running {
		return nil
	}

	s.scheduler.Start(ctx)
	s.running = true
	return nil
}

// Stop stops the cron scheduler.
func (s *CronScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running || s.scheduler == nil {
		return
	}

	s.scheduler.Stop()
	s.running = false
}

// ScheduleJob schedules a job with a cron expression.
func (s *CronScheduler) ScheduleJob(
	ctx context.Context, jobID string, cronExpr string, handler CronJobFunc,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return ErrSchedulerNotRunning
	}

	// Parse cron expression
	trigger, err := quartz.NewCronTrigger(cronExpr)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidCronExpression, err.Error())
	}

	// Create job
	job := &cronJob{
		jobID:   jobID,
		handler: handler,
		ctx:     ctx,
	}

	// Create job key
	jobKey := quartz.NewJobKey(jobID)

	// Schedule the job
	if scheduleErr := s.scheduler.ScheduleJob(quartz.NewJobDetail(job, jobKey), trigger); scheduleErr != nil {
		return fmt.Errorf("failed to schedule job: %w", scheduleErr)
	}

	s.jobKeys[jobID] = jobKey
	return nil
}

// UnscheduleJob removes a scheduled job.
func (s *CronScheduler) UnscheduleJob(jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	jobKey, exists := s.jobKeys[jobID]
	if !exists {
		return fmt.Errorf("job %s not found", jobID)
	}

	if deleteErr := s.scheduler.DeleteJob(jobKey); deleteErr != nil {
		return fmt.Errorf("failed to unschedule job: %w", deleteErr)
	}

	delete(s.jobKeys, jobID)
	return nil
}

// RescheduleJob updates a job's schedule.
func (s *CronScheduler) RescheduleJob(
	ctx context.Context, jobID string, cronExpr string, handler CronJobFunc,
) error {
	// Unschedule existing job (ignore error if not found)
	_ = s.UnscheduleJob(jobID)

	// Schedule with new expression
	return s.ScheduleJob(ctx, jobID, cronExpr, handler)
}

// IsScheduled returns true if a job is scheduled.
func (s *CronScheduler) IsScheduled(jobID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.jobKeys[jobID]
	return exists
}

// GetNextFireTime returns the next fire time for a job.
func (s *CronScheduler) GetNextFireTime(jobID string) (time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobKey, exists := s.jobKeys[jobID]
	if !exists {
		return time.Time{}, fmt.Errorf("job %s not found", jobID)
	}

	scheduledJob, getErr := s.scheduler.GetScheduledJob(jobKey)
	if getErr != nil {
		return time.Time{}, fmt.Errorf("failed to get scheduled job: %w", getErr)
	}

	return time.Unix(0, scheduledJob.NextRunTime()), nil
}

// GetScheduledJobIDs returns all scheduled job IDs.
func (s *CronScheduler) GetScheduledJobIDs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]string, 0, len(s.jobKeys))
	for id := range s.jobKeys {
		ids = append(ids, id)
	}
	return ids
}

// ScheduledJobCount returns the number of scheduled jobs.
func (s *CronScheduler) ScheduledJobCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.jobKeys)
}

// IsRunning returns true if the scheduler is running.
func (s *CronScheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// ValidateCronExpression validates a cron expression without scheduling.
func ValidateCronExpression(cronExpr string) error {
	_, err := quartz.NewCronTrigger(cronExpr)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidCronExpression, err.Error())
	}
	return nil
}

// GetNextRunTimes returns the next N run times for a cron expression.
// If fewer than count times can be computed, it returns what it could compute.
func GetNextRunTimes(cronExpr string, count int) ([]time.Time, error) {
	trigger, err := quartz.NewCronTrigger(cronExpr)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidCronExpression, err.Error())
	}

	times := make([]time.Time, 0, count)
	current := time.Now()

	var lastErr error
	for range count {
		nextFire, triggerErr := trigger.NextFireTime(current.UnixNano())
		if triggerErr != nil {
			lastErr = triggerErr
			break
		}
		nextTime := time.Unix(0, nextFire)
		times = append(times, nextTime)
		current = nextTime.Add(time.Second)
	}

	// Return partial results even if we encountered an error
	// Only return the error if we got no results at all
	if len(times) == 0 && lastErr != nil {
		return nil, lastErr
	}

	return times, nil
}
