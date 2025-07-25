#!/bin/bash
set -e

# Configuration
REGISTRY=${REGISTRY:-"quay.io/konflux-ci"}
IMAGE_NAME="tejedor"
TAG=${TAG:-"latest"}

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

# Check if we're logged in to the registry
if ! podman login --get-login "$REGISTRY" >/dev/null 2>&1; then
    print_error "Not logged in to registry $REGISTRY"
    print_status "Please run: podman login $REGISTRY"
    exit 1
fi

print_status "Building Tejedor Docker image..."
print_status "Registry: $REGISTRY"
print_status "Image: $IMAGE_NAME"
print_status "Tag: $TAG"

# Build the image
podman build -t "$REGISTRY/$IMAGE_NAME:$TAG" .

# Get the image digest
DIGEST=$(podman inspect "$REGISTRY/$IMAGE_NAME:$TAG" | jq -r '.[0].Digest')

print_status "Image built successfully"
print_status "Digest: $DIGEST"

# Push the image
print_status "Pushing image to registry..."
podman push "$REGISTRY/$IMAGE_NAME:$TAG"

print_status "✅ Image pushed successfully!"
print_status "Full image reference: $REGISTRY/$IMAGE_NAME:$TAG@$DIGEST"

# Update the Tekton task with the new digest
if [ -f "task/prefetch-dependencies-tejedor/0.2/prefetch-dependencies-tejedor.yaml" ]; then
    print_status "Updating Tekton task with new image digest..."
    
    # Create backup
    cp task/prefetch-dependencies-tejedor/0.2/prefetch-dependencies-tejedor.yaml \
       task/prefetch-dependencies-tejedor/0.2/prefetch-dependencies-tejedor.yaml.backup
    
    # Update the digest in the task file
    sed -i.bak "s|@sha256:YOUR_IMAGE_SHA256_HERE|@$DIGEST|g" \
        task/prefetch-dependencies-tejedor/0.2/prefetch-dependencies-tejedor.yaml
    
    print_status "✅ Tekton task updated with new digest"
    print_status "Backup saved as: task/prefetch-dependencies-tejedor/0.2/prefetch-dependencies-tejedor.yaml.backup"
fi

print_status "🎉 Build and push completed successfully!" 