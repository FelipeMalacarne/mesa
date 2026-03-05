package queries

import (
	"context"

	"github.com/felipemalacarne/mesa/internal/domain"
	"github.com/felipemalacarne/mesa/internal/domain/connection"
	"github.com/google/uuid"
)

type ListColumns struct {
	ConnectionID uuid.UUID
	DatabaseName connection.Identifier
	TableName    connection.Identifier
}

type ListColumnsHandler struct {
	repo    connection.Repository
	crypto  domain.Cryptographer
	gateway connection.GatewayFactory
}

func NewListColumnsHandler(
	repo connection.Repository,
	crypto domain.Cryptographer,
	gateway connection.GatewayFactory,
) *ListColumnsHandler {
	return &ListColumnsHandler{repo: repo, crypto: crypto, gateway: gateway}
}

func (h *ListColumnsHandler) Handle(ctx context.Context, query ListColumns) ([]connection.Column, error) {
	conn, err := h.repo.FindByID(ctx, query.ConnectionID)
	if err != nil {
		return nil, err
	}

	if conn == nil {
		return nil, ErrConnectionNotFound
	}

	gateway, err := h.gateway.ForDriver(conn.Driver)
	if err != nil {
		return nil, err
	}

	password, err := h.crypto.Decrypt(conn.Password)
	if err != nil {
		return nil, err
	}
	
	columns, err := gateway.GetColumns(ctx, *conn, password, query.DatabaseName, query.TableName)
	if err !=nil {
		return nil, err
	}

	return columns, nil
}
