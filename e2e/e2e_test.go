package e2e

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"python-index-proxy/config"
	"python-index-proxy/proxy"
	"strings"
	"testing"
	"time"
)

// TestRealPyPIIntegration tests the proxy with real PyPI packages
// This test can run in CI because it uses proper timeouts and error handling
func TestRealPyPIIntegration(t *testing.T) {
	// Create test configuration with real PyPI
	cfg := &config.Config{
		PublicPyPIURL:  "https://pypi.org/simple/",
		PrivatePyPIURL: "https://pypi.org/simple/", // Use same as public for this test
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

	// Test with a well-known, stable package that's unlikely to be removed
	packageName := "six" // A very stable package used by many others

	// Create request with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("/simple/%s/", packageName), http.NoBody)
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

	// Verify response contains expected content
	body := rr.Body.String()
	if body == "" {
		t.Error("Expected non-empty response body")
	}

	// Check that it contains the package name
	if !strings.Contains(body, packageName) {
		t.Errorf("Expected response to contain package name '%s'", packageName)
	}

	// Check content type
	contentType := rr.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("Expected HTML content type, got %s", contentType)
	}
}

// TestRealPyPIPackageDownload tests actual package file downloads
func TestRealPyPIPackageDownload(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		PublicPyPIURL:  "https://pypi.org/simple/",
		PrivatePyPIURL: "https://pypi.org/simple/", // Use same as public for this test
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

	// Test with a specific package file URL
	// Using a well-known package file that's unlikely to change
	packageFileURL := "/packages/source/s/six/six-1.16.0.tar.gz"

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", packageFileURL, http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	proxyInstance.HandleFile(rr, req)

	// Check response - this might fail if the specific file doesn't exist
	// but we should get a proper error response, not a 500
	if rr.Code == http.StatusInternalServerError {
		t.Errorf("Got internal server error, expected either 200 or 404")
	}

	// If we get a 404, that's acceptable for this test
	if rr.Code == http.StatusNotFound {
		t.Log("Package file not found (404) - this is acceptable for this test")
		return
	}

	// If we get a 200, check the content type
	if rr.Code == http.StatusOK {
		contentType := rr.Header().Get("Content-Type")
		if contentType != "application/x-gzip" && contentType != "application/octet-stream" {
			t.Errorf("Expected gzip or octet-stream content type, got %s", contentType)
		}

		// Verify we got some content
		body := rr.Body.String()
		if len(body) == 0 {
			t.Error("Expected non-empty response body for package file")
		}
	}
}

// TestRealPyPIErrorHandling tests error handling with real PyPI
func TestRealPyPIErrorHandling(t *testing.T) {
	cfg := &config.Config{
		PublicPyPIURL:  "https://pypi.org/simple/",
		PrivatePyPIURL: "https://pypi.org/simple/", // Use same as public for this test
		Port:           8080,
		CacheEnabled:   false,
		CacheSize:      100,
		CacheTTL:       1,
	}

	proxyInstance, err := proxy.NewProxy(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Test with a non-existent package
	packageName := "this-package-definitely-does-not-exist-12345"

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("/simple/%s/", packageName), http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	proxyInstance.HandlePackage(rr, req)

	// Should get a 404 for non-existent package
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 for non-existent package, got %d", rr.Code)
	}
}

// TestRealPyPIMixedWithLocal tests the proxy with both real PyPI and local packages
func TestRealPyPIMixedWithLocal(t *testing.T) {
	// Start local PyPI server
	localServer := NewLocalPyPIServer()
	defer localServer.Close()

	cfg := &config.Config{
		PublicPyPIURL:  "https://pypi.org/simple/",
		PrivatePyPIURL: localServer.URL(),
		Port:           8080,
		CacheEnabled:   false,
		CacheSize:      100,
		CacheTTL:       1,
	}

	proxyInstance, err := proxy.NewProxy(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Test 1: Local package (should come from local server)
	packageName := "privatepackage"

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("/simple/%s/", packageName), http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	proxyInstance.HandlePackage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 for local package, got %d", rr.Code)
	}

	sourceHeader := rr.Header().Get("X-PyPI-Source")
	if sourceHeader != localServer.URL() {
		t.Errorf("Expected source header '%s', got %s", localServer.URL(), sourceHeader)
	}

	// Test 2: Real PyPI package (should come from public PyPI)
	packageName = "six"

	req, err = http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("/simple/%s/", packageName), http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rr = httptest.NewRecorder()
	proxyInstance.HandlePackage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 for real package, got %d", rr.Code)
	}

	sourceHeader = rr.Header().Get("X-PyPI-Source")
	if sourceHeader != "https://pypi.org/simple/" {
		t.Errorf("Expected source header 'https://pypi.org/simple/', got %s", sourceHeader)
	}
}

// LocalPyPIServer represents a local PyPI server for testing.
type LocalPyPIServer struct {
	server   *httptest.Server
	packages map[string]PackageInfo
}

// PackageInfo contains information about a package.
type PackageInfo struct {
	Name     string
	Versions []string
	Files    []PackageFile
}

// PackageFile represents a package file.
type PackageFile struct {
	Filename string
	URL      string
	Size     int64
}

// NewLocalPyPIServer creates a new local PyPI server.
func NewLocalPyPIServer() *LocalPyPIServer {
	server := &LocalPyPIServer{
		packages: make(map[string]PackageInfo),
	}

	// Populate with test packages
	server.populateTestPackages()

	server.server = httptest.NewServer(http.HandlerFunc(server.handleRequest))
	return server
}

// populateTestPackages adds test packages to the local server.
func (s *LocalPyPIServer) populateTestPackages() {
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
}

// handleRequest handles HTTP requests to the local PyPI server.
func (s *LocalPyPIServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	path = strings.ReplaceAll(path, "//packages/", "/packages/")
	path = strings.ReplaceAll(path, "//", "/")

	if strings.HasPrefix(path, "/simple/") {
		s.handlePackageIndex(w, r)
		return
	}

	if strings.HasPrefix(path, "/packages/") {
		s.handleFileRequest(w, r)
		return
	}

	http.NotFound(w, r)
}

// handlePackageIndex handles package index requests.
func (s *LocalPyPIServer) handlePackageIndex(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	packageName := strings.TrimPrefix(strings.TrimSuffix(path, "/"), "/simple/")

	pkg, exists := s.packages[packageName]
	if !exists {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(s.generatePackageIndexHTML(pkg)))
}

// handleFileRequest handles package file requests.
func (s *LocalPyPIServer) handleFileRequest(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	filename := strings.TrimPrefix(path, "/packages/")

	// Find the package file
	for _, pkg := range s.packages {
		for _, file := range pkg.Files {
			if strings.HasSuffix(file.URL, filename) {
				w.Header().Set("Content-Type", "application/octet-stream")
				w.Header().Set("Content-Length", fmt.Sprintf("%d", file.Size))
				w.WriteHeader(http.StatusOK)
				// Write dummy content
				w.Write([]byte("dummy package content"))
				return
			}
		}
	}

	http.NotFound(w, r)
}

// generatePackageIndexHTML generates HTML for package index.
func (s *LocalPyPIServer) generatePackageIndexHTML(pkg PackageInfo) string {
	var links strings.Builder
	links.WriteString(fmt.Sprintf("<html><head><title>Links for %s</title></head><body><h1>Links for %s</h1>", pkg.Name, pkg.Name))

	for _, file := range pkg.Files {
		links.WriteString(fmt.Sprintf(`<a href=%q>%s</a><br/>`, file.URL, file.Filename))
	}

	links.WriteString("</body></html>")
	return links.String()
}

// URL returns the server URL.
func (s *LocalPyPIServer) URL() string {
	return s.server.URL
}

// Close closes the server.
func (s *LocalPyPIServer) Close() {
	s.server.Close()
}
