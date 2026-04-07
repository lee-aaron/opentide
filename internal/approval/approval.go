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
	Timestamp     time.Time `json:"timestamp"`
	Action        Action    `json:"action"`          // action at enforcement time
	ApprovedAction *Action  `json:"approved_action,omitempty"` // action at approval time (for TOCTOU diff)
	Decision      Decision  `json:"decision"`
	ActualHash    string    `json:"actual_hash,omitempty"` // hash at enforcement time
	Acknowledged  bool      `json:"acknowledged,omitempty"`
}

// maxAuditEntries is the cap for the in-memory audit log.
const maxAuditEntries = 10000

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
		// Reconstruct the approved action from the decision scope for diffing
		approvedAction := Action{
			SkillName:  decision.Scope.SkillName,
			ActionType: decision.Scope.ActionType,
			Target:     decision.Scope.Target,
		}
		entry := AuditEntry{
			Timestamp:      time.Now(),
			Action:         action,
			ApprovedAction: &approvedAction,
			Decision:       decision,
			ActualHash:     actualHash,
		}
		e.mu.Lock()
		e.audit = append(e.audit, entry)
		if len(e.audit) > maxAuditEntries {
			e.audit = e.audit[len(e.audit)-maxAuditEntries:]
		}
		e.mu.Unlock()
		return fmt.Errorf("action hash mismatch: approved=%s actual=%s (possible TOCTOU attack)", decision.Hash[:16], actualHash[:16])
	}
	return nil
}

func (e *MemoryEngine) AuditLog(_ context.Context, entry AuditEntry) error {
	e.mu.Lock()
	e.audit = append(e.audit, entry)
	if len(e.audit) > maxAuditEntries {
		// Evict oldest entries to stay within cap
		e.audit = e.audit[len(e.audit)-maxAuditEntries:]
	}
	e.mu.Unlock()
	return nil
}

// AuditFilter controls which audit entries are returned.
type AuditFilter struct {
	SkillName  string
	ActionType string
	Since      time.Time
	Until      time.Time
	MismatchOnly bool
}

// SetPolicy creates or updates an approval policy.
func (e *MemoryEngine) SetPolicy(_ context.Context, scope ApprovalScope, allowed bool, reason string) Decision {
	hash := HashAction(Action{
		SkillName:  scope.SkillName,
		ActionType: scope.ActionType,
		Target:     scope.Target,
	})
	d := Decision{
		Allowed:   allowed,
		Reason:    reason,
		Hash:      hash,
		ExpiresAt: time.Now().Add(e.ttl),
		Scope:     scope,
	}
	e.mu.Lock()
	e.policies[scopeKey(scope)] = d
	e.mu.Unlock()
	return d
}

// DeletePolicy removes an approval policy by scope key.
func (e *MemoryEngine) DeletePolicy(_ context.Context, key string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.policies[key]; !ok {
		return false
	}
	delete(e.policies, key)
	return true
}

// ListPolicies returns all active (non-expired) approval policies.
func (e *MemoryEngine) ListPolicies(_ context.Context) []Decision {
	e.mu.RLock()
	defer e.mu.RUnlock()

	now := time.Now()
	var result []Decision
	for _, d := range e.policies {
		if now.Before(d.ExpiresAt) {
			result = append(result, d)
		}
	}
	return result
}

// GetAuditLog returns a paginated, filtered view of the audit log.
func (e *MemoryEngine) GetAuditLog(_ context.Context, offset, limit int, filter *AuditFilter) []AuditEntry {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var filtered []AuditEntry
	for i := len(e.audit) - 1; i >= 0; i-- {
		entry := e.audit[i]
		if filter != nil {
			if filter.SkillName != "" && entry.Action.SkillName != filter.SkillName {
				continue
			}
			if filter.ActionType != "" && entry.Action.ActionType != filter.ActionType {
				continue
			}
			if !filter.Since.IsZero() && entry.Timestamp.Before(filter.Since) {
				continue
			}
			if !filter.Until.IsZero() && entry.Timestamp.After(filter.Until) {
				continue
			}
			if filter.MismatchOnly && (entry.ActualHash == "" || entry.ActualHash == entry.Decision.Hash) {
				continue
			}
		}
		filtered = append(filtered, entry)
	}

	// Apply pagination
	if offset >= len(filtered) {
		return nil
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[offset:end]
}

// AuditLogLen returns the number of entries in the audit log.
func (e *MemoryEngine) AuditLogLen() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.audit)
}

// AcknowledgeEntry marks an audit entry as acknowledged (for incident tracking).
func (e *MemoryEngine) AcknowledgeEntry(_ context.Context, index int) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if index < 0 || index >= len(e.audit) {
		return false
	}
	e.audit[index].Acknowledged = true
	return true
}

// UnacknowledgedMismatches returns the count of TOCTOU mismatches not yet acknowledged.
func (e *MemoryEngine) UnacknowledgedMismatches() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	count := 0
	for _, entry := range e.audit {
		if entry.ActualHash != "" && entry.ActualHash != entry.Decision.Hash && !entry.Acknowledged {
			count++
		}
	}
	return count
}
