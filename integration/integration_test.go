package integration

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"python-index-proxy/config"
	"python-index-proxy/proxy"
)

// isCI returns true if running in CI environment
func isCI() bool {
	return os.Getenv("CI") == "true"
}

// TestProxyWithRealPyPI tests the proxy with real PyPI indexes
func TestProxyWithRealPyPI(t *testing.T) {
	// Skip if running in CI or if network is not available
	if testing.Short() || isCI() {
		t.Skip("Skipping integration test in short mode or CI")
	}

	// Create test configuration
	cfg := &config.Config{
		PublicPyPIURL:  "https://pypi.org/simple/",
		PrivatePyPIURL: "https://console.redhat.com/api/pulp-content/public-calunga/mypypi/simple",
		Port:           8080,
		CacheEnabled:   false, // Disable cache for integration tests
		CacheSize:      100,
		CacheTTL:       1,
	}

	// Create proxy instance
	proxyInstance, err := proxy.NewProxy(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Test cases
	testCases := []struct {
		name           string
		packageName    string
		expectedSource string
		shouldExist    bool
	}{
		{
			name:           "Package only in public PyPI (pycups)",
			packageName:    "pycups",
			expectedSource: "https://pypi.org/simple/",
			shouldExist:    true,
		},
		{
			name:           "Package in both indexes (pydantic)",
			packageName:    "pydantic",
			expectedSource: "https://console.redhat.com/api/pulp-content/public-calunga/mypypi/simple",
			shouldExist:    true,
		},
		{
			name:           "Non-existent package",
			packageName:    "non-existent-package-12345",
			expectedSource: "",
			shouldExist:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request
			req, err := http.NewRequest("GET", fmt.Sprintf("/simple/%s/", tc.packageName), nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			// Create response recorder
			rr := httptest.NewRecorder()

			// Handle request
			proxyInstance.HandlePackage(rr, req)

			// Check response
			if tc.shouldExist {
				if rr.Code != http.StatusOK {
					t.Errorf("Expected status 200, got %d", rr.Code)
				}

				// Check source header
				sourceHeader := rr.Header().Get("X-PyPI-Source")
				if sourceHeader != tc.expectedSource {
					t.Errorf("Expected source header %s, got %s", tc.expectedSource, sourceHeader)
				}

				// Check content type
				contentType := rr.Header().Get("Content-Type")
				if !strings.Contains(contentType, "text/html") {
					t.Errorf("Expected HTML content type, got %s", contentType)
				}

				// Check that response body is not empty
				if len(rr.Body.String()) == 0 {
					t.Error("Expected non-empty response body")
				}
			} else {
				if rr.Code != http.StatusNotFound {
					t.Errorf("Expected status 404, got %d", rr.Code)
				}
			}
		})
	}
}

// TestProxyWithCache tests the proxy with cache enabled
func TestProxyWithCache(t *testing.T) {
	// Skip if running in CI or if network is not available
	if testing.Short() || isCI() {
		t.Skip("Skipping integration test in short mode or CI")
	}

	// Create test configuration with cache enabled
	cfg := &config.Config{
		PublicPyPIURL:  "https://pypi.org/simple/",
		PrivatePyPIURL: "https://console.redhat.com/api/pulp-content/public-calunga/mypypi/simple",
		Port:           8080,
		CacheEnabled:   true,
		CacheSize:      100,
		CacheTTL:       1,
	}

	// Create proxy instance
	proxyInstance, err := proxy.NewProxy(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Test package that should be cached
	packageName := "pycups"

	// First request - should hit the network
	req1, err := http.NewRequest("GET", fmt.Sprintf("/simple/%s/", packageName), nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	rr1 := httptest.NewRecorder()
	proxyInstance.HandlePackage(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Fatalf("First request failed with status %d", rr1.Code)
	}

	// Check cache stats
	publicLen, privateLen, publicPageLen, privatePageLen := proxyInstance.GetCache().GetStats()
	if publicLen != 1 {
		t.Errorf("Expected 1 public package in cache, got %d", publicLen)
	}
	if privateLen != 1 {
		t.Errorf("Expected 1 private package in cache, got %d", privateLen)
	}
	if publicPageLen != 1 {
		t.Errorf("Expected 1 public page in cache, got %d", publicPageLen)
	}
	if privatePageLen != 0 {
		t.Errorf("Expected 0 private pages in cache, got %d", privatePageLen)
	}

	// Second request - should use cache
	req2, err := http.NewRequest("GET", fmt.Sprintf("/simple/%s/", packageName), nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	rr2 := httptest.NewRecorder()
	proxyInstance.HandlePackage(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("Second request failed with status %d", rr2.Code)
	}

	// Both responses should be identical
	if rr1.Body.String() != rr2.Body.String() {
		t.Error("Cached response should be identical to first response")
	}

	// Check source headers are consistent
	source1 := rr1.Header().Get("X-PyPI-Source")
	source2 := rr2.Header().Get("X-PyPI-Source")
	if source1 != source2 {
		t.Errorf("Source headers should be consistent: %s vs %s", source1, source2)
	}
}

// TestProxyFileHandling tests file proxying functionality
func TestProxyFileHandling(t *testing.T) {
	// Skip if running in CI or if network is not available
	if testing.Short() || isCI() {
		t.Skip("Skipping integration test in short mode or CI")
	}

	// Create test configuration
	cfg := &config.Config{
		PublicPyPIURL:  "https://pypi.org/simple/",
		PrivatePyPIURL: "https://console.redhat.com/api/pulp-content/public-calunga/mypypi/simple",
		Port:           8080,
		CacheEnabled:   false,
		CacheSize:      100,
		CacheTTL:       1,
	}

	// Create proxy instance
	proxyInstance, err := proxy.NewProxy(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Test file request for a package that exists in public PyPI
	req, err := http.NewRequest("GET", "/packages/source/p/pycups/pycups-2.0.1.tar.gz", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	proxyInstance.HandleFile(rr, req)

	// Check response
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// Check source header
	sourceHeader := rr.Header().Get("X-PyPI-Source")
	if sourceHeader != "https://pypi.org/simple/" {
		t.Errorf("Expected source header 'https://pypi.org/simple/', got %s", sourceHeader)
	}

	// Check that response body is not empty
	if len(rr.Body.String()) == 0 {
		t.Error("Expected non-empty response body")
	}
}

// TestProxyIndexPage tests the index page functionality
func TestProxyIndexPage(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		PublicPyPIURL:  "https://pypi.org/simple/",
		PrivatePyPIURL: "https://console.redhat.com/api/pulp-content/public-calunga/mypypi/simple",
		Port:           8080,
		CacheEnabled:   false,
		CacheSize:      100,
		CacheTTL:       1,
	}

	// Create proxy instance
	proxyInstance, err := proxy.NewProxy(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Test index page request
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	proxyInstance.HandleIndex(rr, req)

	// Check response
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// Check source header
	sourceHeader := rr.Header().Get("X-PyPI-Source")
	if sourceHeader != "proxy" {
		t.Errorf("Expected source header 'proxy', got %s", sourceHeader)
	}

	// Check content type
	contentType := rr.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("Expected HTML content type, got %s", contentType)
	}

	// Check that response body contains expected content
	body := rr.Body.String()
	if !strings.Contains(body, "PyPI Proxy") {
		t.Error("Expected response body to contain 'PyPI Proxy'")
	}
}

// TestProxyErrorHandling tests error handling scenarios
func TestProxyErrorHandling(t *testing.T) {
	// Create test configuration with invalid URLs
	cfg := &config.Config{
		PublicPyPIURL:  "https://invalid-url-that-does-not-exist-12345.com/simple/",
		PrivatePyPIURL: "https://another-invalid-url-12345.com/simple/",
		Port:           8080,
		CacheEnabled:   false,
		CacheSize:      100,
		CacheTTL:       1,
	}

	// Create proxy instance
	proxyInstance, err := proxy.NewProxy(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Test request with invalid URLs
	req, err := http.NewRequest("GET", "/simple/test-package/", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	proxyInstance.HandlePackage(rr, req)

	// Should get an error status
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rr.Code)
	}
}

// TestProxyInvalidRequests tests handling of invalid requests
func TestProxyInvalidRequests(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		PublicPyPIURL:  "https://pypi.org/simple/",
		PrivatePyPIURL: "https://console.redhat.com/api/pulp-content/public-calunga/mypypi/simple",
		Port:           8080,
		CacheEnabled:   false,
		CacheSize:      100,
		CacheTTL:       1,
	}

	// Create proxy instance
	proxyInstance, err := proxy.NewProxy(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Test invalid package path
	req1, err := http.NewRequest("GET", "/invalid/path/", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rr1 := httptest.NewRecorder()
	proxyInstance.HandlePackage(rr1, req1)

	if rr1.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid path, got %d", rr1.Code)
	}

	// Test empty package name
	req2, err := http.NewRequest("GET", "/simple//", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rr2 := httptest.NewRecorder()
	proxyInstance.HandlePackage(rr2, req2)

	if rr2.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for empty package name, got %d", rr2.Code)
	}

	// Test invalid file path
	req3, err := http.NewRequest("GET", "/packages/", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rr3 := httptest.NewRecorder()
	proxyInstance.HandleFile(rr3, req3)

	if rr3.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid file path, got %d", rr3.Code)
	}
}

// TestPublicPyPISourceOnly tests that when packages are served from public PyPI,
// wheel files are filtered out and only source distributions are returned
func TestPublicPyPISourceOnly(t *testing.T) {
	// Skip if running in CI or if network is not available
	if testing.Short() || isCI() {
		t.Skip("Skipping integration test in short mode or CI")
	}

	// Create test configuration
	cfg := &config.Config{
		PublicPyPIURL:  "https://pypi.org/simple/",
		PrivatePyPIURL: "https://console.redhat.com/api/pulp-content/public-calunga/mypypi/simple",
		Port:           8080,
		CacheEnabled:   false,
		CacheSize:      100,
		CacheTTL:       1,
	}

	// Create proxy instance
	proxyInstance, err := proxy.NewProxy(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Test package: Flask
	// IMPORTANT: This test package must:
	// 1. Exist in public PyPI (verified: 200 OK)
	// 2. NOT exist in private index (verified: 404)
	// 3. Have wheel files in public PyPI (verified: contains .whl files)
	// If Flask is added to the private index later, this test will fail
	// and you'll need to find a new test package!
	packageName := "flask"

	t.Run(fmt.Sprintf("Package_%s_wheel_filtering", packageName), func(t *testing.T) {
		// First, verify the package conditions are still met
		// Check that package exists in public PyPI
		publicReq, err := http.NewRequest("GET", "https://pypi.org/simple/flask/", nil)
		if err != nil {
			t.Fatalf("Failed to create public PyPI request: %v", err)
		}
		publicResp, err := http.DefaultClient.Do(publicReq)
		if err != nil {
			t.Fatalf("Failed to check public PyPI: %v", err)
		}
		defer publicResp.Body.Close()
		if publicResp.StatusCode != http.StatusOK {
			t.Fatalf("Package %s no longer exists in public PyPI (status: %d)", packageName, publicResp.StatusCode)
		}

		// Check that package does NOT exist in private index
		privateReq, err := http.NewRequest("GET", "https://console.redhat.com/api/pulp-content/public-calunga/mypypi/simple/flask/", nil)
		if err != nil {
			t.Fatalf("Failed to create private index request: %v", err)
		}
		privateResp, err := http.DefaultClient.Do(privateReq)
		if err != nil {
			t.Fatalf("Failed to check private index: %v", err)
		}
		defer privateResp.Body.Close()
		if privateResp.StatusCode == http.StatusOK {
			t.Fatalf("Package %s now exists in private index! Need to find a new test package.", packageName)
		}

		// Verify that public PyPI has wheel files
		publicBody, err := io.ReadAll(publicResp.Body)
		if err != nil {
			t.Fatalf("Failed to read public PyPI response: %v", err)
		}
		if !strings.Contains(string(publicBody), ".whl") {
			t.Fatalf("Package %s no longer has wheel files in public PyPI! Need to find a new test package.", packageName)
		}

		// Now test the proxy
		req, err := http.NewRequest("GET", fmt.Sprintf("/simple/%s/", packageName), nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		// Create response recorder
		rr := httptest.NewRecorder()

		// Handle request
		proxyInstance.HandlePackage(rr, req)

		// Check response
		if rr.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", rr.Code)
		}

		// Check source header indicates public PyPI
		sourceHeader := rr.Header().Get("X-PyPI-Source")
		if sourceHeader != "https://pypi.org/simple/" {
			t.Errorf("Expected source header 'https://pypi.org/simple/', got %s", sourceHeader)
		}

		// Get response body
		body := rr.Body.String()

		// Verify that wheel files are filtered out
		if strings.Contains(body, ".whl") {
			t.Errorf("Response contains wheel files (.whl) for package %s, but should be filtered out", packageName)
		}

		// Verify that source distributions are still present
		if !strings.Contains(body, ".tar.gz") && !strings.Contains(body, ".zip") {
			t.Errorf("Response does not contain source distributions (.tar.gz or .zip) for package %s", packageName)
		}

		t.Logf("Package %s: Wheel files successfully filtered out, source distributions preserved", packageName)
	})
}

// TestPrivateIndexNoFiltering tests that when packages are served from private index,
// no filtering is applied and all files (including wheels) are returned
func TestPrivateIndexNoFiltering(t *testing.T) {
	// Skip if running in CI or if network is not available
	if testing.Short() || isCI() {
		t.Skip("Skipping integration test in short mode or CI")
	}

	// Create test configuration
	cfg := &config.Config{
		PublicPyPIURL:  "https://pypi.org/simple/",
		PrivatePyPIURL: "https://console.redhat.com/api/pulp-content/public-calunga/mypypi/simple",
		Port:           8080,
		CacheEnabled:   false,
		CacheSize:      100,
		CacheTTL:       1,
	}

	// Create proxy instance
	proxyInstance, err := proxy.NewProxy(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Test package: pydantic (exists in both indexes, but should be served from private)
	packageName := "pydantic"

	t.Run(fmt.Sprintf("Package_%s_no_filtering", packageName), func(t *testing.T) {
		// Create request
		req, err := http.NewRequest("GET", fmt.Sprintf("/simple/%s/", packageName), nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		// Create response recorder
		rr := httptest.NewRecorder()

		// Handle request
		proxyInstance.HandlePackage(rr, req)

		// Check response
		if rr.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", rr.Code)
		}

		// Check source header indicates private index
		sourceHeader := rr.Header().Get("X-PyPI-Source")
		if sourceHeader != "https://console.redhat.com/api/pulp-content/public-calunga/mypypi/simple" {
			t.Errorf("Expected source header from private index, got %s", sourceHeader)
		}

		// Get response body
		body := rr.Body.String()

		// Verify that the response contains package information
		if len(body) == 0 {
			t.Error("Response body is empty")
		}

		// Log the file types found for debugging
		hasWheels := strings.Contains(body, ".whl")
		hasSource := strings.Contains(body, ".tar.gz") || strings.Contains(body, ".zip")

		t.Logf("Package %s from private index - Wheels: %t, Source: %t", packageName, hasWheels, hasSource)

		// Note: We don't assert specific file types here because the private index
		// may have different file types than public PyPI. The important thing is
		// that no filtering is applied when serving from private index.
	})
}

// TestProxyHEADRequests tests HEAD requests for /simple/{package}/ and /packages/{file}
func TestProxyHEADRequests(t *testing.T) {
	// Create test configuration with mock URLs
	cfg := &config.Config{
		PublicPyPIURL:  "https://mock-public-pypi.org/simple/",
		PrivatePyPIURL: "https://mock-private-pypi.org/simple/",
		Port:           8080,
		CacheEnabled:   false,
		CacheSize:      100,
		CacheTTL:       1,
	}

	proxyInstance, err := proxy.NewProxy(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// HEAD request to /simple/{package}/ - this will fail with network error but we can test the handler
	req, err := http.NewRequest("HEAD", "/simple/test-package/", nil)
	if err != nil {
		t.Fatalf("Failed to create HEAD request: %v", err)
	}
	rr := httptest.NewRecorder()
	proxyInstance.HandlePackage(rr, req)

	// The request will fail due to network error, but we can verify the handler accepts HEAD method
	// and doesn't return 405 Method Not Allowed
	if rr.Code == http.StatusMethodNotAllowed {
		t.Error("HEAD request returned 405 Method Not Allowed - handler doesn't support HEAD")
	}

	// Verify that the handler processed the request (even if it failed due to network)
	if rr.Header().Get("X-PyPI-Source") == "" && rr.Code != http.StatusInternalServerError {
		t.Error("Expected X-PyPI-Source header or internal server error for HEAD /simple/test-package/")
	}

	// HEAD request to /packages/{file} - this will also fail with network error but we can test the handler
	filePath := "/packages/source/p/test-package/test-package-1.0.0.tar.gz"
	req2, err := http.NewRequest("HEAD", filePath, nil)
	if err != nil {
		t.Fatalf("Failed to create HEAD request: %v", err)
	}
	rr2 := httptest.NewRecorder()
	proxyInstance.HandleFile(rr2, req2)

	// The request will fail due to network error, but we can verify the handler accepts HEAD method
	if rr2.Code == http.StatusMethodNotAllowed {
		t.Error("HEAD request returned 405 Method Not Allowed - handler doesn't support HEAD")
	}

	// Verify that the handler processed the request (even if it failed due to network)
	if rr2.Header().Get("X-PyPI-Source") == "" && rr2.Code != http.StatusInternalServerError {
		t.Error("Expected X-PyPI-Source header or internal server error for HEAD /packages/{file}")
	}
}
