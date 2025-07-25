# Kind-based Hermeto Integration Testing

This directory contains comprehensive end-to-end tests that use kind (Kubernetes in Docker) to test the integration between Tejedor and Hermeto in a realistic Kubernetes environment.

## Overview

The kind-based tests validate:

1. **Kind Cluster Setup**: Creates a single-node Kubernetes cluster using kind
2. **Tejedor Image Building**: Builds and loads the Tejedor Docker image into the cluster
3. **Test PyPI Server**: Deploys a mock PyPI server with test packages
4. **Tejedor Proxy Deployment**: Deploys Tejedor as a service in the cluster
5. **Tekton Installation**: Installs the latest Tekton Pipeline
6. **Hermeto Integration**: Tests Hermeto prefetch functionality through the proxy
7. **Proxy Verification**: Validates that the proxy works correctly

## Prerequisites

- **kind**: `go install sigs.k8s.io/kind@latest`
- **podman**: Container engine for building images
- **kubectl**: Kubernetes command-line tool
- **jq**: JSON processor for parsing responses
- **curl**: HTTP client for testing endpoints

## Test Scripts

### 1. Full Test (`kind-hermeto-test.sh`)

Comprehensive test that includes:
- Complete kind cluster setup
- Tejedor image building and loading
- Test PyPI server deployment
- Tekton installation
- Full Hermeto prefetch testing
- Detailed verification steps

### 2. Simple Test (`kind-hermeto-test-simple.sh`)

Streamlined test that focuses on core functionality:
- Basic kind cluster setup
- Essential components only
- Robust error handling
- Faster execution time

## Running the Tests

### Using Makefile

```bash
# Run the full test
make kind-hermeto-test

# Run the simple test (recommended for quick testing)
make kind-hermeto-test-simple
```

### Direct Execution

```bash
# Full test
cd e2e
./kind-hermeto-test.sh

# Simple test
cd e2e
./kind-hermeto-test-simple.sh
```

## Test Flow

### 1. Cluster Setup
```bash
# Creates a single-node kind cluster
kind create cluster --name tejedor-test
```

### 2. Image Building and Loading
```bash
# Build Tejedor image locally
podman build -t tejedor:latest .

# Load image into kind cluster
kind load docker-image tejedor:latest --name tejedor-test
```

### 3. Test PyPI Server
```bash
# Build and deploy test PyPI server
podman build -t tejedor-test-pypi -f Dockerfile .
kubectl run test-pypi-server --image=tejedor-test-pypi --port=8080
kubectl expose pod test-pypi-server --port=8080 --name=test-pypi-service
```

### 4. Tejedor Proxy Deployment
```yaml
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
    spec:
      containers:
      - name: tejedor
        image: tejedor:latest
        command: ["/pypi-proxy"]
        args:
        - "--private-pypi-url=http://test-pypi-service:8080/simple/"
        - "--port=8080"
        - "--cache-enabled=false"
```

### 5. Tekton Installation
```bash
# Install latest Tekton Pipeline
kubectl apply -f https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml
```

### 6. Hermeto Test Task
```yaml
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: hermeto-simple-test
spec:
  params:
  - name: proxy-url
    default: "http://tejedor-proxy-service:8080"
  steps:
  - name: hermeto-test
    image: quay.io/containerbuildsystem/cachi2:latest
    script: |
      # Set proxy environment variables
      export HTTP_PROXY="$(params.proxy-url)"
      export HTTPS_PROXY="$(params.proxy-url)"
      
      # Test Hermeto prefetch
      cachi2 fetch-deps pip --source=/workspace/source --output=/workspace/output
```

## Test Verification

### Proxy Health Check
```bash
# Test proxy health endpoint
curl -s http://127.0.0.1:8080/health | jq .
```

### Package Access Test
```bash
# Test package access through proxy
curl -s http://127.0.0.1:8080/simple/flask/
```

### Hermeto Integration Test
```bash
# Monitor TaskRun execution
kubectl wait --for=condition=Succeeded taskrun/hermeto-simple-test-run --timeout=300s

# Check logs
kubectl logs taskrun/hermeto-simple-test-run
```

## Troubleshooting

### Common Issues

1. **Kind Cluster Creation Fails**
   ```bash
   # Check if Docker/podman is running
   podman ps
   
   # Check available ports
   netstat -an | grep 8080
   ```

2. **Image Loading Issues**
   ```bash
   # Verify image exists
   podman images | grep tejedor
   
   # Check kind cluster status
   kind get clusters
   ```

3. **Tekton Installation Issues**
   ```bash
   # Check Tekton pods
   kubectl get pods -n tekton-pipelines
   
   # Check Tekton controller logs
   kubectl logs -n tekton-pipelines deployment/tekton-pipelines-controller
   ```

4. **Proxy Connectivity Issues**
   ```bash
   # Check Tejedor pod status
   kubectl get pods -l app=tejedor-proxy
   
   # Check Tejedor logs
   kubectl logs deployment/tejedor-proxy
   
   # Test service connectivity
   kubectl port-forward service/tejedor-proxy-service 8080:8080
   curl http://127.0.0.1:8080/health
   ```

### Debug Commands

```bash
# Check cluster status
kubectl get nodes
kubectl get pods --all-namespaces

# Check services
kubectl get services

# Check TaskRun status
kubectl get taskruns

# Get detailed TaskRun info
kubectl describe taskrun hermeto-simple-test-run

# Check Tekton installation
kubectl get pods -n tekton-pipelines
```

## Cleanup

```bash
# Delete the kind cluster
kind delete cluster --name tejedor-test

# Remove local images
podman rmi tejedor:latest tejedor-test-pypi:latest
```

## Expected Results

### Successful Test Run

1. **Cluster Creation**: Kind cluster created successfully
2. **Image Loading**: Tejedor image loaded into cluster
3. **Service Deployment**: Test PyPI server and Tejedor proxy deployed
4. **Tekton Installation**: Tekton Pipeline installed and ready
5. **Proxy Health**: Health endpoint returns healthy status
6. **Hermeto Test**: TaskRun completes successfully (may show warnings for missing packages)
7. **Cleanup**: All resources cleaned up properly

### Test Output Example

```
🚀 Starting Kind + Hermeto E2E Test Suite (Simple Version)
[INFO] Checking prerequisites...
[INFO] Prerequisites check passed
[INFO] Creating kind cluster...
[INFO] Building Tejedor Docker image...
[INFO] Loading Tejedor image into kind cluster...
[INFO] Building test PyPI server...
[INFO] Deploying test PyPI server...
[INFO] Deploying Tejedor proxy...
[INFO] Installing Tekton...
[INFO] Testing proxy functionality...
✅ Proxy health check passed
✅ Package access test passed
[INFO] Creating Hermeto test task...
[INFO] Running Hermeto test...
✅ TaskRun completed successfully
🎉 Kind + Hermeto test completed!
```

## Integration with CI

These tests can be integrated into CI/CD pipelines to validate:

- Tejedor image builds correctly
- Kind cluster setup works
- Tekton integration functions properly
- Hermeto can use Tejedor as a proxy
- End-to-end workflow is functional

The tests provide confidence that the Hermeto integration works in a realistic Kubernetes environment without requiring external registries or complex infrastructure. 