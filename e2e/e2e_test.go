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

const (
	proxyURL    = "http://localhost:8099"
	privateURL  = "http://localhost:8098"
	testTimeout = 30 * time.Second
)

// TestPrivatePackages tests installing packages that only exist in private PyPI.
func TestPrivatePackages(t *testing.T) {
	t.Parallel()

	// Test packages that should be available in private PyPI.
	packages := []string{"flask", "click", "jinja2", "werkzeug"}

	for _, pkg := range packages {
		t.Run(fmt.Sprintf("package_%s", pkg), func(t *testing.T) {
			// Check that package is available through proxy
			resp, err := http.Get(fmt.Sprintf("%s/simple/%s/", proxyURL, pkg))
			if err != nil {
				t.Fatalf("Failed to get package %s: %v", pkg, err)
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					t.Logf("Failed to close response body: %v", err)
				}
			}()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Package %s returned status %d, expected 200", pkg, resp.StatusCode)
			}

			// Verify it's served from private PyPI
			source := resp.Header.Get("X-PyPI-Source")
			if source != privateURL+"/simple/" {
				t.Errorf("Package %s served from %s, expected private PyPI", pkg, source)
			}
		})
	}
}

// TestPublicPackages tests installing packages that only exist in public PyPI.
func TestPublicPackages(t *testing.T) {
	t.Parallel()

	// Test packages that should only be available in public PyPI.
	packages := []string{"urllib3", "certifi", "numpy", "pandas"}

	for _, pkg := range packages {
		t.Run(fmt.Sprintf("package_%s", pkg), func(t *testing.T) {
			// Check that package is available through proxy
			resp, err := http.Get(fmt.Sprintf("%s/simple/%s/", proxyURL, pkg))
			if err != nil {
				t.Fatalf("Failed to get package %s: %v", pkg, err)
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					t.Logf("Failed to close response body: %v", err)
				}
			}()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Package %s returned status %d, expected 200", pkg, resp.StatusCode)
			}

			// Verify it's served from public PyPI
			source := resp.Header.Get("X-PyPI-Source")
			if source != "https://pypi.org/simple/" {
				t.Errorf("Package %s served from %s, expected public PyPI", pkg, source)
			}
		})
	}
}

// TestWheelFileFiltering tests that wheel files are filtered from public PyPI.
func TestWheelFileFiltering(t *testing.T) {
	t.Parallel()

	// Test packages that should have wheel files filtered.
	packages := []string{"numpy", "pandas", "matplotlib"}

	for _, pkg := range packages {
		t.Run(fmt.Sprintf("filtering_%s", pkg), func(t *testing.T) {
			// Get package page from proxy
			resp, err := http.Get(fmt.Sprintf("%s/simple/%s/", proxyURL, pkg))
			if err != nil {
				t.Fatalf("Failed to get package %s: %v", pkg, err)
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					t.Logf("Failed to close response body: %v", err)
				}
			}()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Package %s returned status %d, expected 200", pkg, resp.StatusCode)
			}

			// Read response body
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			content := string(body)

			// Check that source distributions are present
			if !strings.Contains(content, ".tar.gz") {
				t.Errorf("Package %s response missing source distributions (.tar.gz)", pkg)
			}

			// Check that wheel files are filtered out
			if strings.Contains(content, ".whl") {
				t.Errorf("Package %s response contains wheel files (.whl) - should be filtered", pkg)
			}

			// Verify it's served from public PyPI (where filtering should be applied)
			source := resp.Header.Get("X-PyPI-Source")
			if source != "https://pypi.org/simple/" {
				t.Errorf("Package %s served from %s, expected public PyPI for filtering", pkg, source)
			}
		})
	}
}

// TestMixedPackages tests packages that exist in both private and public PyPI.
func TestMixedPackages(t *testing.T) {
	t.Parallel()

	// Test packages that might exist in both sources.
	// These should be served from private PyPI (no filtering)
	packages := []string{"flask", "click", "jinja2"}

	for _, pkg := range packages {
		t.Run(fmt.Sprintf("mixed_%s", pkg), func(t *testing.T) {
			// Get package page from proxy
			resp, err := http.Get(fmt.Sprintf("%s/simple/%s/", proxyURL, pkg))
			if err != nil {
				t.Fatalf("Failed to get package %s: %v", pkg, err)
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					t.Logf("Failed to close response body: %v", err)
				}
			}()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Package %s returned status %d, expected 200", pkg, resp.StatusCode)
			}

			// Read response body
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			content := string(body)

			// Check that package content is present
			if !strings.Contains(content, pkg) {
				t.Errorf("Package %s response missing package content", pkg)
			}

			// Verify it's served from private PyPI (no filtering should be applied)
			source := resp.Header.Get("X-PyPI-Source")
			if source != privateURL+"/simple/" {
				t.Errorf("Package %s served from %s, expected private PyPI", pkg, source)
			}
		})
	}
}

// TestPipInstall tests actual pip installs through the proxy.
func TestPipInstall(t *testing.T) {
	t.Parallel()

	// Create virtual environment for testing
	venvDir := "test-venv-pip"

	// Clean up any existing venv
	if err := os.RemoveAll(venvDir); err != nil {
		t.Logf("Failed to remove existing venv: %v", err)
	}

	// Create virtual environment
	cmd := exec.Command("python3", "-m", "venv", venvDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create virtual environment: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(venvDir); err != nil {
			t.Logf("Failed to remove venv: %v", err)
		}
	}()

	// Test pip install with private packages.
	t.Run("private_packages", func(t *testing.T) {
		// nolint:gosec // proxyURL is a constant, not user input
		cmd := exec.Command(filepath.Join(venvDir, "bin", "pip"), "install",
			"--index-url", proxyURL+"/simple/",
			"--no-deps", "flask==2.3.3")

		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("pip install failed: %v\nOutput: %s", err, string(output))
		}
	})

	// Test pip install with public packages.
	t.Run("public_packages", func(t *testing.T) {
		// nolint:gosec // proxyURL is a constant, not user input
		cmd := exec.Command(filepath.Join(venvDir, "bin", "pip"), "install",
			"--index-url", proxyURL+"/simple/",
			"--no-deps", "urllib3==2.0.7")

		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("pip install failed: %v\nOutput: %s", err, string(output))
		}
	})
}

// TestProxyHealth tests that the proxy health endpoint works.
func TestProxyHealth(t *testing.T) {
	t.Parallel()

	resp, err := http.Get(proxyURL + "/health")
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Health check returned status %d, expected 200", resp.StatusCode)
	}
}

// TestProxyIndex tests that the proxy index endpoint works.
func TestProxyIndex(t *testing.T) {
	t.Parallel()

	resp, err := http.Get(proxyURL + "/simple/")
	if err != nil {
		t.Fatalf("Index check failed: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Index check returned status %d, expected 200", resp.StatusCode)
	}
}
