// Package admin implements the admin dashboard HTTP API.
// This is the security control plane for managing tenants, skills,
// approval policies, and viewing audit logs.
package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/opentide/opentide/internal/approval"
	"github.com/opentide/opentide/internal/config"
	"github.com/opentide/opentide/internal/providers"
	"github.com/opentide/opentide/internal/security"
	"github.com/opentide/opentide/internal/security/secrets"
	"github.com/opentide/opentide/internal/skills"
	"github.com/opentide/opentide/internal/tenant"
	oerr "github.com/opentide/opentide/pkg/errors"
)

// Server is the admin dashboard HTTP API server.
type Server struct {
	tenants     tenant.Store
	skills      skills.Engine
	approvals   *approval.MemoryEngine
	rateLimiter *security.RateLimiter
	registry    *providers.Registry
	secrets     secrets.Store
	config      *config.Config
	logger      *slog.Logger
	mux         *http.ServeMux
	startTime   time.Time
	oauthStates *oauthStateStore
}

// NewServer creates an admin dashboard server.
func NewServer(tenants tenant.Store, skillEngine skills.Engine, approvals *approval.MemoryEngine, rateLimiter *security.RateLimiter, registry *providers.Registry, secretStore secrets.Store, cfg *config.Config, logger *slog.Logger) *Server {
	s := &Server{
		tenants:     tenants,
		skills:      skillEngine,
		approvals:   approvals,
		rateLimiter: rateLimiter,
		registry:    registry,
		secrets:     secretStore,
		config:      cfg,
		logger:      logger,
		mux:         http.NewServeMux(),
		startTime:   time.Now(),
		oauthStates: newOAuthStateStore(),
	}
	s.routes()
	return s
}

// maxRequestBody is the maximum allowed request body size (1MB).
const maxRequestBody = 1 << 20

