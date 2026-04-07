package security

import (
	"fmt"
	"testing"
	"time"
)

func BenchmarkRateLimiterAllow(b *testing.B) {
	cfg := RateLimitConfig{
		MaxTokens:  1000,
		RefillRate: 100,
		CleanupAge: time.Hour,
	}
	rl := NewRateLimiter(cfg)

	b.ResetTimer()
	for b.Loop() {
		rl.Allow("user-1")
	}
}

func BenchmarkRateLimiterManyConcurrent(b *testing.B) {
	cfg := RateLimitConfig{
		MaxTokens:  1000,
		RefillRate: 100,
		CleanupAge: time.Hour,
	}
	rl := NewRateLimiter(cfg)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			rl.Allow(fmt.Sprintf("user-%d", i%100))
			i++
		}
	})
}
