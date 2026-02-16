package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/felipemalacarne/mesa/internal/domain"
	"github.com/felipemalacarne/mesa/internal/domain/connection"
	"github.com/google/uuid"
)

// CreateUserCmd representa a criação de um novo role no banco alvo.
type CreateUserCmd struct {
	ConnectionID uuid.UUID `json:"connection_id"`
	Username     string    `json:"username"`
	Password     string    `json:"password"`
	IsSuperUser  bool      `json:"is_superuser"`
	CanLogin     *bool     `json:"can_login"`
	ConnLimit    *int      `json:"conn_limit"`
}

type CreateUserHandler struct {
	repo     connection.Repository
	crypto   domain.Cryptographer
	gateways connection.GatewayFactory
}

func NewCreateUserHandler(repo connection.Repository, crypto domain.Cryptographer, gateways connection.GatewayFactory) *CreateUserHandler {
	return &CreateUserHandler{repo: repo, crypto: crypto, gateways: gateways}
}

func (h *CreateUserHandler) Handle(ctx context.Context, cmd CreateUserCmd) error {
	if cmd.Username == "postgres" || cmd.Username == "root" {
		return fmt.Errorf("username %s is reserved", cmd.Username)
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

	adminPass, err := h.crypto.Decrypt(conn.Password)
	if err != nil {
		return err
	}

	canLogin := true
	if cmd.CanLogin != nil {
		canLogin = *cmd.CanLogin
	}

	connLimit := -1
	if cmd.ConnLimit != nil {
		connLimit = *cmd.ConnLimit
	}

	username, err := connection.NewIdentifier(cmd.Username)
	if err != nil {
		return err
	}

	newUser := connection.DBUser{
		Name:        username,
		IsSuperUser: cmd.IsSuperUser,
		CanLogin:    canLogin,
		ConnLimit:   connLimit,
	}

	timedCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return gateway.CreateUser(timedCtx, *conn, adminPass, newUser, cmd.Password)
}
