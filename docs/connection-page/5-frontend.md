# 🎨 Tech Spec: UI/Frontend Layer (Connection Dashboard)

**Stack:** React, TanStack Router, TanStack Query, Shadcn/UI, Lucide Icons.
**Foco:** Estrutura de Rotas, Componentes Visuais e UX de Monitoramento.

---

## 1. Estrutura de Rotas (TanStack Router)

Utilizaremos **Nested Routing** (Rotas Aninhadas) para manter o layout (Sidebar e Header) persistente enquanto o usuário navega entre as abas.

```text
src/routes/
├── connections/
│   ├── $connectionId/             <-- Layout Principal (Shell)
│   │   ├── route.tsx              (Carrega Sidebar + Header + Tabs)
│   │   ├── index.tsx              (Redireciona para /overview)
│   │   ├── overview.tsx           (Aba: Visão Geral)
│   │   ├── databases.tsx          (Aba: Bancos de Dados)
│   │   ├── users.tsx              (Aba: Usuários & Roles)
│   │   └── monitor.tsx            (Aba: Sessões em Tempo Real)

```

---

## 2. Layout Principal (`route.tsx`)

Este arquivo define o "Shell" da página de administração.

### Componentes Chave:

1. **ConnectionHeader:**
* **Breadcrumb:** `Connections / Production DB`
* **Status Badge:** "Pulsing Dot" (Verde = Online, Vermelho = Offline).
* **Meta Info:** "Postgres 16.2" • "14ms latency".


2. **NavigationTabs:**
* Links que navegam para as sub-rotas (`overview`, `databases`, `users`, `monitor`).
* Uso do componente `<Link>` do TanStack Router com a prop `activeProps={{ className: 'border-b-2' }}`.



```tsx
// Exemplo Conceitual
export const Route = createFileRoute('/connections/$connectionId')({
  component: ConnectionLayout,
})

function ConnectionLayout() {
  const { connectionId } = Route.useParams()
  // Fetch leve apenas para o título/status
  const { data } = useConnectionOverview(connectionId) 

  return (
    <div className="flex flex-col h-full">
      <header className="border-b p-4 flex justify-between items-center bg-background/95 backdrop-blur">
        <div className="flex items-center gap-2">
           <DatabaseIcon className="h-6 w-6 text-primary" />
           <h1 className="text-xl font-bold">{data?.name}</h1>
           <StatusBadge status={data?.status} />
        </div>
      </header>

      <TabsList className="px-4 border-b">
         <TabLink to="overview">Overview</TabLink>
         <TabLink to="databases">Databases</TabLink>
         <TabLink to="users">Users</TabLink>
         <TabLink to="monitor">Monitor</TabLink>
      </TabsList>

      <div className="flex-1 p-6 bg-muted/10 overflow-auto">
        <Outlet /> {/* Aqui renderiza o conteúdo da aba */}
      </div>
    </div>
  )
}

```

---

## 3. Abas & Funcionalidades (Views)

### 3.1. Aba: Overview (`overview.tsx`)

**Objetivo:** Dashboard rápido. "Está tudo bem?"
**Componentes:**

* **Metric Cards:** Grid 2x2 mostrando Uptime, Sessões Ativas (com Progress Bar), Versão e Latência.
* **Alert Banner:** Se o status for "High Load" ou "Recovery", exibir um `<Alert variant="destructive">`.

### 3.2. Aba: Databases (`databases.tsx`)

**Objetivo:** Listar e Criar Bancos.
**UX:**

* **Tabela (TanStack Table):** Colunas `Name`, `Owner`, `Encoding`, `Size`. Ordenação padrão por `Size` (DESC).
* **Action:** Botão "Create Database" abre um `<Dialog>` com formulário simples (Nome do Banco).
* **Empty State:** Se a lista for vazia, mostrar ilustração e botão de criar.

### 3.3. Aba: Users (`users.tsx`)

**Objetivo:** Gestão de Segurança.
**UX:**

* **Tabela:** Colunas `Username`, `Superuser` (Badge), `Login` (Check/X).
* **Action:** Botão "Add User" abre um `<Sheet>` (painel lateral) para criação de usuário com senha.
* **Feedback:** Toast de sucesso ao criar usuário.

### 3.4. Aba: Monitor (`monitor.tsx`)

**Objetivo:** Task Manager em Tempo Real.
**UX:**

* **Polling:** Usa `useQuery` com `refetchInterval: 5000` (5 segundos).
* **Tabela de Processos:**
* Destaque visual para queries lentas (`Duration > 1s` fica amarelo, `> 1min` fica vermelho).
* Coluna `Query` trunca texto longo, clique expande.


* **Destructive Action:** Botão "Kill" na última coluna.
* **Critical:** Deve abrir um `<AlertDialog>`: *"Are you sure you want to terminate PID 1234? This may rollback transactions."*



---

## 4. Gerenciamento de Estado (Hooks)

Centralize a lógica de data-fetching em hooks customizados para manter os componentes limpos.

```typescript
// hooks/use-connection-data.ts

// Hook para dados estáticos (Databases, Users)
export function useDatabases(connId: string) {
  return useQuery({
    queryKey: ['connection', connId, 'databases'],
    queryFn: () => api.get(`/connections/${connId}/databases`),
    staleTime: 1000 * 60 * 5, // 5 minutos de cache
  })
}

// Hook para dados "vivos" (Monitor)
export function useLiveSessions(connId: string) {
  return useQuery({
    queryKey: ['connection', connId, 'sessions'],
    queryFn: () => api.get(`/connections/${connId}/sessions`),
    refetchInterval: 5000, // Atualiza a cada 5s
    // Otimização: Não refetch se a aba não estiver focada
    refetchOnWindowFocus: true, 
  })
}

// Hook para Ações (Mutations)
export function useKillSession() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ connId, pid }) => api.delete(`/connections/${connId}/sessions/${pid}`),
    onSuccess: (_, { connId }) => {
      // Invalida a lista imediatamente para sumir com o processo morto
      queryClient.invalidateQueries(['connection', connId, 'sessions'])
      toast.success("Process terminated")
    }
  })
}

```

---

## 5. UI Elements & Primitives (Shadcn/UI)

Lista de componentes essenciais para importar do `@/components/ui`:

| Componente | Uso no Dashboard |
| --- | --- |
| **Skeleton** | Loading state inicial para evitar "layout shift". |
| **Badge** | Status (Online/Offline), Roles (Superuser). |
| **Table** | Listagem densa de dados. |
| **Dialog / Sheet** | Formulários de criação (Create DB, Add User). |
| **AlertDialog** | Confirmação de `Kill Session` e `Drop User`. |
| **Progress** | Barra de uso de conexões (45/100). |
| **Toaster** | Notificações de erro ou sucesso das ações. |

---

## 6. Fluxo de Erros (Error Boundaries)

Como lidamos com conexões externas, falhas são esperadas.

* **Auth Error:** Se a API retornar `401/Unauthorized` (senha mudou), mostrar um card de erro no lugar da tabela com botão "Update Password".
* **Unreachable:** Se retornar `502/Bad Gateway`, desabilitar abas interativas e mostrar o status "Offline" no header com data da última verificação.

Este plano cobre toda a experiência do usuário, garantindo uma interface reativa, segura e informativa. 🚀
