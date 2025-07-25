// Package proxy provides the PyPI proxy server implementation that routes requests
// between public and private PyPI repositories.
package proxy

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"python-index-proxy/cache"
	"python-index-proxy/config"
	"python-index-proxy/pypi"
	"regexp"
	"strings"
)

const (
	publicPyPIFileBaseURL = "https://files.pythonhosted.org"
	packagesPath          = "packages"
)

// Proxy represents the PyPI proxy server.
type Proxy struct {
	config *config.Config
	cache  *cache.Cache
	client pypi.PyPIClient
}

// NewProxy creates a new proxy instance.
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

// filterWheelFiles removes wheel file links from HTML content.
// This ensures that only source distributions are served from public PyPI.
func (p *Proxy) filterWheelFiles(htmlContent []byte) []byte {
	content := string(htmlContent)

	// Regular expression to match wheel file links
	// Matches <a href="...">...whl</a> patterns with any additional attributes
	wheelPattern := regexp.MustCompile(`<a[^>]*href="[^"]*\.whl[^"]*"[^>]*>.*?\.whl</a>\s*<br\s*/>?\s*`)

	// Remove wheel file links
	filteredContent := wheelPattern.ReplaceAllString(content, "")

	return []byte(filteredContent)
}

// determineSource determines which index to serve from and gets cached content if available.
func (p *Proxy) determineSource(ctx context.Context, packageName string, publicExists, privateExists bool) (sourceIndex, baseURL string, packagePage []byte, exists bool, err error) {
	var cachedPage cache.PackagePageInfo
	var found bool

	// Log the routing decision
	log.Printf("ROUTING: /simple/%s/ - publicExists=%v, privateExists=%v", packageName, publicExists, privateExists)

	// Check if this package should always use the public index
	if p.config.IsPublicOnlyPackage(packageName) {
		if publicExists {
			sourceIndex = p.config.PublicPyPIURL
			baseURL = p.config.PublicPyPIURL
			log.Printf("ROUTING: /simple/%s/ → PUBLIC_PYPI (public-only package) (%s)", packageName, p.config.PublicPyPIURL)

			// Check cache for public package page
			if p.cache.IsEnabled() {
				cachedPage, found = p.cache.GetPublicPackagePage(packageName)
			}
		} else {
			// Package doesn't exist in public index
			return "", "", nil, false, nil
		}
	} else {
		switch {
		case privateExists:
			// If package exists in private index, serve from there
			sourceIndex = p.config.PrivatePyPIURL
			baseURL = p.config.PrivatePyPIURL
			log.Printf("ROUTING: /simple/%s/ → LOCAL_PYPI (%s)", packageName, p.config.PrivatePyPIURL)

			// Check cache for private package page
			if p.cache.IsEnabled() {
				cachedPage, found = p.cache.GetPrivatePackagePage(packageName)
			}
		case publicExists:
			// If package only exists in public index, serve from there
			sourceIndex = p.config.PublicPyPIURL
			baseURL = p.config.PublicPyPIURL
			log.Printf("ROUTING: /simple/%s/ → PUBLIC_PYPI (%s)", packageName, p.config.PublicPyPIURL)

			// Check cache for public package page
			if p.cache.IsEnabled() {
				cachedPage, found = p.cache.GetPublicPackagePage(packageName)
			}
		default:
			// Package doesn't exist in either index
			return "", "", nil, false, nil
		}
	}

	// If found in cache, use cached content
	if found {
		log.Printf("ROUTING: /simple/%s/ → CACHED (from %s)", packageName, sourceIndex)
		packagePage = cachedPage.HTML
	} else {
		// Get package page from the determined source
		log.Printf("ROUTING: /simple/%s/ → FETCHING (from %s)", packageName, sourceIndex)
		packagePage, err = p.client.GetPackagePage(ctx, baseURL, packageName)
		if err != nil {
			log.Printf("ROUTING: /simple/%s/ → ERROR (from %s): %v", packageName, sourceIndex, err)
			return "", "", nil, false, fmt.Errorf("error retrieving package page: %w", err)
		}

		// Cache the package page for future requests
		if p.cache.IsEnabled() {
			if privateExists {
				p.cache.SetPrivatePackagePage(packageName, packagePage)
			} else {
				p.cache.SetPublicPackagePage(packageName, packagePage)
			}
		}
	}

	exists = true
	return sourceIndex, baseURL, packagePage, exists, nil
}

