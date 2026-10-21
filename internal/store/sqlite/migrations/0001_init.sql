-- Mesa's own data on SQLite. Ids are UUID text; timestamps are RFC 3339 text
-- in UTC with nine fractional digits so that text order is time order.

CREATE TABLE connections (
    id             TEXT PRIMARY KEY,
    name           TEXT    NOT NULL,
    engine         TEXT    NOT NULL,
    environment    TEXT    NOT NULL,
    host           TEXT    NOT NULL,
    port           INTEGER NOT NULL,
    database       TEXT    NOT NULL,
    "user"         TEXT    NOT NULL,
    tls            TEXT    NOT NULL,
    read_only      INTEGER NOT NULL DEFAULT 0,
    created_at     TEXT    NOT NULL,
    updated_at     TEXT    NOT NULL,
    last_opened_at TEXT
);

CREATE TABLE secrets (
    connection_id TEXT PRIMARY KEY,
    blob          BLOB NOT NULL,
    updated_at    TEXT NOT NULL
);

CREATE TABLE audit (
    id            TEXT PRIMARY KEY,
    connection_id TEXT NOT NULL,
    database      TEXT NOT NULL,
    environment   TEXT NOT NULL,
    actor         TEXT NOT NULL DEFAULT '',
    surface       TEXT NOT NULL DEFAULT '',
    summary       TEXT NOT NULL DEFAULT '',
    sql           TEXT NOT NULL DEFAULT '',
    down_sql      TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL,
    error         TEXT NOT NULL DEFAULT '',
    started_at    TEXT NOT NULL,
    finished_at   TEXT
);

CREATE INDEX audit_started ON audit (started_at);
CREATE INDEX audit_connection_started ON audit (connection_id, started_at);

CREATE TABLE layouts (
    connection_id  TEXT NOT NULL,
    database       TEXT NOT NULL,
    positions_json TEXT NOT NULL,
    PRIMARY KEY (connection_id, database)
);

CREATE TABLE history (
    id            TEXT PRIMARY KEY,
    connection_id TEXT    NOT NULL,
    database      TEXT    NOT NULL,
    sql           TEXT    NOT NULL,
    row_count     INTEGER NOT NULL DEFAULT 0,
    duration_ms   INTEGER NOT NULL DEFAULT 0,
    error         TEXT    NOT NULL DEFAULT '',
    ran_at        TEXT    NOT NULL
);

CREATE INDEX history_connection_ran ON history (connection_id, ran_at);

CREATE TABLE saved_queries (
    id            TEXT PRIMARY KEY,
    connection_id TEXT,
    name          TEXT NOT NULL,
    sql           TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);
