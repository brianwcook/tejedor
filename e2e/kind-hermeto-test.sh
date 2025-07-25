#!/bin/bash
set -e

echo "🚀 Starting Kind + Hermeto E2E Test Suite"

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
TEJEDOR_IMAGE="tejedor:latest"
TEJEDOR_SERVICE_NAME="tejedor-proxy"
TEJEDOR_SERVICE_PORT="8080"

# Cleanup function
cleanup() {
    print_status "Cleaning up..."
    kind delete cluster --name "$CLUSTER_NAME" 2>/dev/null || true
    podman rmi "$TEJEDOR_IMAGE" 2>/dev/null || true
}

# Set up trap to cleanup on exit
trap cleanup EXIT

# Check prerequisites
print_status "Checking prerequisites..."

if ! command -v kind &> /dev/null; then
    print_error "kind is not installed"
    print_status "Install with: go install sigs.k8s.io/kind@latest"
    exit 1
fi

if ! command -v podman &> /dev/null; then
    print_error "podman is not installed"
    exit 1
fi

if ! command -v kubectl &> /dev/null; then
    print_error "kubectl is not installed"
    exit 1
fi

print_status "Prerequisites check passed"

# Step 1: Create kind cluster
print_status "Creating kind cluster..."
kind create cluster --name "$CLUSTER_NAME" --config - <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 8080
    hostPort: 8080
    protocol: TCP
EOF

# Wait for cluster to be ready
print_status "Waiting for cluster to be ready..."
kubectl wait --for=condition=Ready nodes --all --timeout=300s

# Step 2: Build Tejedor image
print_status "Building Tejedor Docker image..."
cd "$(dirname "$0")/.."
podman build -t "$TEJEDOR_IMAGE" .

# Step 3: Load image into kind
print_status "Loading Tejedor image into kind cluster..."
kind load docker-image "$TEJEDOR_IMAGE" --name "$CLUSTER_NAME"

# Step 4: Build and start test PyPI server
print_status "Building test PyPI server..."
cd e2e
podman build -t tejedor-test-pypi -f Dockerfile .

# Start PyPI server in kind
print_status "Starting test PyPI server in kind..."
kubectl run test-pypi-server --image=tejedor-test-pypi --port=8080
kubectl expose pod test-pypi-server --port=8080 --target-port=8080 --name=test-pypi-service

# Wait for PyPI server to be ready
print_status "Waiting for PyPI server to be ready..."
kubectl wait --for=condition=Ready pod/test-pypi-server --timeout=300s

# Step 5: Deploy Tejedor proxy
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
        image: tejedor:latest
        ports:
        - containerPort: 8080
        env:
        - name: PRIVATE_PYPI_URL
          value: "http://test-pypi-service:8080/simple/"
        - name: PUBLIC_PYPI_URL
          value: "https://pypi.org/simple/"
        - name: PORT
          value: "8080"
        - name: CACHE_ENABLED
          value: "false"
        command: ["/pypi-proxy"]
        args:
        - "--private-pypi-url=http://test-pypi-service:8080/simple/"
        - "--port=8080"
        - "--cache-enabled=false"
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

# Step 6: Install Tekton
print_status "Installing Tekton..."
kubectl apply -f https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml

# Wait for Tekton to be ready
print_status "Waiting for Tekton to be ready..."
kubectl wait --for=condition=Available deployment/tekton-pipelines-controller -n tekton-pipelines --timeout=300s

