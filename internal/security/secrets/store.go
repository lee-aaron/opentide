package secrets

import (
	"context"
	"time"
)

// SecretMeta is the public metadata for a stored secret. Plaintext is never included.
type SecretMeta struct {
	Provider   string    `json:"provider"`
	Last4      string    `json:"last4"`
	CreatedAt  time.Time `json:"created_at"`
	Source     string    `json:"source"`     // "env" or "store"
	Configured bool      `json:"configured"`
}

// Store is the encrypted secrets store interface.
type Store interface {
	// Put encrypts and stores a secret. Returns metadata.
	Put(ctx context.Context, provider string, plaintext string) (*SecretMeta, error)
	// Get decrypts and returns the plaintext. Internal use only, never expose via API.
	Get(ctx context.Context, provider string) (string, error)
	// Meta returns metadata without decrypting.
	Meta(ctx context.Context, provider string) (*SecretMeta, error)
	// List returns metadata for all stored secrets.
	List(ctx context.Context) ([]SecretMeta, error)
	// Delete removes a stored secret.
	Delete(ctx context.Context, provider string) error
}

// last4 returns the last 4 characters of a string, or the whole string if shorter.
func last4(s string) string {
	if len(s) <= 4 {
		return s
	}
	return s[len(s)-4:]
}
