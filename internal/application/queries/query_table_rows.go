package queries

import (
	"context"

	"github.com/felipemalacarne/mesa/internal/domain"
	"github.com/felipemalacarne/mesa/internal/domain/connection"
	"github.com/google/uuid"
)

type QueryTableRows struct {
	ConnectionID uuid.UUID
	DatabaseName connection.Identifier
	TableName    connection.Identifier
	Limit        int
	Offset       int
	SortBy       *connection.Identifier
	SortOrder    string
}

type QueryTableRowsHandler struct {
	repo    connection.Repository
	crypto  domain.Cryptographer
	gateway connection.GatewayFactory
}

func NewQueryTableRowsHandler(
	repo connection.Repository,
	crypto domain.Cryptographer,
	gateway connection.GatewayFactory,
) *QueryTableRowsHandler {
	return &QueryTableRowsHandler{repo: repo, crypto: crypto, gateway: gateway}
}

func (h *QueryTableRowsHandler) Handle(ctx context.Context, query QueryTableRows) (*connection.TableRows, error) {
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

	rows, err := gateway.QueryTableRows(ctx, *conn, password, query.DatabaseName, query.TableName, query.Limit, query.Offset, query.SortBy, query.SortOrder)
	if err != nil {
		return nil, err
	}

	return rows, nil
}
