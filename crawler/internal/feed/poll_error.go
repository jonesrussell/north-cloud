package feed

import "fmt"

// ErrorType classifies feed poll failures for severity-aware logging and auto-disable.
type ErrorType string

const (
	ErrTypeRateLimited ErrorType = "rate_limited"
	ErrTypeForbidden   ErrorType = "forbidden"
	ErrTypeNotFound    ErrorType = "not_found"
	ErrTypeGone        ErrorType = "gone"
	ErrTypeUpstream    ErrorType = "upstream_failure"
	ErrTypeNetwork     ErrorType = "network"
	ErrTypeParse       ErrorType = "parse_error"
	ErrTypeUnexpected  ErrorType = "unexpected"
)

// LogLevel determines whether a PollError is logged at WARN or ERROR.
type LogLevel int

const (
	LevelWarn LogLevel = iota
	LevelError
)

// PollError represents a classified feed polling failure.
type PollError struct {
	Type       ErrorType
	Level      LogLevel
	StatusCode int
	URL        string
	Cause      error
}

func (e *PollError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("feed poll %s: HTTP %d for %s", e.Type, e.StatusCode, e.URL)
	}

	return fmt.Sprintf("feed poll %s: %s for %s", e.Type, e.Cause, e.URL)
}

func (e *PollError) Unwrap() error { return e.Cause }

// HTTP status code boundaries for classification.
const (
	statusForbidden       = 403
	statusNotFound        = 404
	statusGone            = 410
	statusTooManyRequests = 429
	statusServerErrorLow  = 500
	statusServerErrorHigh = 599
)

// ClassifyHTTPStatus creates a PollError from an HTTP status code.
func ClassifyHTTPStatus(statusCode int, url string) *PollError {
	cause := fmt.Errorf("HTTP %d", statusCode)

	switch {
	case statusCode == statusTooManyRequests:
		return &PollError{Type: ErrTypeRateLimited, Level: LevelWarn, StatusCode: statusCode, URL: url, Cause: cause}
	case statusCode == statusForbidden:
		return &PollError{Type: ErrTypeForbidden, Level: LevelWarn, StatusCode: statusCode, URL: url, Cause: cause}
	case statusCode == statusNotFound:
		return &PollError{Type: ErrTypeNotFound, Level: LevelWarn, StatusCode: statusCode, URL: url, Cause: cause}
	case statusCode == statusGone:
		return &PollError{Type: ErrTypeGone, Level: LevelWarn, StatusCode: statusCode, URL: url, Cause: cause}
	case statusCode >= statusServerErrorLow && statusCode <= statusServerErrorHigh:
		return &PollError{Type: ErrTypeUpstream, Level: LevelWarn, StatusCode: statusCode, URL: url, Cause: cause}
	default:
		return &PollError{Type: ErrTypeUnexpected, Level: LevelError, StatusCode: statusCode, URL: url, Cause: cause}
	}
}

// ClassifyNetworkError creates a PollError for network-level failures (DNS, timeout, etc.).
func ClassifyNetworkError(cause error, url string) *PollError {
	return &PollError{Type: ErrTypeNetwork, Level: LevelWarn, URL: url, Cause: cause}
}

// ClassifyParseError creates a PollError for feed parsing failures.
func ClassifyParseError(cause error, url string) *PollError {
	return &PollError{Type: ErrTypeParse, Level: LevelWarn, URL: url, Cause: cause}
}

// Disable thresholds: consecutive failures before a feed is auto-disabled.
const (
	thresholdNotFound  = 3
	thresholdGone      = 1
	thresholdForbidden = 5
	thresholdUpstream  = 10
	thresholdNetwork   = 10
	thresholdParse     = 5
)

// disableThresholds maps error types to the number of consecutive failures before auto-disable.
// Types not in this map (RateLimited, Unexpected) are never auto-disabled.
var disableThresholds = map[ErrorType]int{ //nolint:exhaustive // RateLimited and Unexpected intentionally excluded.
	ErrTypeNotFound:  thresholdNotFound,
	ErrTypeGone:      thresholdGone,
	ErrTypeForbidden: thresholdForbidden,
	ErrTypeUpstream:  thresholdUpstream,
	ErrTypeNetwork:   thresholdNetwork,
	ErrTypeParse:     thresholdParse,
}

// DisableThreshold returns the auto-disable threshold for an error type.
// Returns (0, false) if the error type should never be auto-disabled.
func DisableThreshold(errType ErrorType) (int, bool) {
	threshold, ok := disableThresholds[errType]
	return threshold, ok
}
