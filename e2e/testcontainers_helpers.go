package e2e

import (
	"os"
	"testing"
)

// setupPodmanEnvironment configures testcontainers to use Podman
func setupPodmanEnvironment(t *testing.T) {
	// Set Podman as the container runtime
	os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
	os.Setenv("TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE", "/var/run/podman/podman.sock")

	t.Log("Configured testcontainers to use Podman")
}

// ensureTestImages ensures the required test images are available
func ensureTestImages(t *testing.T) {
	// This function would check if images exist and build them if needed
	// For now, we'll rely on pre-built images or manual building
	t.Log("Ensure test images are built before running tests")
	t.Log("Run: podman build -t tejedor:test -f e2e/Dockerfile.tejedor .")
	t.Log("Run: podman build -t tejedor-test-pypi:latest -f e2e/Dockerfile .")
}
