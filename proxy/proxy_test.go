package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"python-index-proxy/config"
	"python-index-proxy/pypi"
)

// MockPyPIClient is a mock implementation of the PyPI client for testing
type MockPyPIClient struct {
	publicCalls  map[string]int
	privateCalls map[string]int
	publicExists map[string]bool
	privateExists map[string]bool
	shouldError  bool
}

func NewMockPyPIClient() *MockPyPIClient {
	return &MockPyPIClient{
		publicCalls:  make(map[string]int),
		privateCalls: make(map[string]int),
		publicExists: make(map[string]bool),
		privateExists: make(map[string]bool),
	}
}

// Ensure MockPyPIClient implements PyPIClient interface
var _ pypi.PyPIClient = (*MockPyPIClient)(nil)

func (m *MockPyPIClient) PackageExists(ctx context.Context, baseURL, packageName string) (bool, error) {
	if m.shouldError {
		return false, fmt.Errorf("mock error")
	}

	// Track the call
	if strings.Contains(baseURL, "pypi.org") {
		m.publicCalls[packageName]++
		return m.publicExists[packageName], nil
	} else {
		m.privateCalls[packageName]++
		return m.privateExists[packageName], nil
	}
}

func (m *MockPyPIClient) GetPackagePage(ctx context.Context, baseURL, packageName string) ([]byte, error) {
	if m.shouldError {
		return nil, fmt.Errorf("mock error")
	}
	return []byte(fmt.Sprintf("<html><body>Package %s</body></html>", packageName)), nil
}

func (m *MockPyPIClient) GetPackageFile(ctx context.Context, fileURL string) ([]byte, error) {
	if m.shouldError {
		return nil, fmt.Errorf("mock error")
	}
	return []byte("mock file content"), nil
}

func (m *MockPyPIClient) ProxyFile(ctx context.Context, fileURL string, w http.ResponseWriter) error {
	if m.shouldError {
		return fmt.Errorf("mock error")
	}
	w.Write([]byte("mock file content"))
	return nil
}

// TestProxyCachingWithCacheEnabled tests that caching reduces network calls
func TestProxyCachingWithCacheEnabled(t *testing.T) {
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
	proxyInstance, err := NewProxy(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Replace the client with our mock
	mockClient := NewMockPyPIClient()
	proxyInstance.client = mockClient

	// Set up mock responses
	mockClient.publicExists["test-package"] = true
	mockClient.privateExists["test-package"] = false

	// First request - should make network calls
	publicExists, privateExists, err := proxyInstance.CheckPackageExists(context.Background(), "test-package")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !publicExists {
		t.Error("Expected public package to exist")
	}
	if privateExists {
		t.Error("Expected private package to not exist")
	}

	// Verify network calls were made
	if mockClient.publicCalls["test-package"] != 1 {
		t.Errorf("Expected 1 public call, got %d", mockClient.publicCalls["test-package"])
	}
	if mockClient.privateCalls["test-package"] != 1 {
		t.Errorf("Expected 1 private call, got %d", mockClient.privateCalls["test-package"])
	}

	// Second request for the same package - should use cache, no network calls
	publicExists2, privateExists2, err := proxyInstance.CheckPackageExists(context.Background(), "test-package")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !publicExists2 {
		t.Error("Expected public package to exist")
	}
	if privateExists2 {
		t.Error("Expected private package to not exist")
	}

	// Verify no additional network calls were made
	if mockClient.publicCalls["test-package"] != 1 {
		t.Errorf("Expected 1 public call total, got %d", mockClient.publicCalls["test-package"])
	}
	if mockClient.privateCalls["test-package"] != 1 {
		t.Errorf("Expected 1 private call total, got %d", mockClient.privateCalls["test-package"])
	}

	// Check cache stats
	publicLen, privateLen, publicPageLen, privatePageLen := proxyInstance.GetCache().GetStats()
	if publicLen != 1 {
		t.Errorf("Expected 1 public package in cache, got %d", publicLen)
	}
	if privateLen != 1 {
		t.Errorf("Expected 1 private package in cache, got %d", privateLen)
	}
	if publicPageLen != 0 {
		t.Errorf("Expected 0 public pages in cache, got %d", publicPageLen)
	}
	if privatePageLen != 0 {
		t.Errorf("Expected 0 private pages in cache, got %d", privatePageLen)
	}
}

// TestProxyCachingWithCacheDisabled tests that no caching occurs when disabled
func TestProxyCachingWithCacheDisabled(t *testing.T) {
	// Create test configuration with cache disabled
	cfg := &config.Config{
		PublicPyPIURL:  "https://pypi.org/simple/",
		PrivatePyPIURL: "https://console.redhat.com/api/pulp-content/public-calunga/mypypi/simple",
		Port:           8080,
		CacheEnabled:   false,
		CacheSize:      100,
		CacheTTL:       1,
	}

	// Create proxy instance
	proxyInstance, err := NewProxy(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Replace the client with our mock
	mockClient := NewMockPyPIClient()
	proxyInstance.client = mockClient

	// Set up mock responses
	mockClient.publicExists["test-package"] = true
	mockClient.privateExists["test-package"] = false

	// First request
	_, _, err = proxyInstance.CheckPackageExists(context.Background(), "test-package")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Second request for the same package - should make network calls again
	_, _, err = proxyInstance.CheckPackageExists(context.Background(), "test-package")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify network calls were made twice (no caching)
	if mockClient.publicCalls["test-package"] != 2 {
		t.Errorf("Expected 2 public calls, got %d", mockClient.publicCalls["test-package"])
	}
	if mockClient.privateCalls["test-package"] != 2 {
		t.Errorf("Expected 2 private calls, got %d", mockClient.privateCalls["test-package"])
	}

	// Check cache stats - should be 0 when disabled
	publicLen, privateLen, publicPageLen, privatePageLen := proxyInstance.GetCache().GetStats()
	if publicLen != 0 {
		t.Errorf("Expected 0 public packages in cache, got %d", publicLen)
	}
	if privateLen != 0 {
		t.Errorf("Expected 0 private packages in cache, got %d", privateLen)
	}
	if publicPageLen != 0 {
		t.Errorf("Expected 0 public pages in cache, got %d", publicPageLen)
	}
	if privatePageLen != 0 {
		t.Errorf("Expected 0 private pages in cache, got %d", privatePageLen)
	}
}

// TestProxyCachingPartialCache tests that partial cache hits work correctly
func TestProxyCachingPartialCache(t *testing.T) {
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
	proxyInstance, err := NewProxy(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Replace the client with our mock
	mockClient := NewMockPyPIClient()
	proxyInstance.client = mockClient

	// Set up mock responses
	mockClient.publicExists["test-package"] = true
	mockClient.privateExists["test-package"] = false

	// Manually set cache for public package only
	proxyInstance.GetCache().SetPublicPackage("test-package", true)

	// Request - should use cache for public, make network call for private
	publicExists, privateExists, err := proxyInstance.CheckPackageExists(context.Background(), "test-package")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !publicExists {
		t.Error("Expected public package to exist")
	}
	if privateExists {
		t.Error("Expected private package to not exist")
	}

	// Verify only private call was made (public was cached)
	if mockClient.publicCalls["test-package"] != 0 {
		t.Errorf("Expected 0 public calls (cached), got %d", mockClient.publicCalls["test-package"])
	}
	if mockClient.privateCalls["test-package"] != 1 {
		t.Errorf("Expected 1 private call, got %d", mockClient.privateCalls["test-package"])
	}
}

// TestProxyCachingExpiration tests that cache expiration works correctly
func TestProxyCachingExpiration(t *testing.T) {
	// Create test configuration with very short TTL
	cfg := &config.Config{
		PublicPyPIURL:  "https://pypi.org/simple/",
		PrivatePyPIURL: "https://console.redhat.com/api/pulp-content/public-calunga/mypypi/simple",
		Port:           8080,
		CacheEnabled:   true,
		CacheSize:      100,
		CacheTTL:       0, // 0 hours TTL for testing
	}

	// Create proxy instance
	proxyInstance, err := NewProxy(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Replace the client with our mock
	mockClient := NewMockPyPIClient()
	proxyInstance.client = mockClient

	// Set up mock responses
	mockClient.publicExists["test-package"] = true
	mockClient.privateExists["test-package"] = false

	// First request
	_, _, err = proxyInstance.CheckPackageExists(context.Background(), "test-package")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Wait a bit to ensure expiration
	time.Sleep(10 * time.Millisecond)

	// Second request - should make network calls again due to expiration
	_, _, err = proxyInstance.CheckPackageExists(context.Background(), "test-package")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify network calls were made twice (cache expired)
	if mockClient.publicCalls["test-package"] != 2 {
		t.Errorf("Expected 2 public calls (cache expired), got %d", mockClient.publicCalls["test-package"])
	}
	if mockClient.privateCalls["test-package"] != 2 {
		t.Errorf("Expected 2 private calls (cache expired), got %d", mockClient.privateCalls["test-package"])
	}
}

// TestProxyCachingHTTPRequests tests that HTTP requests use caching correctly
func TestProxyCachingHTTPRequests(t *testing.T) {
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
	proxyInstance, err := NewProxy(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Replace the client with our mock
	mockClient := NewMockPyPIClient()
	proxyInstance.client = mockClient

	// Set up mock responses
	mockClient.publicExists["test-package"] = true
	mockClient.privateExists["test-package"] = false

	// First HTTP request
	req1, err := http.NewRequest("GET", "/simple/test-package/", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	rr1 := httptest.NewRecorder()
	proxyInstance.HandlePackage(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr1.Code)
	}

	// Second HTTP request for the same package
	req2, err := http.NewRequest("GET", "/simple/test-package/", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	rr2 := httptest.NewRecorder()
	proxyInstance.HandlePackage(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr2.Code)
	}

	// Verify network calls were made only once per index (cached on second request)
	if mockClient.publicCalls["test-package"] != 1 {
		t.Errorf("Expected 1 public call (cached on second request), got %d", mockClient.publicCalls["test-package"])
	}
	if mockClient.privateCalls["test-package"] != 1 {
		t.Errorf("Expected 1 private call (cached on second request), got %d", mockClient.privateCalls["test-package"])
	}

	// Verify responses are identical
	if rr1.Body.String() != rr2.Body.String() {
		t.Error("Expected cached responses to be identical")
	}
} 