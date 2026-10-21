GO       ?= go
PNPM     ?= pnpm
BINDIR   := bin
PG_DIR   ?= /tmp/mesa-pg
PG_PORT  ?= 5499

.PHONY: all build desktop web generate test test-integration lint fmt dev-db dev-db-stop clean docker-build package-linux

all: generate web build

build: ## Build every binary (mesa-server expects web/dist; run `make web` first)
	$(GO) build -trimpath -ldflags="-s -w" -o $(BINDIR)/mesa-server ./cmd/server
	$(GO) build -trimpath -ldflags="-s -w" -o $(BINDIR)/mesa ./cmd/cli
	$(GO) build -trimpath -ldflags="-s -w" -o $(BINDIR)/mesa-mcp ./cmd/mcp

ifeq ($(shell uname -s),Darwin)
desktop: export CGO_LDFLAGS := -framework UniformTypeIdentifiers
desktop: export MACOSX_DEPLOYMENT_TARGET := 11.0
endif
desktop: ## Build bin/mesa-desktop (Linux: needs libwebkit2gtk-4.1-dev and libgtk-3-dev)
	CGO_ENABLED=1 $(GO) build -tags desktop,production,webkit2_41 -trimpath -ldflags="-s -w" -o $(BINDIR)/mesa-desktop ./cmd/desktop

web: ## Build the web app into web/dist
	cd web && $(PNPM) install --frozen-lockfile && $(PNPM) run build

generate: ## Regenerate OpenAPI server/client, sqlc
	$(GO) generate ./...
	cd web && $(PNPM) run codegen

test: ## Unit tests (integration tests skip unless MESA_TEST_PG is set)
	$(GO) test ./...

test-integration: ## Run everything against the dev Postgres
	MESA_TEST_PG="postgres://postgres@localhost:$(PG_PORT)/postgres?sslmode=disable" $(GO) test ./...

lint:
	golangci-lint run ./...
	cd web && $(PNPM) run typecheck

fmt:
	gofmt -w ./cmd ./internal

dev-db: ## Start a throwaway PostgreSQL on $(PG_PORT) (needs postgres binaries on PATH)
	@mkdir -p $(PG_DIR) && [ -d $(PG_DIR)/data ] || initdb -D $(PG_DIR)/data -U postgres --auth=trust >/dev/null
	@pg_ctl -D $(PG_DIR)/data -o "-p $(PG_PORT)" -l $(PG_DIR)/server.log start

dev-db-stop:
	@pg_ctl -D $(PG_DIR)/data stop

docker-build:
	docker build -t mesa:latest .

package-linux: desktop ## Linux desktop archive in dist/ (VERSION=x.y.z)
	packaging/linux/package.sh $(BINDIR)/mesa-desktop $$($(GO) env GOARCH) $(or $(VERSION),0.0.0-dev) dist

clean:
	rm -rf bin web/dist
