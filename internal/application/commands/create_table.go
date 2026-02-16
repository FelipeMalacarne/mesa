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

	if err := validateCreateTableCmd(cmd); err != nil {
		return err
	}

	tableDef, indexDefs, err := buildDefinitions(cmd)
	if err != nil {
		return err
	}

	conn, password, err := h.getConnectionDetails(ctx, cmd.ConnectionID)
	if err != nil {
		return err
	}

	gateway, err := h.gateways.ForDriver(conn.Driver)
	if err != nil {
		return err
	}

	return h.executeCreateTableAndIndexes(ctx, gateway, *conn, password, cmd.DatabaseName, tableDef, indexDefs)
}

func validateCreateTableCmd(cmd CreateTableCmd) error {
	if cmd.DatabaseName == "" {
		return fmt.Errorf("database name is required")
	}
	if cmd.Name == "" {
		return fmt.Errorf("table name is required")
	}
	if len(cmd.Columns) == 0 {
		return fmt.Errorf("at least one column is required")
	}
	return nil
}

func buildDefinitions(cmd CreateTableCmd) (*connection.TableDefinition, []connection.IndexDefinition, error) {
	schemaIdentifier, err := connection.NewIdentifier("public")
	if err != nil {
		return nil, nil, fmt.Errorf("invalid default schema: %w", err)
	}

	tableIdentifier, err := connection.NewIdentifier(cmd.Name)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid table name: %w", err)
	}

	columns, columnNames, err := buildColumnDefinitions(cmd.Columns)
	if err != nil {
		return nil, nil, err
	}

	indexes, err := buildIndexDefinitions(cmd.Indexes, columnNames)
	if err != nil {
		return nil, nil, err
	}

	table, err := connection.NewTableDefinition(schemaIdentifier, tableIdentifier, columns)
	if err != nil {
		return nil, nil, err
	}

	return table, indexes, nil
}

func buildColumnDefinitions(cols []TableColumn) ([]connection.ColumnDefinition, map[string]struct{}, error) {
	columns := make([]connection.ColumnDefinition, 0, len(cols))
	columnNames := make(map[string]struct{}, len(cols))

	for _, column := range cols {
		name := strings.TrimSpace(column.Name)
		if name == "" {
			return nil, nil, fmt.Errorf("column name is required")
		}

		if column.Type == "" {
			return nil, nil, fmt.Errorf("column %s requires a type", name)
		}

		columnIdentifier, err := connection.NewIdentifier(name)
		if err != nil {
			return nil, nil, fmt.Errorf("column %s: %w", name, err)
		}

		dataType, err := connection.NewDataType(column.Type, column.Length, column.Precision)
		if err != nil {
			return nil, nil, fmt.Errorf("column %s: %w", name, err)
		}

		columns = append(columns, connection.ColumnDefinition{
			Name:         columnIdentifier,
			DataType:     dataType,
			IsPrimaryKey: column.PrimaryKey,
			IsNullable:   column.Nullable,
		})
		columnNames[strings.ToLower(columnIdentifier.String())] = struct{}{}
	}

	return columns, columnNames, nil
}

func buildIndexDefinitions(idxs []TableIndex, columnNames map[string]struct{}) ([]connection.IndexDefinition, error) {
	indexes := make([]connection.IndexDefinition, 0, len(idxs))
	for _, index := range idxs {
		name := strings.TrimSpace(index.Name)
		if name == "" {
			return nil, fmt.Errorf("index name is required")
		}

		indexIdentifier, err := connection.NewIdentifier(name)
		if err != nil {
			return nil, fmt.Errorf("index %s: %w", name, err)
		}
		if len(index.Columns) == 0 {
			return nil, fmt.Errorf("index %s must reference at least one column", name)
		}

		parsedColumns := make([]connection.Identifier, 0, len(index.Columns))
		for _, col := range index.Columns {

			identifier, err := connection.NewIdentifier(col)
			if err != nil {
				return nil, fmt.Errorf("index %s: invalid column name '%s': %w", name, col, err)
			}

			parsedColumns = append(parsedColumns, identifier)
		}

		if len(parsedColumns) == 0 {
			return nil, fmt.Errorf("index %s must reference at least one column", name)
		}

		indexes = append(indexes, connection.IndexDefinition{
			Name:    indexIdentifier,
			Columns: parsedColumns,
			Method:  resolveIndexMethod(index.Method),
			Unique:  index.Unique,
		})
	}
	return indexes, nil
}

func (h *CreateTableHandler) getConnectionDetails(ctx context.Context, connID uuid.UUID) (*connection.Connection, string, error) {
	conn, err := h.repo.FindByID(ctx, connID)
	if err != nil {
		return nil, "", err
	}
	if conn == nil {
		return nil, "", ErrConnectionNotFound
	}

	password, err := h.crypto.Decrypt(conn.Password)
	if err != nil {
		return nil, "", err
	}

	return conn, password, nil
}

func (h *CreateTableHandler) executeCreateTableAndIndexes(ctx context.Context, gateway connection.SchemaManager, conn connection.Connection, password, dbName string, tableDef *connection.TableDefinition, indexDefs []connection.IndexDefinition) error {
	timedCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	dbNameIdentifier, err := connection.NewIdentifier(dbName)
	if err != nil {
		return fmt.Errorf("invalid database name: %w", err)
	}

	if err := gateway.CreateTable(timedCtx, conn, password, dbNameIdentifier, *tableDef); err != nil {
		return err
	}

	for _, index := range indexDefs {
		if err := gateway.CreateIndex(timedCtx, conn, password, dbNameIdentifier, tableDef.Schema, tableDef.Name, index); err != nil {
			return err
		}
	}

	return nil
}

func resolveIndexMethod(method string) connection.IndexMethod {
	trimmed := strings.TrimSpace(method)
	if trimmed == "" {
		return connection.IndexMethodBTree
	}
	return connection.IndexMethod(strings.ToUpper(trimmed))
}
