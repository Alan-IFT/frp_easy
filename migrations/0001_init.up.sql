CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    applied_at TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE admin (
    id              INTEGER PRIMARY KEY CHECK (id = 1),
    username        TEXT    NOT NULL,
    password_hash   TEXT    NOT NULL,
    updated_at      TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE sessions (
    token       TEXT    PRIMARY KEY,
    csrf_token  TEXT    NOT NULL,
    created_at  TEXT    NOT NULL DEFAULT (datetime('now')),
    expires_at  TEXT    NOT NULL
);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

CREATE TABLE kv (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
-- 预期 key：mode.frpc.enabled, mode.frps.enabled,
--          frps.config (JSON), frpc.serverConn (JSON),
--          frpc.admin (JSON: addr/port/user/pass),
--          loginfail.<ip> (JSON: count/firstAt)

CREATE TABLE proxies (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT    NOT NULL UNIQUE,
    type            TEXT    NOT NULL CHECK (type IN ('tcp','udp','http','https')),
    local_ip        TEXT    NOT NULL DEFAULT '127.0.0.1',
    local_port      INTEGER NOT NULL CHECK (local_port BETWEEN 1 AND 65535),
    remote_port     INTEGER,
    custom_domains  TEXT,    -- JSON 数组；http/https 才用
    enabled         INTEGER NOT NULL DEFAULT 1,
    version         INTEGER NOT NULL DEFAULT 1,  -- last-write-wins 校验
    updated_at      TEXT    NOT NULL DEFAULT (datetime('now')),
    CHECK (
        (type IN ('tcp','udp') AND remote_port IS NOT NULL AND custom_domains IS NULL)
     OR (type IN ('http','https') AND remote_port IS NULL AND custom_domains IS NOT NULL)
    )
);
CREATE UNIQUE INDEX idx_proxies_tcp_remote ON proxies(type, remote_port)
    WHERE type IN ('tcp','udp');
-- customDomain 唯一约束在应用层做（解析 JSON 后比对）。

INSERT INTO schema_migrations(version) VALUES (1);
