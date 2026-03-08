BINARY=kiosk-server
MAIN=./cmd/server
MIGRATIONS=./migrations
MIGRATE_URL?=$(shell grep DB_URL .env 2>/dev/null | cut -d= -f2)

.PHONY: all build run dev clean test migrate-up migrate-down migrate-create docker-up docker-down swag

all: build

## Build the application binary
build:
	@echo "Building..."
	go build -ldflags="-w -s" -o $(BINARY) $(MAIN)

## Run with pretty logging
run: build
	APP_LOG_FORMAT=pretty ./$(BINARY)

## Run with hot reload (requires air: go install github.com/air-verse/air@latest)
dev:
	air

## Run tests
test:
	go test ./... -v -race -timeout 60s

## Run tests with coverage
test-coverage:
	go test ./... -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## Run database migrations up
migrate-up:
	go run ./cmd/migrate up

## Roll back last migration
migrate-down:
	go run ./cmd/migrate down

## Create a new migration (make migrate-create name=add_something)
migrate-create:
	@if [ -z "$(name)" ]; then echo "Usage: make migrate-create name=migration_name"; exit 1; fi
	migrate create -ext sql -dir $(MIGRATIONS) -seq $(name)

## Start Docker services
docker-up:
	docker compose up -d

## Stop Docker services
docker-down:
	docker compose down

## Start only Postgres
db-up:
	docker compose up -d postgres

## Stop Postgres
db-down:
	docker compose stop postgres

## Generate Swagger docs (requires swag: go install github.com/swaggo/swag/cmd/swag@latest)
swag:
	swag init -g cmd/server/main.go -o docs/

## Format code
fmt:
	go fmt ./...
	goimports -w .

## Lint (requires golangci-lint)
lint:
	golangci-lint run ./...

## Clean build artifacts
clean:
	rm -f $(BINARY)
	rm -f coverage.out coverage.html

## Show help
help:
	@grep -E '^##' Makefile | sed 's/## //'
