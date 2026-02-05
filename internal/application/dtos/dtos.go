package dtos

import (
	"time"

	"github.com/felipemalacarne/mesa/internal/domain/connection"
)

type ConnectionDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Driver    string `json:"driver"`
	Host      string `json:"host"`
	Username  string `json:"username"`
	UpdatedAt string `json:"updated_at"`
	CreatedAt string `json:"created_at"`
}

func NewConnectionDTO(conn *connection.Connection) *ConnectionDTO {
	return &ConnectionDTO{
		ID:        conn.ID.String(),
		Name:      conn.Name,
		Driver:    conn.Driver.String(),
		Host:      conn.Host,
		Username:  conn.Username,
		UpdatedAt: conn.UpdatedAt.Format(time.RFC3339),
		CreatedAt: conn.CreatedAt.Format(time.RFC3339),
	}
}
