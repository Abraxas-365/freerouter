package gateway

import (
	"math"
	"testing"
	"time"

	"github.com/Abraxas-365/freerouter/internal/kernel"
)

func TestKeyHealthTracker_NewKeyIsHealthy(t *testing.T) {
	tracker := NewKeyHealthTracker()
	keyID := kernel.NewProviderKeyID("key-1")

	if !tracker.IsHealthy(keyID) {
		t.Fatal("new key should be healthy")
	}

	metrics := tracker.GetMetrics(keyID)
	if metrics.Uptime != 1.0 {
		t.Fatalf("expected uptime 1.0, got %f", metrics.Uptime)
	}
	if metrics.TotalRequests != 0 {
		t.Fatalf("expected 0 requests, got %d", metrics.TotalRequests)
	}
}

func TestKeyHealthTracker_SuccessKeepsHealthy(t *testing.T) {
	tracker := NewKeyHealthTracker()
	keyID := kernel.NewProviderKeyID("key-1")

	for i := 0; i < 10; i++ {
		tracker.ReportSuccess(keyID)
	}

	if !tracker.IsHealthy(keyID) {
		t.Fatal("key with all successes should be healthy")
	}

	metrics := tracker.GetMetrics(keyID)
	if metrics.Uptime != 1.0 {
		t.Fatalf("expected uptime 1.0, got %f", metrics.Uptime)
	}
	if metrics.TotalRequests != 10 {
		t.Fatalf("expected 10 requests, got %d", metrics.TotalRequests)
	}
	if metrics.ConsecutiveErrors != 0 {
		t.Fatalf("expected 0 consecutive errors, got %d", metrics.ConsecutiveErrors)
	}
}

func TestKeyHealthTracker_ConsecutiveErrorsBlacklist(t *testing.T) {
	tracker := NewKeyHealthTracker()
	keyID := kernel.NewProviderKeyID("key-1")

	// Report errors below threshold — should still be healthy
	for i := 0; i < healthErrorThreshold-1; i++ {
		tracker.ReportError(keyID, 500)
	}
	if !tracker.IsHealthy(keyID) {
		t.Fatal("key should be healthy below error threshold")
	}

	// One more error hits the threshold
	tracker.ReportError(keyID, 500)
	if tracker.IsHealthy(keyID) {
		t.Fatal("key should be unhealthy at error threshold")
	}

	metrics := tracker.GetMetrics(keyID)
	if metrics.ConsecutiveErrors != healthErrorThreshold {
		t.Fatalf("expected %d consecutive errors, got %d", healthErrorThreshold, metrics.ConsecutiveErrors)
	}
}

func TestKeyHealthTracker_SuccessResetsConsecutiveErrors(t *testing.T) {
	tracker := NewKeyHealthTracker()
	keyID := kernel.NewProviderKeyID("key-1")

	// Almost hit threshold
	for i := 0; i < healthErrorThreshold-1; i++ {
		tracker.ReportError(keyID, 500)
	}

	// Success resets
	tracker.ReportSuccess(keyID)

	// Error again — should not be at threshold
	tracker.ReportError(keyID, 500)
	if !tracker.IsHealthy(keyID) {
		t.Fatal("key should be healthy after success reset")
	}
}

func TestKeyHealthTracker_PermanentBlacklist(t *testing.T) {
	tracker := NewKeyHealthTracker()
	keyID := kernel.NewProviderKeyID("key-1")

	// 401 = permanent blacklist
	tracker.ReportError(keyID, 401)
	if tracker.IsHealthy(keyID) {
		t.Fatal("key should be permanently blacklisted on 401")
	}

	// Success doesn't un-blacklist a permanently blacklisted key
	tracker.ReportSuccess(keyID)
	if tracker.IsHealthy(keyID) {
		t.Fatal("permanently blacklisted key should stay unhealthy after success")
	}

	metrics := tracker.GetMetrics(keyID)
	if !metrics.PermanentlyBlacklisted {
		t.Fatal("expected PermanentlyBlacklisted=true")
	}
}

func TestKeyHealthTracker_403AlsoPermBlacklist(t *testing.T) {
	tracker := NewKeyHealthTracker()
	keyID := kernel.NewProviderKeyID("key-1")

	tracker.ReportError(keyID, 403)
	if tracker.IsHealthy(keyID) {
		t.Fatal("key should be permanently blacklisted on 403")
	}
}