// HandlePackage handles requests for package information.
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
	publicExists, privateExists, err := p.CheckPackageExists(ctx, packageName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error checking package existence: %v", err), http.StatusInternalServerError)
		return
	}

	// Determine which index to serve from and get content
	sourceIndex, _, packagePage, exists, err := p.determineSource(ctx, packageName, publicExists, privateExists)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error determining source: %v", err), http.StatusInternalServerError)
		return
	}

	if !exists {
		// Package doesn't exist in either index
		http.Error(w, "Package not found", http.StatusNotFound)
		return
	}

	// Add source header
	w.Header().Set(pypi.ResponseHeaderSource, sourceIndex)

	// Set content type
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Filter wheel files only when serving from public PyPI
	var finalContent []byte
	if sourceIndex == p.config.PublicPyPIURL {
		finalContent = p.filterWheelFiles(packagePage)
	} else {
		finalContent = packagePage
	}

	// For HEAD requests, only send headers, not body
	if r.Method == "HEAD" {
		return
	}

	// Write the package page
	if _, err := w.Write(finalContent); err != nil {
		http.Error(w, fmt.Sprintf("Error writing response: %v", err), http.StatusInternalServerError)
		return
	}
}

// HandleFile handles requests for package files.
func (p *Proxy) HandleFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	filePath, err := p.extractFilePath(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fileName := p.extractFileNameFromPath(r.URL.Path)
	packageName := p.extractPackageNameFromFileName(fileName)

	if packageName == "" {
		http.Error(w, "Could not determine package name from file", http.StatusBadRequest)
		return
	}

	// Check if package exists in both indexes
	publicExists, privateExists, err := p.CheckPackageExists(ctx, packageName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error checking package existence: %v", err), http.StatusInternalServerError)
		return
	}

	sourceIndex, fileBaseURL, err := p.determineFileSource(packageName, publicExists, privateExists)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Add source header
	w.Header().Set(pypi.ResponseHeaderSource, sourceIndex)

	// Construct the full file URL
	fileURL := p.constructFileURL(fileBaseURL, r.URL.Path, filePath)

	// Proxy the file
	if err := p.client.ProxyFile(ctx, fileURL, w, r.Method); err != nil {
		http.Error(w, fmt.Sprintf("Error proxying file: %v", err), http.StatusInternalServerError)
		return
	}
}

// HandleIndex handles requests for the index page.
func (p *Proxy) HandleIndex(w http.ResponseWriter, _ *http.Request) {
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

	// Write the index page
	if _, err := w.Write([]byte(indexHTML)); err != nil {
		http.Error(w, fmt.Sprintf("Error writing response: %v", err), http.StatusInternalServerError)
		return
	}
}

// HandleHealth handles health check requests and returns cache statistics.
func (p *Proxy) HandleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set(pypi.ResponseHeaderSource, "proxy")

	publicLen, privateLen, publicPageLen, privatePageLen := p.cache.GetStats()

	response := fmt.Sprintf(`{
        "status": "healthy",
        "cache": {
            "enabled": %t,
            "public_packages": %d,
            "private_packages": %d,
            "public_pages": %d,
            "private_pages": %d
        }
    }`, p.cache.IsEnabled(), publicLen, privateLen, publicPageLen, privatePageLen)

	// Write the response
	if _, err := w.Write([]byte(response)); err != nil {
		http.Error(w, fmt.Sprintf("Error writing response: %v", err), http.StatusInternalServerError)
		return
	}
}

