package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/opentide/opentide/internal/security"
	"github.com/opentide/opentide/pkg/skillspec"

	"log/slog"
)

func signedTestManifest(t *testing.T, name, version string) *skillspec.SignedManifest {
	t.Helper()
	kp, _ := security.GenerateKeyPair()
	m := &skillspec.Manifest{
		Name:        name,
		Version:     version,
		Description: "Test skill " + name,
		Author:      "test-author",
		Security: skillspec.Security{
			Filesystem: "read-only",
		},
		Triggers: skillspec.Triggers{
			ToolName: strings.ReplaceAll(name, "-", "_"),
		},
		Runtime: skillspec.Runtime{
			Image: "opentide/" + name + ":" + version,
		},
	}
	signed, err := security.SignManifest(m, kp.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func TestRegistryPublishAndGet(t *testing.T) {
	store := NewMemoryStore()
	reg := New(store)
	ctx := context.Background()

	signed := signedTestManifest(t, "web-search", "0.1.0")
	if err := reg.Publish(ctx, signed, "opentide/web-search@sha256:abc123"); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	entry, err := reg.Get(ctx, "web-search", "")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if entry.Name != "web-search" {
		t.Errorf("name = %q, want web-search", entry.Name)
	}
	if entry.ImageRef != "opentide/web-search@sha256:abc123" {
		t.Errorf("image_ref = %q", entry.ImageRef)
	}
}

func TestRegistryPublishUnsigned(t *testing.T) {
	store := NewMemoryStore()
	reg := New(store)

	unsigned := &skillspec.SignedManifest{
		Manifest: skillspec.Manifest{
			Name:        "evil",
			Version:     "1.0",
			Description: "bad",
			Author:      "attacker",
			Triggers:    skillspec.Triggers{ToolName: "evil"},
			Runtime:     skillspec.Runtime{Image: "evil:latest"},
		},
		// No signature
	}

	err := reg.Publish(context.Background(), unsigned, "evil:latest")
	if err == nil {
		t.Fatal("expected error for unsigned manifest")
	}
}

func TestRegistrySearch(t *testing.T) {
	store := NewMemoryStore()
	reg := New(store)
	ctx := context.Background()

	for _, name := range []string{"web-search", "calculator", "file-manager"} {
		signed := signedTestManifest(t, name, "0.1.0")
		reg.Publish(ctx, signed, "img:latest")
	}

	result, err := reg.Search(ctx, SearchQuery{Term: "search"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("expected 1 result for 'search', got %d", result.Total)
	}

	result, err = reg.Search(ctx, SearchQuery{})
	if err != nil {
		t.Fatalf("Search all failed: %v", err)
	}
	if result.Total != 3 {
		t.Errorf("expected 3 total skills, got %d", result.Total)
	}
}

func TestRegistryInstall(t *testing.T) {
	store := NewMemoryStore()
	reg := New(store)
	ctx := context.Background()

	signed := signedTestManifest(t, "calculator", "0.1.0")
	reg.Publish(ctx, signed, "img:latest")

	entry, err := reg.Install(ctx, "calculator", "")
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}
	if entry.Name != "calculator" {
		t.Errorf("name = %q", entry.Name)
	}

	// Check download count incremented
	entry2, _ := reg.Get(ctx, "calculator", "0.1.0")
	if entry2.Downloads != 1 {
		t.Errorf("downloads = %d, want 1", entry2.Downloads)
	}
}

func TestRegistryVersioning(t *testing.T) {
	store := NewMemoryStore()
	reg := New(store)
	ctx := context.Background()

	for _, v := range []string{"0.1.0", "0.2.0", "1.0.0"} {
		signed := signedTestManifest(t, "multi-ver", v)
		reg.Publish(ctx, signed, "img:"+v)
	}

	// Latest should be the last published
	entry, _ := reg.Get(ctx, "multi-ver", "")
	if entry.Version != "1.0.0" {
		t.Errorf("latest version = %q, want 1.0.0", entry.Version)
	}

	// Specific version
	entry, _ = reg.Get(ctx, "multi-ver", "0.2.0")
	if entry.Version != "0.2.0" {
		t.Errorf("version = %q, want 0.2.0", entry.Version)
	}

	// List versions
	versions, _ := store.ListVersions(ctx, "multi-ver")
	if len(versions) != 3 {
		t.Errorf("expected 3 versions, got %d", len(versions))
	}
}

// Integration test: full HTTP round-trip
func TestServerIntegration(t *testing.T) {
	store := NewMemoryStore()
	reg := New(store)
	logger := slog.Default()
	srv := NewServer(reg, logger)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := NewClient(ts.URL)
	ctx := context.Background()

	// Publish
	signed := signedTestManifest(t, "http-test", "0.1.0")
	if err := client.Publish(ctx, signed, "test:0.1.0"); err != nil {
		t.Fatalf("client.Publish failed: %v", err)
	}

	// Get
	entry, err := client.Get(ctx, "http-test", "")
	if err != nil {
		t.Fatalf("client.Get failed: %v", err)
	}
	if entry.Name != "http-test" {
		t.Errorf("name = %q", entry.Name)
	}

	// Search
	result, err := client.Search(ctx, "http", "")
	if err != nil {
		t.Fatalf("client.Search failed: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("search total = %d, want 1", result.Total)
	}

	// Install
	installed, err := client.Install(ctx, "http-test", "0.1.0")
	if err != nil {
		t.Fatalf("client.Install failed: %v", err)
	}
	if installed.ImageRef != "test:0.1.0" {
		t.Errorf("image_ref = %q", installed.ImageRef)
	}

	// List versions
	versions, err := client.ListVersions(ctx, "http-test")
	if err != nil {
		t.Fatalf("client.ListVersions failed: %v", err)
	}
	if len(versions) != 1 {
		t.Errorf("expected 1 version, got %d", len(versions))
	}

	// Health check
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer resp.Body.Close()
	var health map[string]string
	json.NewDecoder(resp.Body).Decode(&health)
	if health["status"] != "ok" {
		t.Errorf("health = %v", health)
	}
}

func TestServerPublishUnsigned(t *testing.T) {
	store := NewMemoryStore()
	reg := New(store)
	srv := NewServer(reg, slog.Default())

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// POST an unsigned manifest
	body := `{"signed":{"manifest":{"name":"evil","version":"1.0","description":"bad","author":"x","triggers":{"tool_name":"evil"},"runtime":{"image":"evil"}},"signature":{}},"image_ref":"evil:latest"}`
	resp, err := http.Post(ts.URL+"/v1/skills", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 403 or 400 for unsigned, got %d", resp.StatusCode)
	}
}
