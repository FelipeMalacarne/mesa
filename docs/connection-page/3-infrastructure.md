# 🏗️ Tech Spec: Infrastructure Layer (Connection Administration)

**Pacote:** `internal/infrastructure/driver/postgres`
**Foco:** Implementação Unificada (`DatabaseGateway`), Queries de Sistema (`pg_catalog`) e Gerenciamento de Conexões Dinâmicas.

---

## 1. Visão Geral da Implementação

Para maximizar a eficiência e reutilização de código, utilizaremos uma única struct `PostgresHandler` que satisfaz a interface composta `DatabaseGateway` (Inspector + Administrator).

- **Driver:** `github.com/jackc/pgx/v5/stdlib` (via `database/sql`).
- **Estratégia de Conexão:** Conexões efêmeras. Abrimos, executamos e fechamos. Isso é vital para não exaurir o pool de conexões do banco do usuário (que pode ser pequeno).
- **Segurança:** Uso de `QueryContext` para respeitar timeouts e cancelamentos.

---

## 2. O Handler Unificado (`handler.go`)

Esta struct encapsula a lógica de conexão.

```go
package postgres

import (
    "context"
    "database/sql"
    "fmt"
    _ "github.com/jackc/pgx/v5/stdlib"
    "mesa/internal/domain/connection"
)

type PostgresHandler struct{}

func NewHandler() *PostgresHandler {
    return &PostgresHandler{}
}

// connect: Helper privado para abrir conexão dinâmica
func (h *PostgresHandler) connect(conn connection.Connection, password, dbName string) (*sql.DB, error) {
    dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable&connect_timeout=3",
        conn.Username, password, conn.Host, conn.Port, dbName)

    db, err := sql.Open("pgx", dsn)
    if err != nil {
        return nil, err
    }

    // Configuração conservadora para ferramentas de inspeção
    db.SetMaxOpenConns(2)
    db.SetConnMaxLifetime(30 * time.Second)

    return db, nil
}

```

---

## 3. Implementação do `Inspector` (Leitura Estrutural)

Responsável por listar metadados. Estas queries são seguras e de baixo impacto.

### 3.1. `GetDatabases`

Recupera lista de bancos com tamanho e dono.

```go
func (h *PostgresHandler) GetDatabases(ctx context.Context, conn connection.Connection, password string) ([]connection.Database, error) {
    db, err := h.connect(conn, password, "postgres")
    if err != nil { return nil, err }
    defer db.Close()

    // Query no pg_database
    query := `
        SELECT
            d.datname,
            pg_get_userbyid(d.datdba) as owner,
            pg_encoding_to_char(d.encoding) as encoding,
            pg_database_size(d.datname) as size_bytes
        FROM pg_database d
        WHERE d.datistemplate = false
        ORDER BY d.datname;
    `

    rows, err := db.QueryContext(ctx, query)
    // ... scan para struct connection.Database
}

```

### 3.2. `GetTables` & `GetColumns`

_Mantém a implementação existente focada no `information_schema`._

---

## 4. Implementação do `Administrator` (Runtime & Ações)

Aqui residem as queries mais complexas que acessam tabelas de sistema (`pg_stat_activity`, `pg_roles`).

### 4.1. `GetServerHealth` (Dashboard Header)

Combina múltiplas métricas em uma struct.

```go
func (h *PostgresHandler) GetServerHealth(ctx context.Context, conn connection.Connection, password string) (*connection.ServerHealth, error) {
    db, err := h.connect(conn, password, "postgres")
    if err != nil { return nil, err } // Retorna erro de conexão para a App Layer tratar como "Offline"
    defer db.Close()

    query := `
        SELECT
            version(),
            current_setting('max_connections')::int,
            (SELECT count(*) FROM pg_stat_activity)::int,
            (SELECT extract(epoch FROM (now() - pg_postmaster_start_time())))::bigint
    `

    var health connection.ServerHealth
    var uptimeSeconds int64

    err = db.QueryRowContext(ctx, query).Scan(
        &health.Version,
        &health.MaxConnections,
        &health.ActiveSessions,
        &uptimeSeconds,
    )

    health.Uptime = time.Duration(uptimeSeconds) * time.Second
    health.Status = "ONLINE" // Se chegou aqui, está online.

    return &health, err
}

```

