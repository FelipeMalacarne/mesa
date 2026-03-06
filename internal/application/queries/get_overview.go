package queries

import (
	"context"
	"time"

	"github.com/felipemalacarne/mesa/internal/domain"
	"github.com/felipemalacarne/mesa/internal/domain/connection"
	"github.com/google/uuid"
)

// GetOverviewHandler monta o header do dashboard com os dados de saúde da conexão.
type GetOverviewHandler struct {
	repo     connection.Repository
	crypto   domain.Cryptographer
	gateways connection.GatewayFactory
}

func NewGetOverviewHandler(repo connection.Repository, crypto domain.Cryptographer, gateways connection.GatewayFactory) *GetOverviewHandler {
	return &GetOverviewHandler{repo: repo, crypto: crypto, gateways: gateways}
}

func (h *GetOverviewHandler) Handle(ctx context.Context, connectionID uuid.UUID) (*connection.ServerHealth, int64, error) {
	conn, err := h.repo.FindByID(ctx, connectionID)
	if err != nil {
		return nil, 0, err
	}
	if conn == nil {
		return nil, 0, ErrConnectionNotFound
	}

	gateway, err := h.gateways.ForDriver(conn.Driver)
	if err != nil {
		return nil, 0, err
	}

	password, err := h.crypto.Decrypt(conn.Password)
	if err != nil {
		return nil, 0, err
	}

	timedCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	start := time.Now()
	health, err := gateway.GetServerHealth(timedCtx, *conn, password)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return nil, latency, nil
	}

	return health, latency, nil
}
