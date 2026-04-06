// Package skillspec defines the public skill manifest format.
// Skill authors use this to declare what their skill does, what it needs,
// and what security permissions it requires.
package skillspec

import (
	"fmt"
	"os"
	"slices"

	oerr "github.com/opentide/opentide/pkg/errors"
	"gopkg.in/yaml.v3"
)

// Manifest is the skill.yaml file that ships with every skill.
type Manifest struct {
	Name        string   `yaml:"name"`
	Version     string   `yaml:"version"`
	Description string   `yaml:"description"`
	Author      string   `yaml:"author"`
	License     string   `yaml:"license,omitempty"`
	Security    Security `yaml:"security"`
	Triggers    Triggers `yaml:"triggers,omitempty"`
	Config      []ConfigVar `yaml:"config,omitempty"`
	Runtime     Runtime  `yaml:"runtime"`
}

// Security declares the skill's security requirements.
// Everything not declared is denied.
type Security struct {
	Egress     []string `yaml:"egress,omitempty"`     // allowed host:port pairs
	Filesystem string   `yaml:"filesystem,omitempty"` // "read-only" (default), "read-write" (tmpfs only)
	MaxMemory  string   `yaml:"max_memory,omitempty"` // e.g. "128Mi"
	MaxCPU     string   `yaml:"max_cpu,omitempty"`    // e.g. "0.5"
	Timeout    string   `yaml:"timeout,omitempty"`    // e.g. "30s"
}

// Triggers define how the skill is activated.
type Triggers struct {
	Keywords []string `yaml:"keywords,omitempty"` // simple keyword matching
	Regex    string   `yaml:"regex,omitempty"`    // regex pattern on user message
	ToolName string   `yaml:"tool_name,omitempty"` // tool name for LLM tool use
}

// ConfigVar is a user-configurable variable the skill needs.
type ConfigVar struct {
	Name     string `yaml:"name"`
	EnvVar   string `yaml:"env_var"`            // e.g. "BRAVE_API_KEY"
	Required bool   `yaml:"required,omitempty"`
	Secret   bool   `yaml:"secret,omitempty"`   // if true, passed via secret reference API
}

// Runtime describes how the skill executes.
type Runtime struct {
	Image      string `yaml:"image,omitempty"`      // container image (e.g. "opentide/skill-web-search:0.1.0")
	Dockerfile string `yaml:"dockerfile,omitempty"` // path to Dockerfile for local builds
	Entrypoint string `yaml:"entrypoint,omitempty"` // override container entrypoint
	// For native (in-process) skills, used in dev mode
	Native     string `yaml:"native,omitempty"`     // Go plugin path or "builtin:<name>"
}

// Signature holds the cryptographic signature for a manifest.
type Signature struct {
	PublicKey string `yaml:"public_key"` // hex-encoded Ed25519 public key
	Signature string `yaml:"signature"`  // hex-encoded Ed25519 signature
	SignedAt  string `yaml:"signed_at"`  // RFC3339 timestamp
}

// SignedManifest wraps a manifest with its signature.
type SignedManifest struct {
	Manifest  Manifest  `yaml:"manifest"`
	Signature Signature `yaml:"signature"`
}

// LoadManifest reads and validates a skill.yaml file.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, oerr.Wrap(oerr.CodeSkillNotFound, fmt.Sprintf("cannot read skill manifest: %s", path), err).
			WithFix("Check that the skill directory contains a skill.yaml file")
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, oerr.Wrap(oerr.CodeConfigInvalid, "invalid skill manifest YAML", err)
	}

	if err := m.Validate(); err != nil {
		return nil, err
	}

	return &m, nil
}

// Validate checks that the manifest has all required fields and sane values.
func (m *Manifest) Validate() error {
	if m.Name == "" {
		return oerr.New(oerr.CodeConfigInvalid, "skill manifest missing 'name' field")
	}
	if m.Version == "" {
		return oerr.New(oerr.CodeConfigInvalid, "skill manifest missing 'version' field")
	}
	if m.Description == "" {
		return oerr.New(oerr.CodeConfigInvalid, "skill manifest missing 'description' field")
	}
	if m.Author == "" {
		return oerr.New(oerr.CodeConfigInvalid, "skill manifest missing 'author' field")
	}

	// Validate egress entries
	if slices.Contains(m.Security.Egress, "*") {
		return oerr.New(oerr.CodeSkillEgress, "wildcard egress ('*') is not allowed").
			WithFix("Declare specific host:port pairs in the egress list")
	}

	// Validate filesystem
	switch m.Security.Filesystem {
	case "", "read-only", "read-write":
		// ok
	default:
		return oerr.New(oerr.CodeConfigInvalid, fmt.Sprintf("invalid filesystem mode: %s (must be 'read-only' or 'read-write')", m.Security.Filesystem))
	}

	// Must have either a tool_name trigger or keywords/regex
	if m.Triggers.ToolName == "" && len(m.Triggers.Keywords) == 0 && m.Triggers.Regex == "" {
		return oerr.New(oerr.CodeConfigInvalid, "skill must define at least one trigger (tool_name, keywords, or regex)")
	}

	return nil
}

// MarshalYAML serializes the manifest to YAML bytes.
func (m *Manifest) MarshalYAML() ([]byte, error) {
	return yaml.Marshal(m)
}
