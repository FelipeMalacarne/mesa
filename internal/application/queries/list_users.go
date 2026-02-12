package queries

import (
	"context"
	"time"

	"github.com/felipemalacarne/mesa/internal/application/dtos"
	"github.com/felipemalacarne/mesa/internal/domain"
	"github.com/felipemalacarne/mesa/internal/domain/connection"
	"github.com/google/uuid"
)

// ListUsersHandler retorna roles do banco remoto para a UI de segurança.
type ListUsersHandler struct {
	repo     connection.Repository
	crypto   domain.Cryptographer
	gateways connection.GatewayFactory
}

func NewListUsersHandler(repo connection.Repository, crypto domain.Cryptographer, gateways connection.GatewayFactory) *ListUsersHandler {
	return &ListUsersHandler{repo: repo, crypto: crypto, gateways: gateways}
}

func (h *ListUsersHandler) Handle(ctx context.Context, connectionID uuid.UUID) ([]*dtos.DBUserDTO, error) {
	conn, err := h.repo.FindByID(ctx, connectionID)
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

	users, err := gateway.ListUsers(timedCtx, *conn, password)
	if err != nil {
		return nil, err
	}

	result := make([]*dtos.DBUserDTO, 0, len(users))
	for _, user := range users {
		result = append(result, dtos.NewDBUserDTO(user))
	}

	return result, nil
}
