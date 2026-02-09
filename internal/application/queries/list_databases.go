package queries

import (
	"context"

	"github.com/felipemalacarne/mesa/internal/domain"
	"github.com/felipemalacarne/mesa/internal/domain/connection"
)

type ListDatabases struct{}

type ListDatabasesHandler struct {
	inspector connection.Inspector
	crypto    domain.Cryptographer
}

func NewListDatabasesHandler(inspector connection.Inspector) *ListDatabasesHandler {
	return &ListDatabasesHandler{inspector: inspector}
}

func (h *ListDatabasesHandler) Handle(ctx context.Context, query ListDatabases, conn connection.Connection) ([]connection.Database, error) {
	password, err := h.crypto.Decrypt(conn.Password)
	if err != nil {
		return nil, err
	}

	databases, err := h.inspector.GetDatabases(ctx, conn, password)
	if err != nil {
		return nil, err
	}

	return databases, nil
}
