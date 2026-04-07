-- Encrypted secrets store for API keys managed via admin UI
CREATE TABLE IF NOT EXISTS secrets (
    provider   TEXT PRIMARY KEY,
    ciphertext BYTEA NOT NULL,
    last4      TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
