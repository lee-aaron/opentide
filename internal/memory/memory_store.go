package memory

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// MemoryStore is an in-memory implementation of Store for demo/dev mode.
type MemoryStore struct {
	mu    sync.RWMutex
	notes map[string][]Note // userID -> notes
	seq   atomic.Int64
}

// NewMemoryStore creates an in-memory user memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		notes: make(map[string][]Note),
	}
}

func (s *MemoryStore) Add(_ context.Context, userID, note string) (*Note, error) {
	n := Note{
		ID:        s.seq.Add(1),
		UserID:    userID,
		Text:      note,
		CreatedAt: time.Now().UTC(),
	}

	s.mu.Lock()
	s.notes[userID] = append(s.notes[userID], n)
	s.mu.Unlock()

	return &n, nil
}

func (s *MemoryStore) List(_ context.Context, userID string) ([]Note, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	notes := s.notes[userID]
	result := make([]Note, len(notes))
	copy(result, notes)
	return result, nil
}

func (s *MemoryStore) Delete(_ context.Context, userID string, noteID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	notes := s.notes[userID]
	for i, n := range notes {
		if n.ID == noteID {
			s.notes[userID] = append(notes[:i], notes[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("note %d not found", noteID)
}

func (s *MemoryStore) DeleteAll(_ context.Context, userID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := len(s.notes[userID])
	delete(s.notes, userID)
	return count, nil
}
