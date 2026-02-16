# Mesa Frontend

The web interface for Mesa, built with React, Vite, and the TanStack ecosystem.

## 🚀 Getting Started

1.  **Install dependencies:**
    ```bash
    pnpm install
    ```

2.  **Setup Environment Variables:**
    Ensure the backend URL is correctly pointed. By default, it expects:
    ```env
    VITE_API_BASE=http://localhost:8080/api
    ```

3.  **Run the development server:**
    ```bash
    pnpm dev
    ```

## 🛠 Tech Stack

-   **Framework:** [React](https://react.dev/) + [Vite](https://vitejs.dev/)
-   **Routing:** [TanStack Router](https://tanstack.com/router)
-   **Data Fetching:** [TanStack Query](https://tanstack.com/query)
-   **Styling:** [Tailwind CSS](https://tailwindcss.com/)
-   **UI Components:** [Shadcn/UI](https://ui.shadcn.com/)
-   **API Client:** Generated via `openapi-typescript-codegen`

## 📡 API Client Generation

The frontend uses a generated API client to communicate with the backend. If the backend API definition (`openapi.yaml`) changes, you need to regenerate the client.

From the project root, run:
```bash
make codegen-client
```

This will update `src/api` with the latest types and service calls.

## 🏗 Project Structure

-   `src/routes`: File-based routing configuration.
-   `src/components`: UI components (including Shadcn primitives).
-   `src/api`: Generated API client.
-   `src/hooks`: Custom React hooks.

## 🧪 Testing

Run unit tests with Vitest:
```bash
pnpm test
```

## 📦 Building for Production

To build the application for production:
```bash
pnpm build
```