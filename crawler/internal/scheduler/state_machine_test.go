package scheduler_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/crawler/internal/scheduler"
)

func TestValidateStateTransition(t *testing.T) {
	tests := []struct {
		name    string
		from    scheduler.JobState
		to      scheduler.JobState
		wantErr bool
	}{
		// Valid transitions from pending
		{"pending to scheduled", scheduler.StatePending, scheduler.StateScheduled, false},
		{"pending to running", scheduler.StatePending, scheduler.StateRunning, false},
		{"pending to cancelled", scheduler.StatePending, scheduler.StateCancelled, false},

		// Invalid transitions from pending
		{"pending to completed", scheduler.StatePending, scheduler.StateCompleted, true},
		{"pending to failed", scheduler.StatePending, scheduler.StateFailed, true},
		{"pending to paused", scheduler.StatePending, scheduler.StatePaused, true},

		// Valid transitions from scheduled
		{"scheduled to running", scheduler.StateScheduled, scheduler.StateRunning, false},
		{"scheduled to paused", scheduler.StateScheduled, scheduler.StatePaused, false},
		{"scheduled to cancelled", scheduler.StateScheduled, scheduler.StateCancelled, false},

		// Invalid transitions from scheduled
		{"scheduled to pending", scheduler.StateScheduled, scheduler.StatePending, true},
		{"scheduled to completed", scheduler.StateScheduled, scheduler.StateCompleted, true},
		{"scheduled to failed", scheduler.StateScheduled, scheduler.StateFailed, true},

		// Valid transitions from paused
		{"paused to scheduled", scheduler.StatePaused, scheduler.StateScheduled, false},
		{"paused to cancelled", scheduler.StatePaused, scheduler.StateCancelled, false},

		// Invalid transitions from paused
		{"paused to running", scheduler.StatePaused, scheduler.StateRunning, true},
		{"paused to completed", scheduler.StatePaused, scheduler.StateCompleted, true},
		{"paused to failed", scheduler.StatePaused, scheduler.StateFailed, true},

		// Valid transitions from running
		{"running to completed", scheduler.StateRunning, scheduler.StateCompleted, false},
		{"running to failed", scheduler.StateRunning, scheduler.StateFailed, false},
		{"running to scheduled", scheduler.StateRunning, scheduler.StateScheduled, false}, // retry with backoff
		{"running to cancelled", scheduler.StateRunning, scheduler.StateCancelled, false},

		// Invalid transitions from running
		{"running to pending", scheduler.StateRunning, scheduler.StatePending, true},
		{"running to paused", scheduler.StateRunning, scheduler.StatePaused, true},

		// Valid transitions from completed
		{"completed to scheduled", scheduler.StateCompleted, scheduler.StateScheduled, false}, // recurring job

		// Invalid transitions from completed
		{"completed to running", scheduler.StateCompleted, scheduler.StateRunning, true},
		{"completed to failed", scheduler.StateCompleted, scheduler.StateFailed, true},
		{"completed to cancelled", scheduler.StateCompleted, scheduler.StateCancelled, true},

		// Valid transitions from failed
		{"failed to pending", scheduler.StateFailed, scheduler.StatePending, false}, // manual retry

		// Invalid transitions from failed
		{"failed to running", scheduler.StateFailed, scheduler.StateRunning, true},
		{"failed to completed", scheduler.StateFailed, scheduler.StateCompleted, true},
		{"failed to cancelled", scheduler.StateFailed, scheduler.StateCancelled, true},

		// Terminal state: cancelled (no valid transitions)
		{"cancelled to pending", scheduler.StateCancelled, scheduler.StatePending, true},
		{"cancelled to scheduled", scheduler.StateCancelled, scheduler.StateScheduled, true},
		{"cancelled to running", scheduler.StateCancelled, scheduler.StateRunning, true},
		{"cancelled to completed", scheduler.StateCancelled, scheduler.StateCompleted, true},
		{"cancelled to failed", scheduler.StateCancelled, scheduler.StateFailed, true},
		{"cancelled to paused", scheduler.StateCancelled, scheduler.StatePaused, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := scheduler.ValidateStateTransition(tt.from, tt.to)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateStateTransition(%v, %v) error = %v, wantErr %v", tt.from, tt.to, err, tt.wantErr)
			}
		})
	}
}

