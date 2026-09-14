package gateway

import (
	"context"
	"github.com/redis/go-redis/v9"
	"testing"
)

func TestRateLimiterFailsClosed(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	_ = client.Close()
	limiter := NewRateLimiter(client, DefaultRateLimitConfig(), nil)
	if result, err := limiter.Check(context.Background(), "tenant"); err == nil || (result != nil && result.Allowed) {
		t.Fatal("Redis failure allowed request")
	}
	if allowed, err := limiter.AcquireConcurrency(context.Background(), "tenant"); err == nil || allowed {
		t.Fatal("Redis failure allowed concurrency acquisition")
	}
}
