package e2e

import (
	"os"
	"testing"
)

// setupPodmanEnvironment configures testcontainers to use Podman.
func setupPodmanEnvironment(t *testing.T) {
	// Set Podman as the container runtime
	os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
	os.Setenv("TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE", "/var/run/podman/podman.sock")

	// Additional environment variables for better Podman compatibility
	os.Setenv("TESTCONTAINERS_DOCKER_HOST", "unix:///var/run/podman/podman.sock")
	os.Setenv("DOCKER_HOST", "unix:///var/run/podman/podman.sock")

	// Configure Testcontainers to not treat localhost as a registry
	os.Setenv("TESTCONTAINERS_DOCKER_REGISTRY", "")

	t.Log("Configured testcontainers to use Podman")
	t.Log("TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE:", os.Getenv("TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE"))
	t.Log("TESTCONTAINERS_DOCKER_HOST:", os.Getenv("TESTCONTAINERS_DOCKER_HOST"))
	t.Log("DOCKER_HOST:", os.Getenv("DOCKER_HOST"))
	t.Log("TESTCONTAINERS_DOCKER_REGISTRY:", os.Getenv("TESTCONTAINERS_DOCKER_REGISTRY"))
}
