-- People who signed in through the identity provider, and their browser
-- sessions and API tokens. Mesa keeps no passwords.

CREATE TABLE users (
    id           TEXT PRIMARY KEY,
    subject      TEXT NOT NULL UNIQUE,
    email        TEXT NOT NULL DEFAULT '',
    name         TEXT NOT NULL,
    role         TEXT NOT NULL,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    last_seen_at TEXT
);

CREATE TABLE tokens (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind         TEXT NOT NULL,
    name         TEXT NOT NULL DEFAULT '',
    hash         BLOB NOT NULL UNIQUE,
    created_at   TEXT NOT NULL,
    expires_at   TEXT,
    last_used_at TEXT
);

CREATE INDEX tokens_user ON tokens (user_id, kind);
