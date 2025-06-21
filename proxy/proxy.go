package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"python-index-proxy/cache"
	"python-index-proxy/config"
	"python-index-proxy/pypi"
)

// Proxy represents the PyPI proxy server
type Proxy struct {
	config *config.Config
	cache  *cache.Cache
	client *pypi.Client
}

// NewProxy creates a new proxy instance
func NewProxy(cfg *config.Config) (*Proxy, error) {
	cache, err := cache.NewCache(cfg.CacheSize, cfg.CacheTTL, cfg.CacheEnabled)
	if err != nil {
		return nil, fmt.Errorf("error creating cache: %w", err)
	}

	return &Proxy{
		config: cfg,
		cache:  cache,
		client: pypi.NewClient(),
	}, nil
}

// HandlePackage handles requests for package information
func (p *Proxy) HandlePackage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Extract package name from URL path
	// Expected format: /simple/{package_name}/
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 2 || pathParts[0] != "simple" {
		http.Error(w, "Invalid package path", http.StatusBadRequest)
		return
	}
	
	packageName := pathParts[1]
	if packageName == "" {
		http.Error(w, "Package name is required", http.StatusBadRequest)
		return
	}

	// Check if package exists in both indexes
	publicExists, privateExists, err := p.checkPackageExists(ctx, packageName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error checking package existence: %v", err), http.StatusInternalServerError)
		return
	}

	// Determine which index to serve from
	var sourceIndex string
	var baseURL string

	if privateExists {
		// If package exists in private index, serve from there
		sourceIndex = pypi.ResponseHeaderSourcePrivate
		baseURL = p.config.PrivatePyPIURL
	} else if publicExists {
		// If package only exists in public index, serve from there
		sourceIndex = pypi.ResponseHeaderSourcePublic
		baseURL = p.config.PublicPyPIURL
	} else {
		// Package doesn't exist in either index
		http.Error(w, "Package not found", http.StatusNotFound)
		return
	}

	// Add source header
	w.Header().Set(pypi.ResponseHeaderSource, sourceIndex)

	// Get package page from the determined source
	packagePage, err := p.client.GetPackagePage(ctx, baseURL, packageName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error retrieving package page: %v", err), http.StatusInternalServerError)
		return
	}

	// Set content type
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	
	// Write the package page
	w.Write(packagePage)
}

// HandleFile handles requests for package files
func (p *Proxy) HandleFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Extract file path from URL
	// Expected format: /packages/{file_path}
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 2 || pathParts[0] != "packages" {
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		return
	}
	
	filePath := strings.Join(pathParts[1:], "/")
	if filePath == "" {
		http.Error(w, "File path is required", http.StatusBadRequest)
		return
	}

	// Extract package name from file path to determine source
	// File paths typically follow pattern: {package_name}-{version}-{...}
	fileName := pathParts[len(pathParts)-1]
	packageName := p.extractPackageNameFromFileName(fileName)

	if packageName == "" {
		http.Error(w, "Could not determine package name from file", http.StatusBadRequest)
		return
	}

	// Check if package exists in both indexes
	publicExists, privateExists, err := p.checkPackageExists(ctx, packageName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error checking package existence: %v", err), http.StatusInternalServerError)
		return
	}

	// Determine which index to serve from
	var sourceIndex string
	var baseURL string

	if privateExists {
		// If package exists in private index, serve from there
		sourceIndex = pypi.ResponseHeaderSourcePrivate
		baseURL = p.config.PrivatePyPIURL
	} else if publicExists {
		// If package only exists in public index, serve from there
		sourceIndex = pypi.ResponseHeaderSourcePublic
		baseURL = p.config.PublicPyPIURL
	} else {
		// Package doesn't exist in either index
		http.Error(w, "Package not found", http.StatusNotFound)
		return
	}

	// Add source header
	w.Header().Set(pypi.ResponseHeaderSource, sourceIndex)

	// Construct the full file URL
	fileURL := fmt.Sprintf("%s%s", strings.TrimSuffix(baseURL, "/"), r.URL.Path)

	// Proxy the file from the determined source
	err = p.client.ProxyFile(ctx, fileURL, w)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error proxying file: %v", err), http.StatusInternalServerError)
		return
	}
}

// HandleIndex handles requests for the index page
func (p *Proxy) HandleIndex(w http.ResponseWriter, r *http.Request) {
	// Return a simple index page
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set(pypi.ResponseHeaderSource, "proxy")
	
	indexHTML := `<!DOCTYPE html>
<html>
<head>
    <title>PyPI Proxy</title>
</head>
<body>
    <h1>PyPI Proxy</h1>
    <p>This is a proxy for PyPI packages.</p>
    <p>Use the simple repository API to access packages:</p>
    <ul>
        <li><a href="/simple/">Package Index</a></li>
    </ul>
</body>
</html>`
	
	w.Write([]byte(indexHTML))
}

// checkPackageExists checks if a package exists in both indexes using cache when possible
func (p *Proxy) checkPackageExists(ctx context.Context, packageName string) (bool, bool, error) {
	var publicExists, privateExists bool
	var publicErr, privateErr error

	// Check cache first
	if p.cache.IsEnabled() {
		if info, found := p.cache.GetPublicPackage(packageName); found {
			publicExists = info.Exists
		}
		if info, found := p.cache.GetPrivatePackage(packageName); found {
			privateExists = info.Exists
		}
	}

	// If not in cache or cache disabled, check indexes
	if !p.cache.IsEnabled() || !publicExists {
		publicExists, publicErr = p.client.PackageExists(ctx, p.config.PublicPyPIURL, packageName)
		if publicErr == nil && p.cache.IsEnabled() {
			p.cache.SetPublicPackage(packageName, publicExists)
		}
	}

	if !p.cache.IsEnabled() || !privateExists {
		privateExists, privateErr = p.client.PackageExists(ctx, p.config.PrivatePyPIURL, packageName)
		if privateErr == nil && p.cache.IsEnabled() {
			p.cache.SetPrivatePackage(packageName, privateExists)
		}
	}

	// Return errors if any occurred
	if publicErr != nil {
		return false, false, fmt.Errorf("error checking public index: %w", publicErr)
	}
	if privateErr != nil {
		return false, false, fmt.Errorf("error checking private index: %w", privateErr)
	}

	return publicExists, privateExists, nil
}

// extractPackageNameFromFileName extracts package name from a file name
// Example: "pydantic-2.5.0-py3-none-any.whl" -> "pydantic"
func (p *Proxy) extractPackageNameFromFileName(fileName string) string {
	// Remove file extension
	fileName = strings.TrimSuffix(fileName, ".whl")
	fileName = strings.TrimSuffix(fileName, ".tar.gz")
	fileName = strings.TrimSuffix(fileName, ".zip")
	
	// Split by dash and take the first part
	parts := strings.Split(fileName, "-")
	if len(parts) > 0 {
		return parts[0]
	}
	
	return ""
}

// GetCache returns the cache instance for testing purposes
func (p *Proxy) GetCache() *cache.Cache {
	return p.cache
} 