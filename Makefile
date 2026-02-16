MIGRATIONS_DIR := ./internal/infrastructure/postgres/migrations/
SEEDS_DIR := ./internal/infrastructure/postgres/seeds/
DATABASE_URL := postgres://mesa:mesa@localhost:5432/mesa?sslmode=disable
MIGRATE ?= migrate
SQLC ?= sqlc
OPENAPI_PATH := ./openapi.yaml


%:
	@:

.PHONY: migration
migration:
	@if [ -z "$(name)" ] && [ -z "$(filter-out $@,$(MAKECMDGOALS))" ]; then echo "Usage: make migration <name> (or make migration name=<name>)"; exit 1; fi
	@migration_name="$(name)"; \
	if [ -z "$$migration_name" ]; then migration_name="$(filter-out $@,$(MAKECMDGOALS))"; fi; \
	$(MIGRATE) create -ext sql -dir $(MIGRATIONS_DIR) -seq "$$migration_name"

.PHONY: migrate-up
migrate-up:
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database $(DATABASE_URL) up

.PHONY: migrate-down
migrate-down:
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database $(DATABASE_URL) down

.PHONY: sqlc-generate
sqlc-generate:
	$(SQLC) generate

.PHONY: generate-app-key
generate-app-key:
	@openssl rand -hex 32

.PHONY: codegen-api
codegen-api:
	@oapi-codegen -generate chi,types -package contract -o internal/transport/rest/contract/api.gen.go openapi.yaml

.PHONY: codegen-client
codegen-client:
	pnpm dlx openapi-typescript-codegen@0.29.0 --input $(OPENAPI_PATH) --output web/src/api --client fetch --useOptions

.PHONY: seed
seed:
	@for f in $(SEEDS_DIR)*.sql; do \
		[ -e "$$f" ] || exit 0; \
		echo "Seeding $$f"; \
		psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f "$$f"; \
	done