func (s *Server) routes() {
	// Auth (no middleware)
	s.mux.HandleFunc("POST /admin/api/login", s.handleLogin)
	s.mux.HandleFunc("POST /admin/api/logout", s.handleLogout)
	s.mux.HandleFunc("GET /admin/api/me", s.handleMe)
	s.mux.HandleFunc("GET /admin/api/auth/config", s.handleAuthConfig)

	// Google OAuth (no middleware, redirect-based)
	s.mux.HandleFunc("GET /admin/api/auth/google", s.handleGoogleLogin)
	s.mux.HandleFunc("GET /admin/api/auth/google/callback", s.handleGoogleCallback)

	// Health (no auth)
	s.mux.HandleFunc("GET /admin/health", s.handleHealth)

	// Dashboard (auth required)
	s.mux.HandleFunc("GET /admin/api/status", s.authMiddleware(s.handleStatus))

	// Tenant management (auth required)
	s.mux.HandleFunc("GET /admin/api/tenants", s.authMiddleware(s.handleListTenants))
	s.mux.HandleFunc("POST /admin/api/tenants", s.authMiddleware(s.handleCreateTenant))
	s.mux.HandleFunc("GET /admin/api/tenants/{id}", s.authMiddleware(s.handleGetTenant))
	s.mux.HandleFunc("PUT /admin/api/tenants/{id}", s.authMiddleware(s.handleUpdateTenant))
	s.mux.HandleFunc("DELETE /admin/api/tenants/{id}", s.authMiddleware(s.handleDeleteTenant))

	// Skill management (auth required)
	s.mux.HandleFunc("GET /admin/api/skills", s.authMiddleware(s.handleListSkills))

	// Approval & audit (auth required)
	s.mux.HandleFunc("GET /admin/api/approvals/policies", s.authMiddleware(s.handleListPolicies))
	s.mux.HandleFunc("POST /admin/api/approvals/policies", s.authMiddleware(s.handleCreatePolicy))
	s.mux.HandleFunc("DELETE /admin/api/approvals/policies/{key}", s.authMiddleware(s.handleDeletePolicy))
	s.mux.HandleFunc("GET /admin/api/approvals/audit", s.authMiddleware(s.handleAuditLog))
	s.mux.HandleFunc("POST /admin/api/approvals/audit/{index}/acknowledge", s.authMiddleware(s.handleAcknowledgeAudit))

	// Security (auth required)
	s.mux.HandleFunc("GET /admin/api/security/ratelimit", s.authMiddleware(s.handleRateLimitStatus))

	// Config (auth required)
	s.mux.HandleFunc("GET /admin/api/config", s.authMiddleware(s.handleGetConfig))
	s.mux.HandleFunc("GET /admin/api/config/providers", s.authMiddleware(s.handleProviderStatus))

	// Provider routing (auth required)
	s.mux.HandleFunc("GET /admin/api/providers", s.authMiddleware(s.handleListProviders))
	s.mux.HandleFunc("GET /admin/api/providers/routes", s.authMiddleware(s.handleListRoutes))
	s.mux.HandleFunc("POST /admin/api/providers/routes", s.authMiddleware(s.handleCreateRoute))
	s.mux.HandleFunc("DELETE /admin/api/providers/routes/{index}", s.authMiddleware(s.handleDeleteRoute))
	s.mux.HandleFunc("POST /admin/api/providers/test-route", s.authMiddleware(s.handleTestRoute))

	// Secrets management (auth required)
	s.mux.HandleFunc("GET /admin/api/secrets", s.authMiddleware(s.handleListSecrets))
	s.mux.HandleFunc("POST /admin/api/secrets", s.authMiddleware(s.handleSetSecret))
	s.mux.HandleFunc("DELETE /admin/api/secrets/{provider}", s.authMiddleware(s.handleDeleteSecret))

	// Adapter token management (auth required)
	s.mux.HandleFunc("GET /admin/api/adapters", s.authMiddleware(s.handleListAdapterSecrets))
	s.mux.HandleFunc("POST /admin/api/adapters", s.authMiddleware(s.handleSetAdapterSecret))
	s.mux.HandleFunc("DELETE /admin/api/adapters/{adapter}", s.authMiddleware(s.handleDeleteAdapterSecret))

	// Model management (auth required)
	s.mux.HandleFunc("GET /admin/api/providers/{name}/models", s.authMiddleware(s.handleListModels))
	s.mux.HandleFunc("POST /admin/api/providers/{name}/model", s.authMiddleware(s.handleSetModel))

	// Skill toggle (auth required)
	s.mux.HandleFunc("POST /admin/api/skills/{tool_name}/toggle", s.authMiddleware(s.handleToggleSkill))

	// Serve the React SPA for all other /admin routes
	s.mux.Handle("GET /admin/", spaHandler())
}

// Handler returns the HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.mux
}

