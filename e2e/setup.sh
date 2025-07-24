#!/bin/bash
set -e

echo "🚀 Setting up Tejedor E2E Test Environment"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Cleanup function
cleanup() {
    print_status "Cleaning up..."
    $CONTAINER_ENGINE stop tejedor-test-pypi tejedor-proxy 2>/dev/null || true
    $CONTAINER_ENGINE rm tejedor-test-pypi tejedor-proxy 2>/dev/null || true
}

# Set up trap to cleanup on exit
trap cleanup EXIT

# Check prerequisites
print_status "Checking prerequisites..."

# Detect container engine
if command -v podman &> /dev/null; then
    CONTAINER_ENGINE=podman
elif command -v docker &> /dev/null; then
    CONTAINER_ENGINE=docker
else
    print_error "Neither podman nor docker is installed or not in PATH"
    exit 1
fi

print_status "Using container engine: $CONTAINER_ENGINE"

if ! command -v python3 &> /dev/null; then
    print_error "Python3 is not installed or not in PATH"
    exit 1
fi

if ! command -v go &> /dev/null; then
    print_error "Go is not installed or not in PATH"
    exit 1
fi

print_status "Prerequisites check passed"

# Build and start the test environment
print_status "Building and starting test environment..."

# Clean up any existing containers first
print_status "Cleaning up any existing test containers..."
$CONTAINER_ENGINE stop tejedor-test-pypi tejedor-proxy 2>/dev/null || true
$CONTAINER_ENGINE rm tejedor-test-pypi tejedor-proxy 2>/dev/null || true

cd "$(dirname "$0")"

# Build the test PyPI server
print_status "Building test PyPI server..."
$CONTAINER_ENGINE build -t tejedor-test-pypi -f Dockerfile .

# Start the test PyPI server
print_status "Starting test PyPI server..."
$CONTAINER_ENGINE run -d --name tejedor-test-pypi -p 8098:8098 tejedor-test-pypi

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

# Build tejedor
print_status "Building tejedor..."
cd ..
go build -o e2e/tejedor .

# Start tejedor proxy
print_status "Starting tejedor proxy..."
cd e2e

# Create config for tejedor
cat > config.json << EOF
{
  "public_pypi_url": "https://pypi.org/simple/",
  "private_pypi_url": "http://127.0.0.1:8098/simple/",
  "port": 8099,
  "cache_enabled": false,
  "cache_size": 100,
  "cache_ttl": 1
}
EOF

# Start tejedor in background
./tejedor -config config.json &
TEJEDOR_PID=$!

# Wait for tejedor to be ready
print_status "Waiting for tejedor proxy to be ready..."
for i in {1..15}; do
    if curl -f http://127.0.0.1:8099/simple/ >/dev/null 2>&1; then
        print_status "Tejedor proxy is ready"
        break
    fi
    if [ $i -eq 15 ]; then
        print_error "Tejedor proxy failed to start"
        exit 1
    fi
    sleep 2
done

print_status "✅ Test environment is ready!"
print_status "Proxy URL: http://127.0.0.1:8099"
print_status "Private PyPI URL: http://127.0.0.1:8098"
print_status "Tejedor PID: $TEJEDOR_PID"

# Keep the script running to maintain the test environment
print_status "Test environment is running. Press Ctrl+C to stop."
print_status "Proxy PID: $TEJEDOR_PID"
print_status "Container: tejedor-test-pypi"
wait $TEJEDOR_PID