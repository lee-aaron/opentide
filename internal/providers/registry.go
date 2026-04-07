package providers

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"
)

const defaultRouteTTL = 30 * time.Second

// Registry holds all configured LLM providers and handles route resolution.
type Registry struct {
	providers map[string]Provider
	fallback  string // default provider name

	// Route cache: routes are loaded once and cached with TTL.
	routes   []Route
	routeExp time.Time
	routeTTL time.Duration

	// User overrides: userID -> provider override with expiry.
	overrides map[string]*userOverride

	mu     sync.RWMutex
	logger *slog.Logger
}

type userOverride struct {
	providerName string
	model        string
	expiresAt    time.Time
}

// NewRegistry creates a provider registry with the given default and static routes.
func NewRegistry(fallback string, routes []Route, logger *slog.Logger) *Registry {
	// Sort routes by priority descending (higher priority wins).
	sorted := make([]Route, len(routes))
	copy(sorted, routes)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority > sorted[j].Priority
	})

	return &Registry{
		providers: make(map[string]Provider),
		fallback:  fallback,
		routes:    sorted,
		routeExp:  time.Now().Add(defaultRouteTTL),
		routeTTL:  defaultRouteTTL,
		overrides: make(map[string]*userOverride),
		logger:    logger,
	}
}

// Register adds or replaces a provider in the registry.
func (r *Registry) Register(name string, p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = p
}

// Unregister removes a provider from the registry.
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.providers, name)
}

// Get returns a provider by name.
func (r *Registry) Get(name string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	return p, ok
}

// Default returns the fallback provider.
func (r *Registry) Default() Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if p, ok := r.providers[r.fallback]; ok {
		return p
	}
	// Return first available provider if fallback not found.
	for _, p := range r.providers {
		return p
	}
	return nil
}

// List returns info about all registered providers.
func (r *Registry) List() []ProviderInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	infos := make([]ProviderInfo, 0, len(r.providers))
	for name, p := range r.providers {
		infos = append(infos, ProviderInfo{
			Name:    name,
			Model:   p.ModelID(),
			Healthy: true, // health check updates this
		})
	}
	return infos
}

// Resolve picks the best provider for a given user and channel.
// Resolution order: user override > channel route > global default.
func (r *Registry) Resolve(userID, channelID string) Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 1. Check user override (from /model command).
	if ov, ok := r.overrides[userID]; ok {
		if time.Now().Before(ov.expiresAt) {
			if p, ok := r.providers[ov.providerName]; ok {
				r.logger.Debug("provider resolved via user override",
					"user", userID, "provider", ov.providerName)
				return p
			}
		}
		// Expired, clean up (will be reaped by cleanup goroutine too).
		delete(r.overrides, userID)
	}

	// 2. Check channel routes (sorted by priority descending).
	for _, route := range r.routes {
		if routeMatches(route, channelID) {
			if p, ok := r.providers[route.Provider]; ok {
				r.logger.Debug("provider resolved via route",
					"channel", channelID, "provider", route.Provider, "priority", route.Priority)
				return p
			}
		}
	}

	// 3. Global default.
	if p, ok := r.providers[r.fallback]; ok {
		return p
	}

	// 4. Any available provider.
	for _, p := range r.providers {
		return p
	}
	return nil
}

// SetUserOverride stores a per-user provider override with 24h TTL.
func (r *Registry) SetUserOverride(userID, providerName, model string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.providers[providerName]; !ok {
		return false
	}

	r.overrides[userID] = &userOverride{
		providerName: providerName,
		model:        model,
		expiresAt:    time.Now().Add(24 * time.Hour),
	}
	return true
}

// ClearUserOverride removes a user's provider override.
func (r *Registry) ClearUserOverride(userID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.overrides, userID)
}

// GetUserOverride returns the user's current override, if any.
func (r *Registry) GetUserOverride(userID string) (providerName, model string, ok bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ov, exists := r.overrides[userID]
	if !exists || time.Now().After(ov.expiresAt) {
		return "", "", false
	}
	return ov.providerName, ov.model, true
}

// InvalidateRouteCache forces routes to be refreshed on next Resolve call.
func (r *Registry) InvalidateRouteCache() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routeExp = time.Time{} // expired
}

// UpdateRoutes replaces the route table (called by admin CRUD).
func (r *Registry) UpdateRoutes(routes []Route) {
	sorted := make([]Route, len(routes))
	copy(sorted, routes)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority > sorted[j].Priority
	})

	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes = sorted
	r.routeExp = time.Now().Add(r.routeTTL)
}

// Routes returns the current route table.
func (r *Registry) Routes() []Route {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Route, len(r.routes))
	copy(result, r.routes)
	return result
}

// CleanupOverrides removes expired user overrides. Call periodically.
func (r *Registry) CleanupOverrides() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for userID, ov := range r.overrides {
		if now.After(ov.expiresAt) {
			delete(r.overrides, userID)
		}
	}
}

// StartCleanup runs a background goroutine that reaps expired overrides every 5 minutes.
// Cancel the context to stop it.
func (r *Registry) StartCleanup(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.CleanupOverrides()
			}
		}
	}()
}

// FallbackName returns the name of the default/fallback provider.
func (r *Registry) FallbackName() string {
	return r.fallback
}

// SetFallback changes the default provider name.
func (r *Registry) SetFallback(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fallback = name
}

// routeMatches checks if a route applies to a channel.
func routeMatches(route Route, channelID string) bool {
	if route.ChannelID == "*" {
		return true
	}
	return route.ChannelID == channelID
}
