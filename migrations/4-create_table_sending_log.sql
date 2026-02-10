CREATE TABLE IF NOT EXISTS sending_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    kind TEXT NOT NULL CHECK (kind IN ('daily', 'pair')),
    chats INT NOT NULL,
    groups INT NOT NULL,
    elapsed INT NOT NULL, --milliseconds
    fails INT NOT NULL DEFAULT 0,
    errors TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_sending_log_kind ON sending_logs(kind);
CREATE INDEX IF NOT EXISTS idx_sending_log_created_at ON sending_logs(created_at);