func TestCanPause(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"scheduled job can be paused", string(scheduler.StateScheduled), true},
		{"pending job cannot be paused", string(scheduler.StatePending), false},
		{"running job cannot be paused", string(scheduler.StateRunning), false},
		{"paused job cannot be paused again", string(scheduler.StatePaused), false},
		{"completed job cannot be paused", string(scheduler.StateCompleted), false},
		{"failed job cannot be paused", string(scheduler.StateFailed), false},
		{"cancelled job cannot be paused", string(scheduler.StateCancelled), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &domain.Job{Status: tt.status}
			if got := scheduler.CanPause(job); got != tt.want {
				t.Errorf("CanPause() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCanResume(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"paused job can be resumed", string(scheduler.StatePaused), true},
		{"pending job cannot be resumed", string(scheduler.StatePending), false},
		{"scheduled job cannot be resumed", string(scheduler.StateScheduled), false},
		{"running job cannot be resumed", string(scheduler.StateRunning), false},
		{"completed job cannot be resumed", string(scheduler.StateCompleted), false},
		{"failed job cannot be resumed", string(scheduler.StateFailed), false},
		{"cancelled job cannot be resumed", string(scheduler.StateCancelled), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &domain.Job{Status: tt.status}
			if got := scheduler.CanResume(job); got != tt.want {
				t.Errorf("CanResume() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCanCancel(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"scheduled job can be cancelled", string(scheduler.StateScheduled), true},
		{"running job can be cancelled", string(scheduler.StateRunning), true},
		{"paused job can be cancelled", string(scheduler.StatePaused), true},
		{"pending job can be cancelled", string(scheduler.StatePending), true},
		{"completed job cannot be cancelled", string(scheduler.StateCompleted), false},
		{"failed job cannot be cancelled", string(scheduler.StateFailed), false},
		{"cancelled job cannot be cancelled again", string(scheduler.StateCancelled), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &domain.Job{Status: tt.status}
			if got := scheduler.CanCancel(job); got != tt.want {
				t.Errorf("CanCancel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCanRetry(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"failed job can be retried", string(scheduler.StateFailed), true},
		{"pending job cannot be retried", string(scheduler.StatePending), false},
		{"scheduled job cannot be retried", string(scheduler.StateScheduled), false},
		{"running job cannot be retried", string(scheduler.StateRunning), false},
		{"paused job cannot be retried", string(scheduler.StatePaused), false},
		{"completed job cannot be retried", string(scheduler.StateCompleted), false},
		{"cancelled job cannot be retried", string(scheduler.StateCancelled), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &domain.Job{Status: tt.status}
			if got := scheduler.CanRetry(job); got != tt.want {
				t.Errorf("CanRetry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsTerminalState(t *testing.T) {
	tests := []struct {
		name  string
		state scheduler.JobState
		want  bool
	}{
		{"cancelled is terminal", scheduler.StateCancelled, true},
		{"completed is terminal", scheduler.StateCompleted, true},
		{"failed is terminal", scheduler.StateFailed, true},
		{"pending is not terminal", scheduler.StatePending, false},
		{"scheduled is not terminal", scheduler.StateScheduled, false},
		{"running is not terminal", scheduler.StateRunning, false},
		{"paused is not terminal", scheduler.StatePaused, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scheduler.IsTerminalState(tt.state); got != tt.want {
				t.Errorf("IsTerminalState() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsActiveState(t *testing.T) {
	tests := []struct {
		name  string
		state scheduler.JobState
		want  bool
	}{
		{"running is active", scheduler.StateRunning, true},
		{"pending is not active", scheduler.StatePending, false},
		{"scheduled is not active", scheduler.StateScheduled, false},
		{"paused is not active", scheduler.StatePaused, false},
		{"completed is not active", scheduler.StateCompleted, false},
		{"failed is not active", scheduler.StateFailed, false},
		{"cancelled is not active", scheduler.StateCancelled, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scheduler.IsActiveState(tt.state); got != tt.want {
				t.Errorf("IsActiveState() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSchedulableState(t *testing.T) {
	tests := []struct {
		name  string
		state scheduler.JobState
		want  bool
	}{
		{"pending is schedulable", scheduler.StatePending, true},
		{"scheduled is schedulable", scheduler.StateScheduled, true},
		{"running is not schedulable", scheduler.StateRunning, false},
		{"paused is not schedulable", scheduler.StatePaused, false},
		{"completed is not schedulable", scheduler.StateCompleted, false},
		{"failed is not schedulable", scheduler.StateFailed, false},
		{"cancelled is not schedulable", scheduler.StateCancelled, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scheduler.IsSchedulableState(tt.state); got != tt.want {
				t.Errorf("IsSchedulableState() = %v, want %v", got, tt.want)
			}
		})
	}
}
