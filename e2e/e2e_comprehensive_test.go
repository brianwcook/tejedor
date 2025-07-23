package e2e

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestComprehensiveE2E tests the full end-to-end workflow:
// 1. Build and start a test PyPI server container
// 2. Build and start tejedor proxy
// 3. Test pip installs with various requirements files
// 4. Verify filtering behavior (sdist only from public, bdist allowed from private)
func TestComprehensiveE2E(t *testing.T) {
	// Skip if not in CI or if explicitly disabled
	if testing.Short() {
		t.Skip("Skipping comprehensive e2e test in short mode")
	}

	// Create temporary directory for test artifacts
	tempDir, err := os.MkdirTemp("", "tejedor-e2e-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Step 1: Build the test PyPI server container
	t.Log("Building test PyPI server container...")
	if err := buildTestContainer(t, tempDir); err != nil {
		t.Fatalf("Failed to build test container: %v", err)
	}

	// Step 2: Start the test PyPI server
	t.Log("Starting test PyPI server...")
	containerID, privatePyPIURL, err := startTestContainer(t, tempDir)
	if err != nil {
		t.Fatalf("Failed to start test container: %v", err)
	}
	defer stopTestContainer(t, containerID)

	// Wait for the server to be ready
	t.Log("Waiting for PyPI server to be ready...")
	if err := waitForServerReady(privatePyPIURL); err != nil {
		t.Fatalf("PyPI server not ready: %v", err)
	}

	// Step 3: Build tejedor
	t.Log("Building tejedor...")
	tejedorPath, err := buildTejedor(t, tempDir)
	if err != nil {
		t.Fatalf("Failed to build tejedor: %v", err)
	}

	// Step 4: Start tejedor proxy
	t.Log("Starting tejedor proxy...")
	proxyProcess, proxyURL, err := startTejedorProxy(t, tejedorPath, privatePyPIURL)
	if err != nil {
		t.Fatalf("Failed to start tejedor proxy: %v", err)
	}
	defer stopTejedorProxy(t, proxyProcess)

	// Wait for proxy to be ready
	t.Log("Waiting for tejedor proxy to be ready...")
	if err := waitForServerReady(proxyURL); err != nil {
		t.Fatalf("Tejedor proxy not ready: %v", err)
	}

	// Step 5: Test pip installs with various requirements files
	t.Run("TestPipInstallFromPrivateOnly", func(t *testing.T) {
		testPipInstall(t, proxyURL, "requirements-private-only.txt", "private-only")
	})

	t.Run("TestPipInstallFromPublicOnly", func(t *testing.T) {
		testPipInstall(t, proxyURL, "requirements-public-only.txt", "public-only")
	})

	t.Run("TestPipInstallMixed", func(t *testing.T) {
		testPipInstall(t, proxyURL, "requirements-mixed.txt", "mixed")
	})

	// Step 6: Verify filtering behavior
	t.Run("TestFilteringBehavior", func(t *testing.T) {
		testFilteringBehavior(t, proxyURL)
	})
}

func buildTestContainer(t *testing.T, tempDir string) error {
	cmd := exec.Command("docker", "build", "-t", "tejedor-test-pypi", ".")
	cmd.Dir = filepath.Join(tempDir, "e2e")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func startTestContainer(t *testing.T, tempDir string) (string, string, error) {
	cmd := exec.Command("docker", "run", "-d", "-p", "8080:8080", "--name", "tejedor-test-pypi", "tejedor-test-pypi")
	output, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to start container: %v", err)
	}

	containerID := strings.TrimSpace(string(output))
	privatePyPIURL := "http://localhost:8080/simple/"

	return containerID, privatePyPIURL, nil
}

func stopTestContainer(t *testing.T, containerID string) {
	exec.Command("docker", "stop", containerID).Run()
	exec.Command("docker", "rm", containerID).Run()
}

func waitForServerReady(url string) error {
	client := &http.Client{Timeout: 30 * time.Second}

	for i := 0; i < 30; i++ {
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("server not ready after 60 seconds")
}

func buildTejedor(t *testing.T, tempDir string) (string, error) {
	// Build tejedor binary
	cmd := exec.Command("go", "build", "-o", "tejedor", ".")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to build tejedor: %v", err)
	}

	return filepath.Join(tempDir, "tejedor"), nil
}

func startTejedorProxy(t *testing.T, tejedorPath, privatePyPIURL string) (*exec.Cmd, string, error) {
	// Create config file
	configContent := fmt.Sprintf(`{
		"public_pypi_url": "https://pypi.org/simple/",
		"private_pypi_url": "%s",
		"port": 8081,
		"cache_enabled": false,
		"cache_size": 100,
		"cache_ttl": 1
	}`, privatePyPIURL)

	configPath := filepath.Join(filepath.Dir(tejedorPath), "config.json")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return nil, "", fmt.Errorf("failed to write config: %v", err)
	}

	// Start tejedor
	cmd := exec.Command(tejedorPath, "-config", configPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, "", fmt.Errorf("failed to start tejedor: %v", err)
	}

	proxyURL := "http://localhost:8081/simple/"
	return cmd, proxyURL, nil
}

