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

	// Query otimizada para pegar tabelas do schema public e views
	query := `
		SELECT table_name, table_type 
		FROM information_schema.tables 
		WHERE table_schema = 'public' 
		AND table_type IN ('BASE TABLE', 'VIEW')
		ORDER BY table_name;`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []connection.Table
	for rows.Next() {
		var t connection.Table
		if err := rows.Scan(&t.Name, &t.Type); err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}
	return tables, nil
}
