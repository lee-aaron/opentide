// github skill: read GitHub issues, PRs, and commits via the GitHub API.
// Read-only access. Token via GITHUB_TOKEN environment variable (optional for public repos).
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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

const apiBase = "https://api.github.com"

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
		writeError("missing 'query' argument - e.g. 'issues owner/repo' or 'pr owner/repo/123'")
		return
	}

	result, err := handleQuery(query)
	if err != nil {
		writeError(err.Error())
		return
	}
	writeOutput(result)
}

func handleQuery(query string) (string, error) {
	parts := strings.Fields(query)
	if len(parts) < 2 {
		return "", fmt.Errorf("format: <command> <owner/repo> [args]\nCommands: issues, pr, commits, repo")
	}

	cmd := strings.ToLower(parts[0])
	repo := parts[1]

	// Validate repo format
	repoParts := strings.SplitN(repo, "/", 3)
	if len(repoParts) < 2 {
		return "", fmt.Errorf("repo must be in owner/repo format, got: %s", repo)
	}
	owner := repoParts[0]
	repoName := repoParts[1]

	switch cmd {
	case "issues":
		return fetchIssues(owner, repoName)
	case "pr", "pulls":
		if len(repoParts) == 3 {
			return fetchPR(owner, repoName, repoParts[2])
		}
		return fetchPRs(owner, repoName)
	case "commits":
		return fetchCommits(owner, repoName)
	case "repo", "info":
		return fetchRepo(owner, repoName)
	default:
		return "", fmt.Errorf("unknown command: %s\nAvailable: issues, pr, commits, repo", cmd)
	}
}

func fetchIssues(owner, repo string) (string, error) {
	var issues []map[string]any
	err := githubGet(fmt.Sprintf("/repos/%s/%s/issues?state=open&per_page=10", owner, repo), &issues)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Open issues for %s/%s:\n\n", owner, repo))
	for _, issue := range issues {
		if issue["pull_request"] != nil {
			continue // skip PRs
		}
		num := int(issue["number"].(float64))
		title := issue["title"].(string)
		user := issue["user"].(map[string]any)["login"].(string)
		sb.WriteString(fmt.Sprintf("#%d %s (by @%s)\n", num, title, user))
	}
	if sb.Len() == 0 {
		return "No open issues.", nil
	}
	return sb.String(), nil
}

func fetchPRs(owner, repo string) (string, error) {
	var prs []map[string]any
	err := githubGet(fmt.Sprintf("/repos/%s/%s/pulls?state=open&per_page=10", owner, repo), &prs)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Open PRs for %s/%s:\n\n", owner, repo))
	for _, pr := range prs {
		num := int(pr["number"].(float64))
		title := pr["title"].(string)
		user := pr["user"].(map[string]any)["login"].(string)
		sb.WriteString(fmt.Sprintf("#%d %s (by @%s)\n", num, title, user))
	}
	return sb.String(), nil
}

func fetchPR(owner, repo, number string) (string, error) {
	var pr map[string]any
	err := githubGet(fmt.Sprintf("/repos/%s/%s/pulls/%s", owner, repo, number), &pr)
	if err != nil {
		return "", err
	}

	title := pr["title"].(string)
	state := pr["state"].(string)
	user := pr["user"].(map[string]any)["login"].(string)
	body, _ := pr["body"].(string)
	if len(body) > 500 {
		body = body[:500] + "..."
	}

	return fmt.Sprintf("PR #%s: %s\nState: %s | Author: @%s\n\n%s", number, title, state, user, body), nil
}

func fetchCommits(owner, repo string) (string, error) {
	var commits []map[string]any
	err := githubGet(fmt.Sprintf("/repos/%s/%s/commits?per_page=10", owner, repo), &commits)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Recent commits for %s/%s:\n\n", owner, repo))
	for _, c := range commits {
		sha := c["sha"].(string)[:7]
		commit := c["commit"].(map[string]any)
		msg := commit["message"].(string)
		if idx := strings.IndexByte(msg, '\n'); idx > 0 {
			msg = msg[:idx]
		}
		author := commit["author"].(map[string]any)["name"].(string)
		sb.WriteString(fmt.Sprintf("%s %s (%s)\n", sha, msg, author))
	}
	return sb.String(), nil
}

func fetchRepo(owner, repo string) (string, error) {
	var r map[string]any
	err := githubGet(fmt.Sprintf("/repos/%s/%s", owner, repo), &r)
	if err != nil {
		return "", err
	}

	name := r["full_name"].(string)
	desc, _ := r["description"].(string)
	stars := int(r["stargazers_count"].(float64))
	forks := int(r["forks_count"].(float64))
	lang, _ := r["language"].(string)

	return fmt.Sprintf("%s\n%s\nStars: %d | Forks: %d | Language: %s", name, desc, stars, forks, lang), nil
}

func githubGet(path string, result any) error {
	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest("GET", apiBase+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "OpenTide-GitHub/0.1")

	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 {
		return fmt.Errorf("rate limited - try again later or set GITHUB_TOKEN")
	}
	if resp.StatusCode == 404 {
		return fmt.Errorf("not found - check the repository name")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("GitHub API error: %d %s", resp.StatusCode, resp.Status)
	}

	return json.NewDecoder(resp.Body).Decode(result)
}

func writeOutput(content string) {
	json.NewEncoder(os.Stdout).Encode(Output{Content: content})
}

func writeError(msg string) {
	json.NewEncoder(os.Stdout).Encode(Output{Error: msg})
}
