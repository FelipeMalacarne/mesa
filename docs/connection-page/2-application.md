# 🏗️ Tech Spec: Application Layer (Connection Administration)

**Pacote:** `internal/application/connection`
**Foco:** Casos de Uso (CQRS), DTOs e Orquestração de Segurança.

---

## 1. Visão Geral & Responsabilidades

A Camada de Aplicação atua como a barreira de segurança e orquestração.

1. **Segurança (Decryption):** É a **única** camada autorizada a chamar `crypto.Decrypt()`. A senha em texto plano (`plainText`) vive apenas no escopo de execução do Handler.
2. **Tratamento de Erros de Conexão:** Converte erros de rede (timeout, connection refused) em estados de domínio (ex: `Status: Offline`), evitando que a API retorne erros 500 genéricos para o frontend.
3. **Mapeamento (DTOs):** Transforma entidades ricas do domínio em estruturas JSON otimizadas para o frontend (ex: formatar bytes para "GB").

---

## 2. Queries (Leitura & Monitoramento)

Estes handlers alimentam o Dashboard e a Sidebar. Eles são "Read-Only" e devem ser altamente performáticos.

### 2.1. `GetOverviewHandler` (O Header do Dashboard)

_Responsabilidade:_ Fornecer o "Health Check" imediato da conexão.
_Dependências:_ `Repository`, `Cryptographer`, `Administrator`.

```go
// DTO de Saída
type OverviewDTO struct {
    Status    string `json:"status"`     // "ONLINE", "UNREACHABLE", "AUTH_ERROR"
    Version   string `json:"version"`    // ex: "PostgreSQL 16.2"
    Uptime    string `json:"uptime"`     // Formatado: "14d 2h"
    Sessions  string `json:"sessions"`   // "45/100"
    LatencyMs int64  `json:"latency_ms"` // Ping time
}

// Lógica do Handler
func (h *GetOverviewHandler) Handle(ctx, connID) (*OverviewDTO, error) {
    // 1. Busca Conexão & Descriptografa
    conn, _ := h.repo.FindByID(connID)
    pass, _ := h.crypto.Decrypt(conn.Password)

    // 2. Chama Administrator (Com Timeout curto)
    start := time.Now()
    health, err := h.admin.GetServerHealth(ctx, *conn, pass)
    latency := time.Since(start).Milliseconds()

    // 3. Tratamento de Erro Graceful (Não falha o request)
    if err != nil {
        return &OverviewDTO{Status: "UNREACHABLE", LatencyMs: latency}, nil
    }

    return &OverviewDTO{
        Status:    "ONLINE",
        Version:   health.Version,
        Uptime:    formatDuration(health.Uptime),
        Sessions:  fmt.Sprintf("%d/%d", health.ActiveSessions, health.MaxConnections),
        LatencyMs: latency,
    }, nil
}

```

### 2.2. `ListDatabasesHandler` (Aba Databases)

_Responsabilidade:_ Listar bancos disponíveis com detalhes de tamanho.
_Dependências:_ `Repository`, `Cryptographer`, `Inspector`.

```go
// DTO de Saída
type DatabaseDTO struct {
    Name          string `json:"name"`
    Owner         string `json:"owner"`
    SizeFormatted string `json:"size_formatted"` // "5.2 GB"
    Encoding      string `json:"encoding"`
}

// Nota: Usa o INSPECTOR, não o Administrator.
func (h *ListDatabasesHandler) Handle(...) ([]DatabaseDTO, error) {
    // ... setup connection
    dbs, err := h.inspector.GetDatabases(ctx, *conn, pass)
    // ... map to DTO
}

```

### 2.3. `ListSessionsHandler` (Aba Monitoramento - Live)

_Responsabilidade:_ Listar queries rodando agora. Usado por _polling_ no frontend.
_Dependências:_ `Repository`, `Cryptographer`, `Administrator`.

