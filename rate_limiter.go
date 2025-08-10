package main

import (
	"sync"
	"time"
)

// RateLimiter ensures that operations are not performed more frequently than a specified interval
type RateLimiter struct {
	interval time.Duration
	last     time.Time
	mutex    sync.Mutex
}

// NewRateLimiter creates a new rate limiter with the specified minimum interval
func NewRateLimiter(interval time.Duration) *RateLimiter {
	return &RateLimiter{
		interval: interval,
		last:     time.Time{}, // Zero time means no previous operation
	}
}

// Wait blocks until enough time has passed since the last operation
func (rl *RateLimiter) Wait() {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	if !rl.last.IsZero() {
		// Calculate how long to wait
		elapsed := now.Sub(rl.last)
		if elapsed < rl.interval {
			waitTime := rl.interval - elapsed
			time.Sleep(waitTime)
			now = time.Now() // Update now after sleeping
		}
	}
	
	rl.last = now
}

// GetInterval returns the configured interval
func (rl *RateLimiter) GetInterval() time.Duration {
	return rl.interval
}
