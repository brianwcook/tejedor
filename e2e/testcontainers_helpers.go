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

	t.Log("Configured testcontainers to use Podman")
}
