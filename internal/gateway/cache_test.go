package gateway_test

import (
	"context"
	"testing"
	"time"

	"github.com/Abraxas-365/freerouter/internal/gateway"
	"github.com/Abraxas-365/freerouter/internal/testutil"
)

func TestResponseCache_SetAndGet(t *testing.T) {
	rdb := testutil.RedisClient(t)
	cache := gateway.NewResponseCache(rdb, 60*time.Second)
	ctx := context.Background()

	resp := &gateway.ChatResponse{
		ID:      "chatcmpl-123",
		Object:  "chat.completion",
		Created: 1234567890,
		Model:   "gpt-4o",
		Choices: []gateway.Choice{
			{
				Index:   0,
				Message: &gateway.Message{Role: "assistant", Content: "Hello!"},
			},
		},
		Usage: &gateway.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	key := "cache:resp:test-subject:abc123"
	cache.Set(ctx, key, resp)

	got := cache.Get(ctx, key)
	if got == nil {
		t.Fatal("expected cached response, got nil")
	}
	if got.ID != "chatcmpl-123" {
		t.Errorf("expected ID chatcmpl-123, got %s", got.ID)
	}
	if got.Model != "gpt-4o" {
		t.Errorf("expected model gpt-4o, got %s", got.Model)
	}
	if len(got.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(got.Choices))
	}
	if got.Usage == nil || got.Usage.TotalTokens != 15 {
		t.Error("usage not preserved")
	}
}

func TestResponseCache_MissReturnsNil(t *testing.T) {
	rdb := testutil.RedisClient(t)
	cache := gateway.NewResponseCache(rdb, 60*time.Second)
	ctx := context.Background()

	got := cache.Get(ctx, "cache:resp:subject:nonexistent")
	if got != nil {
		t.Fatal("expected nil for cache miss")
	}
}

func TestResponseCache_TTLExpires(t *testing.T) {
	rdb := testutil.RedisClient(t)
	cache := gateway.NewResponseCache(rdb, 1*time.Second)
	ctx := context.Background()

	resp := &gateway.ChatResponse{ID: "test-ttl", Model: "gpt-4o"}
	key := "cache:resp:test:ttl"
	cache.Set(ctx, key, resp)

	// Should exist
	if got := cache.Get(ctx, key); got == nil {
		t.Fatal("expected cached response before expiry")
	}

	time.Sleep(1500 * time.Millisecond)

	// Should be expired
	if got := cache.Get(ctx, key); got != nil {
		t.Fatal("expected nil after TTL expiry")
	}
}

func TestResponseCache_NilRedis(t *testing.T) {
	cache := gateway.NewResponseCache(nil, 60*time.Second)
	ctx := context.Background()

	// Should not panic
	cache.Set(ctx, "key", &gateway.ChatResponse{ID: "test"})
	got := cache.Get(ctx, "key")
	if got != nil {
		t.Fatal("nil Redis should return nil")
	}
}

func TestResponseCache_NilResponse(t *testing.T) {
	rdb := testutil.RedisClient(t)
	cache := gateway.NewResponseCache(rdb, 60*time.Second)
	ctx := context.Background()

	// Should not panic
	cache.Set(ctx, "key", nil)
	got := cache.Get(ctx, "key")
	if got != nil {
		t.Fatal("nil response should not be cached")
	}
}

func TestGenerateKey_DeterministicForSameRequest(t *testing.T) {
	req := &gateway.ChatRequest{
		Model: "gpt-4o",
		Messages: []gateway.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	key1 := gateway.GenerateKey("subject-1", req)
	key2 := gateway.GenerateKey("subject-1", req)

	if key1 != key2 {
		t.Errorf("same request should produce same key:\n  %s\n  %s", key1, key2)
	}
}

func TestGenerateKey_DifferentForDifferentSubjects(t *testing.T) {
	req := &gateway.ChatRequest{
		Model: "gpt-4o",
		Messages: []gateway.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	key1 := gateway.GenerateKey("subject-1", req)
	key2 := gateway.GenerateKey("subject-2", req)

	if key1 == key2 {
		t.Error("different subjects should produce different keys")
	}
}

func TestGenerateKey_DifferentForDifferentContent(t *testing.T) {
	req1 := &gateway.ChatRequest{
		Model:    "gpt-4o",
		Messages: []gateway.Message{{Role: "user", Content: "Hello"}},
	}
	req2 := &gateway.ChatRequest{
		Model:    "gpt-4o",
		Messages: []gateway.Message{{Role: "user", Content: "Goodbye"}},
	}

	key1 := gateway.GenerateKey("subject-1", req1)
	key2 := gateway.GenerateKey("subject-1", req2)

	if key1 == key2 {
		t.Error("different message content should produce different keys")
	}
}

func TestResponseCache_InvalidateBySubject(t *testing.T) {
	rdb := testutil.RedisClient(t)
	cache := gateway.NewResponseCache(rdb, 60*time.Second)
	ctx := context.Background()

	req := &gateway.ChatRequest{Model: "gpt-4o", Messages: []gateway.Message{{Role: "user", Content: "Hi"}}}
	keyA := gateway.GenerateKey("subject-a", req)
	keyB := gateway.GenerateKey("subject-b", req)

	cache.Set(ctx, keyA, &gateway.ChatResponse{ID: "a"})
	cache.Set(ctx, keyB, &gateway.ChatResponse{ID: "b"})

	n, err := cache.InvalidateBySubject(ctx, "subject-a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 key invalidated, got %d", n)
	}

	if got := cache.Get(ctx, keyA); got != nil {
		t.Fatal("expected subject-a cache entry to be invalidated")
	}
	if got := cache.Get(ctx, keyB); got == nil {
		t.Fatal("expected subject-b cache entry to remain")
	}
}

func TestResponseCache_InvalidateAll(t *testing.T) {
	rdb := testutil.RedisClient(t)
	cache := gateway.NewResponseCache(rdb, 60*time.Second)
	ctx := context.Background()

	req := &gateway.ChatRequest{Model: "gpt-4o", Messages: []gateway.Message{{Role: "user", Content: "Hi"}}}
	keyA := gateway.GenerateKey("subject-a", req)
	keyB := gateway.GenerateKey("subject-b", req)

	cache.Set(ctx, keyA, &gateway.ChatResponse{ID: "a"})
	cache.Set(ctx, keyB, &gateway.ChatResponse{ID: "b"})

	n, err := cache.InvalidateAll(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 keys invalidated, got %d", n)
	}

	if got := cache.Get(ctx, keyA); got != nil {
		t.Fatal("expected subject-a cache entry to be invalidated")
	}
	if got := cache.Get(ctx, keyB); got != nil {
		t.Fatal("expected subject-b cache entry to be invalidated")
	}
}
