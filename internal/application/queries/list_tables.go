package queries

import (
	"context"

	"github.com/felipemalacarne/mesa/internal/domain"
	"github.com/felipemalacarne/mesa/internal/domain/connection"
)

type ListTables struct {
	DatabaseName string
}

type ListTablesHandler struct {
	crypto     domain.Cryptographer
	inspectors connection.InspectorFactory
}

func NewListTablesHandler(crypto domain.Cryptographer, inspectors connection.InspectorFactory) *ListTablesHandler {
	return &ListTablesHandler{crypto: crypto, inspectors: inspectors}
}

func (h *ListTablesHandler) Handle(ctx context.Context, query ListTables, conn connection.Connection) ([]connection.Table, error) {
	inspector, err := h.inspectors.ForDriver(conn.Driver)
	if err != nil {
		return nil, err
	}

	password, err := h.crypto.Decrypt(conn.Password)
	if err != nil {
		return nil, err
	}

	tables, err := inspector.GetTables(ctx, conn, password, query.DatabaseName)
	if err != nil {
		return nil, err
	}

	return tables, nil
}
