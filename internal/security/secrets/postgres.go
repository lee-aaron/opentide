package secrets

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore is a Postgres-backed encrypted secrets store.
// Ciphertext is encrypted in the application layer with AES-256-GCM before storage.
type PostgresStore struct {
	pool *pgxpool.Pool
	key  []byte
}

// NewPostgresStore creates a Postgres-backed secrets store with the given encryption key.
func NewPostgresStore(pool *pgxpool.Pool, encKey []byte) *PostgresStore {
	return &PostgresStore{pool: pool, key: encKey}
}

func (s *PostgresStore) Put(ctx context.Context, provider string, plaintext string) (*SecretMeta, error) {
	ct, err := Encrypt(s.key, []byte(plaintext))
	if err != nil {
		return nil, fmt.Errorf("encrypt: %w", err)
	}

	now := time.Now().UTC()
	l4 := last4(plaintext)

	_, err = s.pool.Exec(ctx,
		`INSERT INTO secrets (provider, ciphertext, last4, created_at)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (provider) DO UPDATE SET ciphertext = $2, last4 = $3, created_at = $4`,
		provider, ct, l4, now,
	)
	if err != nil {
		return nil, fmt.Errorf("store secret: %w", err)
	}

	return &SecretMeta{
		Provider:   provider,
		Last4:      l4,
		CreatedAt:  now,
		Source:     "store",
		Configured: true,
	}, nil
}

func (s *PostgresStore) Get(ctx context.Context, provider string) (string, error) {
	var ct []byte
	err := s.pool.QueryRow(ctx,
		`SELECT ciphertext FROM secrets WHERE provider = $1`, provider,
	).Scan(&ct)
	if err != nil {
		return "", fmt.Errorf("no secret stored for provider %q", provider)
	}

	plaintext, err := Decrypt(s.key, ct)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plaintext), nil
}

func (s *PostgresStore) Meta(ctx context.Context, provider string) (*SecretMeta, error) {
	var l4 string
	var createdAt time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT last4, created_at FROM secrets WHERE provider = $1`, provider,
	).Scan(&l4, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("no secret stored for provider %q", provider)
	}

	return &SecretMeta{
		Provider:   provider,
		Last4:      l4,
		CreatedAt:  createdAt,
		Source:     "store",
		Configured: true,
	}, nil
}

func (s *PostgresStore) List(ctx context.Context) ([]SecretMeta, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT provider, last4, created_at FROM secrets ORDER BY provider`,
	)
	if err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}
	defer rows.Close()

	var result []SecretMeta
	for rows.Next() {
		var m SecretMeta
		if err := rows.Scan(&m.Provider, &m.Last4, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan secret: %w", err)
		}
		m.Source = "store"
		m.Configured = true
		result = append(result, m)
	}
	return result, nil
}

func (s *PostgresStore) Delete(ctx context.Context, provider string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM secrets WHERE provider = $1`, provider)
	if err != nil {
		return fmt.Errorf("delete secret: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("no secret stored for provider %q", provider)
	}
	return nil
}