### 4.2. `ListSessions` (Monitoramento em Tempo Real)

Consulta a view de atividades.

```go
func (h *PostgresHandler) ListSessions(ctx context.Context, conn connection.Connection, password string) ([]connection.Session, error) {
    db, err := h.connect(conn, password, "postgres")
    // ...

    // Exclui conexões 'idle' para reduzir ruído
    query := `
        SELECT pid, usename, datname, state, query, extract(epoch FROM (now() - query_start))
        FROM pg_stat_activity
        WHERE state != 'idle'
        ORDER BY query_start ASC
        LIMIT 50;
    `
    // ... scan para connection.Session
}

```

### 4.3. `KillSession` (Ação Destrutiva)

Encerra um backend específico.

```go
func (h *PostgresHandler) KillSession(ctx context.Context, conn connection.Connection, password string, pid int) error {
    db, err := h.connect(conn, password, "postgres")
    // ...

    // pg_terminate_backend retorna true/false
    var success bool
    query := `SELECT pg_terminate_backend($1)`

    err = db.QueryRowContext(ctx, query, pid).Scan(&success)
    if err != nil { return err }

    if !success {
        return fmt.Errorf("failed to kill PID %d: process not found or permission denied", pid)
    }
    return nil
}

```

### 4.4. `CreateUser` (DDL Management)

⚠️ **Atenção Crítica:** DDL no SQL (`CREATE USER`) geralmente não aceita parâmetros (`$1`) para identificadores. É necessário usar `fmt.Sprintf`, o que exige sanitização cuidadosa.

```go
func (h *PostgresHandler) CreateUser(ctx context.Context, conn connection.Connection, password string, user connection.DBUser, newPass string) error {
    db, err := h.connect(conn, password, "postgres")
    // ...

    // Validação básica para evitar SQL Injection grosseiro
    if !isValidUsername(user.Name) {
        return fmt.Errorf("invalid username format")
    }

    // Construção do comando SQL
    sql := fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s'", user.Name, newPass)
    if user.IsSuperUser {
        sql += " SUPERUSER"
    } else {
        sql += " NOSUPERUSER"
    }
    sql += " LOGIN"

    _, err = db.ExecContext(ctx, sql)
    return err
}

```

---

## 5. A Factory (`factory.go`)

Atualiza a Factory para retornar a interface composta `DatabaseGateway`.

```go
package driver

import (
    "mesa/internal/domain/connection"
    "mesa/internal/infrastructure/driver/postgres"
    // "mesa/internal/infrastructure/driver/mysql"
)

type Factory struct{}

func NewFactory() *Factory {
    return &Factory{}
}

func (f *Factory) ForDriver(driverType connection.Driver) (connection.DatabaseGateway, error) {
    switch driverType {
    case connection.Postgres:
        // Retorna o Handler que implementa tanto Inspector quanto Administrator
        return postgres.NewHandler(), nil

    // case connection.MySQL:
    //     return mysql.NewHandler(), nil

    default:
        return nil, connection.ErrInvalidDriver
    }
}

```

---

## ✅ Checklist de Segurança & Performance

1. **Timeouts:** Todas as chamadas `db.QueryContext` e `db.ExecContext` **DEVEM** usar o `ctx` que vem da Camada de Aplicação (que já tem timeout configurado).
2. **Vazamento de Conexão:** Uso rigoroso de `defer db.Close()` imediatamente após o `h.connect`.
3. **Sanitização:** Na criação de usuários (`CreateUser`), validar que o username contém apenas caracteres alfanuméricos (`[a-zA-Z0-9_]`) antes de interpolar na string SQL.
4. **SSL Mode:** Em produção, o DSN deve configurar `sslmode=require` ou usar certificados, dependendo da configuração da conexão salva. O exemplo usa `disable` apenas para desenvolvimento local.
