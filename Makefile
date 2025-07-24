# Tejedor PyPI Proxy Makefile

.PHONY: build test e2e-test clean-e2e clean-all help

# Default target
all: build

# Build the tejedor binary
build:
	@echo "Building tejedor binary..."
	go build -o tejedor .

# Build Docker image
docker-build:
	@echo "Building Tejedor Docker image..."
	podman build -t tejedor:latest .

# Build and push Docker image
docker-push: docker-build
	@echo "Building and pushing Tejedor Docker image..."
	@./scripts/build-and-push.sh

# Run unit tests
test:
	@echo "Running unit tests..."
	go test ./...

# Clean up any existing e2e test containers and processes
clean-e2e:
	@echo "Cleaning up existing e2e test containers and processes..."
	@podman stop test-pypi-server tejedor-test-pypi tejedor-proxy 2>/dev/null || true
	@podman rm -f test-pypi-server tejedor-test-pypi tejedor-proxy 2>/dev/null || true
	@pgrep -f "tejedor.*config\.json" | xargs -r kill -9 2>/dev/null || true
	@pgrep -f "tejedor.*test-config\.yaml" | xargs -r kill -9 2>/dev/null || true
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
	@podman stop tejedor-test-pypi tejedor-proxy 2>/dev/null || true
	@podman rm -f tejedor-test-pypi tejedor-proxy 2>/dev/null || true
	@echo "🎉 E2E tests completed!"

# Run kind-based Hermeto integration tests
kind-hermeto-test:
	@echo "🚀 Starting Kind + Hermeto E2E tests..."
	@cd e2e && ./kind-hermeto-test.sh
	@echo "🎉 Kind + Hermeto tests completed!"

# Run simple kind-based Hermeto integration tests
kind-hermeto-test-simple:
	@echo "🚀 Starting Simple Kind + Hermeto E2E tests..."
	@cd e2e && ./kind-hermeto-test-simple.sh
	@echo "🎉 Simple Kind + Hermeto tests completed!"

# Run full kind-based Hermeto + Tejedor integration tests
kind-hermeto-test-full:
	@echo "🚀 Starting Full Kind + Hermeto + Tejedor E2E tests..."
	@cd e2e && ./kind-hermeto-test-full.sh
	@echo "🎉 Full Kind + Hermeto + Tejedor tests completed!"

# Run container-based Hermeto integration tests (alternative to kind)
hermeto-e2e-test:
	@echo "🚀 Starting Container-based Hermeto E2E tests..."
	@cd e2e && ./hermeto-e2e-test.sh
	@echo "🎉 Container-based Hermeto tests completed!"

# Run local Hermeto integration tests (no containers required)
hermeto-local-test:
	@echo "🚀 Starting Local Hermeto E2E tests..."
	@cd e2e && ./hermeto-local-test.sh
	@echo "🎉 Local Hermeto tests completed!"

