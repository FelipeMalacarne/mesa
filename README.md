# 🚀 Mesa

**The open-source SQL Control Plane for modern infrastructure.**

Mesa is a web-based SQL client designed for developers and SREs who need a "cloud provider" experience within their own infrastructure. It goes beyond simple queries, offering a full suite for managing database users, permissions, and multi-instance configurations.

## 🌟 Why Mesa?

- **Infrastructure-First:** Designed to run as a lightweight, single-binary pod in your Kubernetes cluster.
- **Security & Trust:** All database credentials are encrypted at rest (AES-256-GCM) and the code is fully audit-at-will under AGPL v3.
- **User Provisioning:** A clean UI to manage DB users (GRANT/REVOKE) without memorizing vendor-specific syntax.
- **Modern UX:** High-performance editor built with Shadcn/UI and React, featuring a sleek, cloud-like dashboard.

## 🛠 Tech Stack

- **Backend:** Go (fiber/echo) - fast, concurrent, and memory-efficient.
- **Frontend:** React (Vite) + Shadcn/UI + Tailwind CSS.
- **Metadata DB:** PostgreSQL (with JSONB for flexible settings).
- **Deployment:** Docker-native, optimized for Kubernetes & ArgoCD.

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
