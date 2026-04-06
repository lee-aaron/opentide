package registry

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// MemoryStore is an in-memory registry store for development and testing.
type MemoryStore struct {
	mu      sync.RWMutex
	entries map[string][]SkillEntry // name -> versions (newest first)
}

// NewMemoryStore creates an in-memory registry store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		entries: make(map[string][]SkillEntry),
	}
}

func (s *MemoryStore) Publish(_ context.Context, entry SkillEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	versions := s.entries[entry.Name]

	// Replace if same version exists
	for i, v := range versions {
		if v.Version == entry.Version {
			versions[i] = entry
			s.entries[entry.Name] = versions
			return nil
		}
	}

	// Prepend (newest first)
	s.entries[entry.Name] = append([]SkillEntry{entry}, versions...)
	return nil
}

func (s *MemoryStore) Get(_ context.Context, name, version string) (*SkillEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions, ok := s.entries[name]
	if !ok || len(versions) == 0 {
		return nil, fmt.Errorf("skill not found: %s", name)
	}

	if version == "" {
		// Return latest
		e := versions[0]
		return &e, nil
	}

	for _, v := range versions {
		if v.Version == version {
			return &v, nil
		}
	}
	return nil, fmt.Errorf("skill %s version %s not found", name, version)
}

func (s *MemoryStore) Search(_ context.Context, q SearchQuery) (*SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var matches []SkillEntry

	for _, versions := range s.entries {
		if len(versions) == 0 {
			continue
		}
		latest := versions[0] // only search latest version

		if q.Author != "" && !strings.EqualFold(latest.Author, q.Author) {
			continue
		}

		if q.Term != "" {
			term := strings.ToLower(q.Term)
			if !strings.Contains(strings.ToLower(latest.Name), term) &&
				!strings.Contains(strings.ToLower(latest.Description), term) {
				continue
			}
		}

		matches = append(matches, latest)
	}

	// Sort by name
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Name < matches[j].Name
	})

	total := len(matches)

	// Apply pagination
	if q.Offset >= len(matches) {
		return &SearchResult{Total: total}, nil
	}
	matches = matches[q.Offset:]
	if q.Limit > 0 && len(matches) > q.Limit {
		matches = matches[:q.Limit]
	}

	return &SearchResult{
		Entries: matches,
		Total:   total,
	}, nil
}

func (s *MemoryStore) IncrementDownloads(_ context.Context, name, version string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	versions := s.entries[name]
	for i, v := range versions {
		if v.Version == version || version == "" {
			versions[i].Downloads++
			return nil
		}
	}
	return nil
}

func (s *MemoryStore) Delete(_ context.Context, name, version string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	versions, ok := s.entries[name]
	if !ok {
		return fmt.Errorf("skill not found: %s", name)
	}

	if version == "" {
		delete(s.entries, name)
		return nil
	}

	for i, v := range versions {
		if v.Version == version {
			s.entries[name] = append(versions[:i], versions[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("skill %s version %s not found", name, version)
}

func (s *MemoryStore) ListVersions(_ context.Context, name string) ([]SkillEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions, ok := s.entries[name]
	if !ok {
		return nil, fmt.Errorf("skill not found: %s", name)
	}

	result := make([]SkillEntry, len(versions))
	copy(result, versions)
	return result, nil
}
