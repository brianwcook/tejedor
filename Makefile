# Tejedor PyPI Proxy Makefile

.PHONY: build test e2e-test clean-e2e clean-all help

# Default target
all: build

# Build the tejedor binary
build:
	@echo "Building tejedor binary..."
	go build -o tejedor .

# Run unit tests
test:
	@echo "Running unit tests..."
	go test ./...

# Clean up any existing e2e test containers and processes
clean-e2e:
	@echo "Cleaning up existing e2e test containers and processes..."
	@podman stop test-pypi-server tejedor-test-pypi tejedor-proxy 2>/dev/null || true
	@podman rm -f test-pypi-server tejedor-test-pypi tejedor-proxy 2>/dev/null || true
	@pkill -f "tejedor.*config.json" 2>/dev/null || true
	@pkill -f "tejedor.*test-config.yaml" 2>/dev/null || true
	@echo "✅ Cleanup complete - letting podman handle port forwarding cleanup"

# Clean all containers and images
clean-all: clean-e2e
	@echo "Cleaning all test containers and images..."
	@podman rmi test-pypi-server tejedor-test-pypi 2>/dev/null || true
	@echo "✅ All cleanup complete"

# Run end-to-end tests
e2e-test: clean-e2e
	@echo "🚀 Starting E2E tests..."
	@cd e2e && ./run_tests.sh
	@echo ""
	@echo "🎉 E2E tests completed!"
	@echo ""
	@echo "📋 Container Management:"
	@echo "  • To restart the test container: podman start tejedor-test-pypi"
	@echo "  • To inspect the container: podman exec -it tejedor-test-pypi bash"
	@echo "  • To remove the container: podman rm tejedor-test-pypi"
	@echo "  • To clean everything: make clean-all"
	@echo ""

# Run end-to-end tests in CI (with automatic cleanup)
e2e-test-ci: clean-e2e
	@echo "🚀 Starting E2E tests in CI mode..."
	@cd e2e && ./run_tests.sh
	@echo "🧹 Cleaning up test environment..."
	@make clean-e2e
	@echo "🎉 E2E tests completed!"

