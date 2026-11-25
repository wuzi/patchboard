// Package clock abstracts the current time so tests can control it.
package clock

import (
	"sync"
	"time"
)

// Clock reports the current time.
type Clock interface {
	Now() time.Time
}

// Real is the wall clock (UTC).
type Real struct{}

func (Real) Now() time.Time { return time.Now().UTC() }

// Fake is a manually driven clock for tests.
type Fake struct {
	mu  sync.Mutex
	now time.Time
}

func NewFake(t time.Time) *Fake { return &Fake{now: t.UTC()} }

func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *Fake) Set(t time.Time) {
	f.mu.Lock()
	f.now = t.UTC()
	f.mu.Unlock()
}

func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	f.now = f.now.Add(d)
	f.mu.Unlock()
}