// CheckPackageExists checks if a package exists in both indexes using cache when possible.
func (p *Proxy) CheckPackageExists(ctx context.Context, packageName string) (publicExists, privateExists bool, err error) {
	var publicErr, privateErr error

	var publicFound, privateFound bool

	// Check cache first
	if p.cache.IsEnabled() {
		if info, found := p.cache.GetPublicPackage(packageName); found {
			publicExists = info.Exists
			publicFound = true
		}
		if info, found := p.cache.GetPrivatePackage(packageName); found {
			privateExists = info.Exists
			privateFound = true
		}
	}

	// If not in cache or cache disabled, check indexes
	if !p.cache.IsEnabled() || !publicFound {
		publicExists, publicErr = p.client.PackageExists(ctx, p.config.PublicPyPIURL, packageName)
		if publicErr == nil && p.cache.IsEnabled() {
			p.cache.SetPublicPackage(packageName, publicExists)
		}
	}

	if !p.cache.IsEnabled() || !privateFound {
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

// extractPackageNameFromFileName extracts package name from a file name.
// Example: "pydantic-2.5.0-py3-none-any.whl" -> "pydantic".
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

// extractFilePath extracts and validates the file path from the request.
func (p *Proxy) extractFilePath(r *http.Request) (string, error) {
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

	switch {
	case len(pathParts) >= 2 && pathParts[0] == packagesPath:
		// Handle /packages/{file_path} format
		filePath := strings.Join(pathParts[1:], "/")
		// Check if we have a valid file path (not just empty or trailing slash)
		if filePath == "" {
			return "", fmt.Errorf("invalid file path")
		}
		return filePath, nil
	case len(pathParts) == 1 && pathParts[0] != packagesPath:
		// Handle direct file requests like /{file_name}
		filePath := pathParts[0]
		if filePath == "" || filePath == packagesPath {
			return "", fmt.Errorf("invalid file path")
		}
		return filePath, nil
	default:
		return "", fmt.Errorf("invalid file path")
	}
}

// extractFileNameFromPath extracts the file name from the URL path.
func (p *Proxy) extractFileNameFromPath(path string) string {
	pathParts := strings.Split(strings.Trim(path, "/"), "/")
	if len(pathParts) == 0 {
		return ""
	}
	return pathParts[len(pathParts)-1]
}

// determineFileSource determines which source to serve the file from.
func (p *Proxy) determineFileSource(packageName string, publicExists, privateExists bool) (sourceIndex, fileBaseURL string, err error) {
	// Check if this package should always use the public index
	if p.config.IsPublicOnlyPackage(packageName) {
		if publicExists {
			return p.config.PublicPyPIURL, publicPyPIFileBaseURL, nil
		}
		return "", "", fmt.Errorf("package not found")
	}

	switch {
	case privateExists:
		// If package exists in private index, serve from there
		return p.config.PrivatePyPIURL, strings.TrimSuffix(strings.TrimSuffix(p.config.PrivatePyPIURL, "/simple/"), "/simple"), nil
	case publicExists:
		// If package only exists in public index, serve from there
		return p.config.PublicPyPIURL, publicPyPIFileBaseURL, nil
	default:
		// Package doesn't exist in either index
		return "", "", fmt.Errorf("package not found")
	}
}

// constructFileURL constructs the full file URL based on the request path.
func (p *Proxy) constructFileURL(fileBaseURL, requestPath, filePath string) string {
	if strings.HasPrefix(requestPath, "/"+packagesPath+"/") {
		// For /packages/ paths, use the original logic
		fileURL := fileBaseURL + requestPath
		// Fix double slashes
		return strings.Replace(fileURL, "//"+packagesPath+"/", "/"+packagesPath+"/", 1)
	}
	// For direct file requests, construct the URL properly
	return fileBaseURL + "/" + filePath
}

// GetCache returns the cache instance for testing purposes.
func (p *Proxy) GetCache() *cache.Cache {
	return p.cache
}
