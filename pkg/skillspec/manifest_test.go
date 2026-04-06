package skillspec

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func validManifest() Manifest {
	return Manifest{
		Name:        "test-skill",
		Version:     "0.1.0",
		Description: "A test skill",
		Author:      "test-author",
		Security: Security{
			Egress:     []string{"api.example.com:443"},
			Filesystem: "read-only",
			MaxMemory:  "128Mi",
			MaxCPU:     "0.5",
			Timeout:    "30s",
		},
		Triggers: Triggers{
			ToolName: "test_tool",
		},
		Runtime: Runtime{
			Image: "opentide/skill-test:0.1.0",
		},
	}
}

func TestValidateRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Manifest)
		wantErr string
	}{
		{"missing name", func(m *Manifest) { m.Name = "" }, "missing 'name'"},
		{"missing version", func(m *Manifest) { m.Version = "" }, "missing 'version'"},
		{"missing description", func(m *Manifest) { m.Description = "" }, "missing 'description'"},
		{"missing author", func(m *Manifest) { m.Author = "" }, "missing 'author'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := validManifest()
			tt.modify(&m)
			err := m.Validate()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if got := err.Error(); !contains(got, tt.wantErr) {
				t.Errorf("error %q does not contain %q", got, tt.wantErr)
			}
		})
	}
}

func TestValidateWildcardEgressRejected(t *testing.T) {
	m := validManifest()
	m.Security.Egress = []string{"api.example.com:443", "*"}
	err := m.Validate()
	if err == nil {
		t.Fatal("expected error for wildcard egress")
	}
	if got := err.Error(); !contains(got, "wildcard") {
		t.Errorf("error %q does not mention wildcard", got)
	}
}

func TestValidateSpecificEgressAllowed(t *testing.T) {
	m := validManifest()
	m.Security.Egress = []string{"api.brave.com:443", "api.google.com:443"}
	if err := m.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateFilesystemModes(t *testing.T) {
	for _, mode := range []string{"", "read-only", "read-write"} {
		m := validManifest()
		m.Security.Filesystem = mode
		if err := m.Validate(); err != nil {
			t.Errorf("filesystem %q should be valid, got: %v", mode, err)
		}
	}

	m := validManifest()
	m.Security.Filesystem = "write-all"
	if err := m.Validate(); err == nil {
		t.Fatal("expected error for invalid filesystem mode")
	}
}

func TestValidateTriggerRequired(t *testing.T) {
	m := validManifest()
	m.Triggers = Triggers{} // no triggers at all
	err := m.Validate()
	if err == nil {
		t.Fatal("expected error when no trigger defined")
	}

	// Each trigger type alone should be sufficient
	for _, tt := range []struct {
		name string
		set  func(*Triggers)
	}{
		{"tool_name", func(tr *Triggers) { tr.ToolName = "my_tool" }},
		{"keywords", func(tr *Triggers) { tr.Keywords = []string{"search"} }},
		{"regex", func(tr *Triggers) { tr.Regex = "^search .*" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := validManifest()
			m.Triggers = Triggers{}
			tt.set(&m.Triggers)
			if err := m.Validate(); err != nil {
				t.Errorf("trigger %s alone should be valid, got: %v", tt.name, err)
			}
		})
	}
}

func TestLoadManifest(t *testing.T) {
	yaml := `name: web-search
version: 0.1.0
description: Search the web
author: opentide
security:
  egress:
    - "api.brave.com:443"
  filesystem: read-only
  max_memory: 128Mi
  max_cpu: "0.5"
  timeout: 30s
triggers:
  tool_name: web_search
config:
  - name: Brave API Key
    env_var: BRAVE_API_KEY
    required: true
    secret: true
runtime:
  image: opentide/skill-web-search:0.1.0
`
	dir := t.TempDir()
	path := filepath.Join(dir, "skill.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	if m.Name != "web-search" {
		t.Errorf("name = %q, want web-search", m.Name)
	}
	if m.Security.Egress[0] != "api.brave.com:443" {
		t.Errorf("egress[0] = %q, want api.brave.com:443", m.Security.Egress[0])
	}
	if m.Triggers.ToolName != "web_search" {
		t.Errorf("tool_name = %q, want web_search", m.Triggers.ToolName)
	}
	if len(m.Config) != 1 || m.Config[0].EnvVar != "BRAVE_API_KEY" {
		t.Errorf("config not parsed correctly: %+v", m.Config)
	}
	if !m.Config[0].Secret {
		t.Error("expected config[0].Secret = true")
	}
}

func TestLoadManifestFileNotFound(t *testing.T) {
	_, err := LoadManifest("/nonexistent/skill.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadManifestInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skill.yaml")
	if err := os.WriteFile(path, []byte(":::bad yaml:::"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadManifest(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestMarshalYAML(t *testing.T) {
	m := validManifest()
	data, err := m.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("MarshalYAML returned empty bytes")
	}
	// Round-trip: should be parseable back
	var m2 Manifest
	if err := yaml.Unmarshal(data, &m2); err != nil {
		t.Fatalf("round-trip unmarshal failed: %v", err)
	}
	if m2.Name != m.Name {
		t.Errorf("round-trip name = %q, want %q", m2.Name, m.Name)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
