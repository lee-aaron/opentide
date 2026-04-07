-- User-controlled memory notes (opt-in cross-channel context)
-- Users add notes with /remember, view with /memories, delete with /forget
CREATE TABLE IF NOT EXISTS user_memory (
    id         BIGSERIAL PRIMARY KEY,
    user_id    TEXT NOT NULL,
    note       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_memory_user_id ON user_memory(user_id, created_at ASC);
