-- Mesa's own data on PostgreSQL.

CREATE TABLE connections (
    id             uuid PRIMARY KEY,
    name           text        NOT NULL,
    engine         text        NOT NULL,
    environment    text        NOT NULL,
    host           text        NOT NULL,
    port           integer     NOT NULL,
    database       text        NOT NULL,
    "user"         text        NOT NULL,
    tls            text        NOT NULL,
    read_only      boolean     NOT NULL DEFAULT false,
    created_at     timestamptz NOT NULL,
    updated_at     timestamptz NOT NULL,
    last_opened_at timestamptz
);

CREATE TABLE secrets (
    connection_id uuid PRIMARY KEY,
    blob          bytea       NOT NULL,
    updated_at    timestamptz NOT NULL
);

CREATE TABLE audit (
    id            uuid PRIMARY KEY,
    connection_id uuid        NOT NULL,
    database      text        NOT NULL,
    environment   text        NOT NULL,
    actor         text        NOT NULL DEFAULT '',
    surface       text        NOT NULL DEFAULT '',
    summary       text        NOT NULL DEFAULT '',
    sql           text        NOT NULL DEFAULT '',
    down_sql      text        NOT NULL DEFAULT '',
    status        text        NOT NULL,
    error         text        NOT NULL DEFAULT '',
    started_at    timestamptz NOT NULL,
    finished_at   timestamptz
);

CREATE INDEX audit_started ON audit (started_at DESC);
CREATE INDEX audit_connection_started ON audit (connection_id, started_at DESC);

CREATE TABLE layouts (
    connection_id  uuid  NOT NULL,
    database       text  NOT NULL,
    positions_json jsonb NOT NULL,
    PRIMARY KEY (connection_id, database)
);

CREATE TABLE history (
    id            uuid PRIMARY KEY,
    connection_id uuid        NOT NULL,
    database      text        NOT NULL,
    sql           text        NOT NULL,
    row_count     bigint      NOT NULL DEFAULT 0,
    duration_ms   bigint      NOT NULL DEFAULT 0,
    error         text        NOT NULL DEFAULT '',
    ran_at        timestamptz NOT NULL
);

CREATE INDEX history_connection_ran ON history (connection_id, ran_at DESC);

CREATE TABLE saved_queries (
    id            uuid PRIMARY KEY,
    connection_id uuid,
    name          text        NOT NULL,
    sql           text        NOT NULL,
    updated_at    timestamptz NOT NULL
);
