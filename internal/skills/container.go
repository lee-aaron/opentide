package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/opentide/opentide/pkg/skillspec"
)

// ContainerEngine runs skills in Docker containers with security constraints.
// Each invocation gets a fresh container with:
// - Read-only root filesystem (tmpfs for scratch if manifest allows read-write)
// - Network restricted to declared egress hosts
// - CPU and memory limits from manifest
// - Timeout enforcement
type ContainerEngine struct {
	mu     sync.RWMutex
	skills map[string]*loadedSkill // keyed by tool_name

	egress *EgressController
}

type loadedSkill struct {
	manifest *skillspec.Manifest
	image    string // resolved container image
}

// NewContainerEngine creates a skill engine that uses Docker containers.
func NewContainerEngine() *ContainerEngine {
	return &ContainerEngine{
		skills: make(map[string]*loadedSkill),
		egress: NewEgressController(),
	}
}

func (e *ContainerEngine) LoadSkill(_ context.Context, manifest *skillspec.Manifest) error {
	if manifest.Triggers.ToolName == "" {
		return fmt.Errorf("skill %s has no tool_name trigger", manifest.Name)
	}

	image := manifest.Runtime.Image
	if image == "" && manifest.Runtime.Dockerfile != "" {
		// Would build from Dockerfile; for now require a pre-built image
		return fmt.Errorf("skill %s: Dockerfile builds not yet supported, provide a pre-built image", manifest.Name)
	}
	if image == "" {
		return fmt.Errorf("skill %s: no container image specified", manifest.Name)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.skills[manifest.Triggers.ToolName] = &loadedSkill{
		manifest: manifest,
		image:    image,
	}
	return nil
}

func (e *ContainerEngine) InvokeSkill(ctx context.Context, toolName string, input Input) (*Output, error) {
	e.mu.RLock()
	skill, ok := e.skills[toolName]
	e.mu.RUnlock()

	if !ok {
		return nil, &SkillNotFoundError{ToolName: toolName}
	}

	timeout := parseTimeout(skill.manifest.Security.Timeout, 30*time.Second)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()

	args := e.buildDockerArgs(skill)

	// Serialize input as JSON, pass via stdin
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, &InvocationError{SkillName: skill.manifest.Name, Cause: err}
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdin = bytes.NewReader(inputJSON)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	duration := time.Since(start)

	if ctx.Err() == context.DeadlineExceeded {
		return nil, &InvocationError{
			SkillName: skill.manifest.Name,
			TimedOut:  true,
		}
	}

	if err != nil {
		// Never expose raw stderr to users (may contain secrets from env vars or configs).
		// Return a generic error message only.
		return &Output{
			Error:    fmt.Sprintf("skill %q execution failed", skill.manifest.Name),
			Duration: duration,
		}, nil
	}

	// Parse structured output if JSON, otherwise return raw text
	var output Output
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		// Not JSON, treat stdout as plain text response
		output = Output{
			Content: strings.TrimSpace(stdout.String()),
		}
	}
	output.Duration = duration
	return &output, nil
}

func (e *ContainerEngine) ListSkills(_ context.Context) ([]Info, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	infos := make([]Info, 0, len(e.skills))
	for toolName, skill := range e.skills {
		infos = append(infos, Info{
			Name:        skill.manifest.Name,
			Version:     skill.manifest.Version,
			Description: skill.manifest.Description,
			Author:      skill.manifest.Author,
			ToolName:    toolName,
			Loaded:      true,
			Security:    securityInfoFromManifest(skill.manifest),
		})
	}
	return infos, nil
}

func (e *ContainerEngine) UnloadSkill(_ context.Context, toolName string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.skills[toolName]; !ok {
		return &SkillNotFoundError{ToolName: toolName}
	}
	delete(e.skills, toolName)
	return nil
}

func (e *ContainerEngine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.skills = make(map[string]*loadedSkill)
	return nil
}

// buildDockerArgs constructs the docker run command arguments from the skill manifest.
func (e *ContainerEngine) buildDockerArgs(skill *loadedSkill) []string {
	m := skill.manifest
	args := []string{
		"run", "--rm",
		"-i",          // stdin for input
		"--read-only", // read-only root filesystem
	}

	// Network: use egress controller to determine network mode
	netArgs := e.egress.NetworkArgs(m.Name, m.Security.Egress)
	args = append(args, netArgs...)

	// Memory limit
	if m.Security.MaxMemory != "" {
		args = append(args, "--memory="+m.Security.MaxMemory)
	}

	// CPU limit
	if m.Security.MaxCPU != "" {
		args = append(args, "--cpus="+m.Security.MaxCPU)
	}

	// Filesystem: if read-write, add a tmpfs for /tmp
	if m.Security.Filesystem == "read-write" {
		args = append(args, "--tmpfs=/tmp:rw,noexec,nosuid,size=64m")
	}

	// Security options: no new privileges, drop all capabilities
	args = append(args,
		"--security-opt=no-new-privileges:true",
		"--cap-drop=ALL",
	)

	// Inject non-secret config as environment variables
	for _, cv := range m.Config {
		if !cv.Secret {
			// Config values are passed via input JSON, but env vars are useful too
			args = append(args, "-e", cv.EnvVar)
		}
	}

	// Container image
	args = append(args, skill.image)

	// Entrypoint override
	if m.Runtime.Entrypoint != "" {
		// Insert --entrypoint before the image
		idx := len(args) - 1
		args = append(args[:idx], append([]string{"--entrypoint", m.Runtime.Entrypoint}, args[idx:]...)...)
	}

	return args
}

// parseTimeout parses a duration string, returning the default if empty or invalid.
func parseTimeout(s string, def time.Duration) time.Duration {
	if s == "" {
		return def
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return d
}
