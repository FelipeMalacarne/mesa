package queries

import (
	"context"

	"github.com/felipemalacarne/mesa/internal/domain"
	"github.com/felipemalacarne/mesa/internal/domain/connection"
	"github.com/google/uuid"
)

type ListTables struct {
	ConnectionID uuid.UUID
	DatabaseName string
}

type ListTablesHandler struct {
	repo     connection.Repository
	crypto   domain.Cryptographer
	gateways connection.GatewayFactory
}

func NewListTablesHandler(repo connection.Repository, crypto domain.Cryptographer, gateways connection.GatewayFactory) *ListTablesHandler {
	return &ListTablesHandler{repo: repo, crypto: crypto, gateways: gateways}
}

func (h *ListTablesHandler) Handle(ctx context.Context, query ListTables) ([]connection.Table, error) {
	conn, err := h.repo.FindByID(ctx, query.ConnectionID)
	if err != nil {
		return nil, err
	}

	if conn == nil {
		return nil, ErrConnectionNotFound
	}

	gateway, err := h.gateways.ForDriver(conn.Driver)
	if err != nil {
		return nil, err
	}

	password, err := h.crypto.Decrypt(conn.Password)
	if err != nil {
		return nil, err
	}

	dbName, err := connection.NewIdentifier(query.DatabaseName)
	if err != nil {
		return nil, err
	}

	return gateway.GetTables(ctx, *conn, password, dbName)
}
