package queries

import (
	"context"

	"github.com/felipemalacarne/mesa/internal/application/dtos"
	"github.com/felipemalacarne/mesa/internal/domain"
	"github.com/felipemalacarne/mesa/internal/domain/connection"
	"github.com/google/uuid"
)

type ListDatabases struct {
	ConnectionID uuid.UUID
}

type ListDatabasesHandler struct {
	repo     connection.Repository
	crypto   domain.Cryptographer
	gateways connection.GatewayFactory
}

func NewListDatabasesHandler(repo connection.Repository, crypto domain.Cryptographer, gateways connection.GatewayFactory) *ListDatabasesHandler {
	return &ListDatabasesHandler{repo: repo, crypto: crypto, gateways: gateways}
}

func (h *ListDatabasesHandler) Handle(ctx context.Context, query ListDatabases) ([]*dtos.DatabaseDTO, error) {
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

	databases, err := gateway.GetDatabases(ctx, *conn, password)
	if err != nil {
		return nil, err
	}

	result := make([]*dtos.DatabaseDTO, 0, len(databases))
	for _, database := range databases {
		result = append(result, dtos.NewDatabaseDTO(database))
	}

	return result, nil
}
