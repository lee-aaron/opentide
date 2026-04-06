package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/opentide/opentide/pkg/skillspec"
)

// Client is a registry API client.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a registry client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Publish uploads a signed skill to the registry.
func (c *Client) Publish(ctx context.Context, signed *skillspec.SignedManifest, imageRef string) error {
	req := PublishRequest{
		Signed:   *signed,
		ImageRef: imageRef,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal publish request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/skills", bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("registry request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.readError(resp)
	}
	return nil
}

// Get retrieves a skill by name. Empty version = latest.
func (c *Client) Get(ctx context.Context, name, version string) (*SkillEntry, error) {
	path := "/v1/skills/" + name
	if version != "" {
		path += "/" + version
	}

	var entry SkillEntry
	if err := c.getJSON(ctx, path, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// Search finds skills matching the query.
func (c *Client) Search(ctx context.Context, term, author string) (*SearchResult, error) {
	params := url.Values{}
	if term != "" {
		params.Set("q", term)
	}
	if author != "" {
		params.Set("author", author)
	}

	path := "/v1/skills"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var result SearchResult
	if err := c.getJSON(ctx, path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Install retrieves a skill and signals to the registry that it was downloaded.
func (c *Client) Install(ctx context.Context, name, version string) (*SkillEntry, error) {
	path := "/v1/skills/" + name
	if version != "" {
		path += "/" + version
	}
	path += "/install"

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("registry request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}

	var entry SkillEntry
	if err := json.NewDecoder(resp.Body).Decode(&entry); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &entry, nil
}

// ListVersions returns all versions of a skill.
func (c *Client) ListVersions(ctx context.Context, name string) ([]SkillEntry, error) {
	var entries []SkillEntry
	if err := c.getJSON(ctx, "/v1/skills/"+name+"/versions", &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("registry request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.readError(resp)
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) readError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	var errResp struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
		return fmt.Errorf("registry error (%d): %s", resp.StatusCode, errResp.Error)
	}
	return fmt.Errorf("registry error (%d): %s", resp.StatusCode, string(body))
}
