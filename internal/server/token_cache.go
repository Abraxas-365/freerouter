package server

import (
	"sync"
	"time"
)

// tokenEntry holds a cached JWT and its expiry time.
type tokenEntry struct {
	jwt       string
	expiresAt time.Time
}

// tokenCache is a thread-safe in-memory cache that maps service-account
// secrets (ik_svc_...) to their exchanged short-lived JWTs.
//
// Entries are evicted lazily on read (if expired) and proactively by a
// background sweeper that runs every sweepInterval.
type tokenCache struct {
	mu      sync.RWMutex
	entries map[string]tokenEntry

	// margin is subtracted from the real token TTL so that we refresh
	// before the JWT actually expires (avoids serving a near-expiry token).
	margin time.Duration

	stop chan struct{} // closed to stop the sweeper
}

// newTokenCache creates a cache with a safety margin and starts a background
// sweeper that purges expired entries every sweepInterval.
func newTokenCache(margin, sweepInterval time.Duration) *tokenCache {
	c := &tokenCache{
		entries: make(map[string]tokenEntry),
		margin:  margin,
		stop:    make(chan struct{}),
	}
	go c.sweeper(sweepInterval)
	return c
}

// Get returns the cached JWT for key, or "" if missing/expired.
func (c *tokenCache) Get(key string) string {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		return ""
	}
	return e.jwt
}

// Set stores a JWT with the given TTL (seconds). The configured margin is
// subtracted so the cache entry expires before the real JWT does.
func (c *tokenCache) Set(key, jwt string, ttlSeconds int) {
	ttl := time.Duration(ttlSeconds)*time.Second - c.margin
	if ttl <= 0 {
		return // token too short-lived to cache
	}
	c.mu.Lock()
	c.entries[key] = tokenEntry{
		jwt:       jwt,
		expiresAt: time.Now().Add(ttl),
	}
	c.mu.Unlock()
}

// Stop terminates the background sweeper goroutine.
func (c *tokenCache) Stop() {
	select {
	case <-c.stop:
	default:
		close(c.stop)
	}
}

// sweeper periodically removes expired entries.
func (c *tokenCache) sweeper(interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-c.stop:
			return
		case now := <-t.C:
			c.mu.Lock()
			for k, e := range c.entries {
				if now.After(e.expiresAt) {
					delete(c.entries, k)
				}
			}
			c.mu.Unlock()
		}
	}
}
