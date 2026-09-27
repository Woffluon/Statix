package auth

import (
	"fmt"
	"sync"
	"time"
)

type RateLimiter struct {
	mu          sync.Mutex
	entries     map[string]*rlEntry
	limit       int           // max allowed failures before lockout
	window      time.Duration // sliding window size
	lockout     time.Duration // lockout duration
	lastCleanup time.Time
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
		entries:     make(map[string]*rlEntry),
		limit:       limit,
		window:      window,
		lockout:     lockout,
		lastCleanup: time.Now(),
	}
}

// Cleanup removes expired entries from the rate limiter.
func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.cleanupLocked(time.Now())
}

func (rl *RateLimiter) cleanupLocked(now time.Time) {
	for k, e := range rl.entries {
		if !e.lockedAt.IsZero() {
			if now.Sub(e.lockedAt) >= rl.lockout {
				delete(rl.entries, k)
			}
		} else if now.Sub(e.lastFail) >= rl.window {
			delete(rl.entries, k)
		}
	}
	rl.lastCleanup = now
}

// Size returns current number of tracked rate limit entries.
func (rl *RateLimiter) Size() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return len(rl.entries)
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

	// Opportunistic sweep: if entries grow or interval elapsed, clean expired entries
	if len(rl.entries) > 100 || now.Sub(rl.lastCleanup) > 1*time.Minute {
		rl.cleanupLocked(now)
	}

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
