package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/felipemalacarne/mesa/internal/domain/connection"
	_ "github.com/jackc/pgx/v5/stdlib" // Importante para o sql.Open("pgx", ...)
)

type Inspector struct{}

func NewInspector() *Inspector {
	return &Inspector{}
}

// connect estabelece uma conexão temporária com o banco do usuário
func (i *Inspector) connect(conn connection.Connection, password, dbName string) (*sql.DB, error) {
	// DSN formatado para o banco específico.
	// Usamos sslmode=disable para o exemplo, mas em produção você deve tratar certificados.
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable&connect_timeout=5",
		conn.Username, password, conn.Host, conn.Port, dbName)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection: %w", err)
	}

	// Configurações agressivas de pool para conexões de inspeção (curta duração)
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(30 * time.Second)

	return db, nil
}

func (i *Inspector) GetDatabases(ctx context.Context, conn connection.Connection, password string) ([]connection.Database, error) {
	// No Postgres, para listar bancos, conectamos geralmente ao banco padrão 'postgres'
	db, err := i.connect(conn, password, "postgres")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := `SELECT datname FROM pg_database WHERE datistemplate = false AND datallowconn = true ORDER BY datname;`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dbs []connection.Database
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		dbs = append(dbs, connection.Database{Name: name})
	}
	return dbs, nil
}

func (i *Inspector) GetTables(ctx context.Context, conn connection.Connection, password string, dbName string) ([]connection.Table, error) {
	db, err := i.connect(conn, password, dbName)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Query para pegar tabelas do schema public e views com stats básicas
	query := `
		SELECT t.table_name,
		       t.table_type,
		       CASE
		           WHEN t.table_type = 'BASE TABLE' THEN COALESCE(st.n_live_tup, 0)
		           ELSE 0
		       END AS row_count,
		       CASE
		           WHEN t.table_type = 'BASE TABLE' THEN pg_total_relation_size(format('%I.%I', t.table_schema, t.table_name))
		           ELSE 0
		       END AS size_bytes
		FROM information_schema.tables t
		LEFT JOIN pg_stat_user_tables st
		  ON st.schemaname = t.table_schema
		 AND st.relname = t.table_name
		WHERE t.table_schema = 'public'
		  AND t.table_type IN ('BASE TABLE', 'VIEW')
		ORDER BY t.table_name;`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []connection.Table
	for rows.Next() {
		var t connection.Table
		if err := rows.Scan(&t.Name, &t.Type, &t.RowCount, &t.Size); err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}
	return tables, nil
}

func (i *Inspector) GetColumns(ctx context.Context, conn connection.Connection, password, dbName, tableName string) ([]connection.Column, error) {
	db, err := i.connect(conn, password, dbName)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := `
		SELECT c.column_name,
		       c.data_type,
		       c.is_nullable,
		       COALESCE(tc.constraint_type = 'PRIMARY KEY', false) AS is_primary,
		       c.column_default
		FROM information_schema.columns c
		LEFT JOIN information_schema.key_column_usage kcu
		  ON c.table_schema = kcu.table_schema
		 AND c.table_name = kcu.table_name
		 AND c.column_name = kcu.column_name
		LEFT JOIN information_schema.table_constraints tc
		  ON tc.table_schema = kcu.table_schema
		 AND tc.table_name = kcu.table_name
		 AND tc.constraint_name = kcu.constraint_name
		 AND tc.constraint_type = 'PRIMARY KEY'
		WHERE c.table_schema = 'public'
		  AND c.table_name = $1
		ORDER BY c.ordinal_position;`

	rows, err := db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []connection.Column
	for rows.Next() {
		var col connection.Column
		var isNullable string
		var defaultValue sql.NullString
		if err := rows.Scan(&col.Name, &col.Type, &isNullable, &col.IsPrimary, &defaultValue); err != nil {
			return nil, err
		}
		col.IsNullable = isNullable == "YES"
		if defaultValue.Valid {
			col.DefaultValue = &defaultValue.String
		}
		columns = append(columns, col)
	}
	return columns, nil
}

func (i *Inspector) Ping(ctx context.Context, conn connection.Connection, password string) error {
	db, err := i.connect(conn, password, "postgres")
	if err != nil {
		return err
	}
	defer db.Close()

	return db.PingContext(ctx)
}
