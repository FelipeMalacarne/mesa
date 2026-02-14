# 🚀 Mesa

**The open-source SQL Control Plane for modern infrastructure.**

Mesa is a web-based SQL client designed for developers and SREs who need a "cloud provider" experience within their own infrastructure. It goes beyond simple queries, offering a full suite for managing database users, permissions, and multi-instance configurations.

## 🌟 Why Mesa?

- **Infrastructure-First:** Designed to run as a lightweight, single-binary pod in your Kubernetes cluster.
- **Security & Trust:** All database credentials are encrypted at rest (AES-256-GCM) and the code is fully audit-at-will under AGPL v3.
- **User Provisioning:** A clean UI to manage DB users (GRANT/REVOKE) without memorizing vendor-specific syntax.
- **Modern UX:** High-performance editor built with Shadcn/UI and React, featuring a sleek, cloud-like dashboard.

## 🛠 Tech Stack

- **Backend:** Go - fast, concurrent, and memory-efficient.
- **Frontend:** React (Vite) + Shadcn/UI + Tailwind CSS.
- **Metadata DB:** PostgreSQL (with JSONB for flexible settings).
- **Deployment:** Docker-native, optimized for Kubernetes & ArgoCD.

## 🚀 Getting Started

### Prerequisites

- [Docker](https://www.docker.com/) and [Docker Compose](https://docs.docker.com/compose/)
- [Go](https://go.dev/) (for local development)
- [Node.js](https://nodejs.org/) & [pnpm](https://pnpm.io/) (for local frontend development)

### Quick Start (Docker)

The easiest way to run Mesa is using Docker Compose.

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/your-username/mesa.git
    cd mesa
    ```

2.  **Start the services:**
    ```bash
    docker-compose up -d --build
    ```

    This will start:
    -   **PostgreSQL**: Database for Mesa metadata.
    -   **API**: Go backend running on port `8080`.
    -   **Frontend**: React app running on port `3000`.

3.  **Access the application:**
    -   Frontend: [http://localhost:3000](http://localhost:3000)
    -   API: [http://localhost:8080](http://localhost:8080)

### Local Development

If you prefer to run services locally without Docker for development:

#### Backend

1.  Ensure you have a PostgreSQL instance running and configured.
2.  Run migrations:
    ```bash
    make migrate-up
    ```
3.  Start the server:
    ```bash
    go run cmd/server/main.go
    ```

#### Frontend

1.  Navigate to the web directory:
    ```bash
    cd web
    ```
2.  Install dependencies:
    ```bash
    pnpm install
    ```
3.  Start the development server:
    ```bash
    pnpm dev
    ```

## 📜 Makefile Commands

We provide a `Makefile` to simplify common tasks:

-   `make migrate-up`: Run database migrations.
-   `make migrate-down`: Revert database migrations.
-   `make migration name=<name>`: Create a new migration file.
-   `make sqlc-generate`: Generate Go code from SQL queries using `sqlc`.
-   `make codegen-client`: Generate the TypeScript API client from `openapi.yaml`.
-   `make seed`: Seed the database with initial data.

## 🔒 Security

We take security seriously. Since this tool manages your database credentials:

- It integrates natively with **Kubernetes Secrets**.
- Supports encryption keys stored in external KMS (optional).
- Every SQL execution is logged for audit purposes.

## ⚖️ License

This project is licensed under the **GNU Affero General Public License v3.0 (AGPL-3.0)**.

We believe in the power of open-source. If you incorporate this software into a network service, you must make your modified source code available to the community. For commercial licensing without AGPL restrictions, please contact felipemalacarne012@gmail.com.

---

Built for the community, powered by Go.