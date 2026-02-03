package queries

import (
	"context"

	"github.com/felipemalacarne/mesa/internal/domain/connection"
)

type ListConnections struct{}

type ListConnectionsHandler struct {
	repo connection.Repository
}

func NewListConnectionsHandler(repo connection.Repository) *ListConnectionsHandler {
	return &ListConnectionsHandler{repo: repo}
}

func (h *ListConnectionsHandler) Handle(ctx context.Context, cmd *ListConnections) ([]*connection.Connection, error) {
	conns, err := h.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	return conns, nil
}
