package commands

import (
	"context"
	"time"

	"github.com/felipemalacarne/mesa/internal/domain"
	"github.com/felipemalacarne/mesa/internal/domain/connection"
	"github.com/google/uuid"
)

// CreateDatabaseCmd descreve o payload para criação de um novo banco remoto.
type CreateDatabaseCmd struct {
	ConnectionID uuid.UUID `json:"connection_id"`
	Name         string    `json:"name"`
	Owner        string    `json:"owner"`
}

type CreateDatabaseHandler struct {
	repo     connection.Repository
	crypto   domain.Cryptographer
	gateways connection.GatewayFactory
}

func NewCreateDatabaseHandler(
	repo connection.Repository,
	crypto domain.Cryptographer,
	gateways connection.GatewayFactory,
) *CreateDatabaseHandler {
	return &CreateDatabaseHandler{repo: repo, crypto: crypto, gateways: gateways}
}

func (h *CreateDatabaseHandler) Handle(ctx context.Context, cmd CreateDatabaseCmd) error {

	name, err := connection.NewIdentifier(cmd.Name)
	if err != nil {
		return err
	}

	owner, err := connection.NewIdentifier(cmd.Owner)
	if err != nil {
		return err
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

	return gateway.CreateDatabase(timedCtx, *conn, password, name, owner)
}
