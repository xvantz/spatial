.PHONY: help up down gen build test lint clean run-go run-node

# Default environment variables
NATS_URL ?= nats://localhost:4222

help: ## Display help for commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

## --- Infrastructure ---

up: ## Start NATS in Docker
	docker compose up nats -d

down: ## Stop all containers
	docker compose down

## --- Code Generation ---

gen: ## Generate Protobuf files for Go and Node.js
	buf generate

## --- Go Worker (Coprocessor) ---

build-go: ## Build Go worker binary
	cd go-worker && go build -o spatial-worker ./cmd/coprocessor/main.go

run-go: ## Run Go worker locally
	cd go-worker && NATS_URL=$(NATS_URL) go run ./cmd/coprocessor/main.go

test-go: ## Run Go tests
	cd go-worker && go test -v ./...

lint-go: ## Run linter for Go
	cd go-worker && golangci-lint run ./cmd/... ./internal/...

## --- Node Plane (Master) ---

install-node: ## Install Node.js dependencies
	cd node-plane && pnpm install

run-node: ## Run Node-plane locally
	cd node-plane && NATS_URL=$(NATS_URL) pnpm run dev

check-node: ## Check types in TypeScript
	cd node-plane && bunx tsc --noEmit

## --- General Commands ---

test: test-go ## Run all tests in the project

bench-go: ## Run Go benchmarks
	cd go-worker && go test -bench=. -benchmem ./internal/engine/...

lint: lint-go check-node ## Run all checks (linters and types)

docker-up: ## Build and start everything in Docker
	docker compose up --build

clean: ## Clean generated files and binaries
	rm -rf go-worker/gen/spatial
	rm -rf node-plane/gen/spatial
	rm -f go-worker/spatial-worker
