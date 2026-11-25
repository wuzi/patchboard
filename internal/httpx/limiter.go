package httpx

import (
	"net/http"
	"sync"
	"time"

	"patchboard/internal/clock"
)

// Rule is a fixed-window rate limit.
type Rule struct {
	Name   string
	Limit  int
	Window time.Duration
}

type windowKey struct {
	rule  string
	key   string
	start int64
}

// Limiter counts calls per (rule, key) in fixed windows. In-memory: one instance per process.
type Limiter struct {
	mu     sync.Mutex
	clock  clock.Clock
	counts map[windowKey]int
	calls  int
}

func NewLimiter(c clock.Clock) *Limiter {
	return &Limiter{clock: c, counts: map[windowKey]int{}}
}

// Allow records a call and reports whether it is within the rule's limit.
func (l *Limiter) Allow(rule Rule, key string) (bool, time.Duration) {
	now := l.clock.Now()
	start := now.Truncate(rule.Window)
	k := windowKey{rule.Name, key, start.UnixNano()}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls++
	if l.calls%1024 == 0 {
		l.sweep(now)
	}
	if l.counts[k] >= rule.Limit {
		return false, start.Add(rule.Window).Sub(now)
	}
	l.counts[k]++
	return true, 0
}

// sweep drops windows that ended more than a day ago (longest rule window is 1h).
func (l *Limiter) sweep(now time.Time) {
	cutoff := now.Add(-24 * time.Hour).UnixNano()
	for k := range l.counts {
		if k.start < cutoff {
			delete(l.counts, k)
		}
	}
}

// RateLimited is the 429 error for a denied call.
func RateLimited(retry time.Duration) *Error {
	e := E(http.StatusTooManyRequests, "rate_limited", "too many requests, slow down")
	e.RetryAfter = retry
	return e
}

// Exceeded reports whether the rule's limit is already reached for key, without recording a call.
func (l *Limiter) Exceeded(rule Rule, key string) (bool, time.Duration) {
	now := l.clock.Now()
	start := now.Truncate(rule.Window)
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.counts[windowKey{rule.Name, key, start.UnixNano()}] >= rule.Limit {
		return true, start.Add(rule.Window).Sub(now)
	}
	return false, 0
}

// Refund gives back one call recorded by Allow in the current window (e.g. a login that succeeded).
func (l *Limiter) Refund(rule Rule, key string) {
	k := windowKey{rule.Name, key, l.clock.Now().Truncate(rule.Window).UnixNano()}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.counts[k] > 0 {
		l.counts[k]--
	}
}
