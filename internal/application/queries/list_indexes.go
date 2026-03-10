package queries

import (
	"context"

	"github.com/felipemalacarne/mesa/internal/domain"
	"github.com/felipemalacarne/mesa/internal/domain/connection"
	"github.com/google/uuid"
)

type ListIndexes struct {
	ConnectionID uuid.UUID
	DatabaseName connection.Identifier
	TableName    connection.Identifier
}

type ListIndexesHandler struct {
	repo    connection.Repository
	crypto  domain.Cryptographer
	gateway connection.GatewayFactory
}

func NewListIndexesHandler(
	repo connection.Repository,
	crypto domain.Cryptographer,
	gateway connection.GatewayFactory,
) *ListIndexesHandler {
	return &ListIndexesHandler{repo: repo, crypto: crypto, gateway: gateway}
}

func (h *ListIndexesHandler) Handle(ctx context.Context, query ListIndexes) ([]connection.Index, error) {
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

	indexes, err := gateway.GetIndexes(ctx, *conn, password, query.DatabaseName, query.TableName)
	if err != nil {
		return nil, err
	}

	return indexes, nil
}
