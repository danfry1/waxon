VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -X main.version=$(VERSION)
BINARY   := waxon

.PHONY: build test lint fmt cover install clean check

build: ## Build the binary
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

test: ## Run tests with race detector
	go test -race ./...

lint: ## Run golangci-lint
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4 run

fmt: ## Format code with gofumpt
	go run mvdan.cc/gofumpt@latest -w .

cover: ## Run tests with coverage
	go test -race -coverprofile=coverage.out ./...

check: fmt lint test ## Format, lint, and test (full CI locally)

install: ## Install the binary
	go install -ldflags "$(LDFLAGS)" .

clean: ## Remove build artifacts
	rm -f $(BINARY) coverage.out
