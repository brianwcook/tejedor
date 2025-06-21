package pypi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient()
	if client == nil {
		t.Fatal("Expected client to be created")
	}
	if client.httpClient == nil {
		t.Fatal("Expected HTTP client to be initialized")
	}
}

func TestPackageExists(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient()
	ctx := context.Background()

	// Test package exists
	exists, err := client.PackageExists(ctx, server.URL, "test-package")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if !exists {
		t.Error("Expected package to exist")
	}
}

func TestPackageNotExists(t *testing.T) {
	// Create test server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient()
	ctx := context.Background()

	// Test package doesn't exist
	exists, err := client.PackageExists(ctx, server.URL, "non-existent-package")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if exists {
		t.Error("Expected package to not exist")
	}
}

func TestGetPackagePage(t *testing.T) {
	expectedContent := "<html><body>Package page</body></html>"
	
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(expectedContent))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient()
	ctx := context.Background()

	// Test getting package page
	content, err := client.GetPackagePage(ctx, server.URL, "test-package")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	
	if string(content) != expectedContent {
		t.Errorf("Expected content %s, got %s", expectedContent, string(content))
	}
}

func TestGetPackagePageNotFound(t *testing.T) {
	// Create test server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient()
	ctx := context.Background()

	// Test getting non-existent package page
	_, err := client.GetPackagePage(ctx, server.URL, "non-existent-package")
	if err == nil {
		t.Error("Expected error for non-existent package")
	}
}

func TestGetPackageFile(t *testing.T) {
	expectedContent := "package file content"
	
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte(expectedContent))
	}))
	defer server.Close()

	client := NewClient()
	ctx := context.Background()

	// Test getting package file
	content, err := client.GetPackageFile(ctx, server.URL)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	
	if string(content) != expectedContent {
		t.Errorf("Expected content %s, got %s", expectedContent, string(content))
	}
}

func TestGetPackageFileNotFound(t *testing.T) {
	// Create test server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient()
	ctx := context.Background()

	// Test getting non-existent file
	_, err := client.GetPackageFile(ctx, server.URL)
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestProxyFile(t *testing.T) {
	expectedContent := "proxied file content"
	expectedHeader := "test-header-value"
	
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("X-Test-Header", expectedHeader)
		w.Write([]byte(expectedContent))
	}))
	defer server.Close()

	client := NewClient()
	ctx := context.Background()

	// Create response recorder
	rr := httptest.NewRecorder()

	// Test proxying file
	err := client.ProxyFile(ctx, server.URL, rr)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	
	// Check response
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
	
	if rr.Body.String() != expectedContent {
		t.Errorf("Expected content %s, got %s", expectedContent, rr.Body.String())
	}
	
	if rr.Header().Get("X-Test-Header") != expectedHeader {
		t.Errorf("Expected header %s, got %s", expectedHeader, rr.Header().Get("X-Test-Header"))
	}
}

func TestProxyFileNotFound(t *testing.T) {
	// Create test server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient()
	ctx := context.Background()

	// Create response recorder
	rr := httptest.NewRecorder()

	// Test proxying non-existent file
	err := client.ProxyFile(ctx, server.URL, rr)
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestPackageNameNormalization(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log the requested path for debugging
		t.Logf("Requested path: %s", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient()
	ctx := context.Background()

	// Test package name normalization (underscores to hyphens)
	exists, err := client.PackageExists(ctx, server.URL, "test_package")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if !exists {
		t.Error("Expected package to exist")
	}
} 