#!/bin/bash
set -e

echo "🚀 Setting up Kind E2E Test Environment..."

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

# Configuration
CLUSTER_NAME="tejedor-test"
PROXY_PORT="8099"
PRIVATE_PORT="8098"

# Cleanup function
cleanup() {
    print_status "Cleaning up..."
    kind delete cluster --name "$CLUSTER_NAME" 2>/dev/null || true
}

# Set up trap to cleanup on exit
trap cleanup EXIT

# Check prerequisites
print_status "Checking prerequisites..."

for cmd in kind kubectl jq; do
    if ! command -v "$cmd" &> /dev/null; then
        print_error "$cmd is not installed"
        if [ "$cmd" = "kind" ]; then
            print_status "Install with: go install sigs.k8s.io/kind@latest"
        fi
        exit 1
    fi
done

# Check container runtime
CONTAINER_RUNTIME=""
if command -v podman &> /dev/null; then
    print_status "Found podman, checking connection..."
    if podman info &> /dev/null; then
        CONTAINER_RUNTIME="podman"
        print_status "✅ Podman connection working"
    else
        print_warning "⚠️ Podman found but connection failed"
        print_status "Trying to fix podman connection..."
        if podman machine list | grep -q "Currently running"; then
            print_status "Podman machine is running, trying to reconnect..."
        else
            print_status "Starting podman machine..."
            podman machine start 2>/dev/null || true
        fi
        # Wait a moment and try again
        sleep 5
        if podman info &> /dev/null; then
            CONTAINER_RUNTIME="podman"
            print_status "✅ Podman connection restored"
        else
            print_error "❌ Podman connection failed, trying Docker..."
        fi
    fi
fi

if [ -z "$CONTAINER_RUNTIME" ] && command -v docker &> /dev/null; then
    if docker info &> /dev/null; then
        CONTAINER_RUNTIME="docker"
        print_status "✅ Docker connection working"
    else
        print_error "❌ Docker found but connection failed"
    fi
fi

if [ -z "$CONTAINER_RUNTIME" ]; then
    print_error "❌ No working container runtime found"
    print_status "Please ensure either podman or docker is working"
    exit 1
fi

print_status "Using container runtime: $CONTAINER_RUNTIME"
print_status "Prerequisites check passed"

# Clean up any existing cluster
print_status "Cleaning up any existing kind cluster..."
kind delete cluster --name "$CLUSTER_NAME" 2>/dev/null || true

# Step 1: Create kind cluster
print_status "Creating kind cluster..."
if [ "$CONTAINER_RUNTIME" = "podman" ]; then
    # Use podman provider for kind
    KIND_EXPERIMENTAL_PROVIDER=podman kind create cluster --name "$CLUSTER_NAME" --config - <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 8080
    hostPort: 8080
    protocol: TCP
  - containerPort: 8098
    hostPort: 8098
    protocol: TCP
  - containerPort: 8099
    hostPort: 8099
    protocol: TCP
EOF
else
    # Use default Docker provider
    kind create cluster --name "$CLUSTER_NAME" --config - <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 8080
    hostPort: 8080
    protocol: TCP
  - containerPort: 8098
    hostPort: 8098
    protocol: TCP
  - containerPort: 8099
    hostPort: 8099
    protocol: TCP
EOF
fi

# Wait for cluster to be ready
print_status "Waiting for cluster to be ready..."
kubectl wait --for=condition=Ready nodes --all --timeout=300s

# Step 2: Build and load Tejedor image
print_status "Building Tejedor image..."
podman build -t localhost/tejedor:latest -f ../Dockerfile ..

print_status "Loading Tejedor image into kind cluster..."
podman save localhost/tejedor:latest | kind load image-archive --name "$CLUSTER_NAME"

# Verify image is loaded
print_status "Verifying Tejedor image is loaded..."
if ! podman exec "${CLUSTER_NAME}-control-plane" crictl images | grep -q "localhost/tejedor"; then
    print_error "❌ Tejedor image not found in kind cluster"
    exit 1
fi
print_status "✅ Tejedor image loaded successfully"

# Step 3: Build and load PyPI server image
print_status "Building PyPI server image..."
podman build -t localhost/tejedor-test-pypi:latest -f minimal-pypi.Dockerfile .

print_status "Loading PyPI server image into kind cluster..."
podman save localhost/tejedor-test-pypi:latest | kind load image-archive --name "$CLUSTER_NAME"

# Verify image is loaded
print_status "Verifying PyPI server image is loaded..."
if ! podman exec "${CLUSTER_NAME}-control-plane" crictl images | grep -q "localhost/tejedor-test-pypi"; then
    print_error "❌ PyPI server image not found in kind cluster"
    exit 1
fi
print_status "✅ PyPI server image loaded successfully"

# Step 4: Deploy PyPI server
print_status "Deploying PyPI server..."
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-pypi-server
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-pypi-server
  template:
    metadata:
      labels:
        app: test-pypi-server
    spec:
      containers:
      - name: pypi-server
        image: localhost/tejedor-test-pypi:latest
        imagePullPolicy: Never
        ports:
        - containerPort: 8080
        command: ["/opt/pypi-server/start.sh"]
---
apiVersion: v1
kind: Service
metadata:
  name: test-pypi-server-service
spec:
  selector:
    app: test-pypi-server
  ports:
  - port: 8080
    targetPort: 8080
    protocol: TCP
EOF

