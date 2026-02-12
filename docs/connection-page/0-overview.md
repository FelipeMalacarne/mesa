# 🏗️ RFC: Connection Dashboard & Administration

**Data:** 12 de Fevereiro de 2026
**Status:** Planejamento
**Objetivo:** Evoluir a visualização de conexões de um simples "Schema Browser" para um **Console de Gerenciamento Cloud-Native**, permitindo monitoramento em tempo real, gestão de segurança e operações administrativas.

---

## 1. Visão Geral da Arquitetura

O sistema seguirá a arquitetura **DDD + CQRS** já estabelecida no projeto Mesa, com uma consolidação estratégica na camada de infraestrutura.

### Decisões Arquiteturais Chaves

1. **Segregação de Interfaces (Domain):**

- **`Inspector`**: Responsável apenas por **Leitura de Estrutura** (Databases, Tables, Columns).
- **`Administrator`**: Responsável por **Runtime & Ações** (Health, Sessions, Users, Kill Process).

2. **Consolidação de Implementação (Infrastructure):**

- Um único struct `PostgresHandler` implementará ambas as interfaces.
- Isso evita duplicação de lógica de conexão (`sql.Open`) e gerenciamento de drivers.

3. **Pattern Gateway:**

- A `Factory` retornará uma interface composta `DatabaseGateway` (que une `Inspector + Administrator`).
- Os Casos de Uso (Application Layer) receberão apenas a interface específica que necessitam (`ISP`).

---

## 2. Modelagem de Domínio (`internal/domain/connection`)

As entidades serão enriquecidas para suportar dados de runtime, e não apenas metadados estáticos.

| Entidade           | Tipo          | Responsabilidade   | Novos Campos                                                      |
| ------------------ | ------------- | ------------------ | ----------------------------------------------------------------- |
| **`ServerHealth`** | Runtime       | Estado do servidor | `Version`, `Uptime`, `ActiveSessions`, `MaxConnections`, `Status` |
| **`DBUser`**       | Segurança     | Usuário do Banco   | `Name`, `IsSuperUser`, `CanLogin`, `ConnLimit`                    |
| **`Session`**      | Monitoramento | Processo ativo     | `PID`, `User`, `Database`, `Query`, `Duration`, `State`           |
| **`Database`**     | Estrutura     | Detalhes do Banco  | `Name`, `Owner`, `Encoding`, `Size` (bytes), `TableCount`         |

---

## 3. Estratégia de Camadas

### Infrastructure Layer (`internal/infrastructure/driver`)

- **Driver Unificado:** `PostgresHandler` implementa `DatabaseGateway`.
- **Conexão:** Uso de `pgx/stdlib` para alta performance.
- **Resiliência:** Uso rigoroso de `context.WithTimeout` (máx 5s para leitura, 10s para ações) para evitar travar o backend.

### Application Layer (`internal/application/connection`)

Divisão clara entre **Queries** (Dados para UI) e **Commands** (Ações do Usuário).

- **Queries (Leitura):**
- `GetOverview`: Retorna `OverviewDTO` (Health + Metrics).
- `ListDatabases`: Retorna lista enriquecida.
- `ListSessions`: Retorna lista de queries ativas (para polling).
- `ListUsers`: Retorna usuários e permissões.

- **Commands (Ação):**
- `KillSession`: Encerra uma query travada.
- `CreateUser`: Cria um novo usuário no banco.
- `CreateDatabase`: Cria um novo banco de dados.

### API Layer (REST)

Endpoints organizados hierarquicamente para suportar o frontend.

```text
GET  /connections/{id}/overview      -> Dashboard Header
GET  /connections/{id}/databases     -> Tab "Databases"
POST /connections/{id}/databases     -> Create DB Action
GET  /connections/{id}/users         -> Tab "Users"
POST /connections/{id}/users         -> Create User Action
GET  /connections/{id}/sessions      -> Tab "Monitor" (Polling)
POST /connections/{id}/sessions/kill -> Kill Action

```

---

## 4. Estratégia de Frontend (TanStack)

O frontend será responsável pela orquestração das chamadas, utilizando "Lazy Loading" para garantir que a página carregue instantaneamente.

1. **Cache Keys:**

- `['connection', id, 'overview']` (TTL: 30s)
- `['connection', id, 'databases']` (TTL: 5min)
- `['connection', id, 'sessions']` (TTL: 0 - Polling 5s)

2. **UX Components:**

- **Header:** Badge de Status (Online/Offline) baseado no sucesso do endpoint `/overview`.
- **Tabs:** Navegação sem recarregar a página.
- **Actions:** Botões destrutivos (`Kill`, `Drop`) devem exigir confirmação dupla.

---

## 5. Plano de Execução

1. **Fase 1: Core Domain & Infra (Backend)**

- Definir interfaces `Inspector`, `Administrator` e `Gateway`.
- Implementar `PostgresHandler` com queries SQL reais (`pg_stat_activity`, etc).
- Atualizar `Factory` para retornar o Gateway.

2. **Fase 2: Application Services (Backend)**

- Criar DTOs e Handlers para Overview, Users e Sessions.
- Injetar a dependência correta (`Gateway`) no `main.go`.

3. **Fase 3: API & HTTP (Backend)**

- Criar rotas e handlers HTTP.
- Testar payloads JSON.

4. **Fase 4: UI Integration (Frontend)**

- Criar Layout da página de Conexão.
- Implementar Tabs e integração com TanStack Query.

---

**Próximo Passo:** Gerar o Markdown técnico específico para a **Fase 1 (Domain & Infra)**, detalhando as structs e as queries SQL exatas. Posso prosseguir?
