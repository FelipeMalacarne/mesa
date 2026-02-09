package queries

import (
	"context"

	"github.com/felipemalacarne/mesa/internal/domain"
	"github.com/felipemalacarne/mesa/internal/domain/connection"
)

type ListDatabases struct{}

type ListDatabasesHandler struct {
	crypto     domain.Cryptographer
	inspectors connection.InspectorFactory
}

func NewListDatabasesHandler(crypto domain.Cryptographer, inspectors connection.InspectorFactory) *ListDatabasesHandler {
	return &ListDatabasesHandler{crypto: crypto, inspectors: inspectors}
}

func (h *ListDatabasesHandler) Handle(ctx context.Context, query ListDatabases, conn connection.Connection) ([]connection.Database, error) {
	inspector, err := h.inspectors.ForDriver(conn.Driver)
	if err != nil {
		return nil, err
	}

	password, err := h.crypto.Decrypt(conn.Password)
	if err != nil {
		return nil, err
	}

	databases, err := inspector.GetDatabases(ctx, conn, password)
	if err != nil {
		return nil, err
	}

	return databases, nil
}
