package gateway

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestRedisForCache(t *testing.T) (*redis.Client, *miniredis.Miniredis, func()) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return rdb, mr, func() {
		rdb.Close()
		mr.Close()
	}
}

func TestResponseCache_SetAndGet(t *testing.T) {
	rdb, _, cleanup := newTestRedisForCache(t)
	defer cleanup()

	cache := NewResponseCache(rdb, 60*time.Second)
	ctx := context.Background()

	resp := &ChatResponse{
		ID:      "chatcmpl-123",
		Object:  "chat.completion",
		Created: 1234567890,
		Model:   "gpt-4o",
		Choices: []Choice{
			{
				Index:   0,
				Message: &Message{Role: "assistant", Content: "Hello!"},
			},
		},
		Usage: &Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	key := "cache:resp:test-tenant:abc123"
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
	rdb, _, cleanup := newTestRedisForCache(t)
	defer cleanup()

	cache := NewResponseCache(rdb, 60*time.Second)
	ctx := context.Background()

	got := cache.Get(ctx, "cache:resp:tenant:nonexistent")
	if got != nil {
		t.Fatal("expected nil for cache miss")
	}
}

func TestResponseCache_TTLExpires(t *testing.T) {
	rdb, mr, cleanup := newTestRedisForCache(t)
	defer cleanup()

	cache := NewResponseCache(rdb, 2*time.Second)
	ctx := context.Background()

	resp := &ChatResponse{ID: "test-ttl", Model: "gpt-4o"}
	key := "cache:resp:test:ttl"
	cache.Set(ctx, key, resp)

	// Should exist
	if got := cache.Get(ctx, key); got == nil {
		t.Fatal("expected cached response before expiry")
	}

	// Fast-forward time in miniredis
	mr.FastForward(3 * time.Second)

	// Should be expired
	if got := cache.Get(ctx, key); got != nil {
		t.Fatal("expected nil after TTL expiry")
	}
}

func TestResponseCache_NilRedis(t *testing.T) {
	cache := NewResponseCache(nil, 60*time.Second)
	ctx := context.Background()

	// Should not panic
	cache.Set(ctx, "key", &ChatResponse{ID: "test"})
	got := cache.Get(ctx, "key")
	if got != nil {
		t.Fatal("nil Redis should return nil")
	}
}

func TestResponseCache_NilResponse(t *testing.T) {
	rdb, _, cleanup := newTestRedisForCache(t)
	defer cleanup()

	cache := NewResponseCache(rdb, 60*time.Second)
	ctx := context.Background()

	// Should not panic
	cache.Set(ctx, "key", nil)
	got := cache.Get(ctx, "key")
	if got != nil {
		t.Fatal("nil response should not be cached")
	}
}

func TestGenerateKey_DeterministicForSameRequest(t *testing.T) {
	req := &ChatRequest{
		Model: "gpt-4o",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	}

	key1 := GenerateKey("tenant-1", req)
	key2 := GenerateKey("tenant-1", req)

	if key1 != key2 {
		t.Errorf("same request should produce same key:\n  %s\n  %s", key1, key2)
	}
}

func TestGenerateKey_DifferentForDifferentTenants(t *testing.T) {
	req := &ChatRequest{
		Model: "gpt-4o",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	}

	key1 := GenerateKey("tenant-1", req)
	key2 := GenerateKey("tenant-2", req)

	if key1 == key2 {
		t.Error("different tenants should produce different keys")
	}
}

func TestGenerateKey_DifferentForDifferentMessages(t *testing.T) {
	req1 := &ChatRequest{
		Model: "gpt-4o",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	}
	req2 := &ChatRequest{
		Model: "gpt-4o",
		Messages: []Message{
			{Role: "user", Content: "Goodbye"},
		},
	}

	key1 := GenerateKey("tenant-1", req1)
	key2 := GenerateKey("tenant-1", req2)

	if key1 == key2 {
		t.Error("different messages should produce different keys")
	}
}

func TestGenerateKey_DifferentForDifferentModels(t *testing.T) {
	msgs := []Message{{Role: "user", Content: "Hello"}}

	key1 := GenerateKey("t", &ChatRequest{Model: "gpt-4o", Messages: msgs})
	key2 := GenerateKey("t", &ChatRequest{Model: "gpt-3.5-turbo", Messages: msgs})

	if key1 == key2 {
		t.Error("different models should produce different keys")
	}
}

func TestGenerateKey_IgnoresStreamAndUserFields(t *testing.T) {
	msgs := []Message{{Role: "user", Content: "Hello"}}

	req1 := &ChatRequest{Model: "gpt-4o", Messages: msgs, Stream: true, User: "alice"}
	req2 := &ChatRequest{Model: "gpt-4o", Messages: msgs, Stream: false, User: "bob"}

	key1 := GenerateKey("t", req1)
	key2 := GenerateKey("t", req2)

	if key1 != key2 {
		t.Error("stream and user fields should not affect cache key")
	}
}

func TestGenerateKey_DifferentTemperature(t *testing.T) {
	msgs := []Message{{Role: "user", Content: "Hello"}}
	t1 := 0.5
	t2 := 0.9

	key1 := GenerateKey("t", &ChatRequest{Model: "gpt-4o", Messages: msgs, Temperature: &t1})
	key2 := GenerateKey("t", &ChatRequest{Model: "gpt-4o", Messages: msgs, Temperature: &t2})

	if key1 == key2 {
		t.Error("different temperature should produce different keys")
	}
}

func TestGenerateKey_HasCorrectPrefix(t *testing.T) {
	key := GenerateKey("my-tenant", &ChatRequest{Model: "gpt-4o"})
	prefix := "cache:resp:my-tenant:"
	if len(key) < len(prefix) || key[:len(prefix)] != prefix {
		t.Errorf("key should start with %q, got %q", prefix, key)
	}
}

// ============================================================================
// Cache Invalidation Tests
// ============================================================================

func TestResponseCache_InvalidateByTenant(t *testing.T) {
	rdb, _, cleanup := newTestRedisForCache(t)
	defer cleanup()

	cache := NewResponseCache(rdb, 60*time.Second)
	ctx := context.Background()

	// Set entries for two tenants
	resp := &ChatResponse{ID: "test", Model: "gpt-4o"}
	cache.Set(ctx, "cache:resp:tenant-1:aaa", resp)
	cache.Set(ctx, "cache:resp:tenant-1:bbb", resp)
	cache.Set(ctx, "cache:resp:tenant-2:ccc", resp)

	// Invalidate tenant-1
	deleted, err := cache.InvalidateByTenant(ctx, "tenant-1")
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 2 {
		t.Fatalf("expected 2 keys deleted, got %d", deleted)
	}

	// tenant-1 keys should be gone
	if cache.Get(ctx, "cache:resp:tenant-1:aaa") != nil {
		t.Fatal("tenant-1 key should be invalidated")
	}
	if cache.Get(ctx, "cache:resp:tenant-1:bbb") != nil {
		t.Fatal("tenant-1 key should be invalidated")
	}

	// tenant-2 keys should remain
	if cache.Get(ctx, "cache:resp:tenant-2:ccc") == nil {
		t.Fatal("tenant-2 key should still exist")
	}
}

func TestResponseCache_InvalidateAll(t *testing.T) {
	rdb, _, cleanup := newTestRedisForCache(t)
	defer cleanup()

	cache := NewResponseCache(rdb, 60*time.Second)
	ctx := context.Background()

	resp := &ChatResponse{ID: "test", Model: "gpt-4o"}
	cache.Set(ctx, "cache:resp:tenant-1:aaa", resp)
	cache.Set(ctx, "cache:resp:tenant-2:bbb", resp)
	cache.Set(ctx, "cache:resp:tenant-3:ccc", resp)

	deleted, err := cache.InvalidateAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 3 {
		t.Fatalf("expected 3 keys deleted, got %d", deleted)
	}

	if cache.Get(ctx, "cache:resp:tenant-1:aaa") != nil {
		t.Fatal("all keys should be invalidated")
	}
}

func TestResponseCache_InvalidateKey(t *testing.T) {
	rdb, _, cleanup := newTestRedisForCache(t)
	defer cleanup()

	cache := NewResponseCache(rdb, 60*time.Second)
	ctx := context.Background()

	resp := &ChatResponse{ID: "test", Model: "gpt-4o"}
	cache.Set(ctx, "cache:resp:tenant-1:aaa", resp)
	cache.Set(ctx, "cache:resp:tenant-1:bbb", resp)

	err := cache.InvalidateKey(ctx, "cache:resp:tenant-1:aaa")
	if err != nil {
		t.Fatal(err)
	}

	if cache.Get(ctx, "cache:resp:tenant-1:aaa") != nil {
		t.Fatal("specific key should be invalidated")
	}
	if cache.Get(ctx, "cache:resp:tenant-1:bbb") == nil {
		t.Fatal("other keys should still exist")
	}
}

func TestResponseCache_InvalidateNilRedis(t *testing.T) {
	cache := NewResponseCache(nil, 60*time.Second)
	ctx := context.Background()

	deleted, err := cache.InvalidateByTenant(ctx, "tenant-1")
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 0 {
		t.Fatal("nil Redis should return 0")
	}

	deleted, err = cache.InvalidateAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 0 {
		t.Fatal("nil Redis should return 0")
	}

	err = cache.InvalidateKey(ctx, "key")
	if err != nil {
		t.Fatal(err)
	}
}
