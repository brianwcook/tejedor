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

# Show help
help:
	@echo "Tejedor PyPI Proxy - Available targets:"
	@echo ""
	@echo "  build      - Build the tejedor binary"
	@echo "  test       - Run unit tests"
	@echo "  e2e-test   - Run end-to-end tests (cleans up first)"
	@echo "  clean-e2e  - Clean up e2e test containers and processes"
	@echo "  clean-all  - Clean all containers and images"
	@echo "  help       - Show this help message"
	@echo "" 