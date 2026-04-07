package providers_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/opentide/opentide/internal/providers"
)

// mockProvider implements providers.Provider for testing.
type mockProvider struct {
	name  string
	model string
}

func (m *mockProvider) Chat(_ context.Context, _ []providers.ChatMessage, _ []providers.Tool) (*providers.Response, error) {
	return &providers.Response{Content: "mock", Model: m.model}, nil
}

func (m *mockProvider) StreamChat(_ context.Context, _ []providers.ChatMessage, _ []providers.Tool) (<-chan providers.StreamEvent, error) {
	ch := make(chan providers.StreamEvent, 1)
	ch <- providers.StreamEvent{Done: true}
	close(ch)
	return ch, nil
}

func (m *mockProvider) ModelID() string { return m.model }
func (m *mockProvider) Name() string    { return m.name }

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestRegistryResolve_DefaultFallback(t *testing.T) {
	reg := providers.NewRegistry("anthropic", nil, testLogger())
	reg.Register("anthropic", &mockProvider{name: "anthropic", model: "claude"})
	reg.Register("openai", &mockProvider{name: "openai", model: "gpt-4o"})

	p := reg.Resolve("user1", "general")
	if p == nil {
		t.Fatal("expected a provider, got nil")
	}
	if p.Name() != "anthropic" {
		t.Errorf("expected anthropic, got %s", p.Name())
	}
}

func TestRegistryResolve_ChannelRoute(t *testing.T) {
	routes := []providers.Route{
		{ChannelID: "engineering", Provider: "anthropic", Priority: 10},
		{ChannelID: "*", Provider: "openai", Priority: 1},
	}
	reg := providers.NewRegistry("openai", routes, testLogger())
	reg.Register("anthropic", &mockProvider{name: "anthropic", model: "claude"})
	reg.Register("openai", &mockProvider{name: "openai", model: "gpt-4o"})

	// Specific channel route
	p := reg.Resolve("user1", "engineering")
	if p.Name() != "anthropic" {
		t.Errorf("expected anthropic for #engineering, got %s", p.Name())
	}

	// Wildcard route
	p = reg.Resolve("user1", "random")
	if p.Name() != "openai" {
		t.Errorf("expected openai for #random, got %s", p.Name())
	}
}

func TestRegistryResolve_UserOverride(t *testing.T) {
	reg := providers.NewRegistry("anthropic", nil, testLogger())
	reg.Register("anthropic", &mockProvider{name: "anthropic", model: "claude"})
	reg.Register("openai", &mockProvider{name: "openai", model: "gpt-4o"})

	// Set override
	ok := reg.SetUserOverride("user1", "openai", "gpt-4o")
	if !ok {
		t.Fatal("expected SetUserOverride to succeed")
	}

	// Override takes precedence
	p := reg.Resolve("user1", "general")
	if p.Name() != "openai" {
		t.Errorf("expected openai via override, got %s", p.Name())
	}

	// Other users unaffected
	p = reg.Resolve("user2", "general")
	if p.Name() != "anthropic" {
		t.Errorf("expected anthropic for user2, got %s", p.Name())
	}

	// Clear override
	reg.ClearUserOverride("user1")
	p = reg.Resolve("user1", "general")
	if p.Name() != "anthropic" {
		t.Errorf("expected anthropic after clearing override, got %s", p.Name())
	}
}

func TestRegistryResolve_OverrideTakesPrecedenceOverRoute(t *testing.T) {
	routes := []providers.Route{
		{ChannelID: "engineering", Provider: "anthropic", Priority: 10},
	}
	reg := providers.NewRegistry("anthropic", routes, testLogger())
	reg.Register("anthropic", &mockProvider{name: "anthropic", model: "claude"})
	reg.Register("openai", &mockProvider{name: "openai", model: "gpt-4o"})

	reg.SetUserOverride("user1", "openai", "gpt-4o")

	// User override beats channel route
	p := reg.Resolve("user1", "engineering")
	if p.Name() != "openai" {
		t.Errorf("expected openai via override, got %s", p.Name())
	}
}

