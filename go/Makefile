CMD_DIR ?= cmd

.PHONY: run build test

run: ## Run the app: go run
	go run ./$(CMD_DIR)

build: ## Build the app
	go build -o bin/kis-open-api ./$(CMD_DIR)

test: ## Run the tests
	go test ./...
