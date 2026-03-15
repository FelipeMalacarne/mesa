# Mesa

**The open-source SQL Control Plane for modern infrastructure.**

Mesa is a web-based database management tool built for developers and SREs.
It goes beyond simple queries — offering a full suite for exploring databases,
inspecting tables, managing users, and monitoring active sessions, all from a clean cloud-like UI.

## Features

- **Connection Management** — Add and switch between multiple PostgreSQL instances.
- **Database & Table Explorer** — Browse databases, tables, columns, indexes, and sample data.
- **Session Monitor** — View and kill active database sessions.
- **User Management** — Create and manage DB users without memorizing SQL syntax.
- **Credential Encryption** — All connection credentials are encrypted at rest (AES-256-GCM).
- **OpenAPI-first** — Typed Go server and auto-generated TypeScript client from a single `openapi.yaml`.

## Tech Stack

- **Backend:** Go, [chi](https://github.com/go-chi/chi), [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen), [sqlc](https://sqlc.dev/), [pgx](https://github.com/jackc/pgx)
- **Frontend:** React (Vite), [TanStack Router](https://tanstack.com/router), Shadcn/UI, Tailwind CSS, [orval](https://orval.dev/)
- **Metadata DB:** PostgreSQL
- **Deployment:** Docker-native, optimized for Kubernetes & ArgoCD

## Getting Started

### Prerequisites

- [Docker](https://www.docker.com/) and [Docker Compose](https://docs.docker.com/compose/)
- [Go](https://go.dev/) (for local development)
- [Node.js](https://nodejs.org/) & [pnpm](https://pnpm.io/) (for local frontend development)

### Quick Start (Docker)

```bash
git clone https://github.com/felipemalacarne/mesa.git
cd mesa
docker-compose up -d --build
```

- Frontend: [http://localhost:3000](http://localhost:3000)
- API: [http://localhost:8080](http://localhost:8080)

### Local Development

#### Backend

```bash
make migrate-up
go run cmd/server/main.go
```

#### Frontend

```bash
cd web
pnpm install
pnpm dev
```

## Makefile Commands

| Command                      | Description                                                |
| ---------------------------- | ---------------------------------------------------------- |
| `make migrate-up`            | Run database migrations                                    |
| `make migrate-down`          | Revert database migrations                                 |
| `make migration name=<name>` | Create a new migration file                                |
| `make sqlc-generate`         | Generate Go code from SQL queries                          |
| `make codegen`               | Generate Go server + TypeScript client from `openapi.yaml` |
| `make generate-app-key`      | Generate a random 32-byte encryption key                   |
| `make seed`                  | Seed the database with initial data                        |

## Security

- All database credentials are encrypted at rest using AES-256-GCM.
- Integrates with Kubernetes Secrets and external KMS providers.
- Every SQL execution is logged for audit purposes.
- Fully auditable source code under AGPL-3.0.

## License

Licensed under the [GNU Affero General Public License v3.0 (AGPL-3.0)](LICENSE).

For commercial licensing without AGPL restrictions, contact felipemalacarne012@gmail.com.
