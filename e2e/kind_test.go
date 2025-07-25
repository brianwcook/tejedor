//go:build kind

package e2e

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const (
	kindProxyURL    = "http://127.0.0.1:8099"
	kindPrivateURL  = "http://127.0.0.1:8098"
	kindTestTimeout = 30 * time.Second
)

// TestKindLocalPriority tests that packages available in both local and public PyPI
// are served from the local PyPI server (priority).
func TestKindLocalPriority(t *testing.T) {
	t.Parallel()

	// Test packages that exist in both local and public PyPI
	// These should be served from local PyPI due to priority
	packages := []string{"flask", "click", "jinja2", "werkzeug", "six", "itsdangerous", "blinker", "requests"}

	for _, pkg := range packages {
		t.Run(fmt.Sprintf("local_priority_%s", pkg), func(t *testing.T) {
			// Check that package is available through proxy
			resp, err := http.Get(fmt.Sprintf("%s/simple/%s/", kindProxyURL, pkg))
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

			// Verify it's served from local PyPI (priority)
			source := resp.Header.Get("X-PyPI-Source")
			expectedSource := kindPrivateURL + "/simple/"
			if source != expectedSource {
				t.Errorf("Package %s served from %s, expected local PyPI (%s)", pkg, source, expectedSource)
			}

			// Verify the package metadata shows local server URLs
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			// Local packages should have relative URLs pointing to local server
			if !strings.Contains(string(body), "/packages/") {
				t.Errorf("Package %s metadata does not contain local server URLs", pkg)
			}

			// Local packages should NOT have public PyPI URLs
			if strings.Contains(string(body), "files.pythonhosted.org") {
				t.Errorf("Package %s metadata contains public PyPI URLs, expected local only", pkg)
			}
		})
	}
}

// TestKindPublicOnly tests that packages only available in public PyPI
// are served from public PyPI.
func TestKindPublicOnly(t *testing.T) {
	t.Parallel()

	// Test packages that only exist in public PyPI
	packages := []string{"numpy", "pandas", "matplotlib", "scipy", "urllib3", "certifi"}

	for _, pkg := range packages {
		t.Run(fmt.Sprintf("public_only_%s", pkg), func(t *testing.T) {
			// Check that package is available through proxy
			resp, err := http.Get(fmt.Sprintf("%s/simple/%s/", kindProxyURL, pkg))
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
			expectedSource := "https://pypi.org/simple/"
			if source != expectedSource {
				t.Errorf("Package %s served from %s, expected public PyPI (%s)", pkg, source, expectedSource)
			}

			// Verify the package metadata shows public PyPI URLs
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			// Public packages should have absolute URLs pointing to public PyPI
			if !strings.Contains(string(body), "files.pythonhosted.org") {
				t.Errorf("Package %s metadata does not contain public PyPI URLs", pkg)
			}
		})
	}
}

// TestKindFileDownload tests that package files are downloaded from the correct source.
func TestKindFileDownload(t *testing.T) {
	t.Parallel()

	// Test file downloads for packages that exist in both indexes
	// These should be downloaded from local PyPI
	localPackages := []struct {
		name     string
		filename string
	}{
		{"flask", "flask-2.3.3.tar.gz"},
		{"click", "click-8.1.7.tar.gz"},
		{"requests", "requests-2.31.0.tar.gz"},
	}

	for _, pkg := range localPackages {
		t.Run(fmt.Sprintf("file_download_local_%s", pkg.name), func(t *testing.T) {
			// Try to download the file through the proxy
			resp, err := http.Get(fmt.Sprintf("%s/packages/%s", kindProxyURL, pkg.filename))
			if err != nil {
				t.Fatalf("Failed to download file %s: %v", pkg.filename, err)
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					t.Logf("Failed to close response body: %v", err)
				}
			}()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("File %s returned status %d, expected 200", pkg.filename, resp.StatusCode)
			}

			// Verify it's served from local PyPI
			source := resp.Header.Get("X-PyPI-Source")
			expectedSource := kindPrivateURL + "/simple/"
			if source != expectedSource {
				t.Errorf("File %s served from %s, expected local PyPI (%s)", pkg.filename, source, expectedSource)
			}
		})
	}

	// Test file downloads for packages that only exist in public PyPI
	// These should be downloaded from public PyPI
	publicPackages := []struct {
		name     string
		filename string
	}{
		{"numpy", "numpy-1.24.3.tar.gz"},
		{"pandas", "pandas-2.0.3.tar.gz"},
	}

	for _, pkg := range publicPackages {
		t.Run(fmt.Sprintf("file_download_public_%s", pkg.name), func(t *testing.T) {
			// Try to download the file through the proxy
			resp, err := http.Get(fmt.Sprintf("%s/packages/%s", kindProxyURL, pkg.filename))
			if err != nil {
				t.Fatalf("Failed to download file %s: %v", pkg.filename, err)
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					t.Logf("Failed to close response body: %v", err)
				}
			}()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("File %s returned status %d, expected 200", pkg.filename, resp.StatusCode)
			}

			// Verify it's served from public PyPI
			source := resp.Header.Get("X-PyPI-Source")
			expectedSource := "https://pypi.org/simple/"
			if source != expectedSource {
				t.Errorf("File %s served from %s, expected public PyPI (%s)", pkg.filename, source, expectedSource)
			}
		})
	}
}

