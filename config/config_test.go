package config

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
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
	if err := os.Setenv("PYPI_PROXY_PRIVATE_PYPI_URL", "https://test.example.com/simple/"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	if err := os.Setenv("PYPI_PROXY_PORT", "9090"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	if err := os.Setenv("PYPI_PROXY_CACHE_ENABLED", "false"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	if err := os.Setenv("PYPI_PROXY_CACHE_SIZE", "1000"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	if err := os.Setenv("PYPI_PROXY_CACHE_TTL_HOURS", "6"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	defer func() {
		if err := os.Unsetenv("PYPI_PROXY_PRIVATE_PYPI_URL"); err != nil {
			t.Errorf("Failed to unset environment variable: %v", err)
		}
		if err := os.Unsetenv("PYPI_PROXY_PORT"); err != nil {
			t.Errorf("Failed to unset environment variable: %v", err)
		}
		if err := os.Unsetenv("PYPI_PROXY_CACHE_ENABLED"); err != nil {
			t.Errorf("Failed to unset environment variable: %v", err)
		}
		if err := os.Unsetenv("PYPI_PROXY_CACHE_SIZE"); err != nil {
			t.Errorf("Failed to unset environment variable: %v", err)
		}
		if err := os.Unsetenv("PYPI_PROXY_CACHE_TTL_HOURS"); err != nil {
			t.Errorf("Failed to unset environment variable: %v", err)
		}
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
	if err := os.Unsetenv("PYPI_PROXY_PRIVATE_PYPI_URL"); err != nil {
		t.Fatalf("Failed to unset environment variable: %v", err)
	}

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
	defer func() {
		if err := os.Remove(tempFile); err != nil {
			t.Errorf("Failed to remove temp file: %v", err)
		}
	}()

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

// TestLoadConfigWithInvalidEnvVars tests LoadConfig with invalid environment variable bindings.
func TestLoadConfigWithInvalidEnvVars(t *testing.T) {
	// Test with valid environment variables
	if err := os.Setenv("PYPI_PROXY_PRIVATE_PYPI_URL", "https://test-private-pypi.com/simple/"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	if err := os.Setenv("PYPI_PROXY_PORT", "9090"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	if err := os.Setenv("PYPI_PROXY_CACHE_ENABLED", "false"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	if err := os.Setenv("PYPI_PROXY_CACHE_SIZE", "5000"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	if err := os.Setenv("PYPI_PROXY_CACHE_TTL_HOURS", "6"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}

	// Reset viper to ensure clean state
	viper.Reset()

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if cfg.PrivatePyPIURL != "https://test-private-pypi.com/simple/" {
		t.Errorf("Expected private PyPI URL from env var, got %s", cfg.PrivatePyPIURL)
	}

	if cfg.Port != 9090 {
		t.Errorf("Expected port from env var, got %d", cfg.Port)
	}

	if cfg.CacheEnabled {
		t.Error("Expected cache disabled from env var")
	}

	if cfg.CacheSize != 5000 {
		t.Errorf("Expected cache size from env var, got %d", cfg.CacheSize)
	}

	if cfg.CacheTTL != 6 {
		t.Errorf("Expected cache TTL from env var, got %d", cfg.CacheTTL)
	}

	// Clean up
	if err := os.Unsetenv("PYPI_PROXY_PRIVATE_PYPI_URL"); err != nil {
		t.Errorf("Failed to unset environment variable: %v", err)
	}
	if err := os.Unsetenv("PYPI_PROXY_PORT"); err != nil {
		t.Errorf("Failed to unset environment variable: %v", err)
	}
	if err := os.Unsetenv("PYPI_PROXY_CACHE_ENABLED"); err != nil {
		t.Errorf("Failed to unset environment variable: %v", err)
	}
	if err := os.Unsetenv("PYPI_PROXY_CACHE_SIZE"); err != nil {
		t.Errorf("Failed to unset environment variable: %v", err)
	}
	if err := os.Unsetenv("PYPI_PROXY_CACHE_TTL_HOURS"); err != nil {
		t.Errorf("Failed to unset environment variable: %v", err)
	}
	viper.Reset()
}

// TestLoadConfigWithConfigFile tests LoadConfig with a config file.
func TestLoadConfigWithConfigFile(t *testing.T) {
	// Create a temporary config file
	tempFile, err := os.CreateTemp("", "test-config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() {
		if err := os.Remove(tempFile.Name()); err != nil {
			t.Errorf("Failed to remove temp file: %v", err)
		}
	}()

	// Write test config to file
	configContent := `
public_pypi_url: "https://test-public-pypi.org/simple/"
private_pypi_url: "https://test-private-pypi.com/simple/"
port: 9090
cache_enabled: false
cache_size: 5000
cache_ttl_hours: 6
`
	if _, err := tempFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
	if err := tempFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Reset viper to ensure clean state
	viper.Reset()

	// Load config from file
	cfg, err := LoadConfig(tempFile.Name())
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if cfg.PublicPyPIURL != "https://test-public-pypi.org/simple/" {
		t.Errorf("Expected public PyPI URL from file, got %s", cfg.PublicPyPIURL)
	}

	if cfg.PrivatePyPIURL != "https://test-private-pypi.com/simple/" {
		t.Errorf("Expected private PyPI URL from file, got %s", cfg.PrivatePyPIURL)
	}

	if cfg.Port != 9090 {
		t.Errorf("Expected port from file, got %d", cfg.Port)
	}

	if cfg.CacheEnabled {
		t.Error("Expected cache disabled from file")
	}

	if cfg.CacheSize != 5000 {
		t.Errorf("Expected cache size from file, got %d", cfg.CacheSize)
	}

	if cfg.CacheTTL != 6 {
		t.Errorf("Expected cache TTL from file, got %d", cfg.CacheTTL)
	}

	viper.Reset()
}

// TestLoadConfigWithInvalidConfigFile tests LoadConfig with an invalid config file.
func TestLoadConfigWithInvalidConfigFile(t *testing.T) {
	// Create a temporary config file with invalid YAML
	tempFile, err := os.CreateTemp("", "test-config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() {
		if err := os.Remove(tempFile.Name()); err != nil {
			t.Errorf("Failed to remove temp file: %v", err)
		}
	}()

	// Write invalid YAML to file
	configContent := `
public_pypi_url: "https://test-public-pypi.org/simple/"
private_pypi_url: "https://test-private-pypi.com/simple/"
port: invalid_port
cache_enabled: not_a_boolean
cache_size: not_a_number
cache_ttl_hours: also_not_a_number
`
	if _, err := tempFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
	if err := tempFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Reset viper to ensure clean state
	viper.Reset()

	// Load config from file - should return error for invalid values
	_, err = LoadConfig(tempFile.Name())
	if err == nil {
		t.Error("Expected error for invalid config values, got nil")
	}

	// Check that the error message contains information about the parsing errors
	errorMsg := err.Error()
	if !strings.Contains(errorMsg, "error unmarshaling config") {
		t.Errorf("Expected error message to contain 'error unmarshaling config', got: %s", errorMsg)
	}

	viper.Reset()
}
