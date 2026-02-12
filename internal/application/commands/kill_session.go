package commands

import (
	"context"
	"time"

	"github.com/felipemalacarne/mesa/internal/domain"
	"github.com/felipemalacarne/mesa/internal/domain/connection"
	"github.com/google/uuid"
)

// KillSessionCmd carrega os dados necessários para encerrar um backend remoto.
type KillSessionCmd struct {
	ConnectionID uuid.UUID `json:"connection_id"`
	PID          int       `json:"pid"`
}

type KillSessionHandler struct {
	repo     connection.Repository
	crypto   domain.Cryptographer
	gateways connection.GatewayFactory
}

func NewKillSessionHandler(repo connection.Repository, crypto domain.Cryptographer, gateways connection.GatewayFactory) *KillSessionHandler {
	return &KillSessionHandler{repo: repo, crypto: crypto, gateways: gateways}
}

func (h *KillSessionHandler) Handle(ctx context.Context, cmd KillSessionCmd) error {
	conn, err := h.repo.FindByID(ctx, cmd.ConnectionID)
	if err != nil {
		return err
	}
	if conn == nil {
		return ErrConnectionNotFound
	}

	gateway, err := h.gateways.ForDriver(conn.Driver)
	if err != nil {
		return err
	}

	password, err := h.crypto.Decrypt(conn.Password)
	if err != nil {
		return err
	}

	timedCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return gateway.KillSession(timedCtx, *conn, password, cmd.PID)
}
