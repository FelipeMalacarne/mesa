package queries

import (
	"context"
	"time"

	"github.com/felipemalacarne/mesa/internal/application/dtos"
	"github.com/felipemalacarne/mesa/internal/domain"
	"github.com/felipemalacarne/mesa/internal/domain/connection"
	"github.com/google/uuid"
)

type PingConnection struct {
	ConnectionID uuid.UUID
}

type PingConnectionResponse struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type PingConnectionHandler struct {
	repo     connection.Repository
	crypto   domain.Cryptographer
	gateways connection.GatewayFactory
}

func NewPingConnectionHandler(
	repo connection.Repository,
	crypto domain.Cryptographer,
	gateways connection.GatewayFactory,
) *PingConnectionHandler {
	return &PingConnectionHandler{
		repo:     repo,
		crypto:   crypto,
		gateways: gateways,
	}
}

func (h *PingConnectionHandler) Handle(ctx context.Context, query PingConnection) (PingConnectionResponse, error) {
	conn, err := h.repo.FindByID(ctx, query.ConnectionID)
	if err != nil {
		return PingConnectionResponse{Status: dtos.StatusError, Error: err.Error()}, nil
	}
	if conn == nil {
		return PingConnectionResponse{Status: dtos.StatusError, Error: "connection not found"}, nil
	}

	password, err := h.crypto.Decrypt(conn.Password)
	if err != nil {
		return PingConnectionResponse{Status: dtos.StatusError, Error: err.Error()}, nil
	}

	gateway, err := h.gateways.ForDriver(conn.Driver)
	if err != nil {
		return PingConnectionResponse{Status: dtos.StatusError, Error: err.Error()}, nil
	}

	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := gateway.Ping(pingCtx, *conn, password); err != nil {
		return PingConnectionResponse{Status: dtos.StatusError, Error: err.Error()}, nil
	}

	return PingConnectionResponse{Status: dtos.StatusOK}, nil
}
