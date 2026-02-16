package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/felipemalacarne/mesa/internal/domain/connection"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Gateway implementa o contrato de runtime e inspeção para Postgres.
type Gateway struct{}

func NewGateway() connection.Gateway {
	return &Gateway{}
}

func (h *Gateway) connect(conn connection.Connection, password string, dbName connection.Identifier) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable&connect_timeout=5",
		conn.Username,
		password,
		conn.Host,
		conn.Port,
		dbName.String(),
	)

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection: %w", err)
	}

	db.SetMaxOpenConns(2)
	db.SetConnMaxLifetime(30 * time.Second)

	return db, nil
}

func (h *Gateway) GetDatabases(ctx context.Context, conn connection.Connection, password string) ([]connection.Database, error) {
	db, err := h.connect(conn, password, postgresDBName())
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := `
SELECT
    d.datname,
    pg_get_userbyid(d.datdba) as owner,
    pg_encoding_to_char(d.encoding) as encoding,
    pg_database_size(d.datname) as size_bytes
FROM pg_database d
WHERE d.datistemplate = false
  AND d.datallowconn = true
ORDER BY d.datname;
`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var databases []connection.Database
	for rows.Next() {
		var database connection.Database
		if err := rows.Scan(&database.Name, &database.Owner, &database.Encoding, &database.Size); err != nil {
			return nil, err
		}
		database.TableCount = 0
		databases = append(databases, database)
	}

	return databases, rows.Err()
}

func (h *Gateway) GetTables(ctx context.Context, conn connection.Connection, password string, dbName connection.Identifier) ([]connection.Table, error) {
	db, err := h.connect(conn, password, dbName)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := `
SELECT
    t.table_name,
    t.table_type,
    COALESCE(pg_total_relation_size(format('%I.%I', t.table_schema, t.table_name)), 0) as size_bytes,
    COALESCE(st.n_live_tup, 0) as row_count
FROM information_schema.tables t
LEFT JOIN pg_stat_user_tables st
  ON st.schemaname = t.table_schema AND st.relname = t.table_name
WHERE t.table_schema = 'public'
  AND t.table_type IN ('BASE TABLE', 'VIEW', 'MATERIALIZED VIEW')
ORDER BY t.table_name;
`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []connection.Table
	for rows.Next() {
		var table connection.Table
		if err := rows.Scan(&table.Name, &table.Type, &table.Size, &table.RowCount); err != nil {
			return nil, err
		}

		table.Type = strings.ToUpper(strings.ReplaceAll(table.Type, "BASE ", ""))
		tables = append(tables, table)
	}

	return tables, rows.Err()
}

func (h *Gateway) GetColumns(ctx context.Context, conn connection.Connection, password string, dbName, tableName connection.Identifier) ([]connection.Column, error) {
	db, err := h.connect(conn, password, dbName)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := `
SELECT
    c.column_name,
    c.data_type,
    c.is_nullable,
    COALESCE(tc.constraint_type = 'PRIMARY KEY', false) as is_primary,
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
ORDER BY c.ordinal_position;
`

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
		if err := rows.Scan(&col.Name, &col.DataType, &isNullable, &col.IsPrimary, &defaultValue); err != nil {
			return nil, err
		}

		col.IsNullable = isNullable == "YES"
		if defaultValue.Valid {
			col.DefaultValue = &defaultValue.String
		}
		columns = append(columns, col)
	}

	return columns, rows.Err()
}

func (h *Gateway) Ping(ctx context.Context, conn connection.Connection, password string) error {
	db, err := h.connect(conn, password, postgresDBName())
	if err != nil {
		return err
	}
	defer db.Close()

	return db.PingContext(ctx)
}