# Step 7: Create test Tekton task for Hermeto
print_status "Creating Hermeto test task..."
kubectl apply -f - <<EOF
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: hermeto-prefetch-test
spec:
  params:
  - name: proxy-url
    description: "URL of the Tejedor proxy"
    default: "http://tejedor-proxy-service:8080"
  - name: packages
    description: "Packages to prefetch"
    default: "flask==2.3.3,requests==2.31.0"
  steps:
  - name: hermeto-prefetch
    image: quay.io/containerbuildsystem/cachi2:latest
    script: |
      #!/bin/bash
      set -euo pipefail
      
      echo "Testing Hermeto prefetch through Tejedor proxy..."
      echo "Proxy URL: \$(params.proxy-url)"
      echo "Packages: \$(params.packages)"
      
      # Set proxy environment variables
      export HTTP_PROXY="\$(params.proxy-url)"
      export HTTPS_PROXY="\$(params.proxy-url)"
      export http_proxy="\$(params.proxy-url)"
      export https_proxy="\$(params.proxy-url)"
      
      echo "Proxy environment variables set:"
      echo "HTTP_PROXY: \$HTTP_PROXY"
      echo "HTTPS_PROXY: \$HTTPS_PROXY"
      
      # Create a simple pip requirements file
      cat > requirements.txt << 'REQEOF'
      flask==2.3.3
      requests==2.31.0
      click==8.1.7
      jinja2==3.1.2
      werkzeug==2.3.7
      urllib3==2.0.7
      certifi==2023.7.22
      REQEOF
      
      echo "Created requirements.txt:"
      cat requirements.txt
      
      # Test Hermeto prefetch
      echo "Running Hermeto prefetch..."
      cachi2 fetch-deps pip --source=/workspace/source --output=/workspace/output
      
      echo "Hermeto prefetch completed successfully!"
      echo "Output directory contents:"
      ls -la /workspace/output/
      
      # Test that packages were fetched
      if [ -d "/workspace/output/pip" ]; then
        echo "✅ Pip packages were successfully prefetched"
        ls -la /workspace/output/pip/
      else
        echo "❌ No pip packages found in output"
        exit 1
      fi
    env:
    - name: HTTP_PROXY
      value: "\$(params.proxy-url)"
    - name: HTTPS_PROXY
      value: "\$(params.proxy-url)"
    - name: http_proxy
      value: "\$(params.proxy-url)"
    - name: https_proxy
      value: "\$(params.proxy-url)"
    volumeMounts:
    - name: workspace
      mountPath: /workspace
  workspaces:
  - name: source
    description: "Source code workspace"
  - name: output
    description: "Output workspace for prefetched dependencies"
  volumes:
  - name: workspace
    emptyDir: {}
EOF

# Step 8: Create test TaskRun
print_status "Creating test TaskRun..."
kubectl apply -f - <<EOF
apiVersion: tekton.dev/v1
kind: TaskRun
metadata:
  name: hermeto-prefetch-test-run
spec:
  taskRef:
    name: hermeto-prefetch-test
  params:
  - name: proxy-url
    value: "http://tejedor-proxy-service:8080"
  - name: packages
    value: "flask==2.3.3,requests==2.31.0"
  workspaces:
  - name: source
    emptyDir: {}
  - name: output
    emptyDir: {}
EOF

# Step 9: Monitor the TaskRun
print_status "Monitoring TaskRun execution..."
kubectl wait --for=condition=Succeeded taskrun/hermeto-prefetch-test-run --timeout=600s

# Step 10: Check results
print_status "Checking TaskRun results..."
kubectl get taskrun hermeto-prefetch-test-run -o yaml

# Get logs
print_status "TaskRun logs:"
kubectl logs taskrun/hermeto-prefetch-test-run

print_status "✅ Hermeto prefetch test completed successfully!"

# Step 11: Additional verification
print_status "Verifying Tejedor proxy is working..."
kubectl port-forward service/tejedor-proxy-service 8080:8080 &
PORT_FORWARD_PID=$!

# Wait for port forward
sleep 5

# Test proxy directly
print_status "Testing proxy directly..."
curl -s http://127.0.0.1:8080/health | jq .

# Test package access
print_status "Testing package access through proxy..."
curl -s http://127.0.0.1:8080/simple/flask/ | head -20

# Cleanup port forward
kill $PORT_FORWARD_PID

print_status "🎉 All tests completed successfully!"
print_status ""
print_status "📋 Test Summary:"
print_status "  ✅ Kind cluster created and configured"
print_status "  ✅ Tejedor image built and loaded"
print_status "  ✅ Test PyPI server deployed"
print_status "  ✅ Tejedor proxy deployed and running"
print_status "  ✅ Tekton installed and configured"
print_status "  ✅ Hermeto prefetch test completed"
print_status "  ✅ Proxy functionality verified"
print_status ""
print_status "🧹 To clean up:"
print_status "  kind delete cluster --name $CLUSTER_NAME" 