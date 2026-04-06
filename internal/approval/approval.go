// Package approval implements the non-bypassable approval engine.
// Every action that crosses a trust boundary goes through approval.
// Hashes are computed at enforcement time from actual requests, not from
// skill self-reported payloads (closing the TOCTOU window in CVE-2026-29607).
package approval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// Action represents a request from a skill to perform an operation.
type Action struct {
	SkillName  string `json:"skill_name"`
	SkillVer   string `json:"skill_version"`
	ActionType string `json:"action_type"` // "network", "filesystem", "shell"
	Target     string `json:"target"`      // e.g. "api.brave.com:443", "/tmp/output.txt"
	Payload    string `json:"payload"`     // serialized action details
}

// ApprovalScope limits what an approval covers.
type ApprovalScope struct {
	SkillName  string `json:"skill_name"`
	ActionType string `json:"action_type"`
	Target     string `json:"target"`
}

// Decision is the result of an approval request.
type Decision struct {
	Allowed   bool          `json:"allowed"`
	Reason    string        `json:"reason"`
	Hash      string        `json:"hash"` // SHA-256 of the action at approval time
	ExpiresAt time.Time     `json:"expires_at"`
	Scope     ApprovalScope `json:"scope"`
}

// AuditEntry records an approval decision for the audit log.
type AuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Action    Action    `json:"action"`
	Decision  Decision  `json:"decision"`
	ActualHash string   `json:"actual_hash,omitempty"` // hash at enforcement time
}

// Engine is the approval engine interface.
type Engine interface {
	RequestApproval(ctx context.Context, action Action) (Decision, error)
	CheckPolicy(ctx context.Context, action Action) (Decision, error)
	Enforce(ctx context.Context, action Action, decision Decision) error
	AuditLog(ctx context.Context, entry AuditEntry) error
}

// HashAction computes a deterministic SHA-256 hash of an action.
// This is used both at approval time and at enforcement time.
// The enforcement layer calls this on the actual request, not on the skill's
// self-reported payload.
func HashAction(a Action) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%s|%s|%s", a.SkillName, a.SkillVer, a.ActionType, a.Target, a.Payload)
	return hex.EncodeToString(h.Sum(nil))
}

// MemoryEngine is an in-memory approval engine for demo/dev mode.
type MemoryEngine struct {
	mu       sync.RWMutex
	policies map[string]Decision // scope key -> decision
	audit    []AuditEntry
	ttl      time.Duration
	autoApprove bool // for demo mode only
}

// NewMemoryEngine creates an in-memory approval engine.
// If autoApprove is true (demo mode), all actions are auto-approved.
func NewMemoryEngine(ttl time.Duration, autoApprove bool) *MemoryEngine {
	return &MemoryEngine{
		policies:    make(map[string]Decision),
		ttl:         ttl,
		autoApprove: autoApprove,
	}
}

func scopeKey(s ApprovalScope) string {
	return fmt.Sprintf("%s|%s|%s", s.SkillName, s.ActionType, s.Target)
}

func (e *MemoryEngine) RequestApproval(_ context.Context, action Action) (Decision, error) {
	hash := HashAction(action)

	if e.autoApprove {
		d := Decision{
			Allowed:   true,
			Reason:    "auto-approved (demo mode)",
			Hash:      hash,
			ExpiresAt: time.Now().Add(e.ttl),
			Scope: ApprovalScope{
				SkillName:  action.SkillName,
				ActionType: action.ActionType,
				Target:     action.Target,
			},
		}
		e.mu.Lock()
		e.policies[scopeKey(d.Scope)] = d
		e.mu.Unlock()
		return d, nil
	}

	// In non-demo mode, this would prompt the user via the messaging adapter.
	// For now, return a pending decision that requires explicit approval.
	return Decision{
		Allowed: false,
		Reason:  "awaiting user approval",
		Hash:    hash,
		Scope: ApprovalScope{
			SkillName:  action.SkillName,
			ActionType: action.ActionType,
			Target:     action.Target,
		},
	}, nil
}

func (e *MemoryEngine) CheckPolicy(_ context.Context, action Action) (Decision, error) {
	scope := ApprovalScope{
		SkillName:  action.SkillName,
		ActionType: action.ActionType,
		Target:     action.Target,
	}

	e.mu.RLock()
	d, ok := e.policies[scopeKey(scope)]
	e.mu.RUnlock()

	if !ok {
		return Decision{Allowed: false, Reason: "no policy found"}, nil
	}
	if time.Now().After(d.ExpiresAt) {
		return Decision{Allowed: false, Reason: "approval expired"}, nil
	}
	return d, nil
}

// Enforce checks that the actual action at execution time matches what was approved.
// This is the TOCTOU defense: we re-hash the action from the actual request and
// compare it to the hash stored in the decision.
func (e *MemoryEngine) Enforce(_ context.Context, action Action, decision Decision) error {
	actualHash := HashAction(action)
	if actualHash != decision.Hash {
		entry := AuditEntry{
			Timestamp:  time.Now(),
			Action:     action,
			Decision:   decision,
			ActualHash: actualHash,
		}
		e.mu.Lock()
		e.audit = append(e.audit, entry)
		e.mu.Unlock()
		return fmt.Errorf("action hash mismatch: approved=%s actual=%s (possible TOCTOU attack)", decision.Hash[:16], actualHash[:16])
	}
	return nil
}

func (e *MemoryEngine) AuditLog(_ context.Context, entry AuditEntry) error {
	e.mu.Lock()
	e.audit = append(e.audit, entry)
	e.mu.Unlock()
	return nil
}
