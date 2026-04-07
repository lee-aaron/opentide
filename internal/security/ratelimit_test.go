package security

import (
	"testing"
	"time"
)

func TestRateLimiterAllow(t *testing.T) {
	cfg := RateLimitConfig{
		MaxTokens:  3,
		RefillRate: 1.0, // 1 per second
		CleanupAge: time.Minute,
	}
	rl := NewRateLimiter(cfg)

	// First 3 requests should be allowed (burst)
	for i := 0; i < 3; i++ {
		if !rl.Allow("user1") {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 4th should be rate limited
	if rl.Allow("user1") {
		t.Error("4th request should be rate limited")
	}
}

func TestRateLimiterRefill(t *testing.T) {
	cfg := RateLimitConfig{
		MaxTokens:  2,
		RefillRate: 100.0, // fast refill for testing
		CleanupAge: time.Minute,
	}
	rl := NewRateLimiter(cfg)

	// Exhaust tokens
	rl.Allow("user1")
	rl.Allow("user1")
	if rl.Allow("user1") {
		t.Error("should be rate limited")
	}

	// Wait for refill
	time.Sleep(50 * time.Millisecond)

	// Should be allowed again
	if !rl.Allow("user1") {
		t.Error("should be allowed after refill")
	}
}

func TestRateLimiterSeparateKeys(t *testing.T) {
	cfg := RateLimitConfig{
		MaxTokens:  1,
		RefillRate: 0.01,
		CleanupAge: time.Minute,
	}
	rl := NewRateLimiter(cfg)

	// User1 exhausts their limit
	rl.Allow("user1")
	if rl.Allow("user1") {
		t.Error("user1 should be rate limited")
	}

	// User2 should still be allowed
	if !rl.Allow("user2") {
		t.Error("user2 should be allowed")
	}
}

func TestRateLimitKey(t *testing.T) {
	key := RateLimitKey("user", "123", "chat")
	if key != "user:123:chat" {
		t.Errorf("key = %q, want user:123:chat", key)
	}
}

func TestRateLimitError(t *testing.T) {
	err := &RateLimitError{Key: "user:123"}
	if err.Error() != "rate limited: user:123" {
		t.Errorf("error = %q", err.Error())
	}
}
