package queries

import (
	"context"

	"github.com/felipemalacarne/mesa/internal/application/dtos"
	"github.com/felipemalacarne/mesa/internal/domain"
	"github.com/felipemalacarne/mesa/internal/domain/connection"
)

type ListTables struct {
	DatabaseName string
}

type ListTablesHandler struct {
	crypto   domain.Cryptographer
	gateways connection.GatewayFactory
}

func NewListTablesHandler(crypto domain.Cryptographer, gateways connection.GatewayFactory) *ListTablesHandler {
	return &ListTablesHandler{crypto: crypto, gateways: gateways}
}

func (h *ListTablesHandler) Handle(ctx context.Context, query ListTables, conn connection.Connection) ([]*dtos.TableDTO, error) {
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

	tables, err := gateway.GetTables(ctx, conn, password, dbName)
	if err != nil {
		return nil, err
	}

	result := make([]*dtos.TableDTO, 0, len(tables))
	for _, table := range tables {
		result = append(result, dtos.NewTableDTO(table))
	}

	return result, nil
}
