package queries

import (
	"context"
	"time"

	"github.com/felipemalacarne/mesa/internal/application/dtos"
	"github.com/felipemalacarne/mesa/internal/domain"
	"github.com/felipemalacarne/mesa/internal/domain/connection"
)

type ListConnections struct{}

type ListConnectionsHandler struct {
	repo       connection.Repository
	crypto     domain.Cryptographer
	inspectors connection.InspectorFactory
}

func NewListConnectionsHandler(
	repo connection.Repository,
	crypto domain.Cryptographer,
	inspectors connection.InspectorFactory,
) *ListConnectionsHandler {
	return &ListConnectionsHandler{
		repo:       repo,
		crypto:     crypto,
		inspectors: inspectors,
	}
}

func (h *ListConnectionsHandler) Handle(ctx context.Context, query ListConnections) ([]*dtos.ConnectionDTO, error) {
	conns, err := h.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	connDTOs := make([]*dtos.ConnectionDTO, len(conns))

	for i, conn := range conns {
		status, statusErr := h.checkStatus(ctx, conn)
		dto := dtos.NewConnectionDTO(conn)
		dto.Status = status
		dto.StatusErr = statusErr
		connDTOs[i] = dto
	}

	return connDTOs, nil
}

func (h *ListConnectionsHandler) checkStatus(ctx context.Context, conn *connection.Connection) (string, string) {
	inspector, err := h.inspectors.ForDriver(conn.Driver)
	if err != nil {
		return dtos.StatusError, err.Error()
	}

	password, err := h.crypto.Decrypt(conn.Password)
	if err != nil {
		return dtos.StatusError, err.Error()
	}

	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := inspector.Ping(pingCtx, *conn, password); err != nil {
		return dtos.StatusError, err.Error()
	}

	return dtos.StatusOK, ""
}