# Run all CI checks locally (same as GitHub Actions)
ci-ready: clean-all
	@echo "🚀 Running all CI checks locally (matching GitHub Actions)..."
	@echo ""
	
	@echo "🔧 Step 1/8: Checking and installing required tools..."
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
	
	@echo "Installing tools dependencies (using tools/go.mod)..."
	@cd tools && go mod download
	@echo "✅ Tools dependencies installed"
	@echo ""
	
	@echo "📦 Step 2/8: Installing dependencies..."
	@go mod download
	@echo "✅ Dependencies installed"
	@echo ""
	
	@echo "🧪 Step 3/8: Running unit tests (same as CI)..."
	@go test -v -race -coverprofile=coverage.out ./cache ./config ./pypi ./proxy
	@echo "✅ Unit tests passed"
	@echo ""
	
	@echo "🔗 Step 4/8: Running integration tests (same as CI)..."
	@CI=true go test -v -race -coverprofile=integration-coverage.out ./integration
	@echo "✅ Integration tests passed"
	@echo ""
	
	@echo "🐳 Step 5/8: Running e2e tests (same as CI)..."
	@make e2e-test-ci
	@echo "✅ E2E tests passed"
	@echo ""
	
	@echo "📊 Step 6/8: Merging coverage reports (same as CI)..."
	@echo "mode: set" > combined-coverage.out
	@tail -n +2 coverage.out >> combined-coverage.out
	@tail -n +2 integration-coverage.out >> combined-coverage.out
	@go tool cover -html=combined-coverage.out -o coverage.html
	@echo "✅ Coverage reports merged"
	@echo ""
	
	@echo "🎯 Step 7/8: Checking coverage threshold (same as CI)..."
	@COVERAGE=$$(go tool cover -func=combined-coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	echo "Code coverage: $${COVERAGE}%"; \
	if (( $$(echo "$${COVERAGE} < 80" | bc -l) )); then \
		echo "❌ Coverage $${COVERAGE}% is below threshold 80%"; \
		exit 1; \
	else \
		echo "✅ Coverage $${COVERAGE}% meets threshold 80%"; \
	fi
	@echo ""
	
	@echo "🔍 Step 8/8: Running linting (same as CI)..."
	@go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 run
	@echo "✅ Linting passed"
	@echo ""
	
	@echo "🔒 Step 9/9: Running security scan (same as CI)..."
	@go run github.com/securego/gosec/v2/cmd/gosec@v2.19.0 -fmt=json -out=security-report.json -exclude=main.go ./cache ./config ./pypi ./proxy ./integration
	@if [ -f security-report.json ]; then \
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
	fi
	@echo ""
	
	@echo "🏗️ Step 10/10: Building for all platforms (same as CI)..."
	@GOOS=linux GOARCH=amd64 go build -o pypi-proxy-linux-amd64 .
	@GOOS=linux GOARCH=arm64 go build -o pypi-proxy-linux-arm64 .
	@GOOS=darwin GOARCH=amd64 go build -o pypi-proxy-darwin-amd64 .
	@GOOS=darwin GOARCH=arm64 go build -o pypi-proxy-darwin-arm64 .
	@GOOS=windows GOARCH=amd64 go build -o pypi-proxy-windows-amd64.exe .
	@echo "✅ All platform builds successful"
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
	@echo "✅ Coverage threshold: PASS"
	@echo "✅ Linting: PASS"
	@echo "✅ Security scan: PASS"
	@echo "✅ Multi-platform builds: PASS"
	@echo ""
	@echo "🚀 You can now push with confidence!"

# Run linting using go run
lint:
	@echo "🔍 Running linting..."
	@go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 run
	@echo "✅ Linting passed"

# Run security scan using go run
security:
	@echo "🔒 Running security scan..."
	@go run github.com/securego/gosec/v2/cmd/gosec@v2.19.0 -fmt=json -out=security-report.json -exclude=main.go ./cache ./config ./pypi ./proxy ./integration
	@if [ -f security-report.json ]; then \
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
	fi

# Show help
help:
	@echo "Tejedor PyPI Proxy - Available targets:"
	@echo ""
	@echo "  build      - Build the tejedor binary"
	@echo "  test       - Run unit tests"
	@echo "  lint       - Run linting (using tools/go.mod)"
	@echo "  security   - Run security scan (using tools/go.mod)"
	@echo "  e2e-test   - Run end-to-end tests (leaves environment running)"
	@echo "  e2e-test-ci - Run end-to-end tests in CI (with cleanup)"
	@echo "  kind-hermeto-test-simple - Run simple Kind + Hermeto integration tests"
	@echo "  kind-hermeto-test-full   - Run full Kind + Hermeto + Tejedor integration tests"
	@echo "  ci-ready   - Run ALL CI checks locally (installs tools, tests, builds, lint, security)"
	@echo "  clean-e2e  - Clean up e2e test containers and processes"
	@echo "  clean-all  - Clean all containers and images"
	@echo "  help       - Show this help message"
	@echo ""
	@echo "📋 Tool Management:"
	@echo "  • Tools use 'go run' approach (no global installations)"
	@echo "  • Use 'make lint' or 'make security' to run individual tools"
	@echo "  • Tools are automatically downloaded when needed" 