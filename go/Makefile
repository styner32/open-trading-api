CMD_DIR ?= cmd

.PHONY: run test

run: ## Run the app: go run
	go run $(CMD_DIR)/main.go

build: ## Build the app
	go build -o bin/kis-open-api $(CMD_DIR)/main.go

test: ## Run the tests
	go test ./...