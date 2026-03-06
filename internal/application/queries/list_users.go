package queries

import (
	"context"
	"time"

	"github.com/felipemalacarne/mesa/internal/domain"
	"github.com/felipemalacarne/mesa/internal/domain/connection"
	"github.com/google/uuid"
)

type ListUsers struct {
	ConnectionID uuid.UUID
}

// ListUsersHandler retorna roles do banco remoto para a UI de segurança.
type ListUsersHandler struct {
	repo     connection.Repository
	crypto   domain.Cryptographer
	gateways connection.GatewayFactory
}

func NewListUsersHandler(repo connection.Repository, crypto domain.Cryptographer, gateways connection.GatewayFactory) *ListUsersHandler {
	return &ListUsersHandler{repo: repo, crypto: crypto, gateways: gateways}
}

func (h *ListUsersHandler) Handle(ctx context.Context, query ListUsers) ([]connection.DBUser, error) {
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

	timedCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return gateway.ListUsers(timedCtx, *conn, password)
}
