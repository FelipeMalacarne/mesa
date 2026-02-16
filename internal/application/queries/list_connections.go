package queries

import (
	"context"

	"github.com/felipemalacarne/mesa/internal/application/dtos"
	"github.com/felipemalacarne/mesa/internal/domain/connection"
)

type ListConnections struct{}

type ListConnectionsHandler struct {
	repo connection.Repository
}

func NewListConnectionsHandler(repo connection.Repository) *ListConnectionsHandler {
	return &ListConnectionsHandler{
		repo: repo,
	}
}

func (h *ListConnectionsHandler) Handle(ctx context.Context, query ListConnections) ([]*dtos.ConnectionDTO, error) {
	conns, err := h.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	connDTOs := make([]*dtos.ConnectionDTO, len(conns))

	for i, conn := range conns {
		dto := dtos.NewConnectionDTO(conn)
		dto.Status = "unknown"
		connDTOs[i] = dto
	}

	return connDTOs, nil
}
