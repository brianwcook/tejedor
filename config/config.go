package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// Config holds the application configuration
type Config struct {
	PublicPyPIURL  string `mapstructure:"public_pypi_url"`
	PrivatePyPIURL string `mapstructure:"private_pypi_url"`
	Port           int    `mapstructure:"port"`
	CacheEnabled   bool   `mapstructure:"cache_enabled"`
	CacheSize      int    `mapstructure:"cache_size"`
	CacheTTL       int    `mapstructure:"cache_ttl_hours"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		PublicPyPIURL:  "https://pypi.org/simple/",
		PrivatePyPIURL: "",
		Port:           8080,
		CacheEnabled:   true,
		CacheSize:      20000,
		CacheTTL:       12,
	}
}

// LoadConfig loads configuration from file and environment variables
func LoadConfig(configPath string) (*Config, error) {
	config := DefaultConfig()

	// Set up viper
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	// Environment variables
	viper.SetEnvPrefix("PYPI_PROXY")
	viper.AutomaticEnv()

	// Bind environment variables
	viper.BindEnv("public_pypi_url", "PYPI_PROXY_PUBLIC_PYPI_URL")
	viper.BindEnv("private_pypi_url", "PYPI_PROXY_PRIVATE_PYPI_URL")
	viper.BindEnv("port", "PYPI_PROXY_PORT")
	viper.BindEnv("cache_enabled", "PYPI_PROXY_CACHE_ENABLED")
	viper.BindEnv("cache_size", "PYPI_PROXY_CACHE_SIZE")
	viper.BindEnv("cache_ttl_hours", "PYPI_PROXY_CACHE_TTL_HOURS")

	// If config file is specified, use it
	if configPath != "" {
		viper.SetConfigFile(configPath)
	}

	// Read config file if it exists
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Unmarshal config
	if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Validate required fields
	if config.PrivatePyPIURL == "" {
		return nil, fmt.Errorf("private_pypi_url is required")
	}

	return config, nil
}

// CreateDefaultConfigFile creates a default config file
func CreateDefaultConfigFile(path string) error {
	config := DefaultConfig()
	config.PrivatePyPIURL = "https://console.redhat.com/api/pulp-content/public-calunga/mypypi/simple"

	viper.Set("public_pypi_url", config.PublicPyPIURL)
	viper.Set("private_pypi_url", config.PrivatePyPIURL)
	viper.Set("port", config.Port)
	viper.Set("cache_enabled", config.CacheEnabled)
	viper.Set("cache_size", config.CacheSize)
	viper.Set("cache_ttl_hours", config.CacheTTL)

	return viper.WriteConfigAs(path)
} 