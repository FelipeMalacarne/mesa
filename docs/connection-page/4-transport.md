# 🏗️ Tech Spec: API Layer & OpenAPI (Connection Administration)

**Pacote:** `internal/transport/rest`
**Foco:** Definição de Rotas RESTful, Handlers HTTP, DTOs de Request/Response e Documentação Swagger/OpenAPI 3.0.

---

## 1. Estrutura de Rotas (Chi Router)

Organização hierárquica para manter a API limpa e previsível.

### Arquivo: `internal/api/routes.go`

```go
func (s *Server) RegisterRoutes() {
    s.router.Route("/api/connections", func(r chi.Router) {
        // ... Rotas de CRUD da conexão (já existentes)

        // Sub-rotas para o Dashboard de Gerenciamento
        r.Route("/{connectionID}", func(r chi.Router) {
            // Dashboard Header
            r.Get("/overview", s.GetConnectionOverview)

            // Tab: Databases
            r.Get("/databases", s.ListDatabases)
            r.Post("/databases", s.CreateDatabase)

            // Tab: Users (Security)
            r.Get("/users", s.ListUsers)
            r.Post("/users", s.CreateUser)

            // Tab: Monitor (Sessions)
            r.Get("/sessions", s.ListSessions)
            r.Delete("/sessions/{pid}", s.KillSession) // DELETE é semântico aqui
        })
    })
}

```

---

## 2. Handlers HTTP (`internal/api/connection_admin.go`)

Implementação dos endpoints que orquestram a chamada para a camada de Aplicação.

### 2.1. Overview (GET)

Retorna o estado de saúde do servidor.

```go
func (s *Server) GetConnectionOverview(w http.ResponseWriter, r *http.Request) {
    id, err := uuid.Parse(chi.URLParam(r, "connectionID"))
    if err != nil {
        s.respondError(w, http.StatusBadRequest, "Invalid connection ID")
        return
    }

    // Chama Application Layer
    dto, err := s.app.Queries.GetOverview.Handle(r.Context(), id)
    if err != nil {
        // Se a conexão não existe no nosso banco
        s.respondError(w, http.StatusNotFound, "Connection not found")
        return
    }

    // Retorna 200 OK mesmo se o status for "Unreachable" 
    // (O frontend decide mostrar badge vermelho)
    s.respondJSON(w, http.StatusOK, dto)
}

```

### 2.2. Monitoramento de Sessões (GET)

Endpoint leve para polling.

```go
func (s *Server) ListSessions(w http.ResponseWriter, r *http.Request) {
    id, _ := uuid.Parse(chi.URLParam(r, "connectionID"))

    sessions, err := s.app.Queries.ListSessions.Handle(r.Context(), id)
    if err != nil {
        // Erro de conexão com o banco remoto (ex: timeout)
        s.respondError(w, http.StatusBadGateway, "Failed to reach remote database")
        return
    }

    s.respondJSON(w, http.StatusOK, sessions)
}

```

### 2.3. Matar Sessão (DELETE)

Ação destrutiva.

```go
func (s *Server) KillSession(w http.ResponseWriter, r *http.Request) {
    connID, _ := uuid.Parse(chi.URLParam(r, "connectionID"))
    pid, err := strconv.Atoi(chi.URLParam(r, "pid"))
    
    if err != nil {
        s.respondError(w, http.StatusBadRequest, "Invalid PID")
        return
    }

    cmd := commands.KillSessionCmd{
        ConnectionID: connID,
        PID:          pid,
    }

    if err := s.app.Commands.KillSession.Handle(r.Context(), cmd); err != nil {
        s.respondError(w, http.StatusInternalServerError, err.Error())
        return
    }

    w.WriteHeader(http.StatusNoContent) // 204
}

```

---

## 3. OpenAPI 3.0 Specification (`openapi.yaml`)

Este documento é o contrato para o Frontend. Você pode usar ferramentas para gerar os tipos TypeScript (como `openapi-typescript-codegen`) a partir daqui.