func (h *Gateway) GetServerHealth(ctx context.Context, conn connection.Connection, password string) (*connection.ServerHealth, error) {
	db, err := h.connect(conn, password, postgresDBName())
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := `
SELECT
    version(),
    current_setting('max_connections')::int,
    (SELECT count(*) FROM pg_stat_activity)::int,
    EXTRACT(EPOCH FROM (now() - pg_postmaster_start_time()))::bigint;
`

	var health connection.ServerHealth
	var uptimeSeconds int64
	if err := db.QueryRowContext(ctx, query).Scan(&health.Version, &health.MaxConnections, &health.ActiveSessions, &uptimeSeconds); err != nil {
		return nil, err
	}

	health.Uptime = time.Duration(uptimeSeconds) * time.Second
	health.Status = "ONLINE"

	return &health, nil
}

func (h *Gateway) ListSessions(ctx context.Context, conn connection.Connection, password string) ([]connection.Session, error) {
	db, err := h.connect(conn, password, postgresDBName())
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := `
SELECT
    pid,
    usename,
    datname,
    state,
    COALESCE(query, ''),
    EXTRACT(EPOCH FROM (now() - query_start))::bigint as duration,
    query_start
FROM pg_stat_activity
WHERE state != 'idle'
  AND pid <> pg_backend_pid()
ORDER BY query_start DESC
LIMIT 50;
`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions := make([]connection.Session, 0)
	for rows.Next() {
		var session connection.Session
		var durationSeconds int64
		if err := rows.Scan(&session.PID, &session.User, &session.Database, &session.State, &session.Query, &durationSeconds, &session.StartedAt); err != nil {
			return nil, err
		}

		session.Duration = time.Duration(durationSeconds) * time.Second
		sessions = append(sessions, session)
	}

	return sessions, rows.Err()
}

func (h *Gateway) KillSession(ctx context.Context, conn connection.Connection, password string, pid int) error {
	db, err := h.connect(conn, password, postgresDBName())
	if err != nil {
		return err
	}
	defer db.Close()

	var success bool
	if err := db.QueryRowContext(ctx, "SELECT pg_terminate_backend($1)", pid).Scan(&success); err != nil {
		return err
	}

	if !success {
		return fmt.Errorf("failed to terminate pid %d: permission denied or not found", pid)
	}

	return nil
}

func (h *Gateway) ListUsers(ctx context.Context, conn connection.Connection, password string) ([]connection.DBUser, error) {
	db, err := h.connect(conn, password, postgresDBName())
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := `
SELECT rolname, rolsuper, rolcanlogin, rolconnlimit
FROM pg_roles
WHERE rolname NOT LIKE 'pg_%'
ORDER BY rolname;
`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []connection.DBUser
	for rows.Next() {
		var user connection.DBUser
		var name string
		if err := rows.Scan(&name, &user.IsSuperUser, &user.CanLogin, &user.ConnLimit); err != nil {
			return nil, err
		}

		identifier, err := connection.NewIdentifier(name)
		if err != nil {
			// Skip users with invalid identifiers to avoid breaking the entire list
			// In a real scenario, we might want to log this
			log.Printf("[ERROR]skipping user with invalid name '%s': %v", name, err)
			continue
		}
		user.Name = identifier
		users = append(users, user)
	}

	return users, rows.Err()
}

func (h *Gateway) CreateUser(ctx context.Context, conn connection.Connection, password string, user connection.DBUser, newPass string) error {
	db, err := h.connect(conn, password, postgresDBName())
	if err != nil {
		return err
	}
	defer db.Close()

	builder := strings.Builder{}
	builder.WriteString("CREATE USER ")
	builder.WriteString(user.Name.Quoted())
	builder.WriteString(" WITH ")
	builder.WriteString("PASSWORD ")
	builder.WriteString(quoteLiteral(newPass))
	builder.WriteString(" ")
	if user.IsSuperUser {
		builder.WriteString("SUPERUSER ")
	} else {
		builder.WriteString("NOSUPERUSER ")
	}

	if user.CanLogin {
		builder.WriteString("LOGIN ")
	} else {
		builder.WriteString("NOLOGIN ")
	}

	if user.ConnLimit >= 0 {
		fmt.Fprintf(&builder, "CONNECTION LIMIT %d ", user.ConnLimit)
	}

	query := strings.TrimSpace(builder.String())

	_, err = db.ExecContext(ctx, query)
	return err
}

