package gateway

import (
	"testing"
	"time"
)

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		code      int
		retryable bool
	}{
		{200, false},
		{400, false},
		{401, false}, // auth error, not retryable (handled separately)
		{403, false},
		{404, false},
		{408, true},
		{429, true},
		{500, true},
		{502, true},
		{503, true},
		{504, true},
	}

	for _, tt := range tests {
		if got := IsRetryable(tt.code); got != tt.retryable {
			t.Errorf("IsRetryable(%d) = %v, want %v", tt.code, got, tt.retryable)
		}
	}
}

func TestIsAuthError(t *testing.T) {
	tests := []struct {
		code int
		auth bool
	}{
		{200, false},
		{400, false},
		{401, true},
		{403, true},
		{404, false},
		{500, false},
	}

	for _, tt := range tests {
		if got := IsAuthError(tt.code); got != tt.auth {
			t.Errorf("IsAuthError(%d) = %v, want %v", tt.code, got, tt.auth)
		}
	}
}

func TestRetryDelay_ExponentialBackoff(t *testing.T) {
	d0 := RetryDelay(0)
	d1 := RetryDelay(1)
	d2 := RetryDelay(2)

	if d0 != 500*time.Millisecond {
		t.Errorf("RetryDelay(0) = %v, want 500ms", d0)
	}
	if d1 != 1*time.Second {
		t.Errorf("RetryDelay(1) = %v, want 1s", d1)
	}
	if d2 != 2*time.Second {
		t.Errorf("RetryDelay(2) = %v, want 2s", d2)
	}
}

func TestRetryDelay_CappedAt5s(t *testing.T) {
	for attempt := 0; attempt < 20; attempt++ {
		d := RetryDelay(attempt)
		if d > 5*time.Second {
			t.Errorf("RetryDelay(%d) = %v, exceeds 5s cap", attempt, d)
		}
	}
}