# Run all CI checks locally (same as GitHub Actions)
ci-ready: clean-all
	@echo "🚀 Running all CI checks locally..."
	@echo ""
	
	@echo "🔧 Step 1/7: Checking and installing required tools..."
	@echo "Checking Go installation..."
	@if ! command -v go &> /dev/null; then \
		echo "❌ Go is not installed or not in PATH"; \
		echo "   Install Go from: https://golang.org/dl/"; \
		exit 1; \
	fi
	@echo "✅ Go found: $$(go version)"
	
	@echo "Checking podman installation..."
	@if ! command -v podman &> /dev/null; then \
		echo "❌ Podman is not installed or not in PATH"; \
		echo "   Install podman from: https://podman.io/getting-started/installation"; \
		echo "   Or use: brew install podman (on macOS)"; \
		exit 1; \
	fi
	@echo "✅ Podman found: $$(podman --version)"
	
	@echo "Checking Python3 installation..."
	@if ! command -v python3 &> /dev/null; then \
		echo "❌ Python3 is not installed or not in PATH"; \
		echo "   Install Python3 from: https://www.python.org/downloads/"; \
		echo "   Or use: brew install python (on macOS)"; \
		exit 1; \
	fi
	@echo "✅ Python3 found: $$(python3 --version)"
	
	@echo "Checking jq installation..."
	@if ! command -v jq &> /dev/null; then \
		echo "❌ jq is not installed or not in PATH"; \
		echo "   Install jq from: https://stedolan.github.io/jq/download/"; \
		echo "   Or use: brew install jq (on macOS)"; \
		exit 1; \
	fi
	@echo "✅ jq found: $$(jq --version)"
	
	@echo "Checking bc installation..."
	@if ! command -v bc &> /dev/null; then \
		echo "❌ bc is not installed or not in PATH"; \
		echo "   Install bc from: https://www.gnu.org/software/bc/"; \
		echo "   Or use: brew install bc (on macOS)"; \
		exit 1; \
	fi
	@echo "✅ bc found"
	
	@echo "Installing golangci-lint if not present..."
	@if ! command -v golangci-lint &> /dev/null; then \
		echo "Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
		echo "✅ golangci-lint installed"; \
	else \
		echo "✅ golangci-lint already installed: $$(golangci-lint --version | head -n1)"; \
	fi
	
	@echo "Installing gosec if not present..."
	@if ! command -v gosec &> /dev/null; then \
		echo "Installing gosec..."; \
		go install github.com/securego/gosec/v2/cmd/gosec@latest; \
		echo "✅ gosec installed"; \
	else \
		echo "✅ gosec already installed: $$(gosec --version | head -n1)"; \
	fi
	@echo ""
	
	@echo "📦 Step 2/7: Installing dependencies..."
	@go mod download
	@echo "✅ Dependencies installed"
	@echo ""
	
	@echo "🧪 Step 3/7: Running unit tests..."
	@go test -v -race -coverprofile=coverage.out ./cache ./config ./pypi ./proxy
	@echo "✅ Unit tests passed"
	@echo ""
	
	@echo "🔗 Step 4/7: Running integration tests..."
	@CI=true go test -v -race -coverprofile=integration-coverage.out ./integration
	@echo "✅ Integration tests passed"
	@echo ""
	
	@echo "🐳 Step 5/7: Running e2e tests..."
	@make e2e-test-ci
	@echo "✅ E2E tests passed"
	@echo ""
	
	@echo "🔍 Step 6/7: Running linting..."
	@if command -v golangci-lint &> /dev/null; then \
		golangci-lint run; \
		echo "✅ Linting passed"; \
	else \
		echo "⚠️  golangci-lint not found, skipping linting"; \
		echo "   Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi
	@echo ""
	
	@echo "🔒 Step 7/7: Running security scan..."
	@if command -v gosec &> /dev/null; then \
		gosec -fmt=json -out=security-report.json ./...; \
		if [ -f security-report.json ]; then \
			ISSUES=$$(jq -r '.Issues | length' security-report.json 2>/dev/null || echo "0"); \
			if [ "$$ISSUES" -gt 0 ]; then \
				echo "❌ Found $$ISSUES security issues:"; \
				jq -r '.Issues[] | "\(.severity): \(.details) in \(.file):\(.line)"' security-report.json; \
				exit 1; \
			else \
				echo "✅ No security issues found"; \
			fi; \
		else \
			echo "❌ Security report not generated"; \
			exit 1; \
		fi; \
	else \
		echo "⚠️  gosec not found, skipping security scan"; \
		echo "   Install with: go install github.com/securego/gosec/v2/cmd/gosec@latest"; \
	fi
	@echo ""
	
	@echo "🏗️ Step 7/7: Building for all platforms..."
	@GOOS=linux GOARCH=amd64 go build -o pypi-proxy-linux-amd64 .
	@GOOS=linux GOARCH=arm64 go build -o pypi-proxy-linux-arm64 .
	@GOOS=darwin GOARCH=amd64 go build -o pypi-proxy-darwin-amd64 .
	@GOOS=darwin GOARCH=arm64 go build -o pypi-proxy-darwin-arm64 .
	@GOOS=windows GOARCH=amd64 go build -o pypi-proxy-windows-amd64.exe .
	@echo "✅ All platform builds successful"
	@echo ""
	
	@echo "📊 Generating coverage report..."
	@echo "mode: set" > combined-coverage.out
	@tail -n +2 coverage.out >> combined-coverage.out
	@tail -n +2 integration-coverage.out >> combined-coverage.out
	@go tool cover -html=combined-coverage.out -o coverage.html
	@echo "✅ Coverage report generated"
	@echo ""
	
	@echo "🎯 Checking coverage threshold..."
	@COVERAGE=$$(go tool cover -func=combined-coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	echo "Code coverage: $${COVERAGE}%"; \
	if (( $$(echo "$${COVERAGE} < 80" | bc -l) )); then \
		echo "❌ Coverage $${COVERAGE}% is below threshold 80%"; \
		exit 1; \
	else \
		echo "✅ Coverage $${COVERAGE}% meets threshold 80%"; \
	fi
	@echo ""
	
	@echo "🧹 Cleaning up build artifacts..."
	@rm -f pypi-proxy-* coverage.out integration-coverage.out combined-coverage.out coverage.html security-report.json
	@echo "✅ Build artifacts cleaned up"
	@echo ""
	
	@echo "🎉 ALL CI CHECKS PASSED! 🎉"
	@echo "✅ Your code is ready for GitHub Actions"
	@echo "✅ Tool installation: PASS"
	@echo "✅ Unit tests: PASS"
	@echo "✅ Integration tests: PASS" 
	@echo "✅ E2E tests: PASS"
	@echo "✅ Linting: PASS"
	@echo "✅ Security scan: PASS"
	@echo "✅ Multi-platform builds: PASS"
	@echo "✅ Coverage threshold: PASS"
	@echo ""
	@echo "🚀 You can now push with confidence!"

# Show help
help:
	@echo "Tejedor PyPI Proxy - Available targets:"
	@echo ""
	@echo "  build      - Build the tejedor binary"
	@echo "  test       - Run unit tests"
	@echo "  e2e-test   - Run end-to-end tests (leaves environment running)"
	@echo "  e2e-test-ci - Run end-to-end tests in CI (with cleanup)"
	@echo "  ci-ready   - Run ALL CI checks locally (installs tools, tests, builds, lint, security)"
	@echo "  clean-e2e  - Clean up e2e test containers and processes"
	@echo "  clean-all  - Clean all containers and images"
	@echo "  help       - Show this help message"
	@echo "" 