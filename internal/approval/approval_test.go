package approval

import (
	"context"
	"testing"
	"time"
)

func TestHashAction_Deterministic(t *testing.T) {
	a := Action{SkillName: "web-search", SkillVer: "0.1.0", ActionType: "network", Target: "api.brave.com:443", Payload: "query=test"}
	h1 := HashAction(a)
	h2 := HashAction(a)
	if h1 != h2 {
		t.Fatalf("hash not deterministic: %s != %s", h1, h2)
	}
}

func TestHashAction_DifferentActions(t *testing.T) {
	a1 := Action{SkillName: "web-search", SkillVer: "0.1.0", ActionType: "network", Target: "api.brave.com:443", Payload: "query=test"}
	a2 := Action{SkillName: "web-search", SkillVer: "0.1.0", ActionType: "network", Target: "evil.com:443", Payload: "query=test"}
	if HashAction(a1) == HashAction(a2) {
		t.Fatal("different actions should produce different hashes")
	}
}

func TestMemoryEngine_AutoApprove(t *testing.T) {
	engine := NewMemoryEngine(5*time.Minute, true)
	ctx := context.Background()

	action := Action{SkillName: "test", SkillVer: "1.0", ActionType: "network", Target: "example.com:443", Payload: "data"}
	decision, err := engine.RequestApproval(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.Allowed {
		t.Fatal("auto-approve should allow")
	}
	if decision.Hash == "" {
		t.Fatal("decision should have hash")
	}
}

func TestMemoryEngine_NoAutoApprove(t *testing.T) {
	engine := NewMemoryEngine(5*time.Minute, false)
	ctx := context.Background()

	action := Action{SkillName: "test", SkillVer: "1.0", ActionType: "network", Target: "example.com:443", Payload: "data"}
	decision, err := engine.RequestApproval(ctx, action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Allowed {
		t.Fatal("non-auto mode should not auto-approve")
	}
}

func TestMemoryEngine_Enforce_HashMatch(t *testing.T) {
	engine := NewMemoryEngine(5*time.Minute, true)
	ctx := context.Background()

	action := Action{SkillName: "test", SkillVer: "1.0", ActionType: "network", Target: "example.com:443", Payload: "data"}
	decision, _ := engine.RequestApproval(ctx, action)

	// Same action at enforcement time should pass
	err := engine.Enforce(ctx, action, decision)
	if err != nil {
		t.Fatalf("enforce should pass for matching action: %v", err)
	}
}

func TestMemoryEngine_Enforce_HashMismatch_TOCTOU(t *testing.T) {
	engine := NewMemoryEngine(5*time.Minute, true)
	ctx := context.Background()

	// Get approval for benign action
	benign := Action{SkillName: "test", SkillVer: "1.0", ActionType: "network", Target: "example.com:443", Payload: "safe-data"}
	decision, _ := engine.RequestApproval(ctx, benign)

	// Try to enforce with a different (malicious) action
	malicious := Action{SkillName: "test", SkillVer: "1.0", ActionType: "network", Target: "evil.com:443", Payload: "exfiltrate"}
	err := engine.Enforce(ctx, malicious, decision)
	if err == nil {
		t.Fatal("enforce should FAIL for TOCTOU attack (different action than approved)")
	}
}

func TestMemoryEngine_PolicyExpiry(t *testing.T) {
	engine := NewMemoryEngine(1*time.Millisecond, true)
	ctx := context.Background()

	action := Action{SkillName: "test", SkillVer: "1.0", ActionType: "network", Target: "example.com:443", Payload: "data"}
	engine.RequestApproval(ctx, action)

	// Wait for expiry
	time.Sleep(5 * time.Millisecond)

	decision, _ := engine.CheckPolicy(ctx, action)
	if decision.Allowed {
		t.Fatal("expired policy should not allow")
	}
}

func TestMemoryEngine_PolicyScope(t *testing.T) {
	engine := NewMemoryEngine(5*time.Minute, true)
	ctx := context.Background()

	// Approve action for skill A
	actionA := Action{SkillName: "skill-a", SkillVer: "1.0", ActionType: "network", Target: "example.com:443", Payload: "data"}
	engine.RequestApproval(ctx, actionA)

	// Check policy for skill B (same target, different skill)
	actionB := Action{SkillName: "skill-b", SkillVer: "1.0", ActionType: "network", Target: "example.com:443", Payload: "data"}
	decision, _ := engine.CheckPolicy(ctx, actionB)
	if decision.Allowed {
		t.Fatal("approval for skill A should NOT grant access to skill B")
	}
}
