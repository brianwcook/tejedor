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
	// ResponseHeaderSource indicates which index the package is being served from
	ResponseHeaderSource = "X-PyPI-Source"
	// ResponseHeaderSourcePublic indicates the package is from public PyPI
	ResponseHeaderSourcePublic = "public"
	// ResponseHeaderSourcePrivate indicates the package is from private PyPI
	ResponseHeaderSourcePrivate = "private"
)

// Client represents a PyPI client
type Client struct {
	httpClient *http.Client
}

// NewClient creates a new PyPI client
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// PackageExists checks if a package exists in the specified index
func (c *Client) PackageExists(ctx context.Context, baseURL, packageName string) (bool, error) {
	// Normalize the package name for URL
	normalizedName := strings.ToLower(strings.ReplaceAll(packageName, "_", "-"))
	
	// Construct the package URL
	packageURL := fmt.Sprintf("%s%s/", strings.TrimSuffix(baseURL, "/"), normalizedName)
	
	// Make HEAD request to check if package exists
	req, err := http.NewRequestWithContext(ctx, "HEAD", packageURL, nil)
	if err != nil {
		return false, fmt.Errorf("error creating request: %w", err)
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()
	
	// Package exists if we get a 200 OK
	return resp.StatusCode == http.StatusOK, nil
}

// GetPackagePage retrieves the package page from the specified index
func (c *Client) GetPackagePage(ctx context.Context, baseURL, packageName string) ([]byte, error) {
	// Normalize the package name for URL
	normalizedName := strings.ToLower(strings.ReplaceAll(packageName, "_", "-"))
	
	// Construct the package URL
	packageURL := fmt.Sprintf("%s%s/", strings.TrimSuffix(baseURL, "/"), normalizedName)
	
	// Make GET request to retrieve package page
	req, err := http.NewRequestWithContext(ctx, "GET", packageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()
	
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

// GetPackageFile retrieves a specific package file from the specified index
func (c *Client) GetPackageFile(ctx context.Context, fileURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()
	
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

// ProxyFile proxies a file from the specified URL to the response writer
func (c *Client) ProxyFile(ctx context.Context, fileURL string, w http.ResponseWriter) error {
	req, err := http.NewRequestWithContext(ctx, "GET", fileURL, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()
	
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