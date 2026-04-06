// Package registry implements the skill registry for publishing, searching,
// verifying, and installing skills. Skills are stored with their signed
// manifests and container image references.
package registry

import (
	"context"
	"fmt"
	"time"

	"github.com/opentide/opentide/internal/security"
	"github.com/opentide/opentide/pkg/skillspec"
)

// SkillEntry is a skill stored in the registry.
type SkillEntry struct {
	Name        string                  `json:"name"`
	Version     string                  `json:"version"`
	Description string                  `json:"description"`
	Author      string                  `json:"author"`
	License     string                  `json:"license,omitempty"`
	ImageRef    string                  `json:"image_ref"`    // container image digest
	Signed      *skillspec.SignedManifest `json:"signed"`      // signed manifest
	PublishedAt time.Time               `json:"published_at"`
	Downloads   int64                   `json:"downloads"`
}

// SearchQuery filters skills in the registry.
type SearchQuery struct {
	Term   string `json:"term,omitempty"`   // free text search
	Author string `json:"author,omitempty"` // filter by author
	Limit  int    `json:"limit,omitempty"`  // max results (default 20)
	Offset int    `json:"offset,omitempty"` // pagination offset
}

// SearchResult is a page of search results.
type SearchResult struct {
	Entries []SkillEntry `json:"entries"`
	Total   int          `json:"total"`
}

// Store is the registry storage backend.
type Store interface {
	// Publish stores a new skill version. Overwrites if same name+version exists.
	Publish(ctx context.Context, entry SkillEntry) error

	// Get retrieves a specific skill version. Empty version = latest.
	Get(ctx context.Context, name, version string) (*SkillEntry, error)

	// Search finds skills matching the query.
	Search(ctx context.Context, q SearchQuery) (*SearchResult, error)

	// IncrementDownloads bumps the download counter.
	IncrementDownloads(ctx context.Context, name, version string) error

	// Delete removes a skill version from the registry.
	Delete(ctx context.Context, name, version string) error

	// ListVersions returns all versions of a skill, newest first.
	ListVersions(ctx context.Context, name string) ([]SkillEntry, error)
}

// Registry orchestrates publishing and verification.
type Registry struct {
	store Store
}

// New creates a registry with the given store.
func New(store Store) *Registry {
	return &Registry{store: store}
}

// Publish validates, verifies the signature, and stores a skill.
func (r *Registry) Publish(ctx context.Context, signed *skillspec.SignedManifest, imageRef string) error {
	// Validate the manifest
	if err := signed.Manifest.Validate(); err != nil {
		return fmt.Errorf("invalid manifest: %w", err)
	}

	// Verify the signature
	if err := security.VerifyManifest(signed); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	entry := SkillEntry{
		Name:        signed.Manifest.Name,
		Version:     signed.Manifest.Version,
		Description: signed.Manifest.Description,
		Author:      signed.Manifest.Author,
		License:     signed.Manifest.License,
		ImageRef:    imageRef,
		Signed:      signed,
		PublishedAt: time.Now().UTC(),
	}

	return r.store.Publish(ctx, entry)
}

// Get retrieves a skill by name and optional version.
func (r *Registry) Get(ctx context.Context, name, version string) (*SkillEntry, error) {
	return r.store.Get(ctx, name, version)
}

// Search finds skills matching the query.
func (r *Registry) Search(ctx context.Context, q SearchQuery) (*SearchResult, error) {
	if q.Limit <= 0 {
		q.Limit = 20
	}
	if q.Limit > 100 {
		q.Limit = 100
	}
	return r.store.Search(ctx, q)
}

// Install retrieves a skill and increments the download counter.
func (r *Registry) Install(ctx context.Context, name, version string) (*SkillEntry, error) {
	entry, err := r.store.Get(ctx, name, version)
	if err != nil {
		return nil, err
	}

	// Best-effort download count increment
	r.store.IncrementDownloads(ctx, name, version)

	return entry, nil
}
