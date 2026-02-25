.PHONY: all build run test clean migrate-up migrate-down seed db-reset deps lint help

# Variables
BINARY_NAME=fabricalaser-api
MAIN_PATH=cmd/server/main.go
DB_NAME=fabricalaser
DB_USER=fabricalaser
DB_HOST=localhost
DB_PORT=5432

# Colors
GREEN=\033[0;32m
NC=\033[0m # No Color

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "$(GREEN)%-15s$(NC) %s\n", $$1, $$2}'

all: deps build ## Install deps and build

deps: ## Install Go dependencies
	@echo "Installing dependencies..."
	go mod tidy
	go mod download

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	go build -o bin/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Binary created: bin/$(BINARY_NAME)"

run: ## Run the server (development)
	@echo "Starting server..."
	go run $(MAIN_PATH)

test: ## Run tests
	go test -v ./...

test-cover: ## Run tests with coverage
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

clean: ## Clean build artifacts
	rm -rf bin/
	rm -f coverage.out coverage.html

lint: ## Run linter
	golangci-lint run

# Database commands
db-create: ## Create PostgreSQL database
	@echo "Creating database $(DB_NAME)..."
	sudo -u postgres psql -c "CREATE USER $(DB_USER) WITH PASSWORD 'fabricalaser_password';" || true
	sudo -u postgres psql -c "CREATE DATABASE $(DB_NAME) OWNER $(DB_USER);" || true
	sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE $(DB_NAME) TO $(DB_USER);"
	@echo "Database created successfully"

migrate-up: ## Apply all migrations
	@echo "Applying migrations..."
	@for file in migrations/*.sql; do \
		echo "Running $$file..."; \
		PGPASSWORD=fabricalaser_password psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f $$file; \
	done
	@echo "Migrations complete"

migrate-down: ## Rollback (drop all tables - DANGEROUS)
	@echo "WARNING: This will drop all tables!"
	@read -p "Are you sure? [y/N] " confirm && [ "$$confirm" = "y" ] || exit 1
	PGPASSWORD=fabricalaser_password psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -c "\
		DROP TABLE IF EXISTS price_references CASCADE; \
		DROP TABLE IF EXISTS volume_discounts CASCADE; \
		DROP TABLE IF EXISTS tech_rates CASCADE; \
		DROP TABLE IF EXISTS engrave_types CASCADE; \
		DROP TABLE IF EXISTS materials CASCADE; \
		DROP TABLE IF EXISTS technologies CASCADE; \
		DROP TABLE IF EXISTS users CASCADE;"

seed: ## Load seed data
	@echo "Loading seed data..."
	PGPASSWORD=fabricalaser_password psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -f seeds/001_initial_data.sql
	@echo "Seed data loaded"

db-reset: migrate-down migrate-up seed ## Reset database (drop + migrate + seed)
	@echo "Database reset complete"

db-status: ## Show database tables and counts
	@echo "Database status:"
	@PGPASSWORD=fabricalaser_password psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d $(DB_NAME) -c "\
		SELECT 'users' as table_name, COUNT(*) as count FROM users UNION ALL \
		SELECT 'technologies', COUNT(*) FROM technologies UNION ALL \
		SELECT 'materials', COUNT(*) FROM materials UNION ALL \
		SELECT 'engrave_types', COUNT(*) FROM engrave_types UNION ALL \
		SELECT 'tech_rates', COUNT(*) FROM tech_rates UNION ALL \
		SELECT 'volume_discounts', COUNT(*) FROM volume_discounts UNION ALL \
		SELECT 'price_references', COUNT(*) FROM price_references;"

# Deployment
deploy: build ## Build and restart service
	@echo "Deploying..."
	sudo systemctl restart fabricalaser-api
	@echo "Deployed successfully"

logs: ## Show service logs
	sudo journalctl -u fabricalaser-api -f

status: ## Show service status
	sudo systemctl status fabricalaser-api
