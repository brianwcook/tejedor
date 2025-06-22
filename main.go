package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"python-index-proxy/config"
	"python-index-proxy/proxy"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
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
