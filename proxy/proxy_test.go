package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"python-index-proxy/config"
	"python-index-proxy/pypi"
	"strings"
	"testing"
	"time"
)

// MockPyPIClient is a mock implementation of the PyPI client for testing.
type MockPyPIClient struct {
	publicCalls   map[string]int
	privateCalls  map[string]int
	publicExists  map[string]bool
	privateExists map[string]bool
	shouldError   bool
}

func NewMockPyPIClient() *MockPyPIClient {
	return &MockPyPIClient{
		publicCalls:   make(map[string]int),
		privateCalls:  make(map[string]int),
		publicExists:  make(map[string]bool),
		privateExists: make(map[string]bool),
	}
}

// Ensure MockPyPIClient implements PyPIClient interface.
var _ pypi.PyPIClient = (*MockPyPIClient)(nil)

func (m *MockPyPIClient) PackageExists(_ context.Context, baseURL, packageName string) (bool, error) {
	if m.shouldError {
		return false, fmt.Errorf("mock error")
	}

	// Track the call
	if strings.Contains(baseURL, "pypi.org") {
		m.publicCalls[packageName]++
		return m.publicExists[packageName], nil
	}
	m.privateCalls[packageName]++
	return m.privateExists[packageName], nil
}

func (m *MockPyPIClient) GetPackagePage(_ context.Context, _, packageName string) ([]byte, error) {
	if m.shouldError {
		return nil, fmt.Errorf("mock error")
	}
	return []byte(fmt.Sprintf("<html><body>Package %s</body></html>", packageName)), nil
}

func (m *MockPyPIClient) GetPackageFile(_ context.Context, _ string) ([]byte, error) {
	if m.shouldError {
		return nil, fmt.Errorf("mock error")
	}
	return []byte("mock file content"), nil
}

func (m *MockPyPIClient) ProxyFile(_ context.Context, _ string, w http.ResponseWriter, _ string) error {
	if m.shouldError {
		return fmt.Errorf("mock error")
	}
	if _, err := w.Write([]byte("mock file content")); err != nil {
		return fmt.Errorf("mock write error: %w", err)
	}
	return nil
}

// TestProxyCachingWithCacheEnabled tests that caching reduces network calls.
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
	mockClient.publicExists["test"] = true
	mockClient.privateExists["test"] = false

	// First request - should make network calls
	publicExists, privateExists, err := proxyInstance.CheckPackageExists(context.Background(), "test")
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
	if mockClient.publicCalls["test"] != 1 {
		t.Errorf("Expected 1 public call, got %d", mockClient.publicCalls["test"])
	}
	if mockClient.privateCalls["test"] != 1 {
		t.Errorf("Expected 1 private call, got %d", mockClient.privateCalls["test"])
	}

	// Second request for the same package - should use cache, no network calls
	publicExists2, privateExists2, err := proxyInstance.CheckPackageExists(context.Background(), "test")
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
	if mockClient.publicCalls["test"] != 1 {
		t.Errorf("Expected 1 public call total, got %d", mockClient.publicCalls["test"])
	}
	if mockClient.privateCalls["test"] != 1 {
		t.Errorf("Expected 1 private call total, got %d", mockClient.privateCalls["test"])
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

// TestProxyCachingWithCacheDisabled tests that no caching occurs when disabled.
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
	mockClient.publicExists["test"] = true
	mockClient.privateExists["test"] = false

	// First request
	_, _, err = proxyInstance.CheckPackageExists(context.Background(), "test")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Second request for the same package - should make network calls again
	_, _, err = proxyInstance.CheckPackageExists(context.Background(), "test")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify network calls were made twice (no caching)
	if mockClient.publicCalls["test"] != 2 {
		t.Errorf("Expected 2 public calls, got %d", mockClient.publicCalls["test"])
	}
	if mockClient.privateCalls["test"] != 2 {
		t.Errorf("Expected 2 private calls, got %d", mockClient.privateCalls["test"])
	}

	// Check cache stats - should be empty since cache is disabled
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

// TestProxyCachingPartialCache tests that partial cache hits work correctly.
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
	mockClient.publicExists["test"] = true
	mockClient.privateExists["test"] = false

	// First request - should make network calls
	_, _, err = proxyInstance.CheckPackageExists(context.Background(), "test")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify network calls were made
	if mockClient.publicCalls["test"] != 1 {
		t.Errorf("Expected 1 public call, got %d", mockClient.publicCalls["test"])
	}
	if mockClient.privateCalls["test"] != 1 {
		t.Errorf("Expected 1 private call, got %d", mockClient.privateCalls["test"])
	}

	// Clear private cache but keep public cache
	proxyInstance.GetCache().ClearPrivateOnly()

	// Second request - should use public cache, make private network call
	_, _, err = proxyInstance.CheckPackageExists(context.Background(), "test")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify only private network call was made
	if mockClient.publicCalls["test"] != 1 {
		t.Errorf("Expected 1 public call total, got %d", mockClient.publicCalls["test"])
	}
	if mockClient.privateCalls["test"] != 2 {
		t.Errorf("Expected 2 private calls total, got %d", mockClient.privateCalls["test"])
	}
}

