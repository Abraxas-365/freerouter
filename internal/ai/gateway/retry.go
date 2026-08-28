package gateway

import (
	"time"
)

const (
	MaxRetries     = 2
	RetryBaseDelay = 500 * time.Millisecond
)

// retryableStatusCodes are HTTP status codes that warrant a retry.
var retryableStatusCodes = map[int]bool{
	408: true, // Request Timeout
	429: true, // Too Many Requests (provider-side rate limit)
	500: true, // Internal Server Error
	502: true, // Bad Gateway
	503: true, // Service Unavailable
	504: true, // Gateway Timeout
}

// IsRetryable returns true if the HTTP status code should trigger a retry.
func IsRetryable(statusCode int) bool {
	return retryableStatusCodes[statusCode]
}

// IsAuthError returns true if the error indicates permanently bad credentials.
func IsAuthError(statusCode int) bool {
	return statusCode == 401 || statusCode == 403
}

// RetryDelay returns the backoff delay for a given attempt (0-based).
// Uses exponential backoff: 500ms, 1s, 2s, ...
func RetryDelay(attempt int) time.Duration {
	d := RetryBaseDelay
	for i := 0; i < attempt; i++ {
		d *= 2
	}
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}
