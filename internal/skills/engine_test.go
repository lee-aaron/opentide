package skills

import (
	"context"
	"testing"
	"time"

	"github.com/opentide/opentide/pkg/skillspec"
)

func testManifest(toolName, image string) *skillspec.Manifest {
	return &skillspec.Manifest{
		Name:        "test-skill",
		Version:     "0.1.0",
		Description: "A test skill",
		Author:      "test",
		Security: skillspec.Security{
			MaxMemory:  "128Mi",
			MaxCPU:     "0.5",
			Filesystem: "read-only",
			Timeout:    "5s",
		},
		Triggers: skillspec.Triggers{
			ToolName: toolName,
		},
		Runtime: skillspec.Runtime{
			Image: image,
		},
	}
}

func TestContainerEngineLoadSkill(t *testing.T) {
	e := NewContainerEngine()
	defer e.Close()
	ctx := context.Background()

	m := testManifest("test_tool", "alpine:latest")
	if err := e.LoadSkill(ctx, m); err != nil {
		t.Fatalf("LoadSkill failed: %v", err)
	}

	skills, err := e.ListSkills(ctx)
	if err != nil {
		t.Fatalf("ListSkills failed: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].ToolName != "test_tool" {
		t.Errorf("tool_name = %q, want test_tool", skills[0].ToolName)
	}
}

func TestContainerEngineLoadSkillNoToolName(t *testing.T) {
	e := NewContainerEngine()
	defer e.Close()

	m := testManifest("", "alpine:latest")
	if err := e.LoadSkill(context.Background(), m); err == nil {
		t.Fatal("expected error for missing tool_name")
	}
}

func TestContainerEngineLoadSkillNoImage(t *testing.T) {
	e := NewContainerEngine()
	defer e.Close()

	m := testManifest("test_tool", "")
	if err := e.LoadSkill(context.Background(), m); err == nil {
		t.Fatal("expected error for missing image")
	}
}

func TestContainerEngineUnloadSkill(t *testing.T) {
	e := NewContainerEngine()
	defer e.Close()
	ctx := context.Background()

	m := testManifest("test_tool", "alpine:latest")
	e.LoadSkill(ctx, m)

	if err := e.UnloadSkill(ctx, "test_tool"); err != nil {
		t.Fatalf("UnloadSkill failed: %v", err)
	}

	skills, _ := e.ListSkills(ctx)
	if len(skills) != 0 {
		t.Fatalf("expected 0 skills after unload, got %d", len(skills))
	}
}

func TestContainerEngineUnloadSkillNotFound(t *testing.T) {
	e := NewContainerEngine()
	defer e.Close()

	err := e.UnloadSkill(context.Background(), "nonexistent")
	if _, ok := err.(*SkillNotFoundError); !ok {
		t.Fatalf("expected SkillNotFoundError, got %T: %v", err, err)
	}
}

func TestContainerEngineInvokeSkillNotFound(t *testing.T) {
	e := NewContainerEngine()
	defer e.Close()

	_, err := e.InvokeSkill(context.Background(), "nonexistent", Input{})
	if _, ok := err.(*SkillNotFoundError); !ok {
		t.Fatalf("expected SkillNotFoundError, got %T: %v", err, err)
	}
}

func TestBuildDockerArgs(t *testing.T) {
	m := &skillspec.Manifest{
		Name:    "test",
		Version: "1.0",
		Security: skillspec.Security{
			MaxMemory:  "256Mi",
			MaxCPU:     "1.0",
			Filesystem: "read-write",
			Egress:     []string{"api.example.com:443"},
		},
		Config: []skillspec.ConfigVar{
			{Name: "API Key", EnvVar: "API_KEY", Secret: true},
			{Name: "Region", EnvVar: "REGION", Secret: false},
		},
		Runtime: skillspec.Runtime{
			Image: "myskill:latest",
		},
	}
	skill := &loadedSkill{manifest: m, image: m.Runtime.Image}
	e := NewContainerEngine()
	args := e.buildDockerArgs(skill)

	// Check key security flags are present
	assertContains(t, args, "--network=none")
	assertContains(t, args, "--read-only")
	assertContains(t, args, "--memory=256Mi")
	assertContains(t, args, "--cpus=1.0")
	assertContains(t, args, "--security-opt=no-new-privileges:true")
	assertContains(t, args, "--cap-drop=ALL")

	// tmpfs for read-write filesystem
	found := false
	for _, a := range args {
		if a == "--tmpfs=/tmp:rw,noexec,nosuid,size=64m" {
			found = true
		}
	}
	if !found {
		t.Error("expected tmpfs mount for read-write filesystem")
	}

	// Secret config should NOT be passed as env
	for i, a := range args {
		if a == "-e" && i+1 < len(args) && args[i+1] == "API_KEY" {
			t.Error("secret config var API_KEY should not be passed as env")
		}
	}

	// Non-secret config SHOULD be passed
	foundRegion := false
	for i, a := range args {
		if a == "-e" && i+1 < len(args) && args[i+1] == "REGION" {
			foundRegion = true
		}
	}
	if !foundRegion {
		t.Error("non-secret config var REGION should be passed as env")
	}

	// Image should be last arg
	if args[len(args)-1] != "myskill:latest" {
		t.Errorf("last arg = %q, want myskill:latest", args[len(args)-1])
	}
}

func TestBuildDockerArgsReadOnly(t *testing.T) {
	m := &skillspec.Manifest{
		Name:    "test",
		Version: "1.0",
		Security: skillspec.Security{
			Filesystem: "read-only",
		},
		Runtime: skillspec.Runtime{
			Image: "myskill:latest",
		},
	}
	skill := &loadedSkill{manifest: m, image: m.Runtime.Image}
	e := NewContainerEngine()
	args := e.buildDockerArgs(skill)

	// Should NOT have tmpfs when read-only
	for _, a := range args {
		if a == "--tmpfs=/tmp:rw,noexec,nosuid,size=64m" {
			t.Error("should not have tmpfs for read-only filesystem")
		}
	}
}

func TestBuildDockerArgsEntrypoint(t *testing.T) {
	m := &skillspec.Manifest{
		Name:    "test",
		Version: "1.0",
		Runtime: skillspec.Runtime{
			Image:      "myskill:latest",
			Entrypoint: "/custom/entry",
		},
	}
	skill := &loadedSkill{manifest: m, image: m.Runtime.Image}
	e := NewContainerEngine()
	args := e.buildDockerArgs(skill)

	found := false
	for i, a := range args {
		if a == "--entrypoint" && i+1 < len(args) && args[i+1] == "/custom/entry" {
			found = true
		}
	}
	if !found {
		t.Error("expected --entrypoint /custom/entry in docker args")
	}
}

func TestParseTimeout(t *testing.T) {
	tests := []struct {
		input    string
		def      time.Duration
		expected time.Duration
	}{
		{"30s", time.Minute, 30 * time.Second},
		{"5m", time.Second, 5 * time.Minute},
		{"", time.Minute, time.Minute},
		{"invalid", time.Minute, time.Minute},
	}

	for _, tt := range tests {
		got := parseTimeout(tt.input, tt.def)
		if got != tt.expected {
			t.Errorf("parseTimeout(%q, %v) = %v, want %v", tt.input, tt.def, got, tt.expected)
		}
	}
}

func TestProcessEngineLoadSkill(t *testing.T) {
	e := NewProcessEngine()
	defer e.Close()
	ctx := context.Background()

	m := testManifest("proc_tool", "")
	m.Runtime.Native = "echo hello"
	if err := e.LoadSkill(ctx, m); err != nil {
		t.Fatalf("LoadSkill failed: %v", err)
	}

	skills, _ := e.ListSkills(ctx)
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
}

func TestProcessEngineLoadSkillNoCommand(t *testing.T) {
	e := NewProcessEngine()
	defer e.Close()

	m := testManifest("proc_tool", "")
	m.Runtime.Image = ""
	if err := e.LoadSkill(context.Background(), m); err == nil {
		t.Fatal("expected error for missing native command")
	}
}

func TestProcessEngineInvokeEcho(t *testing.T) {
	e := NewProcessEngine()
	defer e.Close()
	ctx := context.Background()

	m := testManifest("echo_tool", "")
	m.Runtime.Native = "echo hello-from-skill"
	e.LoadSkill(ctx, m)

	out, err := e.InvokeSkill(ctx, "echo_tool", Input{ToolName: "echo_tool"})
	if err != nil {
		t.Fatalf("InvokeSkill failed: %v", err)
	}
	if out.Content != "hello-from-skill" {
		t.Errorf("content = %q, want hello-from-skill", out.Content)
	}
	if out.Duration == 0 {
		t.Error("expected non-zero duration")
	}
}

func TestSkillNotFoundError(t *testing.T) {
	err := &SkillNotFoundError{ToolName: "missing"}
	if err.Error() != "skill not found: missing" {
		t.Errorf("error = %q", err.Error())
	}
}

func TestInvocationError(t *testing.T) {
	err := &InvocationError{SkillName: "test", TimedOut: true}
	if err.Error() != "skill test timed out" {
		t.Errorf("error = %q", err.Error())
	}

	inner := &SkillNotFoundError{ToolName: "x"}
	err2 := &InvocationError{SkillName: "test", Cause: inner}
	if err2.Unwrap() != inner {
		t.Error("Unwrap should return inner error")
	}
}

func assertContains(t *testing.T, args []string, want string) {
	t.Helper()
	for _, a := range args {
		if a == want {
			return
		}
	}
	t.Errorf("args %v does not contain %q", args, want)
}
