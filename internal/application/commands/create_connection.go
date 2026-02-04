package commands

import (
	"context"

	"github.com/felipemalacarne/mesa/internal/domain"
	"github.com/felipemalacarne/mesa/internal/domain/connection"
)

type CreateConnection struct {
	Name     string `json:"name"`
	Driver   string `json:"driver"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type CreateConnectionHandler struct {
	repo   connection.Repository
	crypto domain.Cryptographer
}

func NewCreateConnectionHandler(r connection.Repository, c domain.Cryptographer) *CreateConnectionHandler {
	return &CreateConnectionHandler{repo: r, crypto: c}
}

func (h *CreateConnectionHandler) Handle(ctx context.Context, cmd CreateConnection) error {
	encryptedPass, err := h.crypto.Encrypt(cmd.Password)
	if err != nil {
		return err
	}

	conn, err := connection.NewConnection(
		cmd.Name,
		cmd.Driver,
		cmd.Host,
		cmd.Port,
		cmd.Username,
		encryptedPass,
	)
	if err != nil {
		return err
	}

	return h.repo.Save(ctx, conn)
}
