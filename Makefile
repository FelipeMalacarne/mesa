MIGRATIONS_DIR := ./internal/infrastructure/postgres/migrations/
DATABASE_URL := postgres://mesa:mesa@localhost:5432/mesa?sslmode=disable
MIGRATE ?= migrate
SQLC ?= sqlc

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

.PHONY: generate-master-key
generate-app-key:
	@openssl rand -hex 32

