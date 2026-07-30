CMD_DIR ?= cmd

.PHONY: run build test takesnapshot creditbalance now

now:
	go run ./cmd/agent report intraday-pulse


run: ## Run the app: go run
	go run ./$(CMD_DIR)

DATE ?= $(shell date +%Y%m%d)

takesnapshot:
	go run ./cmd/agent report market-snapshot --date $(DATE)

SYMBOLS ?=
WATCHLIST ?= watchlist.txt
creditbalance: ## Credit balance: make creditbalance or make creditbalance SYMBOLS="005930"
	@if [ -n "$(SYMBOLS)" ]; then \
		go run ./cmd/agent report credit-balance $(SYMBOLS); \
	else \
		go run ./cmd/agent report credit-balance --watchlist $(WATCHLIST); \
	fi
build: ## Build the app
	go build -o bin/kis-open-api ./$(CMD_DIR)

test: ## Run the tests
	go test ./...
