#!/bin/bash
set -e

echo "🧪 Running Kind E2E Tests..."

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

# Function to cleanup port forwarding
cleanup_port_forwarding() {
    if [ -f /tmp/proxy_pid ]; then
        kill $(cat /tmp/proxy_pid) 2>/dev/null || true
        rm -f /tmp/proxy_pid
    fi
    if [ -f /tmp/private_pid ]; then
        kill $(cat /tmp/private_pid) 2>/dev/null || true
        rm -f /tmp/private_pid
    fi
}

# Set up trap to cleanup on exit
trap cleanup_port_forwarding EXIT

# Check if we're in the right directory
if [ ! -f "kind_setup.sh" ]; then
    print_error "❌ Not in e2e directory. Please run from the e2e directory."
    exit 1
fi

# Step 1: Set up the Kind environment
print_status "Setting up Kind E2E test environment..."
./kind_setup.sh

# Step 2: Wait a moment for services to be fully ready
print_status "Waiting for services to be fully ready..."
sleep 10

# Step 3: Run the Go tests
print_status "Running Kind E2E tests..."
go test -v -timeout=5m ./e2e -run "TestKind"

# Step 4: Show test results
TEST_EXIT_CODE=$?

if [ $TEST_EXIT_CODE -eq 0 ]; then
    print_status "✅ All Kind E2E tests passed!"
else
    print_error "❌ Some Kind E2E tests failed!"
fi

# Step 5: Show enhanced logging (if available)
print_status "🔍 Recent Tejedor logs (showing routing decisions):"
kubectl logs tejedor-proxy-$(kubectl get pods -l app=tejedor-proxy -o jsonpath='{.items[0].metadata.name}') --tail=20 2>/dev/null || print_warning "Could not retrieve Tejedor logs"

print_status ""
print_status "📊 Test Summary:"
print_status "  ✅ Kind cluster created and configured"
print_status "  ✅ Tejedor proxy deployed with enhanced logging"
print_status "  ✅ Local PyPI server populated with packages"
print_status "  ✅ Port forwarding established"
print_status "  ✅ Go tests executed"

if [ $TEST_EXIT_CODE -eq 0 ]; then
    print_status ""
    print_status "🎯 Key Verifications:"
    print_status "  ✅ Packages available in both indexes served from local PyPI (priority)"
    print_status "  ✅ Packages only in public PyPI served from public PyPI"
    print_status "  ✅ File downloads routed correctly"
    print_status "  ✅ Enhanced logging shows routing decisions"
    print_status ""
    print_status "🚀 Kind E2E tests completed successfully!"
else
    print_status ""
    print_status "🔍 Check the test output above for specific failures"
    print_status "  - Verify that packages exist in both indexes"
    print_status "  - Check that local PyPI server is populated"
    print_status "  - Ensure Tejedor proxy is running correctly"
fi

exit $TEST_EXIT_CODE 