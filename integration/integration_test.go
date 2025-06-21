package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"python-index-proxy/cache"
	"python-index-proxy/config"
	"python-index-proxy/proxy"
)

// TestProxyWithRealPyPI tests the proxy with real PyPI indexes
func TestProxyWithRealPyPI(t *testing.T) {
	// Skip if running in CI or if network is not available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
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
			expectedSource: "public",
			shouldExist:    true,
		},
		{
			name:           "Package in both indexes (pydantic)",
			packageName:    "pydantic",
			expectedSource: "private",
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
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
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
	cache := proxyInstance.GetCache()
	publicLen, privateLen := cache.GetStats()
	t.Logf("Cache stats after first request: public=%d, private=%d", publicLen, privateLen)

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
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
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
	if sourceHeader != "public" {
		t.Errorf("Expected source header 'public', got %s", sourceHeader)
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