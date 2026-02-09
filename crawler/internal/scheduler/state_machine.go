package scheduler

import (
	"fmt"

	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

// JobState represents a job state in the state machine.
type JobState string

const (
	StatePending   JobState = "pending"
	StateScheduled JobState = "scheduled"
	StateRunning   JobState = "running"
	StatePaused    JobState = "paused"
	StateCompleted JobState = "completed"
	StateFailed    JobState = "failed"
	StateCancelled JobState = "cancelled"
)

// ValidateStateTransition checks if a state transition is valid.
// Returns an error if the transition is not allowed.
func ValidateStateTransition(from, to JobState) error {
	validTransitions := map[JobState][]JobState{
		StatePending: {
			StateScheduled, // When interval is set and enabled
			StateRunning,   // Immediate execution
			StateCancelled, // Manual cancellation
		},
		StateScheduled: {
			StateRunning,   // When next_run_at reached
			StatePending,   // Force-run: queued for immediate execution
			StatePaused,    // Manual pause
			StateCancelled, // Manual cancellation
		},
		StatePaused: {
			StateScheduled, // Manual resume
			StatePending,   // Force-run: queued for immediate execution
			StateCancelled, // Manual cancellation
		},
		StateRunning: {
			StateCompleted, // Successful execution
			StateFailed,    // Execution error, no retries left
			StateScheduled, // Execution error, retry scheduled with backoff
			StateCancelled, // Manual cancellation during execution
		},
		StateCompleted: {
			StateScheduled, // Recurring job auto-reschedules
		},
		StateFailed: {
			StatePending, // Manual retry (resets to pending)
		},
		// Terminal states (no transitions from cancelled)
		StateCancelled: {},
	}

	allowedStates, exists := validTransitions[from]
	if !exists {
		return fmt.Errorf("unknown source state: %s", from)
	}

	for _, allowed := range allowedStates {
		if allowed == to {
			return nil
		}
	}

	return fmt.Errorf("invalid state transition from %s to %s", from, to)
}

// CanPause checks if a job can be paused in its current state.
func CanPause(job *domain.Job) bool {
	return job.Status == string(StateScheduled)
}

// CanResume checks if a job can be resumed from its current state.
func CanResume(job *domain.Job) bool {
	return job.Status == string(StatePaused)
}

// CanCancel checks if a job can be cancelled in its current state.
func CanCancel(job *domain.Job) bool {
	state := JobState(job.Status)
	return state == StateScheduled ||
		state == StateRunning ||
		state == StatePaused ||
		state == StatePending
}

// CanRetry checks if a job can be retried (only failed jobs).
func CanRetry(job *domain.Job) bool {
	return job.Status == string(StateFailed)
}

// IsTerminalState checks if a state is terminal (no further transitions).
func IsTerminalState(state JobState) bool {
	return state == StateCancelled || state == StateCompleted || state == StateFailed
}

// IsActiveState checks if a job is actively running.
func IsActiveState(state JobState) bool {
	return state == StateRunning
}

// IsSchedulableState checks if a job can be scheduled for execution.
func IsSchedulableState(state JobState) bool {
	return state == StatePending || state == StateScheduled
}
