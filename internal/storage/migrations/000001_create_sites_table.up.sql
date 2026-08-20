CREATE TABLE IF NOT EXISTS sites (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    url           TEXT NOT NULL,
    interval_sec  INTEGER DEFAULT 30,
    last_check    DATETIME,
    last_status   INTEGER,          -- HTTP статус ответа или 0, если ошибка
    response_time INTEGER,          -- время ответа в миллисекундах
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_url ON sites(url);