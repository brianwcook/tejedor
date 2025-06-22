package pypi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
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

	baseURL := makeBaseURL(server.URL)
	// Test package exists
	exists, err := client.PackageExists(ctx, baseURL, "test-package")
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

	baseURL := makeBaseURL(server.URL)
	// Test package doesn't exist
	exists, err := client.PackageExists(ctx, baseURL, "non-existent-package")
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
			if _, err := w.Write([]byte(expectedContent)); err != nil {
				t.Errorf("Error writing response: %v", err)
			}
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient()
	ctx := context.Background()

	baseURL := makeBaseURL(server.URL)
	// Test getting package page
	content, err := client.GetPackagePage(ctx, baseURL, "test-package")
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

	baseURL := makeBaseURL(server.URL)
	// Test getting non-existent package page
	_, err := client.GetPackagePage(ctx, baseURL, "non-existent-package")
	if err == nil {
		t.Error("Expected error for non-existent package")
	}
}

func TestGetPackageFile(t *testing.T) {
	expectedContent := "package file content"

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		if _, err := w.Write([]byte(expectedContent)); err != nil {
			t.Errorf("Error writing response: %v", err)
		}
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
		if _, err := w.Write([]byte(expectedContent)); err != nil {
			t.Errorf("Error writing response: %v", err)
		}
	}))
	defer server.Close()

	client := NewClient()
	ctx := context.Background()

	// Create response recorder
	rr := httptest.NewRecorder()

	// Test proxying file
	err := client.ProxyFile(ctx, server.URL, rr, "GET")
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
	err := client.ProxyFile(ctx, server.URL, rr, "GET")
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

	baseURL := makeBaseURL(server.URL)
	// Test package name normalization (underscores to hyphens)
	exists, err := client.PackageExists(ctx, baseURL, "test_package")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if !exists {
		t.Error("Expected package to exist")
	}
}

func TestPackageExistsWithError(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{},
	}

	// Test with invalid URL to trigger error
	exists, err := client.PackageExists(context.Background(), "invalid://url", "test-package")
	if err == nil {
		t.Error("Expected error for invalid URL, got nil")
	}
	if exists {
		t.Error("Expected false for invalid URL")
	}
}

func TestGetPackagePageWithError(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{},
	}

	// Test with invalid URL to trigger error
	content, err := client.GetPackagePage(context.Background(), "invalid://url", "test-package")
	if err == nil {
		t.Error("Expected error for invalid URL, got nil")
	}
	if content != nil {
		t.Error("Expected nil content for invalid URL")
	}
}

func TestGetPackageFileWithError(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{},
	}

	// Test with invalid URL to trigger error
	content, err := client.GetPackageFile(context.Background(), "invalid://url")
	if err == nil {
		t.Error("Expected error for invalid URL, got nil")
	}
	if content != nil {
		t.Error("Expected nil content for invalid URL")
	}
}

func TestProxyFileWithError(t *testing.T) {
	client := &Client{
		httpClient: &http.Client{},
	}

	// Test with invalid URL to trigger error
	req, err := http.NewRequest("GET", "/test", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	rr := httptest.NewRecorder()

	err = client.ProxyFile(context.Background(), "invalid://url", rr, req.Method)
	if err == nil {
		t.Error("Expected error for invalid URL, got nil")
	}
}

func makeBaseURL(serverURL string) string {
	u, _ := url.Parse(serverURL)
	u.Path = "/"
	return u.String()
}
