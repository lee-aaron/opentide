-- Initial schema: conversations, audit log, rate limits
CREATE TABLE IF NOT EXISTS conversations (
    id         BIGSERIAL PRIMARY KEY,
    user_id    TEXT NOT NULL,
    channel_id TEXT NOT NULL,
    role       TEXT NOT NULL,
    content    TEXT NOT NULL,
    tool_call_id TEXT DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_conversations_user_id ON conversations(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_conversations_channel_id ON conversations(channel_id, created_at DESC);

CREATE TABLE IF NOT EXISTS audit_log (
    id         BIGSERIAL PRIMARY KEY,
    event_type TEXT NOT NULL,
    payload    JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_log_type ON audit_log(event_type, created_at DESC);

CREATE TABLE IF NOT EXISTS rate_limits (
    key        TEXT PRIMARY KEY,
    tokens     DOUBLE PRECISION NOT NULL,
    last_fill  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