func TestKeyHealthTracker_429DoesNotPermBlacklist(t *testing.T) {
	tracker := NewKeyHealthTracker()
	keyID := kernel.NewProviderKeyID("key-1")

	tracker.ReportError(keyID, 429)
	if !tracker.IsHealthy(keyID) {
		t.Fatal("single 429 should not blacklist")
	}
}

func TestKeyHealthTracker_UptimeCalculation(t *testing.T) {
	tracker := NewKeyHealthTracker()
	keyID := kernel.NewProviderKeyID("key-1")

	// 8 successes + 2 errors = 80% uptime
	for i := 0; i < 8; i++ {
		tracker.ReportSuccess(keyID)
	}
	tracker.ReportError(keyID, 500)
	tracker.ReportError(keyID, 500)

	metrics := tracker.GetMetrics(keyID)
	expected := 0.8
	if math.Abs(metrics.Uptime-expected) > 0.01 {
		t.Fatalf("expected uptime ~%.2f, got %.2f", expected, metrics.Uptime)
	}
}

func TestKeyHealthTracker_UptimePenalty(t *testing.T) {
	tracker := NewKeyHealthTracker()

	// No history = no penalty
	keyID := kernel.NewProviderKeyID("key-no-history")
	if p := tracker.UptimePenalty(keyID); p != 0 {
		t.Fatalf("expected 0 penalty for new key, got %f", p)
	}

	// 100% uptime = no penalty
	key100 := kernel.NewProviderKeyID("key-100")
	for i := 0; i < 20; i++ {
		tracker.ReportSuccess(key100)
	}
	if p := tracker.UptimePenalty(key100); p != 0 {
		t.Fatalf("expected 0 penalty for 100%% uptime, got %f", p)
	}

	// 80% uptime = some penalty
	key80 := kernel.NewProviderKeyID("key-80")
	for i := 0; i < 8; i++ {
		tracker.ReportSuccess(key80)
	}
	tracker.ReportError(key80, 500)
	tracker.ReportError(key80, 500)
	penalty := tracker.UptimePenalty(key80)
	if penalty <= 0 {
		t.Fatalf("expected positive penalty for 80%% uptime, got %f", penalty)
	}

	// Permanently blacklisted = +Inf
	keyBlack := kernel.NewProviderKeyID("key-black")
	tracker.ReportError(keyBlack, 401)
	if p := tracker.UptimePenalty(keyBlack); !math.IsInf(p, 1) {
		t.Fatalf("expected +Inf penalty for blacklisted key, got %f", p)
	}
}

func TestKeyHealthTracker_MultipleKeysIndependent(t *testing.T) {
	tracker := NewKeyHealthTracker()
	key1 := kernel.NewProviderKeyID("key-1")
	key2 := kernel.NewProviderKeyID("key-2")

	// Blacklist key1
	for i := 0; i < healthErrorThreshold; i++ {
		tracker.ReportError(key1, 500)
	}

	// key2 should be unaffected
	if !tracker.IsHealthy(key2) {
		t.Fatal("key2 should be healthy — independent of key1")
	}
	if tracker.IsHealthy(key1) {
		t.Fatal("key1 should be unhealthy")
	}
}

func TestKeyHealthTracker_BlacklistExpires(t *testing.T) {
	tracker := NewKeyHealthTracker()
	keyID := kernel.NewProviderKeyID("key-1")

	// Get the health entry and manually set an old error time
	h := tracker.getOrCreate(keyID)
	h.mu.Lock()
	h.consecutiveErrors = healthErrorThreshold
	h.lastErrorTime = time.Now().Add(-healthBlacklistDuration - time.Second)
	h.mu.Unlock()

	// Blacklist should have expired
	if !tracker.IsHealthy(keyID) {
		t.Fatal("key should be healthy after blacklist window expires")
	}

	// Consecutive errors should be reset
	h.mu.Lock()
	if h.consecutiveErrors != 0 {
		t.Fatalf("expected consecutive errors reset to 0, got %d", h.consecutiveErrors)
	}
	h.mu.Unlock()
}

func TestKeyHealthTracker_ConcurrentAccess(t *testing.T) {
	tracker := NewKeyHealthTracker()
	keyID := kernel.NewProviderKeyID("key-1")

	done := make(chan struct{})
	for i := 0; i < 100; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			tracker.ReportSuccess(keyID)
			tracker.IsHealthy(keyID)
			tracker.GetMetrics(keyID)
			tracker.ReportError(keyID, 500)
			tracker.UptimePenalty(keyID)
		}()
	}

	for i := 0; i < 100; i++ {
		<-done
	}

	// Just verify no panics or races
	_ = tracker.GetMetrics(keyID)
}
