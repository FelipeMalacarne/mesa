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
	Port      int    `json:"port"`
	Username  string `json:"username"`
	UpdatedAt string `json:"updated_at"`
	CreatedAt string `json:"created_at"`
}

type DatabaseDTO struct {
	Name string `json:"name"`
}

type TableDTO struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Size     int64  `json:"size"`
	RowCount int64  `json:"row_count"`
}

func NewConnectionDTO(conn *connection.Connection) *ConnectionDTO {
	return &ConnectionDTO{
		ID:        conn.ID.String(),
		Name:      conn.Name,
		Driver:    conn.Driver.String(),
		Host:      conn.Host,
		Port:      conn.Port,
		Username:  conn.Username,
		UpdatedAt: conn.UpdatedAt.Format(time.RFC3339),
		CreatedAt: conn.CreatedAt.Format(time.RFC3339),
	}
}

func NewDatabaseDTO(database connection.Database) *DatabaseDTO {
	return &DatabaseDTO{
		Name: database.Name,
	}
}

func NewTableDTO(table connection.Table) *TableDTO {
	return &TableDTO{
		Name:     table.Name,
		Type:     table.Type,
		Size:     table.Size,
		RowCount: table.RowCount,
	}
}
