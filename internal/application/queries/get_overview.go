package queries

import (
	"context"
	"time"

	"github.com/felipemalacarne/mesa/internal/application/dtos"
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

func (h *GetOverviewHandler) Handle(ctx context.Context, connectionID uuid.UUID) (*dtos.OverviewDTO, error) {
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

	timedCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	start := time.Now()
	health, err := gateway.GetServerHealth(timedCtx, *conn, password)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return dtos.NewOverviewUnreachableDTO(latency), nil
	}

	return dtos.NewOverviewDTO(health, latency), nil
}
