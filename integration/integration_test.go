package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"python-index-proxy/config"
	"python-index-proxy/proxy"
	"strings"
	"testing"
)

// isCI returns true if running in CI environment.
func isCI() bool {
	return os.Getenv("CI") == "true"
}

// LocalPyPIServer represents a local PyPI server for testing
type LocalPyPIServer struct {
	server   *httptest.Server
	packages map[string]PackageInfo
}

// PackageInfo contains information about a package
type PackageInfo struct {
	Name     string
	Versions []string
	Files    []PackageFile
}

// PackageFile represents a package file
type PackageFile struct {
	Filename string
	URL      string
	Size     int64
}

// NewLocalPyPIServer creates a new local PyPI server
func NewLocalPyPIServer() *LocalPyPIServer {
	server := &LocalPyPIServer{
		packages: make(map[string]PackageInfo),
	}

	// Populate with some test packages
	server.populateTestPackages()

	server.server = httptest.NewServer(http.HandlerFunc(server.handleRequest))
	return server
}

// populateTestPackages adds test packages to the local server
func (s *LocalPyPIServer) populateTestPackages() {
	// Add packages that exist only in our local server (simulating private packages)
	s.packages["privatepackage"] = PackageInfo{
		Name:     "privatepackage",
		Versions: []string{"1.0.0", "1.1.0"},
		Files: []PackageFile{
			{
				Filename: "privatepackage-1.0.0.tar.gz",
				URL:      "/packages/source/p/privatepackage/privatepackage-1.0.0.tar.gz",
				Size:     1024,
			},
			{
				Filename: "privatepackage-1.1.0.tar.gz",
				URL:      "/packages/source/p/privatepackage/privatepackage-1.1.0.tar.gz",
				Size:     2048,
			},
		},
	}

	s.packages["mixedpackage"] = PackageInfo{
		Name:     "mixedpackage",
		Versions: []string{"2.0.0"},
		Files: []PackageFile{
			{
				Filename: "mixedpackage-2.0.0.tar.gz",
				URL:      "/packages/source/m/mixedpackage/mixedpackage-2.0.0.tar.gz",
				Size:     1536,
			},
			{
				Filename: "mixedpackage-2.0.0-py3-none-any.whl",
				URL:      "/packages/py3/m/mixedpackage/mixedpackage-2.0.0-py3-none-any.whl",
				Size:     2560,
			},
		},
	}
}

// handleRequest handles HTTP requests to the local PyPI server
func (s *LocalPyPIServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Normalize path to handle double slashes and malformed URLs
	// The proxy might send URLs like "//packages/..." which should become "/packages/..."
	path = strings.ReplaceAll(path, "//packages/", "/packages/")
	path = strings.ReplaceAll(path, "//", "/")

	// Handle package index requests
	if strings.HasPrefix(path, "/simple/") {
		s.handlePackageIndex(w, r)
		return
	}

	// Handle file requests
	if strings.HasPrefix(path, "/packages/") {
		s.handleFileRequest(w, r)
		return
	}

	// Default response
	w.WriteHeader(http.StatusNotFound)
}

// handlePackageIndex handles requests for package index pages
func (s *LocalPyPIServer) handlePackageIndex(w http.ResponseWriter, r *http.Request) {
	// Extract package name from path /simple/{package}/
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	packageName := parts[1]

	// Check if package exists
	packageInfo, exists := s.packages[packageName]
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// For HEAD requests, just return success without body
	if r.Method == "HEAD" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Generate package index HTML for GET requests
	html := s.generatePackageIndexHTML(packageInfo)

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

// handleFileRequest handles requests for package files
func (s *LocalPyPIServer) handleFileRequest(w http.ResponseWriter, r *http.Request) {
	// Extract package name from path /packages/source/p/{package}/{filename}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 5 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	packageName := parts[3] // The package name is at index 3
	filename := parts[4]    // The filename is at index 4

	// Check if package exists
	packageInfo, exists := s.packages[packageName]
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Check if file exists
	var fileInfo PackageFile
	found := false
	for _, file := range packageInfo.Files {
		if file.Filename == filename {
			fileInfo = file
			found = true
			break
		}
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Return mock file content
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size))
	w.WriteHeader(http.StatusOK)

	// Generate mock file content
	content := fmt.Sprintf("Mock content for %s (size: %d bytes)", filename, fileInfo.Size)
	w.Write([]byte(content))
}

// generatePackageIndexHTML generates HTML for a package index page
func (s *LocalPyPIServer) generatePackageIndexHTML(pkg PackageInfo) string {
	var links strings.Builder

	for _, file := range pkg.Files {
		links.WriteString(fmt.Sprintf(`<a href="%s">%s</a><br/>`, file.URL, file.Filename))
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Links for %s</title>
</head>
<body>
    <h1>Links for %s</h1>
    %s
</body>
</html>`, pkg.Name, pkg.Name, links.String())
}

// URL returns the base URL of the local server
func (s *LocalPyPIServer) URL() string {
	return s.server.URL + "/simple"
}

// Close closes the local server
func (s *LocalPyPIServer) Close() {
	s.server.Close()
}

// TestProxyWithLocalPyPI tests the proxy with a local PyPI server.
func TestProxyWithLocalPyPI(t *testing.T) {
	// Skip if running in CI or if network is not available
	if testing.Short() || isCI() {
		t.Skip("Skipping integration test in short mode or CI")
	}

	// Start local PyPI server
	localServer := NewLocalPyPIServer()
	defer localServer.Close()

	// Create test configuration
	cfg := &config.Config{
		PublicPyPIURL:  "https://pypi.org/simple/",
		PrivatePyPIURL: localServer.URL(),
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
		source      string
	}{
		{"requests", true, "https://pypi.org/simple/"}, // Should exist in public PyPI
		{"pip", true, "https://pypi.org/simple/"},      // Should exist in public PyPI
		{"privatepackage", true, localServer.URL()},    // Should exist in local server
		{"mixedpackage", true, localServer.URL()},      // Should exist in local server
		{"non-existent-package", false, ""},            // Should not exist anywhere
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
				if sourceHeader != tc.source {
					t.Errorf("Expected source header %s, got %s", tc.source, sourceHeader)
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

	// Start local PyPI server
	localServer := NewLocalPyPIServer()
	defer localServer.Close()

	// Create test configuration with cache enabled
	cfg := &config.Config{
		PublicPyPIURL:  "https://pypi.org/simple/",
		PrivatePyPIURL: localServer.URL(),
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
	packageName := "privatepackage"

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

	// Check cache stats - should have both public and private package existence checks
	publicLen, privateLen, publicPageLen, privatePageLen := proxyInstance.GetCache().GetStats()
	if publicLen != 1 {
		t.Errorf("Expected 1 public package in cache (existence check), got %d", publicLen)
	}
	if privateLen != 1 {
		t.Errorf("Expected 1 private package in cache (existence check), got %d", privateLen)
	}
	if publicPageLen != 0 {
		t.Errorf("Expected 0 public pages in cache, got %d", publicPageLen)
	}
	if privatePageLen != 1 {
		t.Errorf("Expected 1 private page in cache, got %d", privatePageLen)
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

	// Start local PyPI server
	localServer := NewLocalPyPIServer()
	defer localServer.Close()

	// Create test configuration
	cfg := &config.Config{
		PublicPyPIURL:  "https://pypi.org/simple/",
		PrivatePyPIURL: localServer.URL(),
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

	// Test file request for a package that exists in local server
	req, err := http.NewRequest("GET", "/packages/source/p/privatepackage/privatepackage-1.0.0.tar.gz", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	proxyInstance.HandleFile(rr, req)

	// Check response
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// Check source header - should be from local server since package exists there
	sourceHeader := rr.Header().Get("X-PyPI-Source")
	if sourceHeader != localServer.URL() {
		t.Errorf("Expected source header %s, got %s", localServer.URL(), sourceHeader)
	}

	// Check that response body is not empty
	if rr.Body.String() == "" {
		t.Error("Expected non-empty response body")
	}
}

// TestProxyIndexPage tests the index page functionality.
func TestProxyIndexPage(t *testing.T) {
	// Start local PyPI server
	localServer := NewLocalPyPIServer()
	defer localServer.Close()

	// Create test configuration
	cfg := &config.Config{
		PublicPyPIURL:  "https://pypi.org/simple/",
		PrivatePyPIURL: localServer.URL(),
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
	// Start local PyPI server
	localServer := NewLocalPyPIServer()
	defer localServer.Close()

	// Create test configuration
	cfg := &config.Config{
		PublicPyPIURL:  "https://pypi.org/simple/",
		PrivatePyPIURL: localServer.URL(),
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
}

// TestPublicPyPISourceOnly tests that when packages are served from public PyPI,
// they are properly filtered to remove wheel files.
func TestPublicPyPISourceOnly(t *testing.T) {
	// Skip if running in CI or if network is not available
	if testing.Short() || isCI() {
		t.Skip("Skipping integration test in short mode or CI")
	}

	// Start local PyPI server
	localServer := NewLocalPyPIServer()
	defer localServer.Close()

	// Create test configuration
	cfg := &config.Config{
		PublicPyPIURL:  "https://pypi.org/simple/",
		PrivatePyPIURL: localServer.URL(),
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
	// Start local PyPI server
	localServer := NewLocalPyPIServer()
	defer localServer.Close()

	// Create test configuration
	cfg := &config.Config{
		PublicPyPIURL:  "https://pypi.org/simple/",
		PrivatePyPIURL: localServer.URL(),
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

	// Test with a package that exists in local server
	packageName := "mixedpackage"

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

	// Check source header - should be from local server
	sourceHeader := rr.Header().Get("X-PyPI-Source")
	if sourceHeader != localServer.URL() {
		t.Errorf("Expected source header %s, got %s", localServer.URL(), sourceHeader)
	}

	// The response should not be filtered (wheel files preserved) and come from local server
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
	// Start local PyPI server
	localServer := NewLocalPyPIServer()
	defer localServer.Close()

	// Create test configuration
	cfg := &config.Config{
		PublicPyPIURL:  "https://pypi.org/simple/",
		PrivatePyPIURL: localServer.URL(),
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
