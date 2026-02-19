-- name: UpsertConnection :exec
INSERT INTO connections (
    id,
    name,
    driver,
    host,
    port,
    username,
    password,
    updated_at,
    created_at
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?
)
ON CONFLICT (id) DO UPDATE
SET name = excluded.name,
    driver = excluded.driver,
    host = excluded.host,
    port = excluded.port,
    username = excluded.username,
    password = excluded.password,
    updated_at = excluded.updated_at,
    created_at = excluded.created_at;

-- name: GetConnection :one
SELECT id, name, driver, host, port, username, password, updated_at, created_at
FROM connections
WHERE id = ?;

-- name: ListConnections :many
SELECT id, name, driver, host, port, username, password, updated_at, created_at
FROM connections
ORDER BY created_at DESC
LIMIT 100;

-- name: DeleteConnection :exec
DELETE FROM connections
WHERE id = ?;
