package connection

import (
	"context"
	"errors"
)

type IndexMethod string

const (
	IndexMethodBTree IndexMethod = "BTREE"
	IndexMethodHash  IndexMethod = "HASH"
	IndexMethodGin   IndexMethod = "GIN" // Muito comum no Postgres para JSONB
)

// SchemaManager define ações de criação e destruição de estrutura (DDL)
type SchemaManager interface {
	// --- Table Management ---
	CreateTable(ctx context.Context, conn Connection, password string, dbName Identifier, def TableDefinition) error
	// DropTable(ctx context.Context, conn Connection, password string, dbName string, schema, tableName Identifier) error
	// TruncateTable(ctx context.Context, conn Connection, password string, dbName string, schema, tableName Identifier) error

	// --- Index Management ---
	CreateIndex(ctx context.Context, conn Connection, password string, dbName, schema, tableName Identifier, def IndexDefinition) error
	DropIndex(ctx context.Context, conn Connection, password string, schema, indexName Identifier) error

	// --- Column Management (O Futuro do seu app) ---
	// AddColumn(...)
	// DropColumn(...)
}

// IndexDefinition representa um índice a ser criado com a tabela
type IndexDefinition struct {
	Name    Identifier   // ex: "idx_users_email"
	Columns []Identifier // ex: ["email", "tenant_id"]
	Method  IndexMethod  // BTREE (padrão)
	Unique  bool
}

type ColumnDefinition struct {
	Name         Identifier
	DataType     DataType
	IsPrimaryKey bool
	IsAutoGen    bool
	IsNullable   bool
	IsUnique     bool
	DefaultValue *DefaultValue // Continua ponteiro pois é opcional
}

// TableDefinition protegida contra estados inválidos
type TableDefinition struct {
	Schema  Identifier
	Name    Identifier
	Columns []ColumnDefinition
	Comment string
}

var (
	ErrOneColumnRequired     = errors.New("a table must have at least one column")
	ErrOnePrimaryKeyRequired = errors.New("a table must have at least one primary key columnw")
	ErrMultiplePrimaryKeys   = errors.New("a table cannot have more than one primary key column")
)

func NewTableDefinition(schema, name Identifier, columns []ColumnDefinition) (*TableDefinition, error) {
	if len(columns) == 0 {
		return nil, ErrOneColumnRequired
	}

	err := validatePrimaryKey(columns)
	if err != nil {
		return nil, err
	}

	return &TableDefinition{
		Schema:  schema,
		Name:    name,
		Columns: columns,
	}, nil
}

func validatePrimaryKey(columns []ColumnDefinition) error {
	foundPK := false

	for i := range columns {
		if columns[i].IsPrimaryKey {
			if foundPK {
				return ErrMultiplePrimaryKeys
			}
			foundPK = true
		}
	}

	if !foundPK {
		return ErrOnePrimaryKeyRequired
	}

	return nil
}
