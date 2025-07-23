// Package pypi provides a client for interacting with PyPI repositories.
package pypi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// ResponseHeaderSource indicates which index the package is being served from.
	ResponseHeaderSource = "X-PyPI-Source"
	// ResponseHeaderSourcePublic indicates the package is from public PyPI.
	ResponseHeaderSourcePublic = "public"
	// ResponseHeaderSourcePrivate indicates the package is from private PyPI.
	ResponseHeaderSourcePrivate = "private"
)

// PyPIClient defines the interface for PyPI client operations.
//
//nolint:revive // This interface name is intentionally descriptive and used throughout the codebase
type PyPIClient interface {
	PackageExists(ctx context.Context, baseURL, packageName string) (bool, error)
	GetPackagePage(ctx context.Context, baseURL, packageName string) ([]byte, error)
	GetPackageFile(ctx context.Context, fileURL string) ([]byte, error)
	ProxyFile(ctx context.Context, fileURL string, w http.ResponseWriter, method string) error
}

// HTTPClient represents a PyPI client.
type HTTPClient struct {
	httpClient *http.Client
}

// Ensure HTTPClient implements PyPIClient interface.
var _ PyPIClient = (*HTTPClient)(nil)

// NewClient creates a new PyPI client.
func NewClient() *HTTPClient {
	return &HTTPClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// joinURL robustly joins a base URL and a path.
func joinURL(base, path string) (string, error) {
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	ref, err := url.Parse(path)
	if err != nil {
		return "", err
	}
	return baseURL.ResolveReference(ref).String(), nil
}

// PackageExists checks if a package exists in the specified index.
func (c *HTTPClient) PackageExists(ctx context.Context, baseURL, packageName string) (bool, error) {
	// Normalize the package name for URL
	normalizedName := strings.ToLower(strings.ReplaceAll(packageName, "_", "-"))

	// Ensure base URL ends with a trailing slash for proper path joining
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	// Construct the package URL robustly
	packageURL, err := joinURL(baseURL, normalizedName+"/")
	if err != nil {
		return false, fmt.Errorf("error joining URL: %w", err)
	}

	// Make HEAD request to check if package exists
	req, err := http.NewRequestWithContext(ctx, "HEAD", packageURL, http.NoBody)
	if err != nil {
		return false, fmt.Errorf("error creating request: %w", err)
	}

	// Create a client that doesn't follow redirects for package existence checks
	noRedirectClient := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := noRedirectClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("error making request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log the error but don't fail the function
			// This is a common pattern for defer close operations
			_ = closeErr // explicitly ignore error
		}
	}()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
		// Fallback to GET request if HEAD is not supported or returns 404
		getReq, err := http.NewRequestWithContext(ctx, "GET", packageURL, http.NoBody)
		if err != nil {
			return false, fmt.Errorf("error creating GET request: %w", err)
		}
		getResp, err := c.httpClient.Do(getReq)
		if err != nil {
			return false, fmt.Errorf("error making GET request: %w", err)
		}
		defer func() {
			if closeErr := getResp.Body.Close(); closeErr != nil {
				// Log the error but don't fail the function
				// This is a common pattern for defer close operations
				_ = closeErr // explicitly ignore error
			}
		}()
		return getResp.StatusCode == http.StatusOK, nil
	}
	// Treat 3xx redirects as "package not found" for private servers
	// This prevents false positives when private servers redirect to public PyPI
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return false, nil
	}
	// Package does not exist
	return false, nil
}

// GetPackagePage retrieves the package page from the specified index.
func (c *HTTPClient) GetPackagePage(ctx context.Context, baseURL, packageName string) ([]byte, error) {
	// Normalize the package name for URL
	normalizedName := strings.ToLower(strings.ReplaceAll(packageName, "_", "-"))

	// Ensure base URL ends with a trailing slash for proper path joining
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	// Construct the package URL robustly
	packageURL, err := joinURL(baseURL, normalizedName+"/")
	if err != nil {
		return nil, fmt.Errorf("error joining URL: %w", err)
	}

	// Make GET request to retrieve package page
	req, err := http.NewRequestWithContext(ctx, "GET", packageURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log the error but don't fail the function
			// This is a common pattern for defer close operations
			_ = closeErr // explicitly ignore error
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("package not found: %s", packageName)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	return body, nil
}

// GetPackageFile retrieves a specific package file from the specified index.
func (c *HTTPClient) GetPackageFile(ctx context.Context, fileURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fileURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log the error but don't fail the function
			// This is a common pattern for defer close operations
			_ = closeErr // explicitly ignore error
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("file not found: %s", fileURL)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	return body, nil
}

// ProxyFile proxies a file from the specified URL to the response writer.
func (c *HTTPClient) ProxyFile(ctx context.Context, fileURL string, w http.ResponseWriter, method string) error {
	req, err := http.NewRequestWithContext(ctx, method, fileURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error making request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log the error but don't fail the function
			// This is a common pattern for defer close operations
			_ = closeErr // explicitly ignore error
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("file not found: %s", fileURL)
	}

	// Copy headers from the original response
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Copy the response body
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return fmt.Errorf("error copying response body: %w", err)
	}

	return nil
}
