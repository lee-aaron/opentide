// Web search skill: reads JSON input from stdin, queries Brave Search API, returns results.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

type BraveResponse struct {
	Web struct {
		Results []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
		} `json:"results"`
	} `json:"web"`
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

	apiKey := os.Getenv("BRAVE_API_KEY")
	if apiKey == "" {
		writeError("BRAVE_API_KEY not set")
		return
	}

	results, err := search(apiKey, query)
	if err != nil {
		writeError("search failed: " + err.Error())
		return
	}

	writeOutput(results)
}

func search(apiKey, query string) (string, error) {
	u := fmt.Sprintf("https://api.search.brave.com/res/v1/web/search?q=%s&count=5", url.QueryEscape(query))
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-Subscription-Token", apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Brave API returned %d: %s", resp.StatusCode, string(body))
	}

	var brave BraveResponse
	if err := json.NewDecoder(resp.Body).Decode(&brave); err != nil {
		return "", err
	}

	var sb strings.Builder
	for i, r := range brave.Web.Results {
		fmt.Fprintf(&sb, "%d. **%s**\n   %s\n   %s\n\n", i+1, r.Title, r.URL, r.Description)
	}

	if sb.Len() == 0 {
		return "No results found.", nil
	}
	return sb.String(), nil
}

func writeOutput(content string) {
	json.NewEncoder(os.Stdout).Encode(Output{Content: content})
}

func writeError(msg string) {
	json.NewEncoder(os.Stdout).Encode(Output{Error: msg})
}
