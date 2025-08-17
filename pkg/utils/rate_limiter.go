package utils

import (
	"sync"
	"time"
)


type RateLimiter struct {
	mu           sync.Mutex
	requestTimes []time.Time
	maxRequests  int
	window       time.Duration
	minDelay     time.Duration
}


func NewRateLimiter(maxRequests int, window time.Duration, minDelay time.Duration) *RateLimiter {
	return &RateLimiter{
		requestTimes: make([]time.Time, 0, maxRequests),
		maxRequests:  maxRequests,
		window:       window,
		minDelay:     minDelay,
	}
}


func (r *RateLimiter) Wait() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.enforceMinDelay()
	r.cleanupOldRequests(now)

	if len(r.requestTimes) >= r.maxRequests {
		now = r.waitForOldestToExpire(now)
		r.cleanupOldRequests(now)
	}

	r.requestTimes = append(r.requestTimes, now)
}

func (r *RateLimiter) enforceMinDelay() time.Time {
	now := time.Now()
	if len(r.requestTimes) > 0 {
		lastRequest := r.requestTimes[len(r.requestTimes)-1]
		timeSinceLast := now.Sub(lastRequest)
		if timeSinceLast < r.minDelay {
			time.Sleep(r.minDelay - timeSinceLast)
			now = time.Now()
		}
	}
	return now
}

func (r *RateLimiter) cleanupOldRequests(now time.Time) {
	cutoff := now.Add(-r.window)
	validRequests := make([]time.Time, 0, r.maxRequests)
	for _, t := range r.requestTimes {
		if t.After(cutoff) {
			validRequests = append(validRequests, t)
		}
	}
	r.requestTimes = validRequests
}

func (r *RateLimiter) waitForOldestToExpire(now time.Time) time.Time {
	oldestRequest := r.requestTimes[0]
	waitTime := r.window - now.Sub(oldestRequest)
	if waitTime > 0 {
		time.Sleep(waitTime + time.Millisecond*100)
		now = time.Now()
	}
	return now
}




func TraktRateLimiter() *RateLimiter {
	return NewRateLimiter(800, 5*time.Minute, 400*time.Millisecond)
}