func stopTejedorProxy(t *testing.T, cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		cmd.Process.Kill()
	}
}

func testPipInstall(t *testing.T, proxyURL, requirementsFile, testName string) {
	// Create virtual environment
	venvDir := filepath.Join(t.TempDir(), fmt.Sprintf("venv-%s", testName))

	cmd := exec.Command("python3", "-m", "venv", venvDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create virtual environment: %v", err)
	}

	// Create requirements file
	requirementsContent := getRequirementsContent(requirementsFile)
	requirementsPath := filepath.Join(venvDir, "requirements.txt")
	if err := os.WriteFile(requirementsPath, []byte(requirementsContent), 0644); err != nil {
		t.Fatalf("Failed to write requirements file: %v", err)
	}

	// Install packages using tejedor as index
	pipCmd := exec.Command(filepath.Join(venvDir, "bin", "pip"), "install", "-r", requirementsPath, "-i", proxyURL)
	pipCmd.Stdout = os.Stdout
	pipCmd.Stderr = os.Stderr

	if err := pipCmd.Run(); err != nil {
		t.Fatalf("Failed to install packages: %v", err)
	}

	t.Logf("Successfully installed packages for %s", testName)
}

func testFilteringBehavior(t *testing.T, proxyURL string) {
	// Test that packages from public PyPI only return source distributions
	// and packages from private PyPI can return both source and wheel distributions

	client := &http.Client{Timeout: 30 * time.Second}

	// Test a package that should only be available from public PyPI (source only)
	resp, err := client.Get(proxyURL + "numpy/")
	if err != nil {
		t.Fatalf("Failed to get numpy package: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200 for numpy, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)

	// Check that numpy response contains source distributions but no wheel files
	if !strings.Contains(bodyStr, ".tar.gz") {
		t.Error("Expected numpy response to contain source distributions (.tar.gz)")
	}

	if strings.Contains(bodyStr, ".whl") {
		t.Error("Expected numpy response to NOT contain wheel files (.whl)")
	}

	// Test a package that should be available from private PyPI (can have wheels)
	resp, err = client.Get(proxyURL + "flask/")
	if err != nil {
		t.Fatalf("Failed to get flask package: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200 for flask, got %d", resp.StatusCode)
	}

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr = string(body)

	// Flask should be available from private PyPI and can have both source and wheel
	if !strings.Contains(bodyStr, "flask") {
		t.Error("Expected flask response to contain flask package")
	}

	t.Log("Filtering behavior test passed")
}

func getRequirementsContent(requirementsFile string) string {
	switch requirementsFile {
	case "requirements-private-only.txt":
		return `flask==2.3.3
click==8.1.7
jinja2==3.1.2
werkzeug==2.3.7
markupsafe==2.1.3
itsdangerous==2.1.2
blinker==1.6.3`
	case "requirements-public-only.txt":
		return `numpy==1.24.3
pandas==2.0.3
matplotlib==3.7.2
scipy==1.11.1`
	case "requirements-mixed.txt":
		return `flask==2.3.3
numpy==1.24.3
requests==2.31.0
click==8.1.7`
	default:
		return ""
	}
}
