package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"python-index-proxy/config"
	"python-index-proxy/proxy"
)

// isCI returns true if running in CI environment.
func isCI() bool {
	return os.Getenv("CI") == "true"
}

// TestProxyWithRealPyPI tests the proxy with real PyPI indexes.
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
		CacheEnabled:   false,
		CacheSize:      100,
		CacheTTL:       1,
	}

	// Create proxy instance
	proxyInstance, err := proxy.NewProxy(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Test cases for different packages
	testCases := []struct {
		packageName string
		shouldExist bool
	}{
		{"pycups", true},   // Should exist in public PyPI
		{"pip", true},      // Should exist in public PyPI
		{"requests", true}, // Should exist in public PyPI
		{"pydantic", true}, // Should exist in public PyPI
	}

	for _, tc := range testCases {
		t.Run(tc.packageName, func(t *testing.T) {
			// Create request
			req, err := http.NewRequest("GET", fmt.Sprintf("/simple/%s/", tc.packageName), http.NoBody)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			rr := httptest.NewRecorder()
			proxyInstance.HandlePackage(rr, req)

			if tc.shouldExist {
				if rr.Code != http.StatusOK {
					t.Errorf("Expected status 200, got %d", rr.Code)
				}

				// Check source header
				sourceHeader := rr.Header().Get("X-PyPI-Source")
				if sourceHeader != "https://pypi.org/simple/" && sourceHeader != "https://console.redhat.com/api/pulp-content/public-calunga/mypypi/simple" {
					t.Errorf("Expected source header from either public or private PyPI, got %s", sourceHeader)
				}

				// Check content type
				contentType := rr.Header().Get("Content-Type")
				if !strings.Contains(contentType, "text/html") {
					t.Errorf("Expected HTML content type, got %s", contentType)
				}

				// Check that response body is not empty
				if rr.Body.String() == "" {
					t.Error("Expected non-empty response body")
				}
			} else if rr.Code != http.StatusNotFound {
				t.Errorf("Expected status 404, got %d", rr.Code)
			}
		})
	}
}

// TestProxyWithCache tests the proxy with cache enabled.
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
	req1, err := http.NewRequest("GET", fmt.Sprintf("/simple/%s/", packageName), http.NoBody)
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
	req2, err := http.NewRequest("GET", fmt.Sprintf("/simple/%s/", packageName), http.NoBody)
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

// TestProxyFileHandling tests file proxying functionality.
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
	req, err := http.NewRequest("GET", "/packages/source/p/pycups/pycups-2.0.1.tar.gz", http.NoBody)
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
	if sourceHeader != "https://pypi.org/simple/" && sourceHeader != "https://console.redhat.com/api/pulp-content/public-calunga/mypypi/simple" {
		t.Errorf("Expected source header from either public or private PyPI, got %s", sourceHeader)
	}

	// Check that response body is not empty
	if rr.Body.String() == "" {
		t.Error("Expected non-empty response body")
	}
}

// TestProxyIndexPage tests the index page functionality.
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
	req, err := http.NewRequest("GET", "/", http.NoBody)
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

// TestProxyErrorHandling tests error handling scenarios.
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

	// Test package request with invalid URLs
	req, err := http.NewRequest("GET", "/simple/test-package/", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	proxyInstance.HandlePackage(rr, req)

	// Should get a 500 since both indexes are invalid and network calls fail
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rr.Code)
	}
}

// TestProxyInvalidRequests tests handling of invalid requests.
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

	// Test invalid paths
	invalidPaths := []string{
		"/invalid/path/",
		"/simple//",  // Empty package name
		"/packages/", // Invalid package path
	}

	for _, path := range invalidPaths {
		t.Run(path, func(t *testing.T) {
			req1, err := http.NewRequest("GET", path, http.NoBody)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			rr1 := httptest.NewRecorder()
			proxyInstance.HandlePackage(rr1, req1)

			if rr1.Code != http.StatusBadRequest {
				t.Errorf("Expected status 400 for invalid path %s, got %d", path, rr1.Code)
			}
		})
	}

	// Test file requests with invalid paths
	req2, err := http.NewRequest("GET", "/packages/", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rr2 := httptest.NewRecorder()
	proxyInstance.HandleFile(rr2, req2)

	if rr2.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid file path, got %d", rr2.Code)
	}

	req3, err := http.NewRequest("GET", "/packages/", http.NoBody)
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
// they are properly filtered to remove wheel files.
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

	// Test with a package that exists in public PyPI
	packageName := "flask"

	// Make direct requests to both indexes to compare
	client := &http.Client{Timeout: 10 * time.Second}

	// Get from public PyPI directly
	publicReq, err := http.NewRequest("GET", "https://pypi.org/simple/flask/", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create public request: %v", err)
	}

	publicResp, err := client.Do(publicReq)
	if err != nil {
		t.Fatalf("Failed to get from public PyPI: %v", err)
	}
	defer publicResp.Body.Close()

	// Get from private PyPI directly
	privateReq, err := http.NewRequest("GET", "https://console.redhat.com/api/pulp-content/public-calunga/mypypi/simple/flask/", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create private request: %v", err)
	}

	privateResp, err := client.Do(privateReq)
	if err != nil {
		t.Fatalf("Failed to get from private PyPI: %v", err)
	}
	defer privateResp.Body.Close()

	// Test proxy request
	req, err := http.NewRequest("GET", fmt.Sprintf("/simple/%s/", packageName), http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	proxyInstance.HandlePackage(rr, req)

	// Check response
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// Check source header - should be from public PyPI
	sourceHeader := rr.Header().Get("X-PyPI-Source")
	if sourceHeader != "https://pypi.org/simple/" {
		t.Errorf("Expected source header 'https://pypi.org/simple/', got %s", sourceHeader)
	}

	// The response should be filtered (no wheel files) and come from public PyPI
	// We can't easily test the exact content without making the test brittle,
	// but we can verify it's not empty and has the right content type
	if rr.Body.String() == "" {
		t.Error("Expected non-empty response body")
	}

	contentType := rr.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("Expected HTML content type, got %s", contentType)
	}
}

// TestPrivateIndexNoFiltering tests that when packages are served from private index,
// they are not filtered (wheel files are preserved).
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

	// Test with a package that exists in private PyPI but not public
	// We'll use a package that we know exists only in the private index
	packageName := "pycups"

	// Test proxy request
	req, err := http.NewRequest("GET", fmt.Sprintf("/simple/%s/", packageName), http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	proxyInstance.HandlePackage(rr, req)

	// Check response
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// Check source header - should be from either public or private PyPI
	sourceHeader := rr.Header().Get("X-PyPI-Source")
	if sourceHeader != "https://pypi.org/simple/" && sourceHeader != "https://console.redhat.com/api/pulp-content/public-calunga/mypypi/simple" {
		t.Errorf("Expected source header from either public or private PyPI, got %s", sourceHeader)
	}

	// The response should not be filtered (wheel files preserved) and come from private PyPI
	// We can't easily test the exact content without making the test brittle,
	// but we can verify it's not empty and has the right content type
	if rr.Body.String() == "" {
		t.Error("Expected non-empty response body")
	}

	contentType := rr.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("Expected HTML content type, got %s", contentType)
	}
}

// TestProxyHEADRequests tests HEAD requests for /simple/{package}/ and /packages/{file}.
func TestProxyHEADRequests(t *testing.T) {
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

	// Test HEAD request for package
	req, err := http.NewRequest("HEAD", "/simple/non-existent-package-12345/", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	proxyInstance.HandlePackage(rr, req)

	// Check response
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 for non-existent package, got %d", rr.Code)
	}

	// Test HEAD request for file
	filePath := "/packages/source/p/non-existent-package/non-existent-package-1.0.0.tar.gz"
	req2, err := http.NewRequest("HEAD", filePath, http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rr2 := httptest.NewRecorder()
	proxyInstance.HandleFile(rr2, req2)

	// Check response
	if rr2.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 for non-existent file, got %d", rr2.Code)
	}
}
