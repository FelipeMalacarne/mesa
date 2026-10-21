# syntax=docker/dockerfile:1.7
# Mesa service image: the HTTP API with the web app embedded.
# Build:  docker build -t mesa:latest .
# Run:    docker run -p 8080:8080 -e MESA_APP_KEY=$(openssl rand -base64 32) -v mesa:/data mesa:latest

FROM node:22-alpine AS web
WORKDIR /src/web
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml ./
RUN --mount=type=cache,target=/root/.local/share/pnpm/store pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm run build

FROM golang:1.26-alpine AS go
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath \
      -ldflags="-s -w -X github.com/felipemalacarne/mesa/internal/platform.Version=${VERSION}" \
      -o /out/mesa-server ./cmd/server && \
    CGO_ENABLED=0 go build -trimpath \
      -ldflags="-s -w -X github.com/felipemalacarne/mesa/internal/platform.Version=${VERSION}" \
      -o /out/mesa ./cmd/cli

FROM alpine:3.21
RUN addgroup -S mesa && adduser -S mesa -G mesa && mkdir -p /data && chown mesa:mesa /data
COPY --from=go /out/mesa-server /out/mesa /usr/local/bin/
USER mesa
ENV MESA_HOME=/data \
    MESA_STORE=sqlite \
    MESA_STORE_DSN=/data/mesa.db \
    MESA_LISTEN=:8080 \
    MESA_LOG_FORMAT=json
VOLUME ["/data"]
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["mesa-server"]
