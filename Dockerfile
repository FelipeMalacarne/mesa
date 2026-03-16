FROM node:20-alpine AS builder-web
WORKDIR /app/web
COPY web/package.json web/pnpm-lock.yaml ./
RUN corepack enable && corepack prepare pnpm@latest --activate
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm run build

FROM golang:1.24-alpine AS builder-go
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=builder-web /app/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /mesa ./cmd/server

FROM alpine:3.21
RUN addgroup -S mesa && adduser -S mesa -G mesa
COPY --from=builder-go /mesa /usr/local/bin/mesa
USER mesa
EXPOSE 8080
CMD ["mesa"]
