package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/felipemalacarne/mesa/internal/domain"
	"github.com/felipemalacarne/mesa/internal/domain/connection"
	"github.com/google/uuid"
)

type UpdateTableRowCmd struct {
	ConnectionID uuid.UUID
	DatabaseName connection.Identifier
	TableName    connection.Identifier
	Where        map[connection.Identifier]any
	Set          map[connection.Identifier]any
}

type UpdateTableRowHandler struct {
	repo     connection.Repository
	crypto   domain.Cryptographer
	gateways connection.GatewayFactory
}

func NewUpdateTableRowHandler(
	repo connection.Repository,
	crypto domain.Cryptographer,
	gateways connection.GatewayFactory,
) *UpdateTableRowHandler {
	return &UpdateTableRowHandler{repo: repo, crypto: crypto, gateways: gateways}
}

func (h *UpdateTableRowHandler) Handle(ctx context.Context, cmd UpdateTableRowCmd) error {
	if len(cmd.Where) == 0 {
		return fmt.Errorf("%w: where clause is required", ErrInvalidInput)
	}
	if len(cmd.Set) == 0 {
		return fmt.Errorf("%w: set clause is required", ErrInvalidInput)
	}

	conn, err := h.repo.FindByID(ctx, cmd.ConnectionID)
	if err != nil {
		return err
	}
	if conn == nil {
		return ErrConnectionNotFound
	}

	password, err := h.crypto.Decrypt(conn.Password)
	if err != nil {
		return err
	}

	gateway, err := h.gateways.ForDriver(conn.Driver)
	if err != nil {
		return err
	}

	timedCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return gateway.UpdateTableRow(timedCtx, *conn, password, cmd.DatabaseName, cmd.TableName, cmd.Where, cmd.Set)
}