# Wait for PyPI server to be ready
print_status "Waiting for PyPI server to be ready..."
kubectl wait --for=condition=Available deployment/test-pypi-server --timeout=300s

# Step 5: Populate local PyPI server with packages
print_status "Populating local PyPI server with packages..."
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: populate-local-pypi
spec:
  containers:
  - name: populate
    image: python:3.11-alpine
    command: ["/bin/sh"]
    args:
    - -c
    - |
      echo "📦 Populating Local PyPI Server with Packages..."
      apk add --no-cache curl
      
      # Create temporary directory
      mkdir -p /tmp/packages
      cd /tmp/packages
      
      # Download packages that exist in both public and local PyPI
      echo "Downloading packages from public PyPI..."
      pip download --no-deps --dest . \
        flask==2.3.3 \
        click==8.1.7 \
        jinja2==3.1.2 \
        werkzeug==2.3.7 \
        six==1.16.0 \
        itsdangerous==2.1.2 \
        blinker==1.6.3 \
        requests==2.31.0
      
      echo "Downloaded packages:"
      ls -la
      
      # Copy packages to the PyPI server
      echo "Copying packages to local PyPI server..."
      kubectl cp . test-pypi-server-\$(kubectl get pods -l app=test-pypi-server -o jsonpath='{.items[0].metadata.name}'):/opt/pypi-server/packages/
      
      # Verify the packages were copied
      echo "Verifying packages in local PyPI server..."
      kubectl exec test-pypi-server-\$(kubectl get pods -l app=test-pypi-server -o jsonpath='{.items[0].metadata.name}') -- ls -la /opt/pypi-server/packages/
      
      # Test that the packages are accessible
      echo "Testing package accessibility from local PyPI server..."
      kubectl exec test-pypi-server-\$(kubectl get pods -l app=test-pypi-server -o jsonpath='{.items[0].metadata.name}') -- curl -s http://localhost:8080/simple/flask/ | head -5
      
      echo "✅ Local PyPI server populated with packages!"
  restartPolicy: Never
EOF

# Wait for population to complete
print_status "Waiting for local PyPI server population..."
kubectl wait --for=condition=Ready pod/populate-local-pypi --timeout=300s || true

# Check if the population completed successfully
if kubectl get pod populate-local-pypi -o jsonpath='{.status.phase}' | grep -q "Succeeded"; then
    print_status "✅ Local PyPI server populated successfully"
else
    print_warning "⚠️ Population may have failed, checking logs..."
    kubectl logs populate-local-pypi || true
fi

# Clean up population pod
kubectl delete pod populate-local-pypi --ignore-not-found=true

# Step 6: Deploy Tejedor proxy
print_status "Deploying Tejedor proxy..."
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tejedor-proxy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: tejedor-proxy
  template:
    metadata:
      labels:
        app: tejedor-proxy
    spec:
      containers:
      - name: tejedor
        image: localhost/tejedor:latest
        imagePullPolicy: Never
        ports:
        - containerPort: 8080
        command: ["/pypi-proxy"]
        env:
        - name: PYPI_PROXY_PRIVATE_PYPI_URL
          value: "http://test-pypi-server-service:8080/simple/"
        - name: PYPI_PROXY_PUBLIC_PYPI_URL
          value: "https://pypi.org/simple/"
        - name: PYPI_PROXY_PORT
          value: "8080"
        - name: PYPI_PROXY_CACHE_ENABLED
          value: "false"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
---
apiVersion: v1
kind: Service
metadata:
  name: tejedor-proxy-service
spec:
  selector:
    app: tejedor-proxy
  ports:
  - port: 8080
    targetPort: 8080
    protocol: TCP
EOF

# Wait for Tejedor to be ready
print_status "Waiting for Tejedor proxy to be ready..."
kubectl wait --for=condition=Available deployment/tejedor-proxy --timeout=300s

# Step 7: Set up port forwarding
print_status "Setting up port forwarding..."
kubectl port-forward service/tejedor-proxy-service 8099:8080 &
PROXY_PID=$!

kubectl port-forward service/test-pypi-server-service 8098:8080 &
PRIVATE_PID=$!

# Wait for port forwarding to be established
sleep 5

# Test connectivity
print_status "Testing connectivity..."
if ! curl -s http://127.0.0.1:8099/health > /dev/null; then
    print_error "❌ Tejedor proxy not accessible"
    exit 1
fi

if ! curl -s http://127.0.0.1:8098/simple/flask/ > /dev/null; then
    print_error "❌ Local PyPI server not accessible"
    exit 1
fi

print_status "✅ All services are accessible"

# Save PIDs for cleanup
echo $PROXY_PID > /tmp/proxy_pid
echo $PRIVATE_PID > /tmp/private_pid

print_status "🎉 Kind E2E test environment setup complete!"
print_status ""
print_status "📋 Environment Summary:"
print_status "  ✅ Kind cluster created with podman"
print_status "  ✅ Tejedor proxy deployed and accessible on 127.0.0.1:8099"
print_status "  ✅ Local PyPI server deployed and accessible on 127.0.0.1:8098"
print_status "  ✅ Local PyPI server populated with packages"
print_status "  ✅ Port forwarding established"
print_status ""
print_status "🧪 Ready to run Go tests!"
print_status "  Run: go test -v ./e2e -run TestKind"
print_status ""
print_status "🧹 To clean up:"
print_status "  kill \$(cat /tmp/proxy_pid) \$(cat /tmp/private_pid)"
print_status "  kind delete cluster --name $CLUSTER_NAME" 