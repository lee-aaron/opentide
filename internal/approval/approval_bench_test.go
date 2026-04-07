package approval

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func BenchmarkHashAction(b *testing.B) {
	a := Action{
		SkillName:  "web-search",
		SkillVer:   "0.1.0",
		ActionType: "network",
		Target:     "api.brave.com:443",
		Payload:    `{"query":"test search","count":5}`,
	}

	b.ResetTimer()
	for b.Loop() {
		HashAction(a)
	}
}

func BenchmarkRequestApproval(b *testing.B) {
	engine := NewMemoryEngine(5*time.Minute, true) // auto-approve
	ctx := context.Background()
	a := Action{
		SkillName:  "web-search",
		SkillVer:   "0.1.0",
		ActionType: "network",
		Target:     "api.brave.com:443",
		Payload:    `{"query":"benchmark"}`,
	}

	b.ResetTimer()
	for b.Loop() {
		engine.RequestApproval(ctx, a)
	}
}

func BenchmarkEnforce(b *testing.B) {
	engine := NewMemoryEngine(5*time.Minute, true)
	ctx := context.Background()
	a := Action{
		SkillName:  "web-search",
		SkillVer:   "0.1.0",
		ActionType: "network",
		Target:     "api.brave.com:443",
		Payload:    `{"query":"benchmark"}`,
	}
	decision, _ := engine.RequestApproval(ctx, a)

	b.ResetTimer()
	for b.Loop() {
		engine.Enforce(ctx, a, decision)
	}
}

func BenchmarkCheckPolicy(b *testing.B) {
	engine := NewMemoryEngine(5*time.Minute, true)
	ctx := context.Background()

	// Pre-populate policies
	for i := range 100 {
		a := Action{
			SkillName:  fmt.Sprintf("skill-%d", i),
			SkillVer:   "0.1.0",
			ActionType: "network",
			Target:     fmt.Sprintf("api%d.example.com:443", i),
			Payload:    "test",
		}
		engine.RequestApproval(ctx, a)
	}

	a := Action{
		SkillName:  "skill-50",
		SkillVer:   "0.1.0",
		ActionType: "network",
		Target:     "api50.example.com:443",
		Payload:    "test",
	}

	b.ResetTimer()
	for b.Loop() {
		engine.CheckPolicy(ctx, a)
	}
}