```yaml
openapi: 3.0.3
info:
  title: Mesa Connection Admin API
  version: 1.0.0
  description: API para gerenciamento e inspeção de conexões de banco de dados.

paths:
  # --- DASHBOARD HEADER ---
  /api/connections/{connectionID}/overview:
    get:
      summary: Get Server Health & Overview
      parameters:
        - $ref: '#/components/parameters/ConnectionID'
      responses:
        '200':
          description: Server status and metrics
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/OverviewResponse'

  # --- TAB: DATABASES ---
  /api/connections/{connectionID}/databases:
    get:
      summary: List databases
      parameters:
        - $ref: '#/components/parameters/ConnectionID'
      responses:
        '200':
          description: List of databases
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/DatabaseDTO'
    post:
      summary: Create a new database
      parameters:
        - $ref: '#/components/parameters/ConnectionID'
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                name: { type: string }
      responses:
        '201': { description: Database created }

  # --- TAB: USERS ---
  /api/connections/{connectionID}/users:
    get:
      summary: List database users (roles)
      parameters:
        - $ref: '#/components/parameters/ConnectionID'
      responses:
        '200':
          description: List of users
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/DBUser'
    post:
      summary: Create a new database user
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/CreateUserRequest'
      responses:
        '201': { description: User created }

  # --- TAB: MONITOR ---
  /api/connections/{connectionID}/sessions:
    get:
      summary: List active sessions (Live Monitor)
      parameters:
        - $ref: '#/components/parameters/ConnectionID'
      responses:
        '200':
          description: Active sessions
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/Session'
  
  /api/connections/{connectionID}/sessions/{pid}:
    delete:
      summary: Kill a specific session
      parameters:
        - $ref: '#/components/parameters/ConnectionID'
        - name: pid
          in: path
          required: true
          schema: { type: integer }
      responses:
        '204': { description: Session killed successfully }

components:
  parameters:
    ConnectionID:
      name: connectionID
      in: path
      required: true
      schema: { type: string, format: uuid }

  schemas:
    OverviewResponse:
      type: object
      properties:
        status: 
          type: string
          enum: [ONLINE, UNREACHABLE, AUTH_ERROR]
        version: { type: string, example: "PostgreSQL 15.2" }
        uptime: { type: string, example: "14d 2h" }
        sessions: { type: string, example: "45/100" }
        latency_ms: { type: integer, example: 24 }

    DatabaseDTO:
      type: object
      properties:
        name: { type: string }
        owner: { type: string }
        size_formatted: { type: string, example: "5.2 GB" }
        encoding: { type: string }

    DBUser:
      type: object
      properties:
        name: { type: string }
        is_superuser: { type: boolean }
        can_login: { type: boolean }

    CreateUserRequest:
      type: object
      required: [username, password]
      properties:
        username: { type: string }
        password: { type: string }
        is_superuser: { type: boolean }

    Session:
      type: object
      properties:
        pid: { type: integer }
        user: { type: string }
        database: { type: string }
        state: { type: string }
        query: { type: string }
        duration: { type: string }
        is_slow: { type: boolean }

```

---

## 4. Tratamento de Erros HTTP

Para garantir que o frontend saiba lidar com os problemas de rede, padronize os códigos de erro:

| Código | Significado | Causa Comum |
| --- | --- | --- |
| **404** | Not Found | Connection ID inválido ou não existe no repositório. |
| **502** | Bad Gateway | O Mesa não conseguiu conectar no banco do usuário (Timeout, Host Unreachable). |
| **401** | Unauthorized | A senha salva no Mesa foi rejeitada pelo banco de destino (Auth Error). |
| **400** | Bad Request | Tentativa de criar usuário com caracteres inválidos ou matar PID inexistente. |

Isso conclui a especificação completa da Feature! 🚀
Você tem agora os docs de **Domain**, **Application**, **Infrastructure** e **API**.
