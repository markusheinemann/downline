package openskynetwork

import (
	"errors"
	"fmt"
	"net/http"
	"time"
)

var ErrRateLimited = errors.New("opensky: rate limited")

// APIError is returned when the API responds with a status code outside the
// 2xx range. Use errors.As to inspect it, for example, to back off on 429.
type APIError struct {
	StatusCode int
	Status     string
	// Body holds at most the first 1024 bytes of the response body.
	Body string
}

type RateLimitError struct {
	*APIError
	RetryAfter time.Duration
}

// Error returns the HTTP status followed by the response body, if any.
func (e *APIError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("opensky: %s", e.Status)
	}
	return fmt.Sprintf("opensky: %s: %s", e.Status, e.Body)
}

func (e *APIError) Is(target error) bool {
	return target == ErrRateLimited && e.StatusCode == http.StatusTooManyRequests
}
