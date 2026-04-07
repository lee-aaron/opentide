// json-tools skill: JSON parsing, formatting, querying, and validation.
// Supports: validate, format/pretty, minify, query (dot notation), keys, type.
// No network access needed.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type Input struct {
	Arguments map[string]any `json:"arguments"`
}

type Output struct {
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

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
		writeError("missing 'query' argument")
		return
	}

	result, err := processJSON(query)
	if err != nil {
		writeError(err.Error())
		return
	}
	writeOutput(result)
}

func processJSON(query string) (string, error) {
	// Parse "operation: json_content" format
	parts := strings.SplitN(query, ":", 2)
	if len(parts) < 2 {
		// Try to parse as raw JSON
		return formatJSON(query)
	}

	op := strings.TrimSpace(strings.ToLower(parts[0]))
	content := strings.TrimSpace(parts[1])

	switch op {
	case "validate":
		var v any
		if err := json.Unmarshal([]byte(content), &v); err != nil {
			return fmt.Sprintf("Invalid JSON: %s", err.Error()), nil
		}
		return "Valid JSON", nil

	case "format", "pretty":
		return formatJSON(content)

	case "minify", "compact":
		var v any
		if err := json.Unmarshal([]byte(content), &v); err != nil {
			return "", fmt.Errorf("invalid JSON: %w", err)
		}
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil

	case "keys":
		var m map[string]any
		if err := json.Unmarshal([]byte(content), &m); err != nil {
			return "", fmt.Errorf("expected JSON object: %w", err)
		}
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		return fmt.Sprintf("Keys: %s", strings.Join(keys, ", ")), nil

	case "type":
		var v any
		if err := json.Unmarshal([]byte(content), &v); err != nil {
			return "", fmt.Errorf("invalid JSON: %w", err)
		}
		return fmt.Sprintf("Type: %s", jsonType(v)), nil

	case "query", "get":
		// Format: "query: path.to.key | {json}"
		queryParts := strings.SplitN(content, "|", 2)
		if len(queryParts) < 2 {
			return "", fmt.Errorf("query format: query: path.to.key | {json}")
		}
		path := strings.TrimSpace(queryParts[0])
		jsonStr := strings.TrimSpace(queryParts[1])
		return queryJSON(path, jsonStr)

	default:
		// Try to format as JSON
		return formatJSON(query)
	}
}

func formatJSON(s string) (string, error) {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func queryJSON(path, jsonStr string) (string, error) {
	var v any
	if err := json.Unmarshal([]byte(jsonStr), &v); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}

	keys := strings.Split(path, ".")
	current := v
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		switch c := current.(type) {
		case map[string]any:
			val, ok := c[key]
			if !ok {
				return fmt.Sprintf("Key not found: %s", key), nil
			}
			current = val
		default:
			return fmt.Sprintf("Cannot traverse into %s at key: %s", jsonType(current), key), nil
		}
	}

	b, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func jsonType(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return "unknown"
	}
}

func writeOutput(content string) {
	json.NewEncoder(os.Stdout).Encode(Output{Content: content})
}

func writeError(msg string) {
	json.NewEncoder(os.Stdout).Encode(Output{Error: msg})
}
