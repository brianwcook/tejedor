// Package main provides the entry point for the PyPI proxy application.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"python-index-proxy/config"
	"python-index-proxy/proxy"
	"time"

	"github.com/gorilla/mux"
)

func main() {
	var configPath string
	var privatePyPIURL string
	var publicPyPIURL string
	var port int
	var cacheEnabled bool
	var cacheSize int
	var cacheTTL int

	flag.StringVar(&configPath, "config", "", "Path to configuration file")
	flag.StringVar(&privatePyPIURL, "private-pypi-url", "", "URL of the private PyPI server")
	flag.StringVar(&publicPyPIURL, "public-pypi-url", "", "URL of the public PyPI server (default: https://pypi.org/simple/)")
	flag.IntVar(&port, "port", 0, "Port to listen on (default: 8080)")
	flag.BoolVar(&cacheEnabled, "cache-enabled", true, "Enable caching (default: true)")
	flag.IntVar(&cacheSize, "cache-size", 0, "Cache size in entries (default: 20000)")
	flag.IntVar(&cacheTTL, "cache-ttl-hours", 0, "Cache TTL in hours (default: 12)")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	// Override config with CLI flags if provided
	if privatePyPIURL != "" {
		cfg.PrivatePyPIURL = privatePyPIURL
	}
	if publicPyPIURL != "" {
		cfg.PublicPyPIURL = publicPyPIURL
	}
	if port != 0 {
		cfg.Port = port
	}
	if !cacheEnabled {
		cfg.CacheEnabled = false
	}
	if cacheSize != 0 {
		cfg.CacheSize = cacheSize
	}
	if cacheTTL != 0 {
		cfg.CacheTTL = cacheTTL
	}

	// Validate required fields
	if cfg.PrivatePyPIURL == "" {
		log.Fatal("private_pypi_url is required (set via config file, environment variable, or --private-pypi-url flag)")
	}

	// Create proxy instance
	proxyInstance, err := proxy.NewProxy(cfg)
	if err != nil {
		log.Fatalf("Error creating proxy: %v", err)
	}

	// Create router
	router := mux.NewRouter()

	// Set up routes
	router.HandleFunc("/", proxyInstance.HandleIndex).Methods("GET")
	router.HandleFunc("/simple/", proxyInstance.HandleIndex).Methods("GET")
	router.HandleFunc("/simple/{package}/", proxyInstance.HandlePackage).Methods("GET", "HEAD")
	router.HandleFunc("/packages/{file:.*}", proxyInstance.HandleFile).Methods("GET", "HEAD")
	router.HandleFunc("/health", proxyInstance.HandleHealth).Methods("GET")

	// Add middleware for logging
	router.Use(loggingMiddleware)

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("Starting PyPI proxy server on port %d", cfg.Port)
	log.Printf("Public PyPI URL: %s", cfg.PublicPyPIURL)
	log.Printf("Private PyPI URL: %s", cfg.PrivatePyPIURL)
	log.Printf("Cache enabled: %v", cfg.CacheEnabled)
	if cfg.CacheEnabled {
		log.Printf("Cache size: %d entries", cfg.CacheSize)
		log.Printf("Cache TTL: %d hours", cfg.CacheTTL)
	}

	// Create server with timeouts to prevent DoS attacks
	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}

// loggingMiddleware logs HTTP requests.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
