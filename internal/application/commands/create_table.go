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

type TableColumn struct {
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Length       *int    `json:"length,omitempty"`
	Precision    *int    `json:"precision,omitempty"`
	Nullable     bool    `json:"nullable"`
	PrimaryKey   bool    `json:"primary_key"`
	DefaultValue *string `json:"default_value"`
}

type TableIndex struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique"`
	Method  string   `json:"method"`
}

type CreateTableCmd struct {
	ConnectionID uuid.UUID
	DatabaseName string
	Name         string
	Columns      []TableColumn
	Indexes      []TableIndex
}

type CreateTableHandler struct {
	repo     connection.Repository
	crypto   domain.Cryptographer
	gateways connection.GatewayFactory
}

func NewCreateTableHandler(repo connection.Repository, crypto domain.Cryptographer, gateways connection.GatewayFactory) *CreateTableHandler {
	return &CreateTableHandler{repo: repo, crypto: crypto, gateways: gateways}
}

func (h *CreateTableHandler) Handle(ctx context.Context, cmd CreateTableCmd) error {
	cmd.Name = strings.TrimSpace(cmd.Name)
	cmd.DatabaseName = strings.TrimSpace(cmd.DatabaseName)

	schemaIdentifier, err := connection.NewIdentifier("public")
	if err != nil {
		return fmt.Errorf("invalid default schema: %w", err)
	}

	tableIdentifier, err := connection.NewIdentifier(cmd.Name)
	if err != nil {
		return fmt.Errorf("invalid table name: %w", err)
	}

	// Build Columns
	var columns []connection.ColumnDefinition
	knownColumns := make(map[string]struct{})

	for _, col := range cmd.Columns {
		// NewColumnDefinition handles name, type, length, precision, nullable, pk, default value validation
		def, err := connection.NewColumnDefinition(
			col.Name,
			col.Type,
			col.Length,
			col.Precision,
			col.Nullable,
			col.PrimaryKey,
			col.DefaultValue,
		)
		if err != nil {
			return fmt.Errorf("column %s: %w", col.Name, err)
		}
		columns = append(columns, def)
		knownColumns[col.Name] = struct{}{}
	}

	// Build Indexes
	var indexes []connection.IndexDefinition
	for _, idx := range cmd.Indexes {
		for _, colName := range idx.Columns {
			if _, ok := knownColumns[colName]; !ok {
				return fmt.Errorf("index %s references unknown column: %s", idx.Name, colName)
			}
		}

		def, err := connection.NewIndexDefinition(
			idx.Name,
			idx.Columns,
			idx.Method,
			idx.Unique,
		)
		if err != nil {
			return fmt.Errorf("index %s: %w", idx.Name, err)
		}
		indexes = append(indexes, def)
	}

	tableDef, err := connection.NewTableDefinition(schemaIdentifier, tableIdentifier, columns)
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

	dbNameIdentifier, err := connection.NewIdentifier(cmd.DatabaseName)
	if err != nil {
		return fmt.Errorf("invalid database name: %w", err)
	}

	if err := gateway.CreateTable(timedCtx, *conn, password, dbNameIdentifier, *tableDef); err != nil {
		return err
	}

	for _, index := range indexes {
		if err := gateway.CreateIndex(timedCtx, *conn, password, dbNameIdentifier, tableDef.Schema, tableDef.Name, index); err != nil {
			return err
		}
	}

	return nil
}
