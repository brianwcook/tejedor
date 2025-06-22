# PyPI Proxy Makefile

# Variables
BINARY_NAME=pypi-proxy
MAIN_FILE=main.go
CONFIG_FILE=config.yaml

# Go variables
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build flags
LDFLAGS=-ldflags "-X main.Version=$(shell git describe --tags --always --dirty)"

# Default target
.DEFAULT_GOAL := build

# Build the application
.PHONY: build
build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) $(MAIN_FILE)

# Build for multiple platforms
.PHONY: build-all
build-all: build-linux build-darwin build-windows

.PHONY: build-linux
build-linux:
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-linux $(MAIN_FILE)

.PHONY: build-darwin
build-darwin:
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin $(MAIN_FILE)

.PHONY: build-windows
build-windows:
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-windows.exe $(MAIN_FILE)

# Clean build artifacts
.PHONY: clean
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME)-linux
	rm -f $(BINARY_NAME)-darwin
	rm -f $(BINARY_NAME)-windows.exe

# Run the application
.PHONY: run
run: build
	./$(BINARY_NAME)

# Run with specific config
.PHONY: run-config
run-config: build
	./$(BINARY_NAME) -config $(CONFIG_FILE)

# Run in development mode
.PHONY: dev
dev:
	$(GOCMD) run $(MAIN_FILE)

# Run with development config
.PHONY: dev-config
dev-config:
	$(GOCMD) run $(MAIN_FILE) -config $(CONFIG_FILE)

# Install dependencies
.PHONY: deps
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Run tests
.PHONY: test
test:
	$(GOTEST) ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	$(GOTEST) -cover ./...

# Run tests with verbose output
.PHONY: test-verbose
test-verbose:
	$(GOTEST) -v ./...

# Run unit tests only
.PHONY: test-unit
test-unit:
	$(GOTEST) ./cache/...
	$(GOTEST) ./config/...
	$(GOTEST) ./pypi/...

# Run integration tests only
.PHONY: test-integration
test-integration:
	$(GOTEST) ./integration/...

# Run integration tests with cache testing
.PHONY: test-integration-cache
test-integration-cache:
	$(GOTEST) -v ./integration/ -run TestProxyWithCache

# Run tests with race detection
.PHONY: test-race
test-race:
	$(GOTEST) -race ./...

# Format code (uses gofumpt only - same as CI)
# Note: Ensure you're using the same Go version and tool versions as CI (Go 1.21)
.PHONY: fmt
fmt:
	$(shell go env GOPATH)/bin/gofumpt -w .

# Run linter (uses golangci-lint - same as CI)
# Note: Ensure you're using the same Go version and tool versions as CI (Go 1.21)
.PHONY: lint
lint:
	$(shell go env GOPATH)/bin/golangci-lint run

# Install linter
.PHONY: install-lint
install-lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run security scan (uses gosec - same as CI)
# Note: Ensure you're using the same Go version and tool versions as CI (Go 1.21)
.PHONY: security
security:
	$(shell go env GOPATH)/bin/gosec -fmt=json -out=security-report.json ./...

# Install security scanner
.PHONY: install-security
install-security:
	go install github.com/securego/gosec/v2/cmd/gosec@latest

# Create default config file
.PHONY: config
config:
	@if [ ! -f $(CONFIG_FILE) ]; then \
		echo "Creating default config file..."; \
		cp config.yaml $(CONFIG_FILE); \
		echo "Please edit $(CONFIG_FILE) with your private PyPI URL"; \
	else \
		echo "Config file $(CONFIG_FILE) already exists"; \
	fi

# Show help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build              - Build the application"
	@echo "  build-all          - Build for all platforms (Linux, Darwin, Windows)"
	@echo "  clean              - Clean build artifacts"
	@echo "  run                - Build and run the application"
	@echo "  run-config         - Build and run with config file"
	@echo "  dev                - Run in development mode"
	@echo "  dev-config         - Run in development mode with config file"
	@echo "  deps               - Install dependencies"
	@echo "  test               - Run all tests"
	@echo "  test-coverage      - Run tests with coverage"
	@echo "  test-verbose       - Run tests with verbose output"
	@echo "  test-unit          - Run unit tests only"
	@echo "  test-integration   - Run integration tests only"
	@echo "  test-race          - Run tests with race detection"
	@echo "  fmt                - Format code"
	@echo "  lint               - Run linter"
	@echo "  install-lint       - Install linter"
	@echo "  security           - Run security scan"
	@echo "  install-security   - Install security scanner"
	@echo "  config             - Create default config file"
	@echo "  help               - Show this help message" 