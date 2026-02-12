package queries

import (
	"context"
	"time"

	"github.com/felipemalacarne/mesa/internal/application/dtos"
	"github.com/felipemalacarne/mesa/internal/domain"
	"github.com/felipemalacarne/mesa/internal/domain/connection"
	"github.com/google/uuid"
)

// ListSessionsHandler retorna a lista de sessões ativas para monitoramento em tempo real.
type ListSessionsHandler struct {
	repo     connection.Repository
	crypto   domain.Cryptographer
	gateways connection.GatewayFactory
}

func NewListSessionsHandler(repo connection.Repository, crypto domain.Cryptographer, gateways connection.GatewayFactory) *ListSessionsHandler {
	return &ListSessionsHandler{repo: repo, crypto: crypto, gateways: gateways}
}

func (h *ListSessionsHandler) Handle(ctx context.Context, connectionID uuid.UUID) ([]*dtos.SessionDTO, error) {
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

	sessions, err := gateway.ListSessions(timedCtx, *conn, password)
	if err != nil {
		return nil, err
	}

	result := make([]*dtos.SessionDTO, 0, len(sessions))
	for _, session := range sessions {
		result = append(result, dtos.NewSessionDTO(session))
	}

	return result, nil
}
