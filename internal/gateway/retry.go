package gateway

import "time"

const (
	MaxRetries     = 2
	RetryBaseDelay = 500 * time.Millisecond
)

var retryableStatusCodes = map[int]bool{
	408: true,
	429: true,
	500: true,
	502: true,
	503: true,
	504: true,
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
