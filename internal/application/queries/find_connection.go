package queries

import (
	"context"
	"errors"

	"github.com/felipemalacarne/mesa/internal/domain/connection"
	"github.com/google/uuid"
)

var ConnectionNotFoundError = errors.New("connection not found")

type FindConnection struct {
	ConnectionID uuid.UUID
}

type FindConnectionHandler struct {
	repo connection.Repository
}

func NewFindConnectionHandler(repo connection.Repository) *FindConnectionHandler {
	return &FindConnectionHandler{repo: repo}
}

func (h *FindConnectionHandler) Handle(ctx context.Context, query FindConnection) (*connection.Connection, error) {
	conn, err := h.repo.FindByID(ctx, query.ConnectionID)
	if err != nil {
		return nil, err
	}

	if conn == nil {
		return nil, ConnectionNotFoundError
	}

	return conn, nil
}
