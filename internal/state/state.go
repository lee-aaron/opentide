// Package state manages conversation history and audit logging.
package state

import (
	"context"
	"sync"
	"time"

	"github.com/opentide/opentide/internal/providers"
)

// ConversationEntry is a single turn in a conversation.
type ConversationEntry struct {
	Timestamp time.Time            `json:"timestamp"`
	UserID    string               `json:"user_id"`
	ChannelID string               `json:"channel_id"`
	Message   providers.ChatMessage `json:"message"`
}

// Store is the interface for conversation persistence.
// History is scoped per channel (DM or server channel) so conversations don't leak across contexts.
type Store interface {
	SaveMessage(ctx context.Context, entry ConversationEntry) error
	GetHistory(ctx context.Context, channelID string, limit int) ([]ConversationEntry, error)
	Close() error
}

// MemoryStore is an in-memory store for demo/dev mode.
type MemoryStore struct {
	mu      sync.RWMutex
	history map[string][]ConversationEntry // channelID -> entries
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		history: make(map[string][]ConversationEntry),
	}
}

func (s *MemoryStore) SaveMessage(_ context.Context, entry ConversationEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history[entry.ChannelID] = append(s.history[entry.ChannelID], entry)
	return nil
}

func (s *MemoryStore) GetHistory(_ context.Context, channelID string, limit int) ([]ConversationEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := s.history[channelID]
	if limit > 0 && len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}

	result := make([]ConversationEntry, len(entries))
	copy(result, entries)
	return result, nil
}

func (s *MemoryStore) Close() error {
	return nil
}
