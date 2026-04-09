APP_NAME := hzuul
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build run clean tidy lint test vet check coverage help

build: ## Build the binary
	go build $(LDFLAGS) -o bin/$(APP_NAME) ./cmd/hzuul

run: build ## Build and run
	./bin/$(APP_NAME)

tidy: ## Tidy go modules
	go mod tidy

clean: ## Remove build artifacts
	rm -rf bin/

test: ## Run unit tests
	go test ./... -count=1

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint
	golangci-lint run ./...

coverage: ## Run tests with coverage report
	go test ./internal/ai/... ./internal/api/... ./internal/config/... -coverprofile=coverage.out -count=1
	go tool cover -func=coverage.out | tail -1
	@echo "  Run 'go tool cover -html=coverage.out' for detailed report"

check: vet lint test ## Run vet, lint, and tests

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'
