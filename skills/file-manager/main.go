// File manager skill: read, write, list files in the sandboxed /tmp workspace.
// The container runs with read-only root fs + tmpfs on /tmp, so all file
// operations are scoped to the ephemeral scratch space. Nothing persists.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const workspace = "/tmp/workspace"

type Input struct {
	Arguments map[string]any `json:"arguments"`
}

type Output struct {
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

func main() {
	// Ensure workspace exists
	os.MkdirAll(workspace, 0755)

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

	action, _ := input.Arguments["action"].(string)
	switch action {
	case "read":
		filename, _ := input.Arguments["filename"].(string)
		if filename == "" {
			writeError("missing 'filename' argument")
			return
		}
		readFile(filename)
	case "write":
		filename, _ := input.Arguments["filename"].(string)
		content, _ := input.Arguments["content"].(string)
		if filename == "" {
			writeError("missing 'filename' argument")
			return
		}
		writeFile(filename, content)
	case "list":
		listFiles()
	default:
		writeError("unknown action: " + action + " (use 'read', 'write', or 'list')")
	}
}

func readFile(filename string) {
	path, err := safePath(filename)
	if err != nil {
		writeError(err.Error())
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		writeError("failed to read file: " + err.Error())
		return
	}

	writeOutput(string(data))
}

func writeFile(filename, content string) {
	path, err := safePath(filename)
	if err != nil {
		writeError(err.Error())
		return
	}

	// Create parent directories if needed
	os.MkdirAll(filepath.Dir(path), 0755)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		writeError("failed to write file: " + err.Error())
		return
	}

	writeOutput(fmt.Sprintf("Written %d bytes to %s", len(content), filename))
}

func listFiles() {
	var files []string
	filepath.Walk(workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(workspace, path)
		if rel == "." {
			return nil
		}
		if info.IsDir() {
			files = append(files, rel+"/")
		} else {
			files = append(files, fmt.Sprintf("%s (%d bytes)", rel, info.Size()))
		}
		return nil
	})

	if len(files) == 0 {
		writeOutput("Workspace is empty.")
		return
	}
	writeOutput("Files:\n" + strings.Join(files, "\n"))
}

// safePath ensures the filename stays within the workspace (path traversal defense).
func safePath(filename string) (string, error) {
	cleaned := filepath.Clean(filename)
	full := filepath.Join(workspace, cleaned)
	if !strings.HasPrefix(full, workspace) {
		return "", fmt.Errorf("path traversal denied: %s", filename)
	}
	return full, nil
}

func writeOutput(content string) {
	json.NewEncoder(os.Stdout).Encode(Output{Content: content})
}

func writeError(msg string) {
	json.NewEncoder(os.Stdout).Encode(Output{Error: msg})
}
