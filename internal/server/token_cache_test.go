package server

import (
	"testing"
	"time"
)

func TestTokenCache_SetAndGet(t *testing.T) {
	c := newTokenCache(0, time.Hour) // no margin, infrequent sweep
	defer c.Stop()

	c.Set("ik_svc_abc", "jwt-123", 60)

	got := c.Get("ik_svc_abc")
	if got != "jwt-123" {
		t.Fatalf("Get = %q, want %q", got, "jwt-123")
	}
}

func TestTokenCache_MissReturnsEmpty(t *testing.T) {
	c := newTokenCache(0, time.Hour)
	defer c.Stop()

	if got := c.Get("ik_svc_missing"); got != "" {
		t.Fatalf("Get(missing) = %q, want empty", got)
	}
}

func TestTokenCache_ExpiredEntryReturnsEmpty(t *testing.T) {
	c := newTokenCache(0, time.Hour)
	defer c.Stop()

	// Set with 1-second TTL, then wait for it to expire.
	c.Set("ik_svc_short", "jwt-short", 1)
	time.Sleep(1100 * time.Millisecond)

	if got := c.Get("ik_svc_short"); got != "" {
		t.Fatalf("Get(expired) = %q, want empty", got)
	}
}

func TestTokenCache_MarginReducesTTL(t *testing.T) {
	// Margin of 2s on a 1s TTL → effective TTL ≤ 0 → not cached.
	c := newTokenCache(2*time.Second, time.Hour)
	defer c.Stop()

	c.Set("ik_svc_tiny", "jwt-tiny", 1)

	if got := c.Get("ik_svc_tiny"); got != "" {
		t.Fatalf("Get(too-short-to-cache) = %q, want empty", got)
	}
}

func TestTokenCache_OverwriteEntry(t *testing.T) {
	c := newTokenCache(0, time.Hour)
	defer c.Stop()

	c.Set("ik_svc_key", "jwt-old", 60)
	c.Set("ik_svc_key", "jwt-new", 60)

	if got := c.Get("ik_svc_key"); got != "jwt-new" {
		t.Fatalf("Get = %q, want %q", got, "jwt-new")
	}
}

func TestTokenCache_StopIsIdempotent(t *testing.T) {
	c := newTokenCache(0, time.Hour)
	c.Stop()
	c.Stop() // must not panic
}