// StatusResponse is the dashboard overview.
type StatusResponse struct {
	Version     string    `json:"version"`
	Uptime      string    `json:"uptime"`
	TenantCount int       `json:"tenant_count"`
	SkillCount  int       `json:"skill_count"`
	ServerTime  time.Time `json:"server_time"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tenantCount := 0
	if tenants, err := s.tenants.List(ctx); err == nil {
		tenantCount = len(tenants)
	}

	skillCount := 0
	if s.skills != nil {
		if skills, err := s.skills.ListSkills(ctx); err == nil {
			skillCount = len(skills)
		}
	}

	s.jsonOK(w, StatusResponse{
		Version:     "0.1.0",
		Uptime:      time.Since(s.startTime).Truncate(time.Second).String(),
		TenantCount: tenantCount,
		SkillCount:  skillCount,
		ServerTime:  time.Now().UTC(),
	})
}

func (s *Server) handleListTenants(w http.ResponseWriter, r *http.Request) {
	tenants, err := s.tenants.List(r.Context())
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonOK(w, tenants)
}

func (s *Server) handleGetTenant(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, err := s.tenants.Get(r.Context(), id)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusNotFound)
		return
	}
	s.jsonOK(w, t)
}

func (s *Server) handleCreateTenant(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	var t tenant.Tenant
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if t.ID == "" {
		s.jsonError(w, "id is required", http.StatusBadRequest)
		return
	}
	if t.Plan.Name == "" {
		t.Plan = tenant.FreePlan
	}

	if err := s.tenants.Create(r.Context(), t); err != nil {
		s.jsonError(w, err.Error(), http.StatusConflict)
		return
	}

	s.logger.Info("tenant created", "id", t.ID, "name", t.Name)
	w.WriteHeader(http.StatusCreated)
	s.jsonOK(w, t)
}

func (s *Server) handleUpdateTenant(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	id := r.PathValue("id")
	var t tenant.Tenant
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	t.ID = id

	if err := s.tenants.Update(r.Context(), t); err != nil {
		s.jsonError(w, err.Error(), http.StatusNotFound)
		return
	}

	s.logger.Info("tenant updated", "id", id)
	s.jsonOK(w, t)
}

func (s *Server) handleDeleteTenant(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.tenants.Delete(r.Context(), id); err != nil {
		s.jsonError(w, err.Error(), http.StatusNotFound)
		return
	}
	s.logger.Info("tenant deleted", "id", id)
	s.jsonOK(w, map[string]string{"status": "deleted"})
}

func (s *Server) handleListSkills(w http.ResponseWriter, r *http.Request) {
	if s.skills == nil {
		s.jsonOK(w, []any{})
		return
	}
	list, err := s.skills.ListSkills(r.Context())
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonOK(w, list)
}

func (s *Server) handleListPolicies(w http.ResponseWriter, r *http.Request) {
	if s.approvals == nil {
		s.jsonOK(w, []any{})
		return
	}
	policies := s.approvals.ListPolicies(r.Context())
	s.jsonOK(w, policies)
}

type createPolicyRequest struct {
	SkillName  string `json:"skill_name"`
	ActionType string `json:"action_type"`
	Target     string `json:"target"`
	Allowed    bool   `json:"allowed"`
	Reason     string `json:"reason"`
}

func (s *Server) handleCreatePolicy(w http.ResponseWriter, r *http.Request) {
	if s.approvals == nil {
		s.jsonError(w, "approval engine not configured", http.StatusNotFound)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	var req createPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.SkillName == "" || req.ActionType == "" {
		s.jsonError(w, "skill_name and action_type are required", http.StatusBadRequest)
		return
	}
	scope := approval.ApprovalScope{
		SkillName:  req.SkillName,
		ActionType: req.ActionType,
		Target:     req.Target,
	}
	d := s.approvals.SetPolicy(r.Context(), scope, req.Allowed, req.Reason)
	s.logger.Info("approval policy created", "skill", req.SkillName, "action", req.ActionType, "allowed", req.Allowed)
	w.WriteHeader(http.StatusCreated)
	s.jsonOK(w, d)
}

func (s *Server) handleDeletePolicy(w http.ResponseWriter, r *http.Request) {
	if s.approvals == nil {
		s.jsonError(w, "approval engine not configured", http.StatusNotFound)
		return
	}
	key := r.PathValue("key")
	if !s.approvals.DeletePolicy(r.Context(), key) {
		s.jsonError(w, "policy not found", http.StatusNotFound)
		return
	}
	s.logger.Info("approval policy deleted", "key", key)
	s.jsonOK(w, map[string]string{"status": "deleted"})
}

func (s *Server) handleAuditLog(w http.ResponseWriter, r *http.Request) {
	if s.approvals == nil {
		s.jsonOK(w, []any{})
		return
	}

	q := r.URL.Query()
	offset := 0
	limit := 50
	if v := q.Get("offset"); v != "" {
		fmt.Sscanf(v, "%d", &offset)
	}
	if v := q.Get("limit"); v != "" {
		fmt.Sscanf(v, "%d", &limit)
	}
	if limit > 200 {
		limit = 200
	}

	filter := &approval.AuditFilter{
		SkillName:    q.Get("skill"),
		ActionType:   q.Get("action_type"),
		MismatchOnly: q.Get("mismatch") == "true",
	}

	entries := s.approvals.GetAuditLog(r.Context(), offset, limit, filter)
	s.jsonOK(w, map[string]any{
		"entries":                  entries,
		"total":                   s.approvals.AuditLogLen(),
		"unacknowledged_mismatches": s.approvals.UnacknowledgedMismatches(),
	})
}

func (s *Server) handleAcknowledgeAudit(w http.ResponseWriter, r *http.Request) {
	if s.approvals == nil {
		s.jsonError(w, "approval engine not configured", http.StatusNotFound)
		return
	}
	var index int
	if _, err := fmt.Sscanf(r.PathValue("index"), "%d", &index); err != nil {
		s.jsonError(w, "invalid index", http.StatusBadRequest)
		return
	}
	if !s.approvals.AcknowledgeEntry(r.Context(), index) {
		s.jsonError(w, "entry not found", http.StatusNotFound)
		return
	}
	s.jsonOK(w, map[string]string{"status": "acknowledged"})
}

func (s *Server) handleGetConfig(w http.ResponseWriter, _ *http.Request) {
	// Return config with sensitive fields redacted
	s.jsonOK(w, map[string]any{
		"gateway": map[string]any{
			"host":      s.config.Gateway.Host,
			"port":      s.config.Gateway.Port,
			"log_level": s.config.Gateway.LogLevel,
			"demo_mode": s.config.Gateway.DemoMode,
			"dev_mode":  s.config.Gateway.DevMode,
		},
		"state": map[string]any{
			"driver": s.config.State.Driver,
		},
		"security": map[string]any{
			"max_message_size": s.config.Security.MaxMessageSize,
			"approval_ttl":     s.config.Security.ApprovalTTL,
			"admin_port":       s.config.Security.AdminPort,
			"admin_secret":     "********",
		},
	})
}

func (s *Server) handleProviderStatus(w http.ResponseWriter, _ *http.Request) {
	if s.registry != nil {
		infos := s.registry.List()
		result := make([]map[string]any, 0, len(infos))
		for _, info := range infos {
			result = append(result, map[string]any{
				"name":       info.Name,
				"model":      info.Model,
				"healthy":    info.Healthy,
				"configured": true,
				"is_default": info.Name == s.registry.FallbackName(),
			})
		}
		s.jsonOK(w, result)
		return
	}
	s.jsonOK(w, []any{})
}

func (s *Server) handleListProviders(w http.ResponseWriter, _ *http.Request) {
	if s.registry == nil {
		s.jsonOK(w, []any{})
		return
	}
	infos := s.registry.List()
	result := make([]map[string]any, 0, len(infos))
	for _, info := range infos {
		result = append(result, map[string]any{
			"name":       info.Name,
			"model":      info.Model,
			"healthy":    info.Healthy,
			"is_default": info.Name == s.registry.FallbackName(),
		})
	}
	s.jsonOK(w, result)
}

func (s *Server) handleListRoutes(w http.ResponseWriter, _ *http.Request) {
	if s.registry == nil {
		s.jsonOK(w, []any{})
		return
	}
	s.jsonOK(w, s.registry.Routes())
}

func (s *Server) handleCreateRoute(w http.ResponseWriter, r *http.Request) {
	if s.registry == nil {
		s.jsonError(w, "provider registry not configured", http.StatusNotFound)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	var route providers.Route
	if err := json.NewDecoder(r.Body).Decode(&route); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if route.Provider == "" {
		s.jsonError(w, "provider is required", http.StatusBadRequest)
		return
	}
	if _, ok := s.registry.Get(route.Provider); !ok {
		s.jsonError(w, fmt.Sprintf("unknown provider: %s", route.Provider), http.StatusBadRequest)
		return
	}

	routes := s.registry.Routes()
	routes = append(routes, route)
	s.registry.UpdateRoutes(routes)
	s.logger.Info("route created", "channel", route.ChannelID, "provider", route.Provider, "priority", route.Priority)
	w.WriteHeader(http.StatusCreated)
	s.jsonOK(w, route)
}

func (s *Server) handleDeleteRoute(w http.ResponseWriter, r *http.Request) {
	if s.registry == nil {
		s.jsonError(w, "provider registry not configured", http.StatusNotFound)
		return
	}
	var index int
	if _, err := fmt.Sscanf(r.PathValue("index"), "%d", &index); err != nil {
		s.jsonError(w, "invalid index", http.StatusBadRequest)
		return
	}

	routes := s.registry.Routes()
	if index < 0 || index >= len(routes) {
		s.jsonError(w, "route not found", http.StatusNotFound)
		return
	}

	routes = append(routes[:index], routes[index+1:]...)
	s.registry.UpdateRoutes(routes)
	s.logger.Info("route deleted", "index", index)
	s.jsonOK(w, map[string]string{"status": "deleted"})
}

type testRouteRequest struct {
	UserID    string `json:"user_id"`
	ChannelID string `json:"channel_id"`
}

func (s *Server) handleTestRoute(w http.ResponseWriter, r *http.Request) {
	if s.registry == nil {
		s.jsonError(w, "provider registry not configured", http.StatusNotFound)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	var req testRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	p := s.registry.Resolve(req.UserID, req.ChannelID)
	if p == nil {
		s.jsonOK(w, map[string]any{
			"resolved":  false,
			"provider":  nil,
			"user_id":   req.UserID,
			"channel_id": req.ChannelID,
		})
		return
	}

	overrideName, _, hasOverride := s.registry.GetUserOverride(req.UserID)
	s.jsonOK(w, map[string]any{
		"resolved":      true,
		"provider":      p.Name(),
		"model":         p.ModelID(),
		"user_id":       req.UserID,
		"channel_id":    req.ChannelID,
		"has_override":  hasOverride,
		"override_name": overrideName,
	})
}

func (s *Server) handleRateLimitStatus(w http.ResponseWriter, _ *http.Request) {
	if s.rateLimiter == nil {
		s.jsonOK(w, map[string]string{"status": "not configured"})
		return
	}
	s.jsonOK(w, s.rateLimiter.Stats())
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	s.jsonOK(w, map[string]string{"status": "ok"})
}

func (s *Server) jsonOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// errorResponse is the structured JSON error returned by the admin API.
type errorResponse struct {
	Code    oerr.Code `json:"code"`
	Message string    `json:"message"`
	Fix     string    `json:"fix,omitempty"`
	DocsURL string    `json:"docs_url,omitempty"`
}

func (s *Server) jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(errorResponse{
		Code:    httpStatusToCode(code),
		Message: msg,
	})
}

func httpStatusToCode(status int) oerr.Code {
	switch status {
	case http.StatusUnauthorized:
		return oerr.CodeAdminAuthRequired
	case http.StatusTooManyRequests:
		return oerr.CodeAdminRateLimited
	case http.StatusBadRequest:
		return oerr.CodeAdminBadRequest
	case http.StatusNotFound:
		return oerr.CodeAdminNotFound
	case http.StatusConflict:
		return oerr.CodeAdminConflict
	default:
		return oerr.CodeAdminInternal
	}
}

// contextKey is an unexported type for context keys in this package.
type contextKey string

// TenantIDKey is the context key for the tenant ID.
const TenantIDKey contextKey = "tenant_id"

// TenantFromContext extracts the tenant ID from the context.
func TenantFromContext(ctx context.Context) string {
	v, _ := ctx.Value(TenantIDKey).(string)
	return v
}
