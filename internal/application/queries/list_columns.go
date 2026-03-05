package queries

import (
	"github.com/felipemalacarne/mesa/internal/domain"
	"github.com/felipemalacarne/mesa/internal/domain/connection"
	"github.com/google/uuid"
)

type ListColumns struct {
	ConnectionID uuid.UUID
	DatabaseName string `json:"database_name"`
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

// func (h *ListColumnsHandler) Handle(ctx context.Context, query ListColumns) ([]string, error) {
//
// }
