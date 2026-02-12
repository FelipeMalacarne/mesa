package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/felipemalacarne/mesa/internal/domain"
	"github.com/felipemalacarne/mesa/internal/domain/connection"
	"github.com/google/uuid"
)

// CreateDatabaseCmd descreve o payload para criação de um novo banco remoto.
type CreateDatabaseCmd struct {
	ConnectionID uuid.UUID `json:"connection_id"`
	Name         string    `json:"name"`
}

type CreateDatabaseHandler struct {
	repo     connection.Repository
	crypto   domain.Cryptographer
	gateways connection.GatewayFactory
}

func NewCreateDatabaseHandler(repo connection.Repository, crypto domain.Cryptographer, gateways connection.GatewayFactory) *CreateDatabaseHandler {
	return &CreateDatabaseHandler{repo: repo, crypto: crypto, gateways: gateways}
}

func (h *CreateDatabaseHandler) Handle(ctx context.Context, cmd CreateDatabaseCmd) error {
	if strings.TrimSpace(cmd.Name) == "" {
		return fmt.Errorf("database name is required")
	}

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

	timedCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return gateway.CreateDatabase(timedCtx, *conn, password, cmd.Name)
}
