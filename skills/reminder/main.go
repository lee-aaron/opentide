// reminder skill: parse reminder requests and return structured reminder data.
// The gateway is responsible for scheduling delivery via the adapter.
// No network access needed.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
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

type ReminderResult struct {
	Message  string `json:"message"`
	Duration string `json:"duration"`
	FireAt   string `json:"fire_at"`
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

	result, err := parseReminder(query)
	if err != nil {
		writeError(err.Error())
		return
	}

	b, _ := json.MarshalIndent(result, "", "  ")
	writeOutput(fmt.Sprintf("Reminder set:\n%s", string(b)))
}

var durationPattern = regexp.MustCompile(`(?i)(?:in\s+)?(\d+)\s*(seconds?|secs?|s|minutes?|mins?|m|hours?|hrs?|h|days?|d)\b`)

func parseReminder(query string) (*ReminderResult, error) {
	match := durationPattern.FindStringSubmatch(query)
	if match == nil {
		return nil, fmt.Errorf("could not parse duration from: %q\nExpected format: 'remind me in 5 minutes to check the oven'", query)
	}

	amount, _ := strconv.Atoi(match[1])
	unit := strings.ToLower(match[2])

	var d time.Duration
	switch {
	case strings.HasPrefix(unit, "s"):
		d = time.Duration(amount) * time.Second
	case strings.HasPrefix(unit, "m"):
		d = time.Duration(amount) * time.Minute
	case strings.HasPrefix(unit, "h"):
		d = time.Duration(amount) * time.Hour
	case strings.HasPrefix(unit, "d"):
		d = time.Duration(amount) * 24 * time.Hour
	default:
		return nil, fmt.Errorf("unknown time unit: %s", unit)
	}

	if d < time.Second {
		return nil, fmt.Errorf("duration too short (minimum 1 second)")
	}
	if d > 7*24*time.Hour {
		return nil, fmt.Errorf("duration too long (maximum 7 days)")
	}

	// Extract the message (everything after the duration part)
	message := durationPattern.ReplaceAllString(query, "")
	message = strings.TrimSpace(message)
	// Clean up common prefixes
	for _, prefix := range []string{"remind me", "remind", "to", "that", "about"} {
		message = strings.TrimPrefix(strings.TrimSpace(message), prefix)
	}
	message = strings.TrimSpace(message)
	if message == "" {
		message = "Reminder!"
	}

	fireAt := time.Now().Add(d)

	return &ReminderResult{
		Message:  message,
		Duration: d.String(),
		FireAt:   fireAt.UTC().Format(time.RFC3339),
	}, nil
}

func writeOutput(content string) {
	json.NewEncoder(os.Stdout).Encode(Output{Content: content})
}

func writeError(msg string) {
	json.NewEncoder(os.Stdout).Encode(Output{Error: msg})
}
