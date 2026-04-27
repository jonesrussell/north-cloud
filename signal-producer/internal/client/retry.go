package client

import (
	"context"
	"errors"
	"time"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// retryOp is the unit of work the retry helper drives. It is invoked once
// initially plus up to len(backoffs) more times.
type retryOp func(ctx context.Context) error

// retry runs op with the supplied exponential backoff schedule. It retries
// on transient errors (errServer wraps + transport-class errors that are
// NOT errClient). It does NOT retry on errClient (4xx) or on context
// cancellation.
//
// Total call sites are bounded: 1 initial + len(backoffs) retries. With the
// production schedule (1s, 5s, 15s) the maximum elapsed wall time spent
// sleeping is 21s, under NFR-002's 25s budget.
func retry(ctx context.Context, backoffs []time.Duration, op retryOp, log infralogger.Logger) error {
	var lastErr error
	totalAttempts := len(backoffs) + 1
	for attempt := range totalAttempts {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		err := op(ctx)
		if err == nil {
			return nil
		}
		lastErr = err
		if !isRetryable(err) {
			return err
		}
		// No backoff after the final attempt.
		if attempt == totalAttempts-1 {
			break
		}
		backoff := backoffs[attempt]
		log.Warn("waaseyaa post failed; will retry",
			infralogger.Int("attempt", attempt+1),
			infralogger.Duration("backoff", backoff),
			infralogger.Error(err),
		)
		if sleepErr := sleepCtx(ctx, backoff); sleepErr != nil {
			return sleepErr
		}
	}
	return lastErr
}

// isRetryable returns true for transport-class failures and errServer (5xx).
// errClient (4xx) is explicitly non-retryable.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errClient) {
		return false
	}
	// errServer (5xx) is retryable; any other non-classified error is
	// treated as a transport/network failure and is also retryable. The
	// only non-retryable terminal class is errClient above.
	return true
}

// sleepCtx waits for d or until ctx is done. Returns ctx.Err() on cancel.
func sleepCtx(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
