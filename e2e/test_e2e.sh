#!/bin/bash
set -e

echo "🚀 Starting Tejedor E2E Test Suite"

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

# Test 1: Install packages from private PyPI only
print_status "Testing pip install from private PyPI only..."
mkdir -p test-venv-private
python3 -m venv test-venv-private

cat > requirements-private.txt << EOF
flask==2.3.3
click==8.1.7
jinja2==3.1.2
werkzeug==2.3.7
six==1.16.0
itsdangerous==2.1.2
blinker==1.6.3
EOF

test-venv-private/bin/pip install -r requirements-private.txt -i http://127.0.0.1:8099/simple/
print_status "✅ Private packages installed successfully"

# Test 2: Install packages from public PyPI only
print_status "Testing pip install from public PyPI only..."
mkdir -p test-venv-public
python3 -m venv test-venv-public

cat > requirements-public.txt << EOF
requests==2.31.0
urllib3==2.0.7
certifi==2023.7.22
charset-normalizer==3.2.0
idna==3.4
EOF

test-venv-public/bin/pip install -r requirements-public.txt -i http://127.0.0.1:8099/simple/
print_status "✅ Public packages installed successfully"

# Test 3: Install mixed packages
print_status "Testing pip install with mixed packages..."
mkdir -p test-venv-mixed
python3 -m venv test-venv-mixed

cat > requirements-mixed.txt << EOF
flask==2.3.3
requests==2.31.0
click==8.1.7
six==1.16.0
EOF

test-venv-mixed/bin/pip install -r requirements-mixed.txt -i http://127.0.0.1:8099/simple/
print_status "✅ Mixed packages installed successfully"

# Test 4: Verify filtering behavior
print_status "Testing filtering behavior..."

# Test that numpy (public only) returns source distributions only
NUMPY_RESPONSE=$(curl -s http://127.0.0.1:8099/simple/numpy/)
if echo "$NUMPY_RESPONSE" | grep -q "\.tar\.gz"; then
    print_status "✅ Numpy response contains source distributions"
else
    print_error "❌ Numpy response missing source distributions"
    exit 1
fi

if echo "$NUMPY_RESPONSE" | grep -q "\.whl"; then
    print_error "❌ Numpy response contains wheel files (should be filtered)"
    exit 1
else
    print_status "✅ Numpy response correctly filtered (no wheel files)"
fi

# Test that flask (private) can have both source and wheel
FLASK_RESPONSE=$(curl -s http://127.0.0.1:8099/simple/flask/)
if echo "$FLASK_RESPONSE" | grep -q "flask"; then
    print_status "✅ Flask response contains flask package"
else
    print_error "❌ Flask response missing flask package"
    exit 1
fi

print_status "✅ All filtering behavior tests passed"

# Cleanup
print_status "Cleaning up test environment..."
kill $TEJEDOR_PID 2>/dev/null || true
rm -rf test-venv-* requirements-*.txt config.json

print_status "🎉 All E2E tests passed successfully!" 