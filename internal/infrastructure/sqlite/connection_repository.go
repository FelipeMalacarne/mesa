package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/felipemalacarne/mesa/internal/domain/connection"
	"github.com/felipemalacarne/mesa/internal/infrastructure/sqlite/sqlc"
	"github.com/google/uuid"
)

type ConnectionRepository struct {
	queries *sqlc.Queries
}

func NewConnectionRepository(db *sql.DB) *ConnectionRepository {
	return &ConnectionRepository{
		queries: sqlc.New(db),
	}
}

func (r *ConnectionRepository) Save(ctx context.Context, conn *connection.Connection) error {
	return r.queries.UpsertConnection(ctx, sqlc.UpsertConnectionParams{
		ID:        conn.ID,
		Name:      conn.Name,
		Driver:    string(conn.Driver),
		Host:      conn.Host,
		Port:      int64(conn.Port),
		Username:  conn.Username,
		Password:  conn.Password,
		UpdatedAt: sql.NullTime{Time: conn.UpdatedAt, Valid: !conn.UpdatedAt.IsZero()},
		CreatedAt: sql.NullTime{Time: conn.CreatedAt, Valid: !conn.CreatedAt.IsZero()},
	})
}

func (r *ConnectionRepository) FindByID(ctx context.Context, id uuid.UUID) (*connection.Connection, error) {
	record, err := r.queries.GetConnection(ctx, id)
	if err != nil {
		return nil, err
	}
	return toDomainConnection(record)
}

func (r *ConnectionRepository) ListAll(ctx context.Context) ([]*connection.Connection, error) {
	rows, err := r.queries.ListConnections(ctx)
	if err != nil {
		return nil, err
	}

	connections := make([]*connection.Connection, 0, len(rows))
	for _, record := range rows {
		conn, err := toDomainConnection(record)
		if err != nil {
			return nil, err
		}
		connections = append(connections, conn)
	}

	return connections, nil
}

func (r *ConnectionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.queries.DeleteConnection(ctx, id)
}

func toDomainConnection(record sqlc.Connection) (*connection.Connection, error) {
	parsedDriver, err := connection.NewDriver(record.Driver)
	if err != nil {
		return nil, err
	}

	var createdAt, updatedAt time.Time
	if record.CreatedAt.Valid {
		createdAt = record.CreatedAt.Time
	}
	if record.UpdatedAt.Valid {
		updatedAt = record.UpdatedAt.Time
	}

	return &connection.Connection{
		ID:        record.ID,
		Name:      record.Name,
		Driver:    *parsedDriver,
		Host:      record.Host,
		Port:      int(record.Port),
		Username:  record.Username,
		Password:  record.Password,
		UpdatedAt: updatedAt,
		CreatedAt: createdAt,
	}, nil
}
