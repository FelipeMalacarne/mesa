-- People who signed in through the identity provider, and their browser
-- sessions and API tokens. Mesa keeps no passwords.

CREATE TABLE users (
    id           uuid PRIMARY KEY,
    subject      text        NOT NULL UNIQUE,
    email        text        NOT NULL DEFAULT '',
    name         text        NOT NULL,
    role         text        NOT NULL,
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL,
    last_seen_at timestamptz
);

CREATE TABLE tokens (
    id           uuid PRIMARY KEY,
    user_id      uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind         text        NOT NULL,
    name         text        NOT NULL DEFAULT '',
    hash         bytea       NOT NULL UNIQUE,
    created_at   timestamptz NOT NULL,
    expires_at   timestamptz,
    last_used_at timestamptz
);

CREATE INDEX tokens_user ON tokens (user_id, kind);
