package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	
	if config.PublicPyPIURL != "https://pypi.org/simple/" {
		t.Errorf("Expected public PyPI URL to be https://pypi.org/simple/, got %s", config.PublicPyPIURL)
	}
	
	if config.Port != 8080 {
		t.Errorf("Expected port to be 8080, got %d", config.Port)
	}
	
	if !config.CacheEnabled {
		t.Error("Expected cache to be enabled by default")
	}
	
	if config.CacheSize != 20000 {
		t.Errorf("Expected cache size to be 20000, got %d", config.CacheSize)
	}
	
	if config.CacheTTL != 12 {
		t.Errorf("Expected cache TTL to be 12 hours, got %d", config.CacheTTL)
	}
}

func TestLoadConfigFromEnvironment(t *testing.T) {
	// Set environment variables
	os.Setenv("PYPI_PROXY_PRIVATE_PYPI_URL", "https://test.example.com/simple/")
	os.Setenv("PYPI_PROXY_PORT", "9090")
	os.Setenv("PYPI_PROXY_CACHE_ENABLED", "false")
	os.Setenv("PYPI_PROXY_CACHE_SIZE", "1000")
	os.Setenv("PYPI_PROXY_CACHE_TTL_HOURS", "6")
	
	defer func() {
		os.Unsetenv("PYPI_PROXY_PRIVATE_PYPI_URL")
		os.Unsetenv("PYPI_PROXY_PORT")
		os.Unsetenv("PYPI_PROXY_CACHE_ENABLED")
		os.Unsetenv("PYPI_PROXY_CACHE_SIZE")
		os.Unsetenv("PYPI_PROXY_CACHE_TTL_HOURS")
	}()
	
	config, err := LoadConfig("")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	
	if config.PrivatePyPIURL != "https://test.example.com/simple/" {
		t.Errorf("Expected private PyPI URL to be https://test.example.com/simple/, got %s", config.PrivatePyPIURL)
	}
	
	if config.Port != 9090 {
		t.Errorf("Expected port to be 9090, got %d", config.Port)
	}
	
	if config.CacheEnabled {
		t.Error("Expected cache to be disabled")
	}
	
	if config.CacheSize != 1000 {
		t.Errorf("Expected cache size to be 1000, got %d", config.CacheSize)
	}
	
	if config.CacheTTL != 6 {
		t.Errorf("Expected cache TTL to be 6 hours, got %d", config.CacheTTL)
	}
}

func TestLoadConfigMissingPrivateURL(t *testing.T) {
	// Ensure no environment variable is set
	os.Unsetenv("PYPI_PROXY_PRIVATE_PYPI_URL")
	
	_, err := LoadConfig("")
	if err == nil {
		t.Error("Expected error when private PyPI URL is missing")
	}
	
	if err.Error() != "private_pypi_url is required" {
		t.Errorf("Expected specific error message, got %v", err)
	}
}

func TestCreateDefaultConfigFile(t *testing.T) {
	tempFile := "test_config.yaml"
	defer os.Remove(tempFile)
	
	err := CreateDefaultConfigFile(tempFile)
	if err != nil {
		t.Fatalf("Expected no error creating config file, got %v", err)
	}
	
	// Verify file was created
	if _, err := os.Stat(tempFile); os.IsNotExist(err) {
		t.Error("Expected config file to be created")
	}
	
	// Load the created config to verify it's valid
	config, err := LoadConfig(tempFile)
	if err != nil {
		t.Fatalf("Expected no error loading created config, got %v", err)
	}
	
	if config.PrivatePyPIURL != "https://console.redhat.com/api/pulp-content/public-calunga/mypypi/simple" {
		t.Errorf("Expected private PyPI URL to be set correctly, got %s", config.PrivatePyPIURL)
	}
} 