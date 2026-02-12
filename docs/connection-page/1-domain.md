# 🏗️ Tech Spec: Domain Layer (Connection Administration)

**Pacote:** `internal/domain/connection`
**Foco:** Definição de Entidades Ricas e Segregação de Interfaces (ISP).

---

## 1. Visão Geral da Arquitetura

Nesta camada, aplicamos o **Interface Segregation Principle (ISP)** para distinguir claramente entre **Exploração de Estrutura** (Metadados estáticos) e **Administração do Servidor** (Runtime e Ações).

---

## 2. Entidades & Value Objects (`models.go`)

As entidades foram enriquecidas para suportar o painel de controle. Elas representam o estado real do banco de dados.

### 2.1. Estrutura (Schema)

Entidades focadas em "O que existe" no banco.

```go
// Database enriquecido
type Database struct {
    Name       string
    Owner      string // Quem é o dono? (pg_database.datdba)
    Encoding   string // ex: "UTF8"
    Size       int64  // Tamanho em bytes
    TableCount int    // Opcional: Custo extra para calcular
}

// Table (Mantida do Inspector original)
type Table struct {
    Name     string
    Type     string // "TABLE", "VIEW", "MATERIALIZED VIEW"
    Size     int64  // Bytes
    RowCount int64  // Estimativa (pg_class.reltuples)
}

// Column (Mantida do Inspector original)
type Column struct {
    Name         string
    DataType     string
    IsNullable   bool
    IsPrimary    bool
    DefaultValue *string
}

```

### 2.2. Runtime & Monitoramento

Entidades focadas em "O que está acontecendo" e "Quem está acessando".

```go
// ServerHealth: O "Header" do Dashboard
type ServerHealth struct {
    Version        string        // ex: "PostgreSQL 16.2 (Debian...)"
    Uptime         time.Duration // Tempo desde o start do postmaster
    ActiveSessions int           // Count de conexões ativas
    MaxConnections int           // Valor de max_connections
    Status         string        // "ONLINE", "RECOVERY", "HIGH_LOAD"
}

// DBUser: Segurança e Permissões (pg_roles)
type DBUser struct {
    Name        string
    IsSuperUser bool
    CanLogin    bool
    ConnLimit   int // -1 = Ilimitado
    // Attributes []string (ex: "NoReplication", "BypassRLS")
}

// Session: Processos ativos (pg_stat_activity)
type Session struct {
    PID       int
    User      string
    Database  string
    State     string        // "active", "idle", "idle in transaction"
    Query     string        // O SQL sendo executado (truncado se necessário)
    Duration  time.Duration // Now() - query_start
    StartedAt time.Time
}

```

---

## 3. Interfaces (Ports)

Aqui definimos os contratos. A implementação real (`infrastructure`) pode ser unificada, mas o domínio vê responsabilidades separadas.

### 3.1. Inspector (`inspector.go`)

Responsável apenas por **LEITURA DE ESTRUTURA**.
_Cache Policy suggestion:_ TTL Longo (Minutos).

```go
type Inspector interface {
    // Listagem Hierárquica
    GetDatabases(ctx context.Context, conn Connection, password string) ([]Database, error)
    GetTables(ctx context.Context, conn Connection, password string, dbName string) ([]Table, error)
    GetColumns(ctx context.Context, conn Connection, password, dbName, tableName string) ([]Column, error)

    // Verificação Básica
    Ping(ctx context.Context, conn Connection, password string) error
}

```

### 3.2. Administrator (`administrator.go`)

Responsável por **MONITORAMENTO (Runtime)** e **AÇÕES (Write)**.
_Cache Policy suggestion:_ TTL Curto (Segundos) ou Sem Cache.

```go
type Administrator interface {
    // --- Dashboard & Health ---
    GetServerHealth(ctx context.Context, conn Connection, password string) (*ServerHealth, error)

    // --- Monitoramento (Live) ---
    ListSessions(ctx context.Context, conn Connection, password string) ([]Session, error)
    KillSession(ctx context.Context, conn Connection, password string, pid int) error

    // --- Gestão de Usuários (Security) ---
    ListUsers(ctx context.Context, conn Connection, password string) ([]DBUser, error)
    CreateUser(ctx context.Context, conn Connection, password string, user DBUser, secret string) error
    DropUser(ctx context.Context, conn Connection, password string, username string) error

    // --- Gestão de Bancos (DDL) ---
    CreateDatabase(ctx context.Context, conn Connection, password string, dbName string) error
    // Note: ListDatabases NÃO está aqui. Está no Inspector.
}

```

---

## 4. Composição & Factory (`gateway.go`)

Para facilitar a implementação na camada de Infraestrutura, criamos uma interface composta.

### 4.1. O Gateway Unificado

Esta interface permite que o driver (infra) implemente tudo de uma vez, mas que o uso (app) seja segregado.

```go
// DatabaseGateway é a união de todos os poderes do driver.
type DatabaseGateway interface {
    Inspector
    Administrator
}

```

### 4.2. A Factory Atualizada (`factory.go`)

A Factory agora retorna o Gateway, permitindo que a camada de Aplicação decida qual "chapéu" (interface) quer usar.

```go
type InspectorFactory interface {
    // Retorna o Gateway completo. O Caller faz o type assertion ou usa a interface completa.
    ForDriver(driver Driver) (DatabaseGateway, error)
}

```

---

## 5. Regras de Domínio Importantes

1. **Segurança de Senha:** Nenhuma entidade de domínio (`Database`, `Session`) deve armazenar a senha da conexão. A senha é passada apenas como argumento nos métodos das interfaces.
2. **Imutabilidade:** As entidades retornadas (`[]Database`, `*ServerHealth`) devem ser tratadas como Snapshots imutáveis do momento da consulta.
3. **Resiliência:** As implementações dessas interfaces **DEVEM** respeitar o `context.Context` para cancelamento e timeouts, pois dependem de I/O externo instável.