```go
type SessionDTO struct {
    PID       int    `json:"pid"`
    User      string `json:"user"`
    Database  string `json:"database"`
    State     string `json:"state"`
    Query     string `json:"query"`     // Truncar se for muito longa?
    Duration  string `json:"duration"`  // "00:00:05"
    IsSlow    bool   `json:"is_slow"`   // true se > 1min (Regra de UI)
}

```

---

## 3. Commands (Ações & Escrita)

Estes handlers alteram o estado do banco de dados remoto. Exigem validação rigorosa.

### 3.1. `KillSessionHandler` (Ação de Emergência)

_Responsabilidade:_ Derrubar uma query travada.
_Dependências:_ `Repository`, `Cryptographer`, `Administrator`.

```go
type KillSessionCmd struct {
    ConnectionID uuid.UUID
    PID          int
}

func (h *KillSessionHandler) Handle(ctx context.Context, cmd KillSessionCmd) error {
    conn, _ := h.repo.FindByID(cmd.ConnectionID)
    pass, _ := h.crypto.Decrypt(conn.Password)

    // Log de Auditoria (Sugestão futura: Quem matou o processo?)
    log.Printf("User requested kill session PID %d on connection %s", cmd.PID, conn.Name)

    return h.admin.KillSession(ctx, *conn, pass, cmd.PID)
}

```

### 3.2. `CreateUserHandler` (Gestão de Acesso)

_Responsabilidade:_ Criar um novo login no Postgres.

```go
type CreateUserCmd struct {
    ConnectionID uuid.UUID
    Username     string
    Password     string // Senha do NOVO usuário
    IsSuperUser  bool
}

func (h *CreateUserHandler) Handle(ctx context.Context, cmd CreateUserCmd) error {
    // Validação de Regra de Negócio Básica
    if cmd.Username == "postgres" || cmd.Username == "root" {
        return errors.New("cannot create reserved user")
    }

    conn, _ := h.repo.FindByID(cmd.ConnectionID)
    adminPass, _ := h.crypto.Decrypt(conn.Password)

    newUser := connection.DBUser{
        Name:        cmd.Username,
        IsSuperUser: cmd.IsSuperUser,
        CanLogin:    true,
    }

    return h.admin.CreateUser(ctx, *conn, adminPass, newUser, cmd.Password)
}

```

---

## 4. Injeção de Dependência (Factory Setup)

Como o driver de infraestrutura (`PostgresHandler`) implementa tanto `Inspector` quanto `Administrator`, a montagem dos handlers no `main.go` (ou `container.go`) deve ser cuidadosa para injetar apenas a interface necessária.

```go
// Exemplo de setup no main/container
func NewApp(repo Repository, crypto Crypto, gateway connection.DatabaseGateway) *App {

    return &App{
        Queries: Queries{
            // O Handler de Listar Tabelas recebe o Gateway como INSPECTOR
            ListDatabases: NewListDatabasesHandler(repo, crypto, gateway), // gateway act as Inspector

            // O Handler de Monitoramento recebe o Gateway como ADMINISTRATOR
            GetOverview:   NewGetOverviewHandler(repo, crypto, gateway),   // gateway act as Administrator
        },
        Commands: Commands{
            KillSession: NewKillSessionHandler(repo, crypto, gateway),
        },
    }
}

```

---

## 5. Tratamento de Erros e Timeouts

A camada de aplicação deve aplicar políticas de timeout defensivas, pois o banco do usuário pode estar do outro lado do mundo ou travado.

| Handler          | Timeout Sugerido | Estratégia de Erro                                     |
| ---------------- | ---------------- | ------------------------------------------------------ |
| `GetOverview`    | **3 segundos**   | Retorna Status "UNREACHABLE" (Sucesso HTTP 200).       |
| `ListSessions`   | **5 segundos**   | Retorna Lista Vazia + Erro HTTP 504 (Gateway Timeout). |
| `KillSession`    | **10 segundos**  | Retorna Erro HTTP 500 ou 400 (se PID não existir).     |
| `CreateDatabase` | **30 segundos**  | Retorna Erro HTTP 500. Operações DDL demoram.          |
