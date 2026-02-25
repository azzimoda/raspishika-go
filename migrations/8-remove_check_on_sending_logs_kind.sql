CREATE TABLE temp_sending_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    kind TEXT NOT NULL, -- Removed CHECK here
    chats INT NOT NULL,
    groups INT NOT NULL,
    elapsed INT NOT NULL, -- milliseconds
    fails INT NOT NULL DEFAULT 0,
    errors TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO temp_sending_logs SELECT * FROM sending_logs;
DROP TABLE sending_logs;
ALTER TABLE temp_sending_logs RENAME TO sending_logs;

CREATE INDEX IF NOT EXISTS idx_sending_log_kind ON sending_logs(kind);
CREATE INDEX IF NOT EXISTS idx_sending_log_created_at ON sending_logs(created_at);
