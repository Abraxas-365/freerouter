package server

import (
	"sync"
	"time"
)

// failureLimiter throttles guesses of a shared secret (the admin token):
// after max failures from one client within window, every attempt from it
// is refused until the window has passed since its first failure. Success
// does not reset the count (an attacker holding the token gains nothing).
// In-process on purpose: token mode is one instance with one admin.
type failureLimiter struct {
	max    int
	window time.Duration
	now    func() time.Time

	mu      sync.Mutex
	clients map[string]*failures
}

type failures struct {
	count int
	since time.Time
}

// maxTrackedClients bounds memory under a spray of source addresses; when
// full, expired entries are dropped and, failing that, the table is reset
// (a flood then costs the attacker the throttle, never the admin a lockout).
const maxTrackedClients = 10000

func newFailureLimiter(max int, window time.Duration) *failureLimiter {
	return &failureLimiter{max: max, window: window, now: time.Now, clients: map[string]*failures{}}
}

// blocked reports whether client is over the limit, and for how long.
func (l *failureLimiter) blocked(client string) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	f := l.clients[client]
	if f == nil {
		return 0, false
	}
	left := l.window - l.now().Sub(f.since)
	if left <= 0 {
		delete(l.clients, client)
		return 0, false
	}
	return left, f.count >= l.max
}

// fail records a failed attempt by client.
func (l *failureLimiter) fail(client string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	f := l.clients[client]
	if f == nil || now.Sub(f.since) >= l.window {
		if len(l.clients) >= maxTrackedClients {
			l.sweep(now)
		}
		l.clients[client] = &failures{count: 1, since: now}
		return
	}
	f.count++
}

func (l *failureLimiter) sweep(now time.Time) {
	for k, f := range l.clients {
		if now.Sub(f.since) >= l.window {
			delete(l.clients, k)
		}
	}
	if len(l.clients) >= maxTrackedClients {
		l.clients = map[string]*failures{}
	}
}