func (h *Gateway) DropUser(ctx context.Context, conn connection.Connection, password string, username connection.Identifier) error {
	db, err := h.connect(conn, password, postgresDBName())
	if err != nil {
		return err
	}
	defer db.Close()

	query := fmt.Sprintf("DROP USER %s", username.Quoted())
	_, err = db.ExecContext(ctx, query)
	return err
}

func (h *Gateway) CreateDatabase(ctx context.Context, conn connection.Connection, password string, dbName, owner connection.Identifier) error {
	db, err := h.connect(conn, password, postgresDBName())
	if err != nil {
		return err
	}
	defer db.Close()

	query := fmt.Sprintf(
		"CREATE DATABASE %s OWNER %s",
		dbName.Quoted(),
		owner.Quoted(),
	)
	_, err = db.ExecContext(ctx, query)
	return err
}

func (h *Gateway) CreateTable(ctx context.Context, conn connection.Connection, password string, dbName connection.Identifier, table connection.TableDefinition) error {
	db, err := h.connect(conn, password, dbName)
	if err != nil {
		return err
	}
	defer db.Close()

	columnDefs := make([]string, 0, len(table.Columns)+1)
	var pkColumns []string

	for _, column := range table.Columns {
		builder := strings.Builder{}
		builder.WriteString(column.Name.Quoted())
		builder.WriteString(" ")
		builder.WriteString(column.DataType.Format())

		if !column.IsNullable {
			builder.WriteString(" NOT NULL")
		}

		if column.DefaultValue != nil {
			defaultValue := column.DefaultValue
			if !defaultValue.IsEmpty() {
				builder.WriteString(" DEFAULT ")
				builder.WriteString(defaultValue.String())
			}
		}

		columnDefs = append(columnDefs, builder.String())

		if column.IsPrimaryKey {
			pkColumns = append(pkColumns, column.Name.Quoted())
		}
	}

	if len(pkColumns) > 0 {
		columnDefs = append(columnDefs, fmt.Sprintf("PRIMARY KEY (%s)", strings.Join(pkColumns, ", ")))
	}

	createTableQuery := fmt.Sprintf(
		"CREATE TABLE %s.%s (%s)",
		table.Schema.Quoted(),
		table.Name.Quoted(),
		strings.Join(columnDefs, ", "),
	)

	_, err = db.ExecContext(ctx, createTableQuery)
	return err
}

func (h *Gateway) CreateIndex(ctx context.Context, conn connection.Connection, password string, dbName, schema, tableName connection.Identifier, index connection.IndexDefinition) error {
	if len(index.Columns) == 0 {
		return fmt.Errorf("index %s must reference at least one column", index.Name)
	}

	indexColumns := make([]string, 0, len(index.Columns))
	for _, columnName := range index.Columns {
		indexColumns = append(indexColumns, columnName.Quoted())
	}

	uniqueKeyword := ""
	if index.Unique {
		uniqueKeyword = "UNIQUE "
	}

	query := fmt.Sprintf(
		"CREATE %sINDEX %s ON %s.%s USING %s (%s)",
		uniqueKeyword,
		index.Name.Quoted(),
		schema.Quoted(),
		tableName.Quoted(),
		index.Method,
		strings.Join(indexColumns, ", "),
	)

	db, err := h.connect(conn, password, dbName)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.ExecContext(ctx, query)
	return err
}

func (h *Gateway) DropIndex(ctx context.Context, conn connection.Connection, password string, dbName, indexName connection.Identifier) error {
	query := fmt.Sprintf(
		"DROP INDEX %s.%s",
		dbName.Quoted(),
		indexName.Quoted(),
	)

	db, err := h.connect(conn, password, dbName)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.ExecContext(ctx, query)
	return err

}

func quoteLiteral(value string) string {
	escaped := strings.ReplaceAll(value, "'", "''")
	return fmt.Sprintf("'%s'", escaped)
}

func postgresDBName() connection.Identifier {
	return connection.MustNewIdentifier("postgres")
}
