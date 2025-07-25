# Local Testing Guide for prefetch-dependencies-tejedor

This guide explains how to test the `prefetch-dependencies-tejedor` Tekton task locally using podman.

## Prerequisites

- podman installed and running
- kubectl configured to access a Kubernetes cluster
- Tekton installed in the cluster
- Access to a private PyPI server

## Step 1: Build and Push Tejedor Image

First, build and push the Tejedor Docker image to your registry:

```bash
# From the project root
make docker-push
```

Or manually:

```bash
# Build the image
podman build -t quay.io/konflux-ci/tejedor:latest .

# Login to registry (if needed)
podman login quay.io

# Push the image
podman push quay.io/konflux-ci/tejedor:latest

# Get the image digest
DIGEST=$(podman inspect quay.io/konflux-ci/tejedor:latest | jq -r '.[0].Digest')
echo "Image digest: $DIGEST"
```

## Step 2: Update Tekton Task with Image Digest

Update the image references in the Tekton task with the actual digest:

```bash
# Replace YOUR_IMAGE_SHA256_HERE with the actual digest
sed -i "s|@sha256:YOUR_IMAGE_SHA256_HERE|@$DIGEST|g" \
    task/prefetch-dependencies-tejedor/0.2/prefetch-dependencies-tejedor.yaml
```

## Step 3: Apply the Tekton Task

Apply the task to your Kubernetes cluster:

```bash
kubectl apply -f task/prefetch-dependencies-tejedor/0.2/prefetch-dependencies-tejedor.yaml
```

## Step 4: Create Test Resources

Create the necessary secrets and PVCs for testing:

```bash
# Create git credentials secret (if needed)
kubectl create secret generic git-credentials \
  --from-file=.git-credentials=~/.git-credentials \
  --from-file=.gitconfig=~/.gitconfig

# Create netrc secret (if needed)
kubectl create secret generic netrc-credentials \
  --from-file=.netrc=~/.netrc

# Create PVC for source code
kubectl apply -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: source-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
EOF
```

## Step 5: Test with Example Pipeline

Apply the test pipeline and run it:

```bash
# Apply the pipeline
kubectl apply -f task/prefetch-dependencies-tejedor/0.2/test-example.yaml

# Apply the pipeline run
kubectl apply -f task/prefetch-dependencies-tejedor/0.2/pipelinerun-example.yaml
```

Alternatively, you can test the task directly:

```bash
# Apply the task run directly
kubectl apply -f task/prefetch-dependencies-tejedor/0.2/taskrun-example.yaml
```

## Step 6: Monitor the Pipeline Run

Check the status of the pipeline run:

```bash
# List pipeline runs
kubectl get pipelineruns

# Get detailed status
kubectl describe pipelinerun test-prefetch-dependencies-tejedor-run

# Check task logs
kubectl logs -f pipelinerun/test-prefetch-dependencies-tejedor-run
```

## Step 7: Verify Tejedor Sidecar

Check that the Tejedor sidecar is running correctly:

```bash
# Get the pod name
POD_NAME=$(kubectl get pods -l tekton.dev/pipelineRun=test-prefetch-dependencies-tejedor-run -o jsonpath='{.items[0].metadata.name}')

# Check sidecar logs
kubectl logs $POD_NAME -c tejedor-sidecar

# Check main task logs
kubectl logs $POD_NAME -c prefetch-dependencies
```

## Troubleshooting

### Common Issues

1. **Image Pull Errors**
   ```bash
   # Check if the image exists
   podman pull quay.io/konflux-ci/tejedor:latest
   
   # Verify the digest
   podman inspect quay.io/konflux-ci/tejedor:latest | jq '.[0].Digest'
   ```

2. **Private PyPI URL Issues**
   - Ensure the private PyPI URL is accessible from the cluster
   - Check network policies and firewall rules
   - Verify the URL format: `https://your-private-pypi.com/simple/`

3. **Python Dependency Detection**
   - Check the input parameter format
   - Verify that pip dependencies are correctly specified
   - Look for the "Python pip dependencies detected" message in logs

4. **Tejedor Sidecar Issues**
   - Check if the sidecar started successfully
   - Verify the health endpoint: `curl http://127.0.0.1:8080/health`
   - Check for port conflicts

### Debug Commands

```bash
# Check Tekton task status
kubectl get task prefetch-dependencies-tejedor

# Check task run logs
kubectl logs -f taskrun/$(kubectl get taskrun -o jsonpath='{.items[0].metadata.name}')

# Check sidecar logs specifically
kubectl logs -f taskrun/$(kubectl get taskrun -o jsonpath='{.items[0].metadata.name}') -c tejedor-sidecar

# Check Hermeto logs
kubectl logs -f taskrun/$(kubectl get taskrun -o jsonpath='{.items[0].metadata.name}') -c prefetch-dependencies
```

## Testing Different Scenarios

### 1. Python Only Dependencies

```yaml
params:
  - name: input
    value: "pip"
  - name: private-pypi-url
    value: "https://your-private-pypi.com/simple/"
```

### 2. Mixed Dependencies

```yaml
params:
  - name: input
    value: |
      [
        {"type": "pip"},
        {"type": "gomod"}
      ]
  - name: private-pypi-url
    value: "https://your-private-pypi.com/simple/"
```

### 3. With Proxy Server

```yaml
params:
  - name: input
    value: "pip"
  - name: private-pypi-url
    value: "https://your-private-pypi.com/simple/"
  - name: proxy-server
    value: "http://proxy.company.com:8080"
```

## Cleanup

```bash
# Delete the pipeline run
kubectl delete pipelinerun test-prefetch-dependencies-tejedor-run

# Delete the pipeline
kubectl delete pipeline test-prefetch-dependencies-tejedor

# Delete the task
kubectl delete task prefetch-dependencies-tejedor

# Clean up secrets and PVCs
kubectl delete secret git-credentials netrc-credentials
kubectl delete pvc source-pvc
``` 