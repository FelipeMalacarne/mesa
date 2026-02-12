package queries

import (
	"context"

	"github.com/felipemalacarne/mesa/internal/application/dtos"
	"github.com/felipemalacarne/mesa/internal/domain"
	"github.com/felipemalacarne/mesa/internal/domain/connection"
)

type ListDatabases struct{}

type ListDatabasesHandler struct {
	crypto   domain.Cryptographer
	gateways connection.GatewayFactory
}

func NewListDatabasesHandler(crypto domain.Cryptographer, gateways connection.GatewayFactory) *ListDatabasesHandler {
	return &ListDatabasesHandler{crypto: crypto, gateways: gateways}
}

func (h *ListDatabasesHandler) Handle(ctx context.Context, query ListDatabases, conn connection.Connection) ([]*dtos.DatabaseDTO, error) {
	gateway, err := h.gateways.ForDriver(conn.Driver)
	if err != nil {
		return nil, err
	}

	password, err := h.crypto.Decrypt(conn.Password)
	if err != nil {
		return nil, err
	}

	databases, err := gateway.GetDatabases(ctx, conn, password)
	if err != nil {
		return nil, err
	}

	result := make([]*dtos.DatabaseDTO, 0, len(databases))
	for _, database := range databases {
		result = append(result, dtos.NewDatabaseDTO(database))
	}

	return result, nil
}
