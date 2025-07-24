#!/bin/bash
set -e

echo "🚀 Running Tejedor E2E Tests"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Cleanup function (not used automatically)
cleanup() {
    print_status "Cleaning up..."
    pkill -f "tejedor.*config.json" 2>/dev/null || true
    podman stop tejedor-test-pypi 2>/dev/null || true
    podman rm tejedor-test-pypi 2>/dev/null || true
    rm -rf test-venv-* config.json
}

# Start the test environment
print_status "Starting test environment..."
./setup.sh &
SETUP_PID=$!

# Wait for environment to be ready
print_status "Waiting for environment to be ready..."
sleep 30

# Wait for PyPI server to be ready
print_status "Waiting for PyPI server to be ready..."
for i in {1..30}; do
    if curl -f http://127.0.0.1:8098/simple/ >/dev/null 2>&1; then
        print_status "PyPI server is ready"
        break
    fi
    if [ $i -eq 30 ]; then
        print_error "PyPI server failed to start"
        exit 1
    fi
    sleep 2
done

# Wait for tejedor proxy to be ready
print_status "Waiting for tejedor proxy to be ready..."
for i in {1..30}; do
    if curl -f http://127.0.0.1:8099/simple/ >/dev/null 2>&1; then
        print_status "Tejedor proxy is ready"
        break
    fi
    if [ $i -eq 30 ]; then
        print_error "Tejedor proxy failed to start"
        exit 1
    fi
    sleep 2
done

print_status "Test environment is ready!"

# Run the Go tests
print_status "Running Go-based E2E tests..."
go test -v -timeout 5m -tags=e2e .

print_status "🎉 All tests completed!"
print_status ""
print_status "📋 Test Environment Management:"
print_status "  • Test environment is still running for inspection"
print_status "  • Proxy URL: http://127.0.0.1:8099"
print_status "  • Private PyPI URL: http://127.0.0.1:8098"
print_status "  • Tejedor PID: $SETUP_PID"
print_status ""
print_status "🧹 To clean up the test environment:"
print_status "  • Run: make clean-e2e"
print_status "  • Or run: make clean-all (removes containers and images)"
print_status ""
print_status "🔍 To inspect the test environment:"
print_status "  • Check proxy logs: ps aux | grep tejedor"
print_status "  • Inspect container: podman exec -it tejedor-test-pypi bash"
print_status "  • Test manually: curl http://127.0.0.1:8099/simple/"
print_status ""
print_status "⚠️  Note: Test environment is still running. Clean up when done."