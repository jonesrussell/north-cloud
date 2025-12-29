package scheduler

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

func TestValidateStateTransition(t *testing.T) {
	tests := []struct {
		name    string
		from    JobState
		to      JobState
		wantErr bool
	}{
		// Valid transitions from pending
		{"pending to scheduled", StatePending, StateScheduled, false},
		{"pending to running", StatePending, StateRunning, false},
		{"pending to cancelled", StatePending, StateCancelled, false},

		// Invalid transitions from pending
		{"pending to completed", StatePending, StateCompleted, true},
		{"pending to failed", StatePending, StateFailed, true},
		{"pending to paused", StatePending, StatePaused, true},

		// Valid transitions from scheduled
		{"scheduled to running", StateScheduled, StateRunning, false},
		{"scheduled to paused", StateScheduled, StatePaused, false},
		{"scheduled to cancelled", StateScheduled, StateCancelled, false},

		// Invalid transitions from scheduled
		{"scheduled to pending", StateScheduled, StatePending, true},
		{"scheduled to completed", StateScheduled, StateCompleted, true},
		{"scheduled to failed", StateScheduled, StateFailed, true},

		// Valid transitions from paused
		{"paused to scheduled", StatePaused, StateScheduled, false},
		{"paused to cancelled", StatePaused, StateCancelled, false},

		// Invalid transitions from paused
		{"paused to running", StatePaused, StateRunning, true},
		{"paused to completed", StatePaused, StateCompleted, true},
		{"paused to failed", StatePaused, StateFailed, true},

		// Valid transitions from running
		{"running to completed", StateRunning, StateCompleted, false},
		{"running to failed", StateRunning, StateFailed, false},
		{"running to scheduled", StateRunning, StateScheduled, false}, // retry with backoff
		{"running to cancelled", StateRunning, StateCancelled, false},

		// Invalid transitions from running
		{"running to pending", StateRunning, StatePending, true},
		{"running to paused", StateRunning, StatePaused, true},

		// Valid transitions from completed
		{"completed to scheduled", StateCompleted, StateScheduled, false}, // recurring job

		// Invalid transitions from completed
		{"completed to running", StateCompleted, StateRunning, true},
		{"completed to failed", StateCompleted, StateFailed, true},
		{"completed to cancelled", StateCompleted, StateCancelled, true},

		// Valid transitions from failed
		{"failed to pending", StateFailed, StatePending, false}, // manual retry

		// Invalid transitions from failed
		{"failed to running", StateFailed, StateRunning, true},
		{"failed to completed", StateFailed, StateCompleted, true},
		{"failed to cancelled", StateFailed, StateCancelled, true},

		// Terminal state: cancelled (no valid transitions)
		{"cancelled to pending", StateCancelled, StatePending, true},
		{"cancelled to scheduled", StateCancelled, StateScheduled, true},
		{"cancelled to running", StateCancelled, StateRunning, true},
		{"cancelled to completed", StateCancelled, StateCompleted, true},
		{"cancelled to failed", StateCancelled, StateFailed, true},
		{"cancelled to paused", StateCancelled, StatePaused, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStateTransition(tt.from, tt.to)
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
		{"scheduled job can be paused", string(StateScheduled), true},
		{"pending job cannot be paused", string(StatePending), false},
		{"running job cannot be paused", string(StateRunning), false},
		{"paused job cannot be paused again", string(StatePaused), false},
		{"completed job cannot be paused", string(StateCompleted), false},
		{"failed job cannot be paused", string(StateFailed), false},
		{"cancelled job cannot be paused", string(StateCancelled), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &domain.Job{Status: tt.status}
			if got := CanPause(job); got != tt.want {
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
		{"paused job can be resumed", string(StatePaused), true},
		{"pending job cannot be resumed", string(StatePending), false},
		{"scheduled job cannot be resumed", string(StateScheduled), false},
		{"running job cannot be resumed", string(StateRunning), false},
		{"completed job cannot be resumed", string(StateCompleted), false},
		{"failed job cannot be resumed", string(StateFailed), false},
		{"cancelled job cannot be resumed", string(StateCancelled), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &domain.Job{Status: tt.status}
			if got := CanResume(job); got != tt.want {
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
		{"scheduled job can be cancelled", string(StateScheduled), true},
		{"running job can be cancelled", string(StateRunning), true},
		{"paused job can be cancelled", string(StatePaused), true},
		{"pending job can be cancelled", string(StatePending), true},
		{"completed job cannot be cancelled", string(StateCompleted), false},
		{"failed job cannot be cancelled", string(StateFailed), false},
		{"cancelled job cannot be cancelled again", string(StateCancelled), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &domain.Job{Status: tt.status}
			if got := CanCancel(job); got != tt.want {
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
		{"failed job can be retried", string(StateFailed), true},
		{"pending job cannot be retried", string(StatePending), false},
		{"scheduled job cannot be retried", string(StateScheduled), false},
		{"running job cannot be retried", string(StateRunning), false},
		{"paused job cannot be retried", string(StatePaused), false},
		{"completed job cannot be retried", string(StateCompleted), false},
		{"cancelled job cannot be retried", string(StateCancelled), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &domain.Job{Status: tt.status}
			if got := CanRetry(job); got != tt.want {
				t.Errorf("CanRetry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsTerminalState(t *testing.T) {
	tests := []struct {
		name  string
		state JobState
		want  bool
	}{
		{"cancelled is terminal", StateCancelled, true},
		{"completed is terminal", StateCompleted, true},
		{"failed is terminal", StateFailed, true},
		{"pending is not terminal", StatePending, false},
		{"scheduled is not terminal", StateScheduled, false},
		{"running is not terminal", StateRunning, false},
		{"paused is not terminal", StatePaused, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTerminalState(tt.state); got != tt.want {
				t.Errorf("IsTerminalState() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsActiveState(t *testing.T) {
	tests := []struct {
		name  string
		state JobState
		want  bool
	}{
		{"running is active", StateRunning, true},
		{"pending is not active", StatePending, false},
		{"scheduled is not active", StateScheduled, false},
		{"paused is not active", StatePaused, false},
		{"completed is not active", StateCompleted, false},
		{"failed is not active", StateFailed, false},
		{"cancelled is not active", StateCancelled, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsActiveState(tt.state); got != tt.want {
				t.Errorf("IsActiveState() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSchedulableState(t *testing.T) {
	tests := []struct {
		name  string
		state JobState
		want  bool
	}{
		{"pending is schedulable", StatePending, true},
		{"scheduled is schedulable", StateScheduled, true},
		{"running is not schedulable", StateRunning, false},
		{"paused is not schedulable", StatePaused, false},
		{"completed is not schedulable", StateCompleted, false},
		{"failed is not schedulable", StateFailed, false},
		{"cancelled is not schedulable", StateCancelled, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSchedulableState(tt.state); got != tt.want {
				t.Errorf("IsSchedulableState() = %v, want %v", got, tt.want)
			}
		})
	}
}
