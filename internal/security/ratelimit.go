package security

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// RateLimiter enforces per-key rate limits using token bucket algorithm.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	config  RateLimitConfig
	done    chan struct{}
}

// RateLimitConfig defines rate limit parameters.
type RateLimitConfig struct {
	MaxTokens  float64       // max tokens per bucket
	RefillRate float64       // tokens added per second
	CleanupAge time.Duration // remove idle buckets after this duration
}

// DefaultRateLimitConfig returns sensible defaults.
// 10 requests per minute per key, burst of 5.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		MaxTokens:  5,
		RefillRate:  10.0 / 60.0, // 10 per minute
		CleanupAge: 10 * time.Minute,
	}
}

type bucket struct {
	tokens   float64
	lastFill time.Time
}

// NewRateLimiter creates an in-memory rate limiter.
// NewRateLimiter creates an in-memory rate limiter.
// Call Close() to stop the background cleanup goroutine.
func NewRateLimiter(cfg RateLimitConfig) *RateLimiter {
	rl := &RateLimiter{
		buckets: make(map[string]*bucket),
		config:  cfg,
		done:    make(chan struct{}),
	}
	go rl.cleanup()
	return rl
}

// Close stops the background cleanup goroutine.
func (rl *RateLimiter) Close() {
	close(rl.done)
}

// Stats returns the current rate limiter statistics.
func (rl *RateLimiter) Stats() RateLimitStats {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return RateLimitStats{
		ActiveBuckets: len(rl.buckets),
		Config:        rl.config,
	}
}

// RateLimitStats holds rate limiter statistics.
type RateLimitStats struct {
	ActiveBuckets int             `json:"active_buckets"`
	Config        RateLimitConfig `json:"config"`
}

// Allow checks whether a request for the given key is allowed.
// Returns true if allowed, false if rate limited.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[key]
	if !ok {
		rl.buckets[key] = &bucket{
			tokens:   rl.config.MaxTokens - 1,
			lastFill: now,
		}
		return true
	}

	// Refill tokens
	elapsed := now.Sub(b.lastFill).Seconds()
	b.tokens += elapsed * rl.config.RefillRate
	if b.tokens > rl.config.MaxTokens {
		b.tokens = rl.config.MaxTokens
	}
	b.lastFill = now

	if b.tokens < 1 {
		return false
	}

	b.tokens--
	return true
}

// RateLimitKey generates a rate limit key from components.
func RateLimitKey(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(parts[0])
	for _, p := range parts[1:] {
		b.WriteByte(':')
		b.WriteString(p)
	}
	return b.String()
}

// RateLimitError is returned when a request is rate limited.
type RateLimitError struct {
	Key string
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited: %s", e.Key)
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.config.CleanupAge)
	defer ticker.Stop()

	for {
		select {
		case <-rl.done:
			return
		case <-ticker.C:
			rl.mu.Lock()
			cutoff := time.Now().Add(-rl.config.CleanupAge)
			for key, b := range rl.buckets {
				if b.lastFill.Before(cutoff) {
					delete(rl.buckets, key)
				}
			}
			rl.mu.Unlock()
		}
	}
}
