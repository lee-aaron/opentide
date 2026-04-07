// url-fetch skill: fetch a URL and return its text content.
// Includes SSRF protection: blocks private IPs, link-local, cloud metadata.
// Follows redirects with IP re-checking at each hop (DNS rebinding defense).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
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

const (
	maxResponseSize = 1 << 20 // 1MB
	maxRedirects    = 5
	requestTimeout  = 25 * time.Second
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
		writeError("missing 'query' argument - provide a URL to fetch")
		return
	}

	url := strings.TrimSpace(query)
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	content, err := fetchURL(url)
	if err != nil {
		writeError(err.Error())
		return
	}

	// Truncate if too long
	runes := []rune(content)
	if len(runes) > 10000 {
		content = string(runes[:10000]) + "\n\n[Truncated - content exceeded 10,000 characters]"
	}

	writeOutput(content)
}

func fetchURL(url string) (string, error) {
	// Create HTTP client with SSRF-safe dialer
	client := &http.Client{
		Timeout: requestTimeout,
		Transport: &http.Transport{
			DialContext: ssrfSafeDialer,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("too many redirects (max %d)", maxRedirects)
			}
			return nil
		},
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	req.Header.Set("User-Agent", "OpenTide-URLFetch/0.1")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Read with size limit
	limited := io.LimitReader(resp.Body, maxResponseSize+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("read error: %w", err)
	}

	if len(body) > maxResponseSize {
		body = body[:maxResponseSize]
		return string(body) + "\n\n[Truncated - response exceeded 1MB]", nil
	}

	return string(body), nil
}

// ssrfSafeDialer resolves DNS and blocks connections to private/internal IPs.
func ssrfSafeDialer(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid address: %w", err)
	}

	// Resolve DNS first
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("DNS lookup failed: %w", err)
	}

	// Check all resolved IPs against blocklist
	for _, ip := range ips {
		if isBlockedIP(ip.IP) {
			return nil, fmt.Errorf("blocked: %s resolves to private/internal IP %s", host, ip.IP)
		}
	}

	// Connect to the first allowed IP
	var dialer net.Dialer
	return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
}

// isBlockedIP checks if an IP is in a private/internal range.
// Blocks: RFC1918, RFC4291 (IPv6 loopback/link-local), cloud metadata (169.254.0.0/16).
func isBlockedIP(ip net.IP) bool {
	// Loopback
	if ip.IsLoopback() {
		return true
	}
	// Link-local
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	// Private
	if ip.IsPrivate() {
		return true
	}
	// Unspecified (0.0.0.0, ::)
	if ip.IsUnspecified() {
		return true
	}
	// Cloud metadata (169.254.0.0/16 - covered by link-local but be explicit)
	_, metadata, _ := net.ParseCIDR("169.254.0.0/16")
	if metadata != nil && metadata.Contains(ip) {
		return true
	}
	return false
}

func writeOutput(content string) {
	json.NewEncoder(os.Stdout).Encode(Output{Content: content})
}

func writeError(msg string) {
	json.NewEncoder(os.Stdout).Encode(Output{Error: msg})
}
