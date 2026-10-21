-- Monitor samples kept as history (store.Metrics).
CREATE TABLE metric_points (
    connection_id uuid             NOT NULL,
    database      text             NOT NULL,
    at_ms         bigint           NOT NULL,
    tps           double precision NOT NULL DEFAULT 0,
    active        integer          NOT NULL DEFAULT 0,
    idle          integer          NOT NULL DEFAULT 0,
    idle_tx       integer          NOT NULL DEFAULT 0,
    latency_ms    double precision NOT NULL DEFAULT 0,
    size_bytes    bigint           NOT NULL DEFAULT 0
);

CREATE INDEX metric_points_series ON metric_points (connection_id, database, at_ms);
