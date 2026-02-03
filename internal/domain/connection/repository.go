package connection

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	Save(ctx context.Context, conn *Connection) error
	FindByID(ctx context.Context, id uuid.UUID) (*Connection, error)
	ListAll(ctx context.Context) ([]*Connection, error)
	Delete(ctx context.Context, id uuid.UUID) error
}
