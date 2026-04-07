package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"log/slog"

	"github.com/opentide/opentide/internal/approval"
	"github.com/opentide/opentide/internal/config"
	"github.com/opentide/opentide/internal/security"
	"github.com/opentide/opentide/internal/tenant"
)

func testServer() (*Server, *httptest.Server) {
	tenants := tenant.NewMemoryStore()
	approvals := approval.NewMemoryEngine(5*time.Minute, true)
	rateLimiter := security.NewRateLimiter(security.DefaultRateLimitConfig())
	cfg := &config.Config{
		Gateway:  config.GatewayConfig{DemoMode: true},
		Security: config.SecurityConfig{AdminSecret: "test-secret"},
	}
	srv := NewServer(tenants, nil, approvals, rateLimiter, nil, nil, cfg, slog.Default())
	ts := httptest.NewServer(srv.Handler())
	return srv, ts
}

func TestAdminStatus(t *testing.T) {
	_, ts := testServer()
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/admin/api/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var status StatusResponse
	json.NewDecoder(resp.Body).Decode(&status)
	if status.Version != "0.1.0" {
		t.Errorf("version = %q", status.Version)
	}
}

func TestAdminTenantCRUD(t *testing.T) {
	_, ts := testServer()
	defer ts.Close()

	// Create tenant
	body := `{"id":"t1","name":"Test Team","platform":"discord","platform_id":"guild-123"}`
	resp, err := http.Post(ts.URL+"/admin/api/tenants", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("create status = %d, want 201", resp.StatusCode)
	}

	// List tenants
	resp, err = http.Get(ts.URL + "/admin/api/tenants")
	if err != nil {
		t.Fatal(err)
	}
	var tenants []tenant.Tenant
	json.NewDecoder(resp.Body).Decode(&tenants)
	resp.Body.Close()
	if len(tenants) != 1 {
		t.Errorf("expected 1 tenant, got %d", len(tenants))
	}

	// Get tenant
	resp, err = http.Get(ts.URL + "/admin/api/tenants/t1")
	if err != nil {
		t.Fatal(err)
	}
	var got tenant.Tenant
	json.NewDecoder(resp.Body).Decode(&got)
	resp.Body.Close()
	if got.Name != "Test Team" {
		t.Errorf("name = %q", got.Name)
	}

	// Delete tenant
	req, _ := http.NewRequest("DELETE", ts.URL+"/admin/api/tenants/t1", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("delete status = %d", resp.StatusCode)
	}
}

func TestAdminDashboard(t *testing.T) {
	_, ts := testServer()
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/admin/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("content-type = %q", ct)
	}
}

func TestAdminHealth(t *testing.T) {
	_, ts := testServer()
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/admin/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var health map[string]string
	json.NewDecoder(resp.Body).Decode(&health)
	if health["status"] != "ok" {
		t.Errorf("health = %v", health)
	}
}

func TestAdminSkillsEmpty(t *testing.T) {
	_, ts := testServer()
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/admin/api/skills")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var skills []any
	json.NewDecoder(resp.Body).Decode(&skills)
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

// testAuthServer creates a non-demo server where auth is enforced.
func testAuthServer() (*Server, *httptest.Server) {
	tenants := tenant.NewMemoryStore()
	approvals := approval.NewMemoryEngine(5*time.Minute, false)
	rateLimiter := security.NewRateLimiter(security.DefaultRateLimitConfig())
	cfg := &config.Config{
		Gateway:  config.GatewayConfig{DemoMode: false},
		Security: config.SecurityConfig{AdminSecret: "test-secret-key"},
	}
	srv := NewServer(tenants, nil, approvals, rateLimiter, nil, nil, cfg, slog.Default())
	ts := httptest.NewServer(srv.Handler())
	return srv, ts
}

func TestAuthRequired(t *testing.T) {
	_, ts := testAuthServer()
	defer ts.Close()

	// Protected endpoints should return 401 without auth
	endpoints := []string{
		"/admin/api/status",
		"/admin/api/tenants",
		"/admin/api/skills",
		"/admin/api/approvals/policies",
		"/admin/api/approvals/audit",
		"/admin/api/security/ratelimit",
	}
	for _, ep := range endpoints {
		resp, err := http.Get(ts.URL + ep)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401", ep, resp.StatusCode)
		}
	}
}

func TestAuthNoAuthEndpoints(t *testing.T) {
	_, ts := testAuthServer()
	defer ts.Close()

	// Health and dashboard don't require auth
	for _, ep := range []string{"/admin/health", "/admin/"} {
		resp, err := http.Get(ts.URL + ep)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", ep, resp.StatusCode)
		}
	}
}

func TestLoginLogout(t *testing.T) {
	_, ts := testAuthServer()
	defer ts.Close()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Wrong secret → 401
	resp, err := client.Post(ts.URL+"/admin/api/login", "application/json",
		strings.NewReader(`{"secret":"wrong"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("bad login: status = %d, want 401", resp.StatusCode)
	}

	// Correct secret → 200 + session cookie
	resp, err = client.Post(ts.URL+"/admin/api/login", "application/json",
		strings.NewReader(`{"secret":"test-secret-key"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("good login: status = %d, want 200", resp.StatusCode)
	}

	var sessionCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == cookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("no session cookie set after login")
	}

	// Use cookie to access protected endpoint
	req, _ := http.NewRequest("GET", ts.URL+"/admin/api/status", nil)
	req.AddCookie(sessionCookie)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("authed status: %d, want 200", resp.StatusCode)
	}

	// /me shows authenticated
	req, _ = http.NewRequest("GET", ts.URL+"/admin/api/me", nil)
	req.AddCookie(sessionCookie)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var me map[string]any
	json.NewDecoder(resp.Body).Decode(&me)
	resp.Body.Close()
	if me["authenticated"] != true {
		t.Errorf("me = %v", me)
	}

	// Logout clears cookie
	req, _ = http.NewRequest("POST", ts.URL+"/admin/api/logout", nil)
	req.AddCookie(sessionCookie)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("logout: status = %d", resp.StatusCode)
	}
}

func TestDemoModeBypassesAuth(t *testing.T) {
	_, ts := testServer() // demo mode
	defer ts.Close()

	// Should work without auth in demo mode
	resp, err := http.Get(ts.URL + "/admin/api/status")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("demo status: %d, want 200", resp.StatusCode)
	}
}

func TestInvalidSessionCookie(t *testing.T) {
	_, ts := testAuthServer()
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/admin/api/status", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: "garbage.token"})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("invalid cookie: status = %d, want 401", resp.StatusCode)
	}
}

func TestApprovalAndAuditEndpoints(t *testing.T) {
	_, ts := testServer() // demo mode, no auth needed
	defer ts.Close()

	// Policies
	resp, err := http.Get(ts.URL + "/admin/api/approvals/policies")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("policies: status = %d", resp.StatusCode)
	}

	// Audit log
	resp, err = http.Get(ts.URL + "/admin/api/approvals/audit")
	if err != nil {
		t.Fatal(err)
	}
	var auditResp map[string]any
	json.NewDecoder(resp.Body).Decode(&auditResp)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("audit: status = %d", resp.StatusCode)
	}
	if auditResp["total"] != float64(0) {
		t.Errorf("audit total = %v, want 0", auditResp["total"])
	}
}

func TestRateLimitEndpoint(t *testing.T) {
	_, ts := testServer() // demo mode
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/admin/api/security/ratelimit")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("ratelimit: status = %d", resp.StatusCode)
	}
}
