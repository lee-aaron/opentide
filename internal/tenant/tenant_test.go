package tenant

import (
	"context"
	"testing"
)

func TestMemoryStoreCRUD(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	tenant := Tenant{
		ID:         "t1",
		Name:       "Test Team",
		Platform:   "discord",
		PlatformID: "guild-123",
		Plan:       FreePlan,
	}

	// Create
	if err := s.Create(ctx, tenant); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Duplicate create should fail
	if err := s.Create(ctx, tenant); err == nil {
		t.Fatal("expected error on duplicate create")
	}

	// Get by ID
	got, err := s.Get(ctx, "t1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Name != "Test Team" {
		t.Errorf("name = %q", got.Name)
	}

	// Get by platform
	got, err = s.GetByPlatform(ctx, "discord", "guild-123")
	if err != nil {
		t.Fatalf("GetByPlatform failed: %v", err)
	}
	if got.ID != "t1" {
		t.Errorf("id = %q", got.ID)
	}

	// Update
	tenant.Name = "Updated Team"
	if err := s.Update(ctx, tenant); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	got, _ = s.Get(ctx, "t1")
	if got.Name != "Updated Team" {
		t.Errorf("name after update = %q", got.Name)
	}

	// List
	all, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("expected 1 tenant, got %d", len(all))
	}

	// Delete
	if err := s.Delete(ctx, "t1"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := s.Get(ctx, "t1"); err == nil {
		t.Error("expected error after delete")
	}
}

func TestGetByPlatformNotFound(t *testing.T) {
	s := NewMemoryStore()
	_, err := s.GetByPlatform(context.Background(), "slack", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent platform")
	}
}

func TestIsSkillAllowed(t *testing.T) {
	tests := []struct {
		name    string
		tenant  Tenant
		skill   string
		allowed bool
	}{
		{
			name:    "no restrictions",
			tenant:  Tenant{},
			skill:   "web_search",
			allowed: true,
		},
		{
			name: "blocked skill",
			tenant: Tenant{
				Settings: Settings{BlockedSkills: []string{"dangerous_skill"}},
			},
			skill:   "dangerous_skill",
			allowed: false,
		},
		{
			name: "allowed list permits",
			tenant: Tenant{
				Settings: Settings{AllowedSkills: []string{"web_search", "calculator"}},
			},
			skill:   "web_search",
			allowed: true,
		},
		{
			name: "allowed list denies unlisted",
			tenant: Tenant{
				Settings: Settings{AllowedSkills: []string{"web_search"}},
			},
			skill:   "file_manager",
			allowed: false,
		},
		{
			name: "blocked takes precedence over allowed",
			tenant: Tenant{
				Settings: Settings{
					AllowedSkills: []string{"evil_skill"},
					BlockedSkills: []string{"evil_skill"},
				},
			},
			skill:   "evil_skill",
			allowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSkillAllowed(&tt.tenant, tt.skill)
			if got != tt.allowed {
				t.Errorf("IsSkillAllowed = %v, want %v", got, tt.allowed)
			}
		})
	}
}

func TestPlans(t *testing.T) {
	if FreePlan.MaxSkills != 5 {
		t.Errorf("FreePlan.MaxSkills = %d", FreePlan.MaxSkills)
	}
	if ProPlan.MaxSkills != 25 {
		t.Errorf("ProPlan.MaxSkills = %d", ProPlan.MaxSkills)
	}
}
