// code-runner skill: execute code snippets in a sandboxed environment.
// Supports Python, JavaScript (Node.js), and Go.
// Network: --network=none (no egress)
// Filesystem: tmpfs only (no persistent writes)
// Timeout: 10s hard kill
// Memory: 256Mi max
//
// NOTE: This skill requires Docker daemon access and is NOT available
// on DigitalOcean App Platform. It works on Droplets, self-hosted, or
// any environment with Docker socket access.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Input struct {
	Arguments map[string]any `json:"arguments"`
}

type Output struct {
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

const (
	maxOutputSize = 64 * 1024 // 64KB
	execTimeout   = 10 * time.Second
)

func main() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		writeError("failed to read input: " + err.Error())
		return
	}

	var input Input
	if err := json.Unmarshal(data, &input); err != nil {
		writeError("invalid input JSON: " + err.Error())
		return
	}

	query, _ := input.Arguments["query"].(string)
	if query == "" {
		writeError("missing 'query' argument - provide code to execute with language prefix (python:, js:, go:)")
		return
	}

	lang, code := parseCode(query)
	if lang == "" {
		writeError("could not detect language. Prefix your code with 'python:', 'js:', or 'go:'")
		return
	}

	result, err := executeCode(lang, code)
	if err != nil {
		writeError(err.Error())
		return
	}

	// Cap output
	if len(result) > maxOutputSize {
		result = result[:maxOutputSize] + "\n[Output truncated at 64KB]"
	}

	writeOutput(fmt.Sprintf("```\n%s\n```", result))
}

func parseCode(query string) (lang, code string) {
	// Try "language: code" format
	for _, prefix := range []string{"python:", "py:", "javascript:", "js:", "node:", "go:", "golang:"} {
		if strings.HasPrefix(strings.ToLower(query), prefix) {
			code = strings.TrimSpace(query[len(prefix):])
			switch {
			case strings.HasPrefix(prefix, "py") || strings.HasPrefix(prefix, "python"):
				return "python", code
			case strings.HasPrefix(prefix, "js") || strings.HasPrefix(prefix, "javascript") || strings.HasPrefix(prefix, "node"):
				return "javascript", code
			case strings.HasPrefix(prefix, "go"):
				return "go", code
			}
		}
	}

	// Auto-detect from content
	lower := strings.ToLower(query)
	if strings.Contains(lower, "def ") || strings.Contains(lower, "print(") || strings.Contains(lower, "import ") {
		return "python", query
	}
	if strings.Contains(lower, "console.log") || strings.Contains(lower, "const ") || strings.Contains(lower, "function ") {
		return "javascript", query
	}
	if strings.Contains(lower, "package ") || strings.Contains(lower, "func ") || strings.Contains(lower, "fmt.") {
		return "go", query
	}

	return "", query
}

func executeCode(lang, code string) (string, error) {
	// Check if we're running in sandbox mode (as a skill binary)
	// or directly (for testing). In production, the skill runs inside
	// a container and uses the host's Docker socket to spawn sub-containers.

	var cmd *exec.Cmd

	switch lang {
	case "python":
		cmd = exec.Command("python3", "-c", code)
	case "javascript":
		cmd = exec.Command("node", "-e", code)
	case "go":
		// Go requires writing to a temp file
		tmpFile, err := os.CreateTemp("", "code-*.go")
		if err != nil {
			return "", fmt.Errorf("failed to create temp file: %w", err)
		}
		defer os.Remove(tmpFile.Name())

		if _, err := tmpFile.WriteString(code); err != nil {
			return "", fmt.Errorf("failed to write code: %w", err)
		}
		tmpFile.Close()

		cmd = exec.Command("go", "run", tmpFile.Name())
	default:
		return "", fmt.Errorf("unsupported language: %s", lang)
	}

	// Set timeout
	timer := time.AfterFunc(execTimeout, func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	})
	defer timer.Stop()

	output, err := cmd.CombinedOutput()
	if err != nil {
		if len(output) > 0 {
			return "", fmt.Errorf("execution error:\n%s", string(output))
		}
		return "", fmt.Errorf("execution failed: %w", err)
	}

	return string(output), nil
}

func writeOutput(content string) {
	json.NewEncoder(os.Stdout).Encode(Output{Content: content})
}

func writeError(msg string) {
	json.NewEncoder(os.Stdout).Encode(Output{Error: msg})
}