// TestProxyCachingExpiration tests that cache expiration works correctly.
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
	mockClient.publicExists["test"] = true
	mockClient.privateExists["test"] = false

	// First request - should make network calls
	_, _, err = proxyInstance.CheckPackageExists(context.Background(), "test")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify network calls were made
	if mockClient.publicCalls["test"] != 1 {
		t.Errorf("Expected 1 public call, got %d", mockClient.publicCalls["test"])
	}
	if mockClient.privateCalls["test"] != 1 {
		t.Errorf("Expected 1 private call, got %d", mockClient.privateCalls["test"])
	}

	// Wait a bit to ensure expiration
	time.Sleep(10 * time.Millisecond)

	// Second request - should make network calls again due to expiration
	_, _, err = proxyInstance.CheckPackageExists(context.Background(), "test")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify network calls were made again
	if mockClient.publicCalls["test"] != 2 {
		t.Errorf("Expected 2 public calls total, got %d", mockClient.publicCalls["test"])
	}
	if mockClient.privateCalls["test"] != 2 {
		t.Errorf("Expected 2 private calls total, got %d", mockClient.privateCalls["test"])
	}
}

// TestProxyCachingHTTPRequests tests that HTTP requests use caching correctly.
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
	mockClient.publicExists["test"] = true
	mockClient.privateExists["test"] = false

	// First HTTP request - should make network calls
	req1, err := http.NewRequest("GET", "/simple/test/", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	rr1 := httptest.NewRecorder()
	proxyInstance.HandlePackage(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr1.Code)
	}

	// Verify network calls were made
	if mockClient.publicCalls["test"] != 1 {
		t.Errorf("Expected 1 public call, got %d", mockClient.publicCalls["test"])
	}
	if mockClient.privateCalls["test"] != 1 {
		t.Errorf("Expected 1 private call, got %d", mockClient.privateCalls["test"])
	}

	// Second HTTP request - should use cache, no network calls
	req2, err := http.NewRequest("GET", "/simple/test/", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	rr2 := httptest.NewRecorder()
	proxyInstance.HandlePackage(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr2.Code)
	}

	// Verify no additional network calls were made
	if mockClient.publicCalls["test"] != 1 {
		t.Errorf("Expected 1 public call total, got %d", mockClient.publicCalls["test"])
	}
	if mockClient.privateCalls["test"] != 1 {
		t.Errorf("Expected 1 private call total, got %d", mockClient.privateCalls["test"])
	}

	// Both responses should be identical
	if rr1.Body.String() != rr2.Body.String() {
		t.Error("Cached response should be identical to first response")
	}
}

