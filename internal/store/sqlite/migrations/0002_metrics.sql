-- Monitor samples kept as history (store.Metrics).
CREATE TABLE metric_points (
    connection_id TEXT    NOT NULL,
    database      TEXT    NOT NULL,
    at_ms         INTEGER NOT NULL,
    tps           REAL    NOT NULL DEFAULT 0,
    active        INTEGER NOT NULL DEFAULT 0,
    idle          INTEGER NOT NULL DEFAULT 0,
    idle_tx       INTEGER NOT NULL DEFAULT 0,
    latency_ms    REAL    NOT NULL DEFAULT 0,
    size_bytes    INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX metric_points_series ON metric_points (connection_id, database, at_ms);
