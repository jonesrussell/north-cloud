package job

import (
	"time"
)

// Priority levels for sources.
const (
	PriorityLow      = "low"
	PriorityNormal   = "normal"
	PriorityHigh     = "high"
	PriorityCritical = "critical"
)

// Base interval values in minutes by priority.
const (
	baseCriticalIntervalMinutes = 15  // 4x/hour
	baseHighIntervalMinutes     = 30  // 2x/hour
	baseNormalIntervalMinutes   = 60  // 1x/hour
	baseLowIntervalMinutes      = 180 // 3x/day
)

// Numeric priority values (higher = scheduled sooner).
const (
	numericPriorityCritical = 100
	numericPriorityHigh     = 75
	numericPriorityNormal   = 50
	numericPriorityLow      = 25
)

// Initial delay values by priority.
const (
	initialDelayNormalMinutes = 5
	initialDelayLowMinutes    = 10
)

// Rate limit thresholds for adjustment.
const (
	lowRateLimitThreshold    = 5
	normalRateLimitThreshold = 10
	highRateLimitThreshold   = 20
)

// Depth thresholds for adjustment.
const (
	shallowDepthThreshold = 2
	mediumDepthThreshold  = 5
)

// Interval adjustment multipliers and divisors.
const (
	multiplierThree   = 3
	multiplierFive    = 5
	divisorTwo        = 2
	divisorFour       = 4
	defaultRateLimit  = 10
	defaultMaxDepth   = 1
	backoffMultiplier = 2
)

// Maximum backoff interval (24 hours in minutes).
const maxBackoffMinutes = 24 * 60

// Base intervals by priority (in minutes).
var priorityBaseIntervals = map[string]int{
	PriorityCritical: baseCriticalIntervalMinutes,
	PriorityHigh:     baseHighIntervalMinutes,
	PriorityNormal:   baseNormalIntervalMinutes,
	PriorityLow:      baseLowIntervalMinutes,
}

// Numeric priority values (higher = scheduled sooner).
var priorityNumericValues = map[string]int{
	PriorityCritical: numericPriorityCritical,
	PriorityHigh:     numericPriorityHigh,
	PriorityNormal:   numericPriorityNormal,
	PriorityLow:      numericPriorityLow,
}

// Initial delays by priority to stagger job starts.
var priorityInitialDelays = map[string]time.Duration{
	PriorityCritical: 0,
	PriorityHigh:     1 * time.Minute,
	PriorityNormal:   initialDelayNormalMinutes * time.Minute,
	PriorityLow:      initialDelayLowMinutes * time.Minute,
}

// ScheduleComputer computes job schedules based on source metadata.
type ScheduleComputer struct{}

// ScheduleInput contains the source metadata used to compute a schedule.
type ScheduleInput struct {
	RateLimit    int    // requests per second allowed
	MaxDepth     int    // crawl depth
	Priority     string // low, normal, high, critical
	FailureCount int    // consecutive failures (for backoff)
}

// ScheduleOutput contains the computed schedule parameters.
type ScheduleOutput struct {
	IntervalMinutes int           // interval between runs
	IntervalType    string        // "minutes" or "hours"
	NumericPriority int           // 0-100, higher = sooner
	InitialDelay    time.Duration // delay before first run
}

// NewScheduleComputer creates a new schedule computer.
func NewScheduleComputer() *ScheduleComputer {
	return &ScheduleComputer{}
}

// ComputeSchedule calculates the optimal schedule for a source.
func (sc *ScheduleComputer) ComputeSchedule(input ScheduleInput) ScheduleOutput {
	priority := input.Priority
	if priority == "" {
		priority = PriorityNormal
	}

	// Start with base interval from priority
	baseInterval := priorityBaseIntervals[priority]
	if baseInterval == 0 {
		baseInterval = priorityBaseIntervals[PriorityNormal]
	}

	// Adjust for rate limit
	intervalMinutes := sc.adjustForRateLimit(baseInterval, input.RateLimit)

	// Adjust for max depth
	intervalMinutes = sc.adjustForDepth(intervalMinutes, input.MaxDepth)

	// Apply exponential backoff if there are failures
	intervalMinutes = sc.applyBackoff(intervalMinutes, input.FailureCount)

	// Determine interval type for readability
	intervalType := "minutes"
	if intervalMinutes >= baseNormalIntervalMinutes && intervalMinutes%baseNormalIntervalMinutes == 0 {
		intervalType = "hours"
	}

	// Get numeric priority
	numericPriority := priorityNumericValues[priority]
	if numericPriority == 0 {
		numericPriority = priorityNumericValues[PriorityNormal]
	}

	// Get initial delay
	initialDelay := priorityInitialDelays[priority]

	return ScheduleOutput{
		IntervalMinutes: intervalMinutes,
		IntervalType:    intervalType,
		NumericPriority: numericPriority,
		InitialDelay:    initialDelay,
	}
}

// adjustForRateLimit adjusts interval based on rate limit.
// Lower rate limits need longer intervals to be polite.
func (sc *ScheduleComputer) adjustForRateLimit(baseInterval, rateLimit int) int {
	if rateLimit <= 0 {
		rateLimit = defaultRateLimit
	}

	switch {
	case rateLimit <= lowRateLimitThreshold:
		// Low rate limit: +50% interval
		return baseInterval * multiplierThree / divisorTwo
	case rateLimit <= normalRateLimitThreshold:
		// Normal: base interval
		return baseInterval
	case rateLimit <= highRateLimitThreshold:
		// Higher rate limit: -25% interval
		return baseInterval * multiplierThree / divisorFour
	default:
		// Very high rate limit: -50% interval
		return baseInterval / divisorTwo
	}
}

// adjustForDepth adjusts interval based on crawl depth.
// Deeper crawls take longer, so space them out more.
func (sc *ScheduleComputer) adjustForDepth(interval, maxDepth int) int {
	if maxDepth <= 0 {
		maxDepth = defaultMaxDepth
	}

	switch {
	case maxDepth <= shallowDepthThreshold:
		// Shallow: base interval
		return interval
	case maxDepth <= mediumDepthThreshold:
		// Medium: +25%
		return interval * multiplierFive / divisorFour
	default:
		// Deep: +50%
		return interval * multiplierThree / divisorTwo
	}
}

// applyBackoff applies exponential backoff based on failure count.
func (sc *ScheduleComputer) applyBackoff(interval, failureCount int) int {
	if failureCount <= 0 {
		return interval
	}

	// Exponential backoff: interval * 2^failures, capped at 24 hours
	backoffInterval := interval
	for i := 0; i < failureCount && backoffInterval < maxBackoffMinutes; i++ {
		backoffInterval *= backoffMultiplier
	}

	if backoffInterval > maxBackoffMinutes {
		backoffInterval = maxBackoffMinutes
	}

	return backoffInterval
}
