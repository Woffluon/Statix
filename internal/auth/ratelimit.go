package auth

import (
	"fmt"
	"sync"
	"time"
)

type RateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rlEntry
	limit   int           // max allowed failures before lockout
	window  time.Duration // sliding window size
	lockout time.Duration // lockout duration
}

type rlEntry struct {
	failures int
	lastFail time.Time
	lockedAt time.Time
}

func NewRateLimiter(limit int, window, lockout time.Duration) *RateLimiter {
	if limit <= 0 {
		limit = 5
	}
	if window <= 0 {
		window = 15 * time.Minute
	}
	if lockout <= 0 {
		lockout = 15 * time.Minute
	}

	return &RateLimiter{
		entries: make(map[string]*rlEntry),
		limit:   limit,
		window:  window,
		lockout: lockout,
	}
}

func (rl *RateLimiter) key(ip, username string) string {
	return fmt.Sprintf("%s:%s", ip, username)
}

func (rl *RateLimiter) Allow(ip, username string) (bool, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	k := rl.key(ip, username)
	e, ok := rl.entries[k]
	if !ok {
		return true, 0
	}

	now := time.Now()

	// Check if currently locked
	if !e.lockedAt.IsZero() {
		elapsed := now.Sub(e.lockedAt)
		if elapsed < rl.lockout {
			return false, rl.lockout - elapsed
		}
		// Lockout expired -> reset entry
		delete(rl.entries, k)
		return true, 0
	}

	// Check if window expired
	if now.Sub(e.lastFail) > rl.window {
		delete(rl.entries, k)
		return true, 0
	}

	if e.failures >= rl.limit {
		e.lockedAt = now
		return false, rl.lockout
	}

	return true, 0
}

func (rl *RateLimiter) Record(ip, username string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	k := rl.key(ip, username)
	now := time.Now()

	e, ok := rl.entries[k]
	if !ok || now.Sub(e.lastFail) > rl.window {
		e = &rlEntry{
			failures: 1,
			lastFail: now,
		}
		rl.entries[k] = e
	} else {
		e.failures++
		e.lastFail = now
	}

	if e.failures >= rl.limit {
		e.lockedAt = now
	}
}

func (rl *RateLimiter) Reset(ip, username string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.entries, rl.key(ip, username))
}
