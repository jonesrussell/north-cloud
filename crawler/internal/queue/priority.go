package queue

import (
	"errors"
)

// Priority represents job priority level.
type Priority int

const (
	// PriorityHigh is for urgent jobs that should be processed first.
	PriorityHigh Priority = 1

	// PriorityNormal is for standard jobs (default).
	PriorityNormal Priority = 2

	// PriorityLow is for background jobs that can wait.
	PriorityLow Priority = 3

	// priorityValueHigh is the integer value for high priority.
	priorityValueHigh = 1
	// priorityValueNormal is the integer value for normal priority.
	priorityValueNormal = 2
	// priorityValueLow is the integer value for low priority.
	priorityValueLow = 3

	// priorityStrNormal is the string representation of normal priority.
	priorityStrNormal = "normal"
)

// String returns the string representation of a priority.
func (p Priority) String() string {
	switch p {
	case PriorityHigh:
		return "high"
	case PriorityNormal:
		return priorityStrNormal
	case PriorityLow:
		return "low"
	default:
		return priorityStrNormal
	}
}

// ParsePriority converts a string or int to a Priority.
func ParsePriority(value any) (Priority, error) {
	switch v := value.(type) {
	case int:
		return parsePriorityInt(v)
	case int64:
		return parsePriorityInt(int(v))
	case string:
		return parsePriorityString(v)
	case Priority:
		return v, nil
	default:
		return PriorityNormal, errors.New("invalid priority type")
	}
}

func parsePriorityInt(v int) (Priority, error) {
	switch v {
	case priorityValueHigh:
		return PriorityHigh, nil
	case priorityValueNormal:
		return PriorityNormal, nil
	case priorityValueLow:
		return PriorityLow, nil
	default:
		return PriorityNormal, errors.New("invalid priority value: must be 1, 2, or 3")
	}
}

func parsePriorityString(v string) (Priority, error) {
	switch v {
	case "high", "1":
		return PriorityHigh, nil
	case priorityStrNormal, "2", "":
		return PriorityNormal, nil
	case "low", "3":
		return PriorityLow, nil
	default:
		return PriorityNormal, errors.New("invalid priority string: must be high, normal, or low")
	}
}

// AllPriorities returns all priority levels in order of precedence (high first).
func AllPriorities() []Priority {
	return []Priority{PriorityHigh, PriorityNormal, PriorityLow}
}

// Weight returns a numeric weight for the priority (lower = more important).
func (p Priority) Weight() int {
	return int(p)
}

// IsValid returns true if the priority is a valid value.
func (p Priority) IsValid() bool {
	return p >= PriorityHigh && p <= PriorityLow
}
