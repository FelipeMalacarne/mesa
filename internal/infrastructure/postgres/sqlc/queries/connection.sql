-- name: UpsertConnection :exec
INSERT INTO connections (
    id,
    name,
    driver,
    host,
    port,
    username,
    password,
    created_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
ON CONFLICT (id) DO UPDATE
SET name = EXCLUDED.name,
    driver = EXCLUDED.driver,
    host = EXCLUDED.host,
    port = EXCLUDED.port,
    username = EXCLUDED.username,
    password = EXCLUDED.password;

-- name: GetConnection :one
SELECT id, name, driver, host, port, username, password, created_at
FROM connections
WHERE id = $1;

-- name: ListConnections :many
SELECT id, name, driver, host, port, username, password, created_at
FROM connections
ORDER BY created_at DESC
LIMIT 100;

-- name: DeleteConnection :exec
DELETE FROM connections
WHERE id = $1;
