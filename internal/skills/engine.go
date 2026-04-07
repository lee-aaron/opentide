// Package skills implements the skill execution engine.
// Skills run in isolated containers (Docker+seccomp in production, process-level in dev).
// Every skill invocation is sandboxed with declared resource limits, filesystem
// restrictions, and network egress controls from the skill manifest.
package skills

import (
	"context"
	"fmt"
	"time"

	"github.com/opentide/opentide/pkg/skillspec"
)

// Input is the payload sent to a skill when invoked.
type Input struct {
	ToolName  string            `json:"tool_name"`
	Arguments map[string]any    `json:"arguments"`
	UserID    string            `json:"user_id"`
	ChannelID string            `json:"channel_id"`
	Config    map[string]string `json:"config"` // resolved config vars (non-secret)
}

// Output is the result returned by a skill invocation.
type Output struct {
	Content  string         `json:"content"`            // text response
	Data     map[string]any `json:"data,omitempty"`     // structured data
	Error    string         `json:"error,omitempty"`    // error message if skill failed
	Duration time.Duration  `json:"duration"`           // execution time
}

// Info describes a loaded skill.
type Info struct {
	Name        string         `json:"name"`
	Version     string         `json:"version"`
	Description string         `json:"description"`
	Author      string         `json:"author"`
	ToolName    string         `json:"tool_name"`
	Loaded      bool           `json:"loaded"`
	Security    *SecurityInfo  `json:"security,omitempty"`
}

// SecurityInfo is the security posture of a loaded skill, derived from its manifest.
type SecurityInfo struct {
	Egress     []string `json:"egress"`     // allowed host:port pairs
	Filesystem string   `json:"filesystem"` // "read-only" or "read-write"
	MaxMemory  string   `json:"max_memory"`
	MaxCPU     string   `json:"max_cpu"`
	Timeout    string   `json:"timeout"`
}

// Engine is the skill execution engine interface.
type Engine interface {
	// LoadSkill registers a skill from its manifest.
	LoadSkill(ctx context.Context, manifest *skillspec.Manifest) error

	// InvokeSkill runs a skill by tool name with the given input.
	InvokeSkill(ctx context.Context, toolName string, input Input) (*Output, error)

	// ListSkills returns info about all loaded skills.
	ListSkills(ctx context.Context) ([]Info, error)

	// UnloadSkill removes a skill from the engine.
	UnloadSkill(ctx context.Context, toolName string) error

	// Close shuts down the engine and cleans up resources.
	Close() error
}

// securityInfoFromManifest extracts SecurityInfo from a skill manifest.
func securityInfoFromManifest(m *skillspec.Manifest) *SecurityInfo {
	s := &m.Security
	egress := s.Egress
	if egress == nil {
		egress = []string{}
	}
	return &SecurityInfo{
		Egress:     egress,
		Filesystem: s.Filesystem,
		MaxMemory:  s.MaxMemory,
		MaxCPU:     s.MaxCPU,
		Timeout:    s.Timeout,
	}
}

// SkillNotFoundError is returned when a skill is not loaded.
type SkillNotFoundError struct {
	ToolName string
}

func (e *SkillNotFoundError) Error() string {
	return fmt.Sprintf("skill not found: %s", e.ToolName)
}

// InvocationError wraps errors from skill execution.
type InvocationError struct {
	SkillName string
	Cause     error
	TimedOut  bool
}

func (e *InvocationError) Error() string {
	if e.TimedOut {
		return fmt.Sprintf("skill %s timed out", e.SkillName)
	}
	return fmt.Sprintf("skill %s failed: %v", e.SkillName, e.Cause)
}

func (e *InvocationError) Unwrap() error {
	return e.Cause
}
