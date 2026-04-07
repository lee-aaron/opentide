// Package memory provides user-controlled cross-channel memory notes.
// Users add notes with /remember, view with /memories, and delete with /forget.
// Memory is opt-in and per-user (not per-channel).
package memory

import (
	"context"
	"time"
)

// Note is a single user memory entry.
type Note struct {
	ID        int64     `json:"id"`
	UserID    string    `json:"user_id"`
	Text      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

// Store is the interface for user memory persistence.
type Store interface {
	// Add saves a new memory note for a user.
	Add(ctx context.Context, userID, note string) (*Note, error)
	// List returns all memory notes for a user, ordered by creation time.
	List(ctx context.Context, userID string) ([]Note, error)
	// Delete removes a specific memory note by ID (must belong to the user).
	Delete(ctx context.Context, userID string, noteID int64) error
	// DeleteAll removes all memory notes for a user.
	DeleteAll(ctx context.Context, userID string) (int, error)
}
