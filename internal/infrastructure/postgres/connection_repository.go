package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/felipemalacarne/mesa/internal/domain/connection"
	"github.com/felipemalacarne/mesa/internal/infrastructure/postgres/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ConnectionRepository struct {
	queries *sqlc.Queries
}

var (
	errNullConnectionID        = errors.New("connection id is NULL")
	errNullConnectionCreatedAt = errors.New("connection created_at is NULL")
)

func NewConnectionRepository(pool *pgxpool.Pool) *ConnectionRepository {
	return &ConnectionRepository{
		queries: sqlc.New(pool),
	}
}

func toDomainConnection(record sqlc.Connection) (*connection.Connection, error) {
	id, err := uuidFromPg(record.ID)
	if err != nil {
		return nil, err
	}

	createdAt, err := timeFromPg(record.CreatedAt)
	if err != nil {
		return nil, err
	}

	parsedDriver, err := connection.NewDriver(record.Driver)
	if err != nil {
		return nil, err
	}

	return &connection.Connection{
		ID:        id,
		Name:      record.Name,
		Driver:    *parsedDriver,
		Host:      record.Host,
		Port:      int(record.Port),
		Username:  record.Username,
		Password:  record.Password,
		CreatedAt: createdAt,
	}, nil
}

func (r *ConnectionRepository) Save(ctx context.Context, conn *connection.Connection) error {
	return r.queries.UpsertConnection(ctx, sqlc.UpsertConnectionParams{
		ID:        pgtype.UUID{Bytes: conn.ID, Valid: true},
		Name:      conn.Name,
		Driver:    string(conn.Driver),
		Host:      conn.Host,
		Port:      int32(conn.Port),
		Username:  conn.Username,
		Password:  conn.Password,
		CreatedAt: pgtype.Timestamptz{Time: conn.CreatedAt, Valid: true},
	})
}

func (r *ConnectionRepository) FindByID(ctx context.Context, id uuid.UUID) (*connection.Connection, error) {
	record, err := r.queries.GetConnection(ctx, pgtype.UUID{Bytes: id, Valid: true})
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
	return r.queries.DeleteConnection(ctx, pgtype.UUID{Bytes: id, Valid: true})
}

func uuidFromPg(value pgtype.UUID) (uuid.UUID, error) {
	if !value.Valid {
		return uuid.Nil, errNullConnectionID
	}

	return uuid.UUID(value.Bytes), nil
}

func timeFromPg(value pgtype.Timestamptz) (time.Time, error) {
	if !value.Valid {
		return time.Time{}, errNullConnectionCreatedAt
	}

	return value.Time, nil
}
