package fetcher

import (
	"errors"
	"net/http"
)

// ErrTooManyRedirects is returned when the redirect hop limit is exceeded.
// The worker uses errors.Is to map this to the canonical last_error reason "too_many_redirects".
var ErrTooManyRedirects = errors.New("too many redirects")

// RedirectPolicy returns a CheckRedirect function that follows redirects until
// the number of redirects reaches maxHops, then returns ErrTooManyRedirects.
// Use with http.Client.CheckRedirect. When maxHops is <= 0, redirects are not limited
// beyond the default http client behavior (10).
func RedirectPolicy(maxHops int) func(*http.Request, []*http.Request) error {
	return func(_ *http.Request, via []*http.Request) error {
		if maxHops > 0 && len(via) >= maxHops {
			return ErrTooManyRedirects
		}
		return nil
	}
}
