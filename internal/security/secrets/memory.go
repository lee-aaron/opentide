package secrets

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type encryptedEntry struct {
	Ciphertext []byte
	Last4      string
	CreatedAt  time.Time
}

// MemoryStore is an in-memory encrypted secrets store.
// Values are encrypted even in memory so heap dumps don't leak plaintext.
type MemoryStore struct {
	mu      sync.RWMutex
	secrets map[string]*encryptedEntry
	key     []byte
}

// NewMemoryStore creates an in-memory secrets store with the given encryption key.
func NewMemoryStore(encKey []byte) *MemoryStore {
	return &MemoryStore{
		secrets: make(map[string]*encryptedEntry),
		key:     encKey,
	}
}

func (m *MemoryStore) Put(_ context.Context, provider string, plaintext string) (*SecretMeta, error) {
	ct, err := Encrypt(m.key, []byte(plaintext))
	if err != nil {
		return nil, fmt.Errorf("encrypt: %w", err)
	}

	entry := &encryptedEntry{
		Ciphertext: ct,
		Last4:      last4(plaintext),
		CreatedAt:  time.Now().UTC(),
	}

	m.mu.Lock()
	m.secrets[provider] = entry
	m.mu.Unlock()

	return &SecretMeta{
		Provider:   provider,
		Last4:      entry.Last4,
		CreatedAt:  entry.CreatedAt,
		Source:     "store",
		Configured: true,
	}, nil
}

func (m *MemoryStore) Get(_ context.Context, provider string) (string, error) {
	m.mu.RLock()
	entry, ok := m.secrets[provider]
	m.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("no secret stored for provider %q", provider)
	}

	plaintext, err := Decrypt(m.key, entry.Ciphertext)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plaintext), nil
}

func (m *MemoryStore) Meta(_ context.Context, provider string) (*SecretMeta, error) {
	m.mu.RLock()
	entry, ok := m.secrets[provider]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no secret stored for provider %q", provider)
	}

	return &SecretMeta{
		Provider:   provider,
		Last4:      entry.Last4,
		CreatedAt:  entry.CreatedAt,
		Source:     "store",
		Configured: true,
	}, nil
}

func (m *MemoryStore) List(_ context.Context) ([]SecretMeta, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]SecretMeta, 0, len(m.secrets))
	for provider, entry := range m.secrets {
		result = append(result, SecretMeta{
			Provider:   provider,
			Last4:      entry.Last4,
			CreatedAt:  entry.CreatedAt,
			Source:     "store",
			Configured: true,
		})
	}
	return result, nil
}

func (m *MemoryStore) Delete(_ context.Context, provider string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.secrets[provider]; !ok {
		return fmt.Errorf("no secret stored for provider %q", provider)
	}
	delete(m.secrets, provider)
	return nil
}
