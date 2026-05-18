CMD_DIR ?= cmd

.PHONY: run build test takesnapshot

run: ## Run the app: go run
	go run ./$(CMD_DIR)

takesnapshot:
	go run ./cmd/agent report market-snapshot --date $$(date +%Y%m%d)

build: ## Build the app
	go build -o bin/kis-open-api ./$(CMD_DIR)

test: ## Run the tests
	go test ./...
