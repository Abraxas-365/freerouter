package gateway

import (
	"math"
	"sync"
	"time"

	"github.com/Abraxas-365/freerouter/internal/kernel"
)

const (
	healthErrorThreshold    = 3                // consecutive errors before temp blacklist
	healthBlacklistDuration = 30 * time.Second // temp blacklist window
	healthMetricsWindow     = 5 * time.Minute  // sliding window for uptime calc
	healthUptimeThreshold   = 0.95             // below this, exponential penalty applies
)

// permanentErrorCodes are HTTP status codes that indicate permanently bad credentials.
var permanentErrorCodes = map[int]bool{
	401: true,
	403: true,
}

type requestOutcome struct {
	timestamp time.Time
	success   bool
}

type keyHealth struct {
	mu                     sync.Mutex
	consecutiveErrors      int
	lastErrorTime          time.Time
	permanentlyBlacklisted bool
	history                []requestOutcome
}

// KeyMetrics exposes the health state of a single provider key.
type KeyMetrics struct {
	Uptime                 float64 `json:"uptime"`
	TotalRequests          int     `json:"total_requests"`
	ConsecutiveErrors      int     `json:"consecutive_errors"`
	PermanentlyBlacklisted bool    `json:"permanently_blacklisted"`
}

// KeyHealthTracker tracks per-key health in-memory using a sliding window.
// It is goroutine-safe.
type KeyHealthTracker struct {
	mu     sync.RWMutex
	health map[kernel.ProviderKeyID]*keyHealth
}

func NewKeyHealthTracker() *KeyHealthTracker {
	return &KeyHealthTracker{
		health: make(map[kernel.ProviderKeyID]*keyHealth),
	}
}

func (t *KeyHealthTracker) getOrCreate(keyID kernel.ProviderKeyID) *keyHealth {
	t.mu.RLock()
	h, ok := t.health[keyID]
	t.mu.RUnlock()
	if ok {
		return h
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if h, ok = t.health[keyID]; ok {
		return h
	}
	h = &keyHealth{}
	t.health[keyID] = h
	return h
}

// IsHealthy returns true if the key should be considered for routing.
// A key is unhealthy if it is permanently blacklisted or has hit the
// consecutive error threshold within the blacklist window.
func (t *KeyHealthTracker) IsHealthy(keyID kernel.ProviderKeyID) bool {
	t.mu.RLock()
	h, ok := t.health[keyID]
	t.mu.RUnlock()
	if !ok {
		return true // no history = healthy
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.permanentlyBlacklisted {
		return false
	}
	if h.consecutiveErrors >= healthErrorThreshold {
		if time.Since(h.lastErrorTime) < healthBlacklistDuration {
			return false
		}
		// Blacklist window expired — give the key another chance
		h.consecutiveErrors = 0
	}
	return true
}

// ReportSuccess records a successful request for the given key.
func (t *KeyHealthTracker) ReportSuccess(keyID kernel.ProviderKeyID) {
	h := t.getOrCreate(keyID)
	h.mu.Lock()
	defer h.mu.Unlock()

	h.consecutiveErrors = 0
	h.history = append(h.history, requestOutcome{time.Now(), true})
	h.pruneHistory()
}

// ReportError records a failed request. If statusCode is 401 or 403
// the key is permanently blacklisted (bad credentials).
func (t *KeyHealthTracker) ReportError(keyID kernel.ProviderKeyID, statusCode int) {
	h := t.getOrCreate(keyID)
	h.mu.Lock()
	defer h.mu.Unlock()

	if permanentErrorCodes[statusCode] {
		h.permanentlyBlacklisted = true
	}

	h.consecutiveErrors++
	h.lastErrorTime = time.Now()
	h.history = append(h.history, requestOutcome{time.Now(), false})
	h.pruneHistory()
}

// GetMetrics returns the current health metrics for a key.
func (t *KeyHealthTracker) GetMetrics(keyID kernel.ProviderKeyID) KeyMetrics {
	t.mu.RLock()
	h, ok := t.health[keyID]
	t.mu.RUnlock()
	if !ok {
		return KeyMetrics{Uptime: 1.0}
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.pruneHistory()

	var successes int
	for _, o := range h.history {
		if o.success {
			successes++
		}
	}

	total := len(h.history)
	uptime := 1.0
	if total > 0 {
		uptime = float64(successes) / float64(total)
	}

	return KeyMetrics{
		Uptime:                 uptime,
		TotalRequests:          total,
		ConsecutiveErrors:      h.consecutiveErrors,
		PermanentlyBlacklisted: h.permanentlyBlacklisted,
	}
}

// UptimePenalty returns a routing penalty for the key.
// 0 = no penalty (uptime >= 95%), exponentially increasing as uptime drops.
// Returns +Inf for permanently blacklisted keys.
func (t *KeyHealthTracker) UptimePenalty(keyID kernel.ProviderKeyID) float64 {
	metrics := t.GetMetrics(keyID)
	if metrics.PermanentlyBlacklisted {
		return math.Inf(1)
	}
	if metrics.Uptime >= healthUptimeThreshold {
		return 0
	}
	gap := healthUptimeThreshold - metrics.Uptime
	return math.Exp(gap*10) - 1
}

// pruneHistory removes entries outside the metrics window. Must hold h.mu.
func (h *keyHealth) pruneHistory() {
	cutoff := time.Now().Add(-healthMetricsWindow)
	i := 0
	for _, o := range h.history {
		if o.timestamp.After(cutoff) {
			h.history[i] = o
			i++
		}
	}
	h.history = h.history[:i]
}
