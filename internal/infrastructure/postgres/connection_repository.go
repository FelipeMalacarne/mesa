package postgres

import (
	"context"
	"time"

	"github.com/felipemalacarne/mesa/internal/domain/connection"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ConnectionRepository struct {
	db *pgxpool.Pool
}

func NewConnectionRepository(pool *pgxpool.Pool) *ConnectionRepository {
	return &ConnectionRepository{
		db: pool,
	}
}

type connectionRecord struct {
	ID        uuid.UUID `db:"id"`
	Name      string    `db:"name"`
	Driver    string    `db:"driver"`
	Host      string    `db:"host"`
	Port      int       `db:"port"`
	Username  string    `db:"username"`
	Password  string    `db:"password"`
	CreatedAt time.Time `db:"created_at"`
}

func (cr *connectionRecord) toDomain() (*connection.Connection, error) {
	driver, err := connection.NewDriver(cr.Driver)
	if err != nil {
		return nil, err
	}

	return &connection.Connection{
		ID:        cr.ID,
		Name:      cr.Name,
		Driver:    *driver,
		Host:      cr.Host,
		Port:      cr.Port,
		Username:  cr.Username,
		Password:  cr.Password,
		CreatedAt: cr.CreatedAt,
	}, nil
}

func (r *ConnectionRepository) Save(ctx context.Context, conn *connection.Connection) error {
	const query = `
		INSERT INTO connections (id, name, driver, host, port, username, password, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE
		SET name = EXCLUDED.name,
			driver = EXCLUDED.driver,
			host = EXCLUDED.host,
			port = EXCLUDED.port,
			username = EXCLUDED.username,
			password = EXCLUDED.password
	`

	_, err := r.db.Exec(
		ctx,
		query,
		conn.ID,
		conn.Name,
		string(conn.Driver),
		conn.Host,
		conn.Port,
		conn.Username,
		conn.Password,
		conn.CreatedAt,
	)
	return err
}

func (r *ConnectionRepository) FindByID(ctx context.Context, id uuid.UUID) (*connection.Connection, error) {
	const query = `
		SELECT id, name, driver, host, port, username, password, created_at
		FROM connections
		WHERE id = $1
	`

	var record connectionRecord
	err := r.db.QueryRow(ctx, query, id).Scan(
		&record.ID,
		&record.Name,
		&record.Driver,
		&record.Host,
		&record.Port,
		&record.Username,
		&record.Password,
		&record.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return record.toDomain()
}

func (r *ConnectionRepository) ListAll(ctx context.Context) ([]*connection.Connection, error) {
	const query = `
		SELECT id, name, driver, host, port, username, password, created_at
		FROM connections
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var connections []*connection.Connection
	for rows.Next() {
		var record connectionRecord
		if err := rows.Scan(
			&record.ID,
			&record.Name,
			&record.Driver,
			&record.Host,
			&record.Port,
			&record.Username,
			&record.Password,
			&record.CreatedAt,
		); err != nil {
			return nil, err
		}

		conn, err := record.toDomain()
		if err != nil {
			return nil, err
		}
		connections = append(connections, conn)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return connections, nil
}

func (r *ConnectionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const query = `
		DELETE FROM connections
		WHERE id = $1
	`

	_, err := r.db.Exec(ctx, query, id)
	return err
}