func TestRegistryResolve_InvalidOverride(t *testing.T) {
	reg := providers.NewRegistry("anthropic", nil, testLogger())
	reg.Register("anthropic", &mockProvider{name: "anthropic", model: "claude"})

	ok := reg.SetUserOverride("user1", "nonexistent", "")
	if ok {
		t.Error("expected SetUserOverride to fail for nonexistent provider")
	}
}

func TestRegistryResolve_PriorityOrdering(t *testing.T) {
	// Lower priority number should lose to higher
	routes := []providers.Route{
		{ChannelID: "*", Provider: "openai", Priority: 1},
		{ChannelID: "*", Provider: "anthropic", Priority: 10},
	}
	reg := providers.NewRegistry("openai", routes, testLogger())
	reg.Register("anthropic", &mockProvider{name: "anthropic", model: "claude"})
	reg.Register("openai", &mockProvider{name: "openai", model: "gpt-4o"})

	p := reg.Resolve("user1", "general")
	if p.Name() != "anthropic" {
		t.Errorf("expected anthropic (priority 10), got %s", p.Name())
	}
}

func TestRegistryList(t *testing.T) {
	reg := providers.NewRegistry("anthropic", nil, testLogger())
	reg.Register("anthropic", &mockProvider{name: "anthropic", model: "claude"})
	reg.Register("openai", &mockProvider{name: "openai", model: "gpt-4o"})

	infos := reg.List()
	if len(infos) != 2 {
		t.Errorf("expected 2 providers, got %d", len(infos))
	}
}

func TestRegistryUpdateRoutes(t *testing.T) {
	reg := providers.NewRegistry("anthropic", nil, testLogger())
	reg.Register("anthropic", &mockProvider{name: "anthropic", model: "claude"})
	reg.Register("openai", &mockProvider{name: "openai", model: "gpt-4o"})

	// Initially no routes, falls back to default
	p := reg.Resolve("user1", "engineering")
	if p.Name() != "anthropic" {
		t.Errorf("expected anthropic default, got %s", p.Name())
	}

	// Add a route
	reg.UpdateRoutes([]providers.Route{
		{ChannelID: "engineering", Provider: "openai", Priority: 10},
	})

	p = reg.Resolve("user1", "engineering")
	if p.Name() != "openai" {
		t.Errorf("expected openai after route update, got %s", p.Name())
	}
}

func TestRegistryCleanupOverrides(t *testing.T) {
	reg := providers.NewRegistry("anthropic", nil, testLogger())
	reg.Register("anthropic", &mockProvider{name: "anthropic", model: "claude"})
	reg.Register("openai", &mockProvider{name: "openai", model: "gpt-4o"})

	// Set override then check it's returned
	reg.SetUserOverride("user1", "openai", "gpt-4o")
	_, _, ok := reg.GetUserOverride("user1")
	if !ok {
		t.Fatal("expected override to exist")
	}

	// Cleanup shouldn't remove non-expired
	reg.CleanupOverrides()
	_, _, ok = reg.GetUserOverride("user1")
	if !ok {
		t.Fatal("expected override to survive cleanup")
	}
}

func TestRegistryStartCleanup(t *testing.T) {
	reg := providers.NewRegistry("anthropic", nil, testLogger())
	reg.Register("anthropic", &mockProvider{name: "anthropic", model: "claude"})

	ctx, cancel := context.WithCancel(context.Background())
	reg.StartCleanup(ctx)

	// Just verify it doesn't panic and can be cancelled
	time.Sleep(10 * time.Millisecond)
	cancel()
}

func TestRegistryGetUserOverride(t *testing.T) {
	reg := providers.NewRegistry("anthropic", nil, testLogger())
	reg.Register("anthropic", &mockProvider{name: "anthropic", model: "claude"})
	reg.Register("openai", &mockProvider{name: "openai", model: "gpt-4o"})

	// No override
	_, _, ok := reg.GetUserOverride("user1")
	if ok {
		t.Error("expected no override for user1")
	}

	// Set and get
	reg.SetUserOverride("user1", "openai", "gpt-4o")
	name, model, ok := reg.GetUserOverride("user1")
	if !ok {
		t.Fatal("expected override to exist")
	}
	if name != "openai" || model != "gpt-4o" {
		t.Errorf("expected openai/gpt-4o, got %s/%s", name, model)
	}
}