// TestProxyHTTPHandlers tests the HTTP handlers directly.
func TestProxyHTTPHandlers(t *testing.T) {
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
	proxyInstance, err := NewProxy(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Replace the client with our mock
	mockClient := NewMockPyPIClient()
	proxyInstance.client = mockClient

	// Set up mock responses
	mockClient.publicExists["test"] = true
	mockClient.privateExists["test"] = false

	// Test index page
	req, err := http.NewRequest("GET", "/", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	rr := httptest.NewRecorder()
	proxyInstance.HandleIndex(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// Test health endpoint
	req, err = http.NewRequest("GET", "/health", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	rr = httptest.NewRecorder()
	proxyInstance.HandleHealth(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// Test package request
	req, err = http.NewRequest("GET", "/packages/source/p/test/test-1.0.0.tar.gz", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	rr = httptest.NewRecorder()
	proxyInstance.HandleFile(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

// TestProxyHTTPHandlersErrorCases tests error cases in HTTP handlers.
func TestProxyHTTPHandlersErrorCases(t *testing.T) {
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
	proxyInstance, err := NewProxy(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Replace the client with our mock
	mockClient := NewMockPyPIClient()
	proxyInstance.client = mockClient

	// Test invalid package path
	req, err := http.NewRequest("GET", "/packages/", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	rr := httptest.NewRecorder()
	proxyInstance.HandleFile(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}

	// Test invalid file path
	req, err = http.NewRequest("GET", "/packages/", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	rr = httptest.NewRecorder()
	proxyInstance.HandleFile(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}

	// Test file request with mock error
	mockClient.shouldError = true
	req, err = http.NewRequest("GET", "/packages/source/p/test/test-1.0.0.tar.gz", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	rr = httptest.NewRecorder()
	proxyInstance.HandleFile(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rr.Code)
	}
}

// TestProxyErrorCases tests various error cases in the proxy.
func TestProxyErrorCases(t *testing.T) {
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
	proxyInstance, err := NewProxy(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Replace the client with our mock
	mockClient := NewMockPyPIClient()
	proxyInstance.client = mockClient

	// Test package request with mock error
	mockClient.shouldError = true
	req, err := http.NewRequest("GET", "/packages/source/p/test-package/test-package-1.0.0.tar.gz", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	rr := httptest.NewRecorder()
	proxyInstance.HandleFile(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rr.Code)
	}

	// Test package request with mock error
	req, err = http.NewRequest("GET", "/packages/source/p/test-package/test-package-1.0.0.tar.gz", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	rr = httptest.NewRecorder()
	proxyInstance.HandleFile(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rr.Code)
	}

	// Test package request with mock error
	req, err = http.NewRequest("GET", "/packages/source/p/test-package/test-package-1.0.0.tar.gz", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	rr = httptest.NewRecorder()
	proxyInstance.HandleFile(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rr.Code)
	}
}

// TestProxyNewProxyError tests NewProxy with invalid cache configuration.
func TestProxyNewProxyError(t *testing.T) {
	// Create test configuration with invalid cache size (negative)
	cfg := &config.Config{
		PublicPyPIURL:  "https://pypi.org/simple/",
		PrivatePyPIURL: "https://console.redhat.com/api/pulp-content/public-calunga/mypypi/simple",
		Port:           8080,
		CacheEnabled:   true,
		CacheSize:      -1, // Invalid cache size
		CacheTTL:       1,
	}

	// Create proxy instance - should fail
	_, err := NewProxy(cfg)
	if err == nil {
		t.Error("Expected error for invalid cache configuration")
	}
}

// TestProxyHandlePackageErrorScenarios tests error scenarios in HandlePackage.
func TestProxyHandlePackageErrorScenarios(t *testing.T) {
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
	proxyInstance, err := NewProxy(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Replace the client with our mock
	mockClient := NewMockPyPIClient()
	proxyInstance.client = mockClient

	// Test package request with mock error in GetPackagePage
	mockClient.shouldError = true
	req, err := http.NewRequest("GET", "/simple/test-package/", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	rr := httptest.NewRecorder()
	proxyInstance.HandlePackage(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rr.Code)
	}

	// Test package request with write error
	mockClient.shouldError = false
	mockClient.publicExists["test-package"] = true
	mockClient.privateExists["test-package"] = false

	req, err = http.NewRequest("GET", "/simple/test-package/", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	rr = httptest.NewRecorder()

	// Create a response writer that fails on write
	failingWriter := &failingResponseWriter{rr}
	proxyInstance.HandlePackage(failingWriter, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rr.Code)
	}
}

// TestProxyHandleFileErrorScenarios tests error scenarios in HandleFile.
func TestProxyHandleFileErrorScenarios(t *testing.T) {
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
	proxyInstance, err := NewProxy(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Replace the client with our mock
	mockClient := NewMockPyPIClient()
	proxyInstance.client = mockClient

	// Test file request with empty package name
	req, err := http.NewRequest("GET", "/packages/source/p//test-package-1.0.0.tar.gz", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	rr := httptest.NewRecorder()
	proxyInstance.HandleFile(rr, req)

	// This should return 404 because the package doesn't exist, not 400
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", rr.Code)
	}

	// Test file request with invalid package name extraction
	req, err = http.NewRequest("GET", "/packages/source/p/test-package/.tar.gz", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	rr = httptest.NewRecorder()
	proxyInstance.HandleFile(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

// TestProxyHandleIndexError tests error scenarios in HandleIndex.
func TestProxyHandleIndexError(t *testing.T) {
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
	proxyInstance, err := NewProxy(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Test index page with write error
	req, err := http.NewRequest("GET", "/", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	rr := httptest.NewRecorder()

	// Create a response writer that fails on write
	failingWriter := &failingResponseWriter{rr}
	proxyInstance.HandleIndex(failingWriter, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rr.Code)
	}
}

// TestProxyHandleHealthError tests error scenarios in HandleHealth.
func TestProxyHandleHealthError(t *testing.T) {
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
	proxyInstance, err := NewProxy(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Test health endpoint with write error
	req, err := http.NewRequest("GET", "/health", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	rr := httptest.NewRecorder()

	// Create a response writer that fails on write
	failingWriter := &failingResponseWriter{rr}
	proxyInstance.HandleHealth(failingWriter, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rr.Code)
	}
}

// TestProxyDetermineSourceError tests error scenarios in determineSource.
func TestProxyDetermineSourceError(t *testing.T) {
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
	proxyInstance, err := NewProxy(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Replace the client with our mock
	mockClient := NewMockPyPIClient()
	proxyInstance.client = mockClient

	// Test determineSource with package that doesn't exist
	sourceIndex, baseURL, packagePage, exists, err := proxyInstance.determineSource(context.Background(), "non-existent-package", false, false)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if exists {
		t.Error("Expected package to not exist")
	}
	if sourceIndex != "" || baseURL != "" || packagePage != nil {
		t.Error("Expected empty values for non-existent package")
	}

	// Test determineSource with mock error
	mockClient.shouldError = true
	mockClient.publicExists["test-package"] = true
	mockClient.privateExists["test-package"] = false

	_, _, _, _, _ = proxyInstance.determineSource(context.Background(), "test-package", true, false)
}

// TestExtractPackageNameFromFileName tests the extractPackageNameFromFileName function.
func TestExtractPackageNameFromFileName(t *testing.T) {
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
	proxyInstance, err := NewProxy(cfg)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Test cases
	testCases := []struct {
		fileName     string
		expectedName string
	}{
		{"pydantic-2.5.0-py3-none-any.whl", "pydantic"},
		{"requests-2.31.0.tar.gz", "requests"},
		{"flask-3.0.0.zip", "flask"},
		{"simple-package-1.0.0-py3-none-any.whl", "simple"},
		{"complex_package_name-1.0.0.tar.gz", "complex_package_name"},
	}

	for _, tc := range testCases {
		t.Run(tc.fileName, func(t *testing.T) {
			result := proxyInstance.extractPackageNameFromFileName(tc.fileName)
			if result != tc.expectedName {
				t.Errorf("Expected %s, got %s", tc.expectedName, result)
			}
		})
	}
}

// failingResponseWriter is a response writer that fails on write for testing error scenarios.
type failingResponseWriter struct {
	*httptest.ResponseRecorder
}

func (f *failingResponseWriter) Write(_ []byte) (int, error) {
	return 0, fmt.Errorf("mock write error")
}
