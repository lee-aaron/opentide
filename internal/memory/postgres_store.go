package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore is a Postgres-backed user memory store.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a Postgres-backed user memory store.
func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) Add(ctx context.Context, userID, note string) (*Note, error) {
	var id int64
	var createdAt time.Time
	err := s.pool.QueryRow(ctx,
		`INSERT INTO user_memory (user_id, note) VALUES ($1, $2)
		 RETURNING id, created_at`,
		userID, note,
	).Scan(&id, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("insert memory: %w", err)
	}

	return &Note{
		ID:        id,
		UserID:    userID,
		Text:      note,
		CreatedAt: createdAt,
	}, nil
}

func (s *PostgresStore) List(ctx context.Context, userID string) ([]Note, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, note, created_at FROM user_memory
		 WHERE user_id = $1 ORDER BY created_at ASC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list memory: %w", err)
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.UserID, &n.Text, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		notes = append(notes, n)
	}
	return notes, nil
}

func (s *PostgresStore) Delete(ctx context.Context, userID string, noteID int64) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM user_memory WHERE id = $1 AND user_id = $2`,
		noteID, userID,
	)
	if err != nil {
		return fmt.Errorf("delete memory: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("note %d not found", noteID)
	}
	return nil
}

func (s *PostgresStore) DeleteAll(ctx context.Context, userID string) (int, error) {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM user_memory WHERE user_id = $1`, userID,
	)
	if err != nil {
		return 0, fmt.Errorf("delete all memory: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
