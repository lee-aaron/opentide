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

// ProcessEngine runs skills as local processes (dev mode only).
// No container isolation. Uses process-level resource limits where available.
// WARNING: Not for production use. Skills have full host access.
type ProcessEngine struct {
	mu       sync.RWMutex
	skills   map[string]*processSkill
	disabled map[string]*processSkill
}

type processSkill struct {
	manifest *skillspec.Manifest
	command  string // resolved command to run
}

// NewProcessEngine creates a skill engine that runs skills as local processes.
func NewProcessEngine() *ProcessEngine {
	return &ProcessEngine{
		skills:   make(map[string]*processSkill),
		disabled: make(map[string]*processSkill),
	}
}

func (e *ProcessEngine) LoadSkill(_ context.Context, manifest *skillspec.Manifest) error {
	if manifest.Triggers.ToolName == "" {
		return fmt.Errorf("skill %s has no tool_name trigger", manifest.Name)
	}

	// In dev mode, use the native field or fall back to the container entrypoint
	command := manifest.Runtime.Native
	if command == "" && manifest.Runtime.Entrypoint != "" {
		command = manifest.Runtime.Entrypoint
	}
	if command == "" {
		return fmt.Errorf("skill %s: no native command or entrypoint specified (required for dev mode)", manifest.Name)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.skills[manifest.Triggers.ToolName] = &processSkill{
		manifest: manifest,
		command:  command,
	}
	return nil
}

func (e *ProcessEngine) InvokeSkill(ctx context.Context, toolName string, input Input) (*Output, error) {
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

	// Split command into program and args
	parts := strings.Fields(skill.command)
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, &InvocationError{SkillName: skill.manifest.Name, Cause: err}
	}

	cmd.Stdin = bytes.NewReader(inputJSON)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set config env vars
	for _, cv := range skill.manifest.Config {
		if !cv.Secret {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", cv.EnvVar, input.Config[cv.EnvVar]))
		}
	}

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
		return &Output{
			Error:    fmt.Sprintf("skill %q execution failed", skill.manifest.Name),
			Duration: duration,
		}, nil
	}

	var output Output
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		output = Output{
			Content: strings.TrimSpace(stdout.String()),
		}
	}
	output.Duration = duration
	return &output, nil
}

func (e *ProcessEngine) ListSkills(_ context.Context) ([]Info, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	infos := make([]Info, 0, len(e.skills)+len(e.disabled))
	for toolName, skill := range e.skills {
		infos = append(infos, Info{
			Name:        skill.manifest.Name,
			Version:     skill.manifest.Version,
			Description: skill.manifest.Description,
			Author:      skill.manifest.Author,
			ToolName:    toolName,
			Loaded:      true,
			Enabled:     true,
			Security:    securityInfoFromManifest(skill.manifest),
		})
	}
	for toolName, skill := range e.disabled {
		infos = append(infos, Info{
			Name:        skill.manifest.Name,
			Version:     skill.manifest.Version,
			Description: skill.manifest.Description,
			Author:      skill.manifest.Author,
			ToolName:    toolName,
			Loaded:      false,
			Enabled:     false,
			Security:    securityInfoFromManifest(skill.manifest),
		})
	}
	return infos, nil
}

func (e *ProcessEngine) UnloadSkill(_ context.Context, toolName string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.skills[toolName]; !ok {
		return &SkillNotFoundError{ToolName: toolName}
	}
	delete(e.skills, toolName)
	return nil
}

func (e *ProcessEngine) DisableSkill(_ context.Context, toolName string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	skill, ok := e.skills[toolName]
	if !ok {
		if _, disabled := e.disabled[toolName]; disabled {
			return nil
		}
		return &SkillNotFoundError{ToolName: toolName}
	}
	e.disabled[toolName] = skill
	delete(e.skills, toolName)
	return nil
}

func (e *ProcessEngine) EnableSkill(_ context.Context, toolName string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	skill, ok := e.disabled[toolName]
	if !ok {
		if _, active := e.skills[toolName]; active {
			return nil
		}
		return &SkillNotFoundError{ToolName: toolName}
	}
	e.skills[toolName] = skill
	delete(e.disabled, toolName)
	return nil
}

func (e *ProcessEngine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.skills = make(map[string]*processSkill)
	return nil
}
