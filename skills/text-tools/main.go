// text-tools skill: text manipulation utilities.
// Supports: word_count, char_count, uppercase, lowercase, title_case,
// reverse, replace, truncate. No network access needed.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
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

	result, err := processText(query)
	if err != nil {
		writeError(err.Error())
		return
	}
	writeOutput(result)
}

func processText(query string) (string, error) {
	// Parse "operation: text" format
	parts := strings.SplitN(query, ":", 2)
	if len(parts) < 2 {
		return analyzeText(query), nil
	}

	op := strings.TrimSpace(strings.ToLower(parts[0]))
	text := strings.TrimSpace(parts[1])

	switch op {
	case "word_count", "words", "wc":
		words := strings.Fields(text)
		return fmt.Sprintf("Word count: %d", len(words)), nil

	case "char_count", "chars", "len", "length":
		return fmt.Sprintf("Character count: %d (bytes: %d)", utf8.RuneCountInString(text), len(text)), nil

	case "uppercase", "upper":
		return strings.ToUpper(text), nil

	case "lowercase", "lower":
		return strings.ToLower(text), nil

	case "title", "title_case":
		return toTitleCase(text), nil

	case "reverse":
		runes := []rune(text)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return string(runes), nil

	case "trim":
		return strings.TrimSpace(text), nil

	case "replace":
		// Format: "replace: old -> new | text"
		replaceParts := strings.SplitN(text, "|", 2)
		if len(replaceParts) < 2 {
			return "", fmt.Errorf("replace format: replace: old -> new | text")
		}
		mapping := strings.SplitN(strings.TrimSpace(replaceParts[0]), "->", 2)
		if len(mapping) < 2 {
			return "", fmt.Errorf("replace format: replace: old -> new | text")
		}
		old := strings.TrimSpace(mapping[0])
		new := strings.TrimSpace(mapping[1])
		return strings.ReplaceAll(strings.TrimSpace(replaceParts[1]), old, new), nil

	case "truncate":
		// Format: "truncate: 100 | text"
		truncParts := strings.SplitN(text, "|", 2)
		if len(truncParts) < 2 {
			return "", fmt.Errorf("truncate format: truncate: length | text")
		}
		var maxLen int
		if _, err := fmt.Sscanf(strings.TrimSpace(truncParts[0]), "%d", &maxLen); err != nil {
			return "", fmt.Errorf("invalid length: %s", truncParts[0])
		}
		content := strings.TrimSpace(truncParts[1])
		runes := []rune(content)
		if len(runes) > maxLen {
			return string(runes[:maxLen]) + "...", nil
		}
		return content, nil

	default:
		// If no recognized operation, just analyze the text
		return analyzeText(query), nil
	}
}

func analyzeText(text string) string {
	words := strings.Fields(text)
	runes := []rune(text)
	lines := strings.Count(text, "\n") + 1

	return fmt.Sprintf("Text analysis:\n- Characters: %d\n- Words: %d\n- Lines: %d\n- Bytes: %d",
		len(runes), len(words), lines, len(text))
}

func toTitleCase(s string) string {
	prev := rune(' ')
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(prev) {
			prev = r
			return unicode.ToTitle(r)
		}
		prev = r
		return unicode.ToLower(r)
	}, s)
}

func writeOutput(content string) {
	json.NewEncoder(os.Stdout).Encode(Output{Content: content})
}

func writeError(msg string) {
	json.NewEncoder(os.Stdout).Encode(Output{Error: msg})
}
