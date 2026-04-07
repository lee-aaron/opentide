// Package tenant implements multi-tenant isolation for shared deployments.
// Each tenant gets isolated state, approval policies, skill configurations,
// and rate limits. Tenants are identified by their messaging platform context
// (e.g., Discord guild ID, Slack workspace ID).
package tenant

import (
	"context"
	"fmt"
	"slices"
	"sync"
)

// Tenant represents an isolated organizational unit.
type Tenant struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Platform    string            `json:"platform"`     // "discord", "slack"
	PlatformID  string            `json:"platform_id"`  // guild ID, workspace ID
	Plan        Plan              `json:"plan"`
	Settings    Settings          `json:"settings"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Plan defines resource limits for a tenant.
type Plan struct {
	Name             string  `json:"name"`              // "free", "pro", "enterprise"
	MaxSkills        int     `json:"max_skills"`
	MaxUsersPerDay   int     `json:"max_users_per_day"`
	MaxMsgsPerMinute float64 `json:"max_msgs_per_minute"`
	MaxMsgLength     int     `json:"max_msg_length"`
}

// Settings holds tenant-specific configuration.
type Settings struct {
	DefaultProvider string   `json:"default_provider,omitempty"`
	AllowedSkills   []string `json:"allowed_skills,omitempty"` // empty = all allowed
	BlockedSkills   []string `json:"blocked_skills,omitempty"`
	SystemPrompt    string   `json:"system_prompt,omitempty"`
	AutoApprove     bool     `json:"auto_approve"`
}

// FreePlan is the default plan for new tenants.
var FreePlan = Plan{
	Name:             "free",
	MaxSkills:        5,
	MaxUsersPerDay:   50,
	MaxMsgsPerMinute: 10,
	MaxMsgLength:     4096,
}

// ProPlan is for paying teams.
var ProPlan = Plan{
	Name:             "pro",
	MaxSkills:        25,
	MaxUsersPerDay:   500,
	MaxMsgsPerMinute: 30,
	MaxMsgLength:     16384,
}

// Store manages tenant persistence.
type Store interface {
	GetByPlatform(ctx context.Context, platform, platformID string) (*Tenant, error)
	Get(ctx context.Context, id string) (*Tenant, error)
	Create(ctx context.Context, t Tenant) error
	Update(ctx context.Context, t Tenant) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]Tenant, error)
}

// MemoryStore is an in-memory tenant store for dev/demo.
type MemoryStore struct {
	mu      sync.RWMutex
	tenants map[string]Tenant // id -> tenant
	byPlatform map[string]string // "platform:platformID" -> id
}

// NewMemoryStore creates an in-memory tenant store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		tenants:    make(map[string]Tenant),
		byPlatform: make(map[string]string),
	}
}

func platformKey(platform, platformID string) string {
	return platform + ":" + platformID
}

func (s *MemoryStore) GetByPlatform(_ context.Context, platform, platformID string) (*Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, ok := s.byPlatform[platformKey(platform, platformID)]
	if !ok {
		return nil, fmt.Errorf("tenant not found for %s:%s", platform, platformID)
	}
	t := s.tenants[id]
	return &t, nil
}

func (s *MemoryStore) Get(_ context.Context, id string) (*Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.tenants[id]
	if !ok {
		return nil, fmt.Errorf("tenant not found: %s", id)
	}
	return &t, nil
}

func (s *MemoryStore) Create(_ context.Context, t Tenant) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tenants[t.ID]; exists {
		return fmt.Errorf("tenant already exists: %s", t.ID)
	}

	s.tenants[t.ID] = t
	if t.PlatformID != "" {
		s.byPlatform[platformKey(t.Platform, t.PlatformID)] = t.ID
	}
	return nil
}

func (s *MemoryStore) Update(_ context.Context, t Tenant) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tenants[t.ID]; !exists {
		return fmt.Errorf("tenant not found: %s", t.ID)
	}

	s.tenants[t.ID] = t
	if t.PlatformID != "" {
		s.byPlatform[platformKey(t.Platform, t.PlatformID)] = t.ID
	}
	return nil
}

func (s *MemoryStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.tenants[id]
	if !ok {
		return fmt.Errorf("tenant not found: %s", id)
	}

	if t.PlatformID != "" {
		delete(s.byPlatform, platformKey(t.Platform, t.PlatformID))
	}
	delete(s.tenants, id)
	return nil
}

func (s *MemoryStore) List(_ context.Context) ([]Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Tenant, 0, len(s.tenants))
	for _, t := range s.tenants {
		result = append(result, t)
	}
	return result, nil
}

// IsSkillAllowed checks if a skill is allowed for a tenant.
func IsSkillAllowed(t *Tenant, skillName string) bool {
	if slices.Contains(t.Settings.BlockedSkills, skillName) {
		return false
	}
	if len(t.Settings.AllowedSkills) == 0 {
		return true
	}
	return slices.Contains(t.Settings.AllowedSkills, skillName)
}
