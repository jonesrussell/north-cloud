package schedule

import (
	"errors"
	"time"
)

const (
	// IntervalTypeMinutes schedules in minute intervals.
	IntervalTypeMinutes = "minutes"

	// IntervalTypeHours schedules in hour intervals.
	IntervalTypeHours = "hours"

	// IntervalTypeDays schedules in day intervals.
	IntervalTypeDays = "days"
)

var (
	// ErrInvalidIntervalType is returned when an interval type is invalid.
	ErrInvalidIntervalType = errors.New("invalid interval type: must be minutes, hours, or days")

	// ErrInvalidIntervalValue is returned when an interval value is invalid.
	ErrInvalidIntervalValue = errors.New("interval must be positive")
)

// IntervalConfig holds configuration for interval scheduling.
type IntervalConfig struct {
	Minutes int
	Type    string
}

// Validate validates the interval configuration.
func (c *IntervalConfig) Validate() error {
	if c.Minutes <= 0 {
		return ErrInvalidIntervalValue
	}

	switch c.Type {
	case IntervalTypeMinutes, IntervalTypeHours, IntervalTypeDays:
		return nil
	default:
		return ErrInvalidIntervalType
	}
}

// ToDuration converts the interval to a time.Duration.
func (c *IntervalConfig) ToDuration() time.Duration {
	switch c.Type {
	case IntervalTypeMinutes:
		return time.Duration(c.Minutes) * time.Minute
	case IntervalTypeHours:
		return time.Duration(c.Minutes) * time.Hour
	case IntervalTypeDays:
		return time.Duration(c.Minutes) * 24 * time.Hour
	default:
		// Default to minutes
		return time.Duration(c.Minutes) * time.Minute
	}
}

// CalculateNextRunAt calculates the next run time based on interval.
func CalculateNextRunAt(interval IntervalConfig, lastRun time.Time) time.Time {
	if lastRun.IsZero() {
		lastRun = time.Now()
	}

	duration := interval.ToDuration()
	nextRun := lastRun.Add(duration)

	// If next run is in the past, calculate from now
	if nextRun.Before(time.Now()) {
		nextRun = time.Now().Add(duration)
	}

	return nextRun
}

// CalculateNextRunAtFromNow calculates the next run time from now.
func CalculateNextRunAtFromNow(interval IntervalConfig) time.Time {
	return time.Now().Add(interval.ToDuration())
}

// IsIntervalTypeValid returns true if the interval type is valid.
func IsIntervalTypeValid(intervalType string) bool {
	switch intervalType {
	case IntervalTypeMinutes, IntervalTypeHours, IntervalTypeDays:
		return true
	default:
		return false
	}
}

// ParseIntervalType normalizes an interval type string.
func ParseIntervalType(intervalType string) string {
	switch intervalType {
	case "minute", "min", "m", IntervalTypeMinutes:
		return IntervalTypeMinutes
	case "hour", "hr", "h", IntervalTypeHours:
		return IntervalTypeHours
	case "day", "d", IntervalTypeDays:
		return IntervalTypeDays
	default:
		return IntervalTypeMinutes // Default
	}
}

// IntervalToCronExpression converts an interval to a cron-like expression string.
// This is for display/compatibility purposes only.
func IntervalToCronExpression(interval IntervalConfig) string {
	switch interval.Type {
	case IntervalTypeMinutes:
		if interval.Minutes == 1 {
			return "* * * * *" // Every minute
		}
		return "0/" + formatInt(interval.Minutes) + " * * * *"
	case IntervalTypeHours:
		if interval.Minutes == 1 {
			return "0 * * * *" // Every hour
		}
		return "0 0/" + formatInt(interval.Minutes) + " * * *"
	case IntervalTypeDays:
		if interval.Minutes == 1 {
			return "0 0 * * *" // Every day
		}
		return "0 0 0/" + formatInt(interval.Minutes) + " * *"
	default:
		return "0 * * * *"
	}
}

func formatInt(n int) string {
	// Simple int to string conversion
	if n == 0 {
		return "0"
	}

	const decimalBase = 10
	result := ""
	for n > 0 {
		digit := n % decimalBase
		result = string(rune('0'+digit)) + result
		n /= decimalBase
	}
	return result
}