// TestKindProxyHealth tests that the proxy health endpoint is working.
func TestKindProxyHealth(t *testing.T) {
	t.Parallel()

	resp, err := http.Get(fmt.Sprintf("%s/health", kindProxyURL))
	if err != nil {
		t.Fatalf("Failed to get health endpoint: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Health endpoint returned status %d, expected 200", resp.StatusCode)
	}

	// Verify it's served from proxy
	source := resp.Header.Get("X-PyPI-Source")
	if source != "proxy" {
		t.Errorf("Health endpoint served from %s, expected proxy", source)
	}
}

// TestKindProxyIndex tests that the proxy index page is working.
func TestKindProxyIndex(t *testing.T) {
	t.Parallel()

	resp, err := http.Get(kindProxyURL)
	if err != nil {
		t.Fatalf("Failed to get index page: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Index page returned status %d, expected 200", resp.StatusCode)
	}

	// Verify it's served from proxy
	source := resp.Header.Get("X-PyPI-Source")
	if source != "proxy" {
		t.Errorf("Index page served from %s, expected proxy", source)
	}
}

// TestKindLocalServerAccess tests direct access to the local PyPI server.
func TestKindLocalServerAccess(t *testing.T) {
	t.Parallel()

	// Test that local packages are accessible directly from local server
	localPackages := []string{"flask", "click", "requests"}

	for _, pkg := range localPackages {
		t.Run(fmt.Sprintf("local_server_%s", pkg), func(t *testing.T) {
			resp, err := http.Get(fmt.Sprintf("%s/simple/%s/", kindPrivateURL, pkg))
			if err != nil {
				t.Fatalf("Failed to get package %s from local server: %v", pkg, err)
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					t.Logf("Failed to close response body: %v", err)
				}
			}()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Package %s from local server returned status %d, expected 200", pkg, resp.StatusCode)
			}

			// Verify the package metadata shows local server URLs
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			// Local packages should have relative URLs pointing to local server
			if !strings.Contains(string(body), "/packages/") {
				t.Errorf("Package %s metadata from local server does not contain local URLs", pkg)
			}
		})
	}

	// Test that public-only packages are NOT accessible from local server
	publicOnlyPackages := []string{"numpy", "pandas"}

	for _, pkg := range publicOnlyPackages {
		t.Run(fmt.Sprintf("local_server_missing_%s", pkg), func(t *testing.T) {
			resp, err := http.Get(fmt.Sprintf("%s/simple/%s/", kindPrivateURL, pkg))
			if err != nil {
				t.Fatalf("Failed to get package %s from local server: %v", pkg, err)
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					t.Logf("Failed to close response body: %v", err)
				}
			}()

			// These packages should NOT be available from local server
			if resp.StatusCode == http.StatusOK {
				t.Errorf("Package %s unexpectedly available from local server", pkg)
			}
		})
	}
}
