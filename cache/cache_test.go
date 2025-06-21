package cache

import (
	"testing"
	"time"
)

func TestNewCache(t *testing.T) {
	// Test creating cache with enabled
	cache, err := NewCache(100, 1, true)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if !cache.IsEnabled() {
		t.Error("Expected cache to be enabled")
	}

	// Test creating cache with disabled
	cache, err = NewCache(100, 1, false)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if cache.IsEnabled() {
		t.Error("Expected cache to be disabled")
	}
}

func TestCacheOperations(t *testing.T) {
	cache, err := NewCache(10, 1, true)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Test setting and getting public package
	cache.SetPublicPackage("test-package", true)
	info, found := cache.GetPublicPackage("test-package")
	if !found {
		t.Error("Expected to find package in cache")
	}
	if !info.Exists {
		t.Error("Expected package to exist")
	}

	// Test setting and getting private package
	cache.SetPrivatePackage("test-package", false)
	info, found = cache.GetPrivatePackage("test-package")
	if !found {
		t.Error("Expected to find package in cache")
	}
	if info.Exists {
		t.Error("Expected package to not exist")
	}

	// Test getting non-existent package
	_, found = cache.GetPublicPackage("non-existent")
	if found {
		t.Error("Expected not to find package in cache")
	}
}

func TestCacheExpiration(t *testing.T) {
	// Create cache with very short TTL for testing
	cache, err := NewCache(10, 0, true) // 0 hours TTL
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Set a package
	cache.SetPublicPackage("test-package", true)

	// Wait a bit to ensure expiration
	time.Sleep(10 * time.Millisecond)

	// Try to get the package - should not be found due to expiration
	_, found := cache.GetPublicPackage("test-package")
	if found {
		t.Error("Expected package to be expired and not found")
	}
}

func TestCacheDisabled(t *testing.T) {
	cache, err := NewCache(10, 1, false)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Try to set packages - should not error but not actually cache
	cache.SetPublicPackage("test-package", true)
	cache.SetPrivatePackage("test-package", true)

	// Try to get packages - should not be found
	_, found := cache.GetPublicPackage("test-package")
	if found {
		t.Error("Expected not to find package when cache is disabled")
	}

	_, found = cache.GetPrivatePackage("test-package")
	if found {
		t.Error("Expected not to find package when cache is disabled")
	}
}

func TestCacheStats(t *testing.T) {
	cache, err := NewCache(10, 1, true)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Initially should be empty
	publicLen, privateLen, publicPageLen, privatePageLen := cache.GetStats()
	if publicLen != 0 || privateLen != 0 || publicPageLen != 0 || privatePageLen != 0 {
		t.Errorf("Expected empty cache, got public: %d, private: %d, publicPage: %d, privatePage: %d", publicLen, privateLen, publicPageLen, privatePageLen)
	}

	// Add some packages
	cache.SetPublicPackage("package1", true)
	cache.SetPublicPackage("package2", false)
	cache.SetPrivatePackage("package1", true)
	cache.SetPublicPackagePage("package1", []byte("<html>test</html>"))
	cache.SetPrivatePackagePage("package1", []byte("<html>test</html>"))

	// Check stats
	publicLen, privateLen, publicPageLen, privatePageLen = cache.GetStats()
	if publicLen != 2 {
		t.Errorf("Expected 2 public packages, got %d", publicLen)
	}
	if privateLen != 1 {
		t.Errorf("Expected 1 private package, got %d", privateLen)
	}
	if publicPageLen != 1 {
		t.Errorf("Expected 1 public page, got %d", publicPageLen)
	}
	if privatePageLen != 1 {
		t.Errorf("Expected 1 private page, got %d", privatePageLen)
	}
}

func TestCacheClear(t *testing.T) {
	cache, err := NewCache(10, 1, true)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Add some packages and pages
	cache.SetPublicPackage("package1", true)
	cache.SetPrivatePackage("package1", true)
	cache.SetPublicPackagePage("package1", []byte("<html>test</html>"))
	cache.SetPrivatePackagePage("package1", []byte("<html>test</html>"))

	// Clear cache
	cache.Clear()

	// Check that packages are no longer found
	_, found := cache.GetPublicPackage("package1")
	if found {
		t.Error("Expected package to be cleared from public cache")
	}

	_, found = cache.GetPrivatePackage("package1")
	if found {
		t.Error("Expected package to be cleared from private cache")
	}

	_, found = cache.GetPublicPackagePage("package1")
	if found {
		t.Error("Expected package page to be cleared from public cache")
	}

	_, found = cache.GetPrivatePackagePage("package1")
	if found {
		t.Error("Expected package page to be cleared from private cache")
	}

	// Check stats
	publicLen, privateLen, publicPageLen, privatePageLen := cache.GetStats()
	if publicLen != 0 || privateLen != 0 || publicPageLen != 0 || privatePageLen != 0 {
		t.Errorf("Expected empty cache after clear, got public: %d, private: %d, publicPage: %d, privatePage: %d", publicLen, privateLen, publicPageLen, privatePageLen)
	}
}

// TestPackagePageCaching tests the new HTML page caching functionality
func TestPackagePageCaching(t *testing.T) {
	cache, err := NewCache(10, 1, true)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Test setting and getting public package page
	htmlContent := []byte("<html><body>Package test-package</body></html>")
	cache.SetPublicPackagePage("test-package", htmlContent)
	info, found := cache.GetPublicPackagePage("test-package")
	if !found {
		t.Error("Expected to find package page in cache")
	}
	if string(info.HTML) != string(htmlContent) {
		t.Error("Expected HTML content to match")
	}

	// Test setting and getting private package page
	cache.SetPrivatePackagePage("test-package", htmlContent)
	info, found = cache.GetPrivatePackagePage("test-package")
	if !found {
		t.Error("Expected to find package page in cache")
	}
	if string(info.HTML) != string(htmlContent) {
		t.Error("Expected HTML content to match")
	}

	// Test getting non-existent package page
	_, found = cache.GetPublicPackagePage("non-existent")
	if found {
		t.Error("Expected not to find package page in cache")
	}
}

func TestPackagePageCachingExpiration(t *testing.T) {
	// Create cache with very short TTL for testing
	cache, err := NewCache(10, 0, true) // 0 hours TTL
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Set a package page
	htmlContent := []byte("<html><body>Package test-package</body></html>")
	cache.SetPublicPackagePage("test-package", htmlContent)

	// Wait a bit to ensure expiration
	time.Sleep(10 * time.Millisecond)

	// Try to get the package page - should not be found due to expiration
	_, found := cache.GetPublicPackagePage("test-package")
	if found {
		t.Error("Expected package page to be expired and not found")
	}
}

func TestPackagePageCachingDisabled(t *testing.T) {
	cache, err := NewCache(10, 1, false)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Try to set package pages - should not error but not actually cache
	htmlContent := []byte("<html><body>Package test-package</body></html>")
	cache.SetPublicPackagePage("test-package", htmlContent)
	cache.SetPrivatePackagePage("test-package", htmlContent)

	// Try to get package pages - should not be found
	_, found := cache.GetPublicPackagePage("test-package")
	if found {
		t.Error("Expected not to find package page when cache is disabled")
	}

	_, found = cache.GetPrivatePackagePage("test-package")
	if found {
		t.Error("Expected not to find package page when cache is disabled")
	}
} 