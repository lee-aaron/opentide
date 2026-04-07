package state

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/opentide/opentide/internal/providers"
)

// PostgresStore persists conversations and audit data in PostgreSQL.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore connects to PostgreSQL and runs migrations.
func NewPostgresStore(ctx context.Context, dsn string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres connect: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}

	s := &PostgresStore{pool: pool}
	if err := s.migrate(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres migrate: %w", err)
	}

	return s, nil
}

func (s *PostgresStore) migrate(ctx context.Context) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS conversations (
			id         BIGSERIAL PRIMARY KEY,
			user_id    TEXT NOT NULL,
			channel_id TEXT NOT NULL,
			role       TEXT NOT NULL,
			content    TEXT NOT NULL,
			tool_call_id TEXT DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_conversations_user_id ON conversations(user_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_conversations_channel_id ON conversations(channel_id, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS audit_log (
			id         BIGSERIAL PRIMARY KEY,
			event_type TEXT NOT NULL,
			payload    JSONB NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_log_type ON audit_log(event_type, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS rate_limits (
			key        TEXT PRIMARY KEY,
			tokens     DOUBLE PRECISION NOT NULL,
			last_fill  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
	}

	for _, m := range migrations {
		if _, err := s.pool.Exec(ctx, m); err != nil {
			return fmt.Errorf("migration failed: %w\nSQL: %s", err, m)
		}
	}
	return nil
}

func (s *PostgresStore) SaveMessage(ctx context.Context, entry ConversationEntry) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO conversations (user_id, channel_id, role, content, tool_call_id, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		entry.UserID,
		entry.ChannelID,
		string(entry.Message.Role),
		entry.Message.Content,
		entry.Message.ToolCallID,
		entry.Timestamp,
	)
	return err
}

func (s *PostgresStore) GetHistory(ctx context.Context, channelID string, limit int) ([]ConversationEntry, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := s.pool.Query(ctx,
		`SELECT user_id, channel_id, role, content, tool_call_id, created_at
		 FROM conversations
		 WHERE channel_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2`,
		channelID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []ConversationEntry
	for rows.Next() {
		var e ConversationEntry
		var role string
		if err := rows.Scan(&e.UserID, &e.ChannelID, &role, &e.Message.Content, &e.Message.ToolCallID, &e.Timestamp); err != nil {
			return nil, err
		}
		e.Message.Role = providers.Role(role)
		entries = append(entries, e)
	}

	// Reverse to chronological order
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	return entries, nil
}

// AuditLog writes a structured event to the audit log.
func (s *PostgresStore) AuditLog(ctx context.Context, eventType string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal audit payload: %w", err)
	}

	_, err = s.pool.Exec(ctx,
		`INSERT INTO audit_log (event_type, payload) VALUES ($1, $2)`,
		eventType, data,
	)
	return err
}

// GetAuditLog retrieves recent audit events.
func (s *PostgresStore) GetAuditLog(ctx context.Context, eventType string, limit int) ([]AuditEntry, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `SELECT event_type, payload, created_at FROM audit_log`
	args := []any{}
	if eventType != "" {
		query += ` WHERE event_type = $1`
		args = append(args, eventType)
	}
	query += ` ORDER BY created_at DESC LIMIT ` + fmt.Sprintf("%d", limit)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []AuditEntry
	for rows.Next() {
		var e AuditEntry
		var payload []byte
		if err := rows.Scan(&e.EventType, &payload, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Payload = json.RawMessage(payload)
		entries = append(entries, e)
	}
	return entries, nil
}

// ConsumeRateToken implements a token bucket rate limiter backed by Postgres.
// Returns true if the request is allowed, false if rate limited.
func (s *PostgresStore) ConsumeRateToken(ctx context.Context, key string, maxTokens float64, refillRate float64) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	var tokens float64
	var lastFill time.Time

	err = tx.QueryRow(ctx,
		`SELECT tokens, last_fill FROM rate_limits WHERE key = $1 FOR UPDATE`,
		key,
	).Scan(&tokens, &lastFill)

	now := time.Now()

	if err != nil {
		// First time: initialize bucket
		_, err = tx.Exec(ctx,
			`INSERT INTO rate_limits (key, tokens, last_fill) VALUES ($1, $2, $3)`,
			key, maxTokens-1, now,
		)
		if err != nil {
			return false, err
		}
		return tx.Commit(ctx) == nil, nil
	}

	// Refill tokens based on elapsed time
	elapsed := now.Sub(lastFill).Seconds()
	tokens += elapsed * refillRate
	if tokens > maxTokens {
		tokens = maxTokens
	}

	if tokens < 1 {
		// Rate limited
		tx.Commit(ctx)
		return false, nil
	}

	// Consume one token
	tokens--
	_, err = tx.Exec(ctx,
		`UPDATE rate_limits SET tokens = $1, last_fill = $2 WHERE key = $3`,
		tokens, now, key,
	)
	if err != nil {
		return false, err
	}

	return tx.Commit(ctx) == nil, nil
}

func (s *PostgresStore) Close() error {
	s.pool.Close()
	return nil
}

// AuditEntry is a structured audit log entry from Postgres.
type AuditEntry struct {
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
}
