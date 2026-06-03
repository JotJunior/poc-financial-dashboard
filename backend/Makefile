# Makefile — Financial Dashboard Backend
# Requisitos: go, golang-migrate (CLI), docker-compose na raiz do projeto

MIGRATE_CMD := migrate
MIGRATIONS_DIR := migrations
DB_URL ?= postgres://financialuser:financialpass@localhost:5432/financial_dashboard?sslmode=disable

.PHONY: build test test-integration lint vet \
        migrate-up migrate-down migrate-create \
        run clean help

# ─── Build ─────────────────────────────────────────────────────────────────────

build:
	go build ./...

vet:
	go vet ./...

lint:
	@if command -v golangci-lint > /dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		go vet ./...; \
		echo "golangci-lint nao instalado — apenas go vet executado"; \
	fi

# ─── Testes ────────────────────────────────────────────────────────────────────

test:
	go test ./... -v -race -count=1

test-integration:
	DATABASE_URL="$(DB_URL)" go test ./... -v -race -count=1 -tags integration

# ─── Servidor ──────────────────────────────────────────────────────────────────

run:
	DATABASE_URL="$(DB_URL)" go run ./cmd/api/main.go

# ─── Migrations (golang-migrate CLI) ──────────────────────────────────────────

migrate-up:
	$(MIGRATE_CMD) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" up

migrate-down:
	$(MIGRATE_CMD) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" down 1

migrate-force:
	$(MIGRATE_CMD) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" force $(VERSION)

migrate-create:
	@if [ -z "$(NAME)" ]; then echo "Uso: make migrate-create NAME=<nome>"; exit 1; fi
	$(MIGRATE_CMD) create -ext sql -dir $(MIGRATIONS_DIR) -seq $(NAME)

migrate-status:
	$(MIGRATE_CMD) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" version

# ─── Utilitários ───────────────────────────────────────────────────────────────

clean:
	go clean ./...
	rm -rf dist/

help:
	@echo "Targets disponíveis:"
	@echo "  build             — go build ./..."
	@echo "  test              — go test ./... (unit)"
	@echo "  test-integration  — go test com tag integration + DB real"
	@echo "  lint              — golangci-lint (fallback: go vet)"
	@echo "  vet               — go vet ./..."
	@echo "  run               — iniciar servidor (dev)"
	@echo "  migrate-up        — aplicar todas as migrations pendentes"
	@echo "  migrate-down      — reverter última migration"
	@echo "  migrate-create    — criar nova migration (NAME=nome)"
	@echo "  migrate-status    — versão atual da migration"
