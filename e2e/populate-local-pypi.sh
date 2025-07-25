#!/bin/bash
set -e

echo "📦 Populating Local PyPI Server with E2E Test Packages..."

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if we're in a Kubernetes context
if ! kubectl get pods &>/dev/null; then
    print_error "No Kubernetes cluster available. Please run this script in a kind cluster context."
    exit 1
fi

# Check if the PyPI server pod exists
if ! kubectl get pod test-pypi-server-* &>/dev/null; then
    print_error "PyPI server pod not found. Please ensure the kind-hermeto-test-full.sh script has been run first."
    exit 1
fi

print_status "Downloading packages from populate-requirements.txt..."

# Create a temporary directory for downloads
TEMP_DIR=$(mktemp -d)
cd "$TEMP_DIR"

# Download packages listed in populate-requirements.txt
print_status "Downloading packages from public PyPI..."
pip download --no-deps --dest . \
    flask==2.3.3 \
    click==8.1.7 \
    jinja2==3.1.2 \
    werkzeug==2.3.7 \
    six==1.16.0 \
    itsdangerous==2.1.2 \
    blinker==1.6.3 \
    requests==2.31.0

print_status "Downloaded packages:"
ls -la

# Copy packages to the PyPI server pod
print_status "Copying packages to local PyPI server..."
kubectl cp . test-pypi-server-$(kubectl get pods -l app=test-pypi-server -o jsonpath='{.items[0].metadata.name}'):/opt/pypi-server/packages/

# Verify the packages were copied
print_status "Verifying packages in local PyPI server..."
kubectl exec test-pypi-server-$(kubectl get pods -l app=test-pypi-server -o jsonpath='{.items[0].metadata.name}') -- ls -la /opt/pypi-server/packages/

# Test that the packages are accessible
print_status "Testing package accessibility from local PyPI server..."
kubectl exec test-pypi-server-$(kubectl get pods -l app=test-pypi-server -o jsonpath='{.items[0].metadata.name}') -- curl -s http://127.0.0.1:8080/simple/flask/ | head -5

print_status "✅ Local PyPI server populated with E2E test packages!"
print_status ""
print_status "📋 Packages now available in local PyPI server:"
echo "  - flask==2.3.3"
echo "  - click==8.1.7"
echo "  - jinja2==3.1.2"
echo "  - werkzeug==2.3.7"
echo "  - six==1.16.0"
echo "  - itsdangerous==2.1.2"
echo "  - blinker==1.6.3"
echo "  - requests==2.31.0"
echo "  - testpackage==1.0.0 (original test package)"

# Clean up
cd - > /dev/null
rm -rf "$TEMP_DIR"

print_status ""
print_status "🎯 Next steps:"
print_status "1. Run the kind-hermeto-test-full.sh script to test with enhanced logging"
print_status "2. Check Tejedor logs to see routing decisions for each package"
print_status "3. Verify that local packages are served from local PyPI and public packages from public PyPI" 