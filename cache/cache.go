package cache

import (
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/v2"
)

// PackageInfo represents information about a package in an index
type PackageInfo struct {
	Exists     bool
	LastUpdate time.Time
}

// PackagePageInfo represents cached HTML content for a package page
type PackagePageInfo struct {
	HTML       []byte
	LastUpdate time.Time
}

// Cache represents the LRU cache for package information and HTML content
type Cache struct {
	publicCache      *lru.Cache[string, PackageInfo]
	privateCache     *lru.Cache[string, PackageInfo]
	publicPageCache  *lru.Cache[string, PackagePageInfo]
	privatePageCache *lru.Cache[string, PackagePageInfo]
	ttl              time.Duration
	enabled          bool
	mu               sync.RWMutex
}

// NewCache creates a new cache instance
func NewCache(size int, ttlHours int, enabled bool) (*Cache, error) {
	if !enabled {
		return &Cache{enabled: false}, nil
	}

	publicCache, err := lru.New[string, PackageInfo](size)
	if err != nil {
		return nil, err
	}

	privateCache, err := lru.New[string, PackageInfo](size)
	if err != nil {
		return nil, err
	}

	publicPageCache, err := lru.New[string, PackagePageInfo](size)
	if err != nil {
		return nil, err
	}

	privatePageCache, err := lru.New[string, PackagePageInfo](size)
	if err != nil {
		return nil, err
	}

	return &Cache{
		publicCache:      publicCache,
		privateCache:     privateCache,
		publicPageCache:  publicPageCache,
		privatePageCache: privatePageCache,
		ttl:              time.Duration(ttlHours) * time.Hour,
		enabled:          true,
	}, nil
}

// GetPublicPackage checks if a package exists in the public index
func (c *Cache) GetPublicPackage(packageName string) (PackageInfo, bool) {
	if !c.enabled {
		return PackageInfo{}, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	info, exists := c.publicCache.Get(packageName)
	if !exists {
		return PackageInfo{}, false
	}

	// Check if entry has expired
	if time.Since(info.LastUpdate) > c.ttl {
		c.publicCache.Remove(packageName)
		return PackageInfo{}, false
	}

	return info, true
}

// GetPrivatePackage checks if a package exists in the private index
func (c *Cache) GetPrivatePackage(packageName string) (PackageInfo, bool) {
	if !c.enabled {
		return PackageInfo{}, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	info, exists := c.privateCache.Get(packageName)
	if !exists {
		return PackageInfo{}, false
	}

	// Check if entry has expired
	if time.Since(info.LastUpdate) > c.ttl {
		c.privateCache.Remove(packageName)
		return PackageInfo{}, false
	}

	return info, true
}

// SetPublicPackage sets package information for the public index
func (c *Cache) SetPublicPackage(packageName string, exists bool) {
	if !c.enabled {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	info := PackageInfo{
		Exists:     exists,
		LastUpdate: time.Now(),
	}

	c.publicCache.Add(packageName, info)
}

// SetPrivatePackage sets package information for the private index
func (c *Cache) SetPrivatePackage(packageName string, exists bool) {
	if !c.enabled {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	info := PackageInfo{
		Exists:     exists,
		LastUpdate: time.Now(),
	}

	c.privateCache.Add(packageName, info)
}

// GetPublicPackagePage retrieves cached HTML content for a public package page
func (c *Cache) GetPublicPackagePage(packageName string) (PackagePageInfo, bool) {
	if !c.enabled {
		return PackagePageInfo{}, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	info, exists := c.publicPageCache.Get(packageName)
	if !exists {
		return PackagePageInfo{}, false
	}

	// Check if entry has expired
	if time.Since(info.LastUpdate) > c.ttl {
		c.publicPageCache.Remove(packageName)
		return PackagePageInfo{}, false
	}

	return info, true
}

// GetPrivatePackagePage retrieves cached HTML content for a private package page
func (c *Cache) GetPrivatePackagePage(packageName string) (PackagePageInfo, bool) {
	if !c.enabled {
		return PackagePageInfo{}, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	info, exists := c.privatePageCache.Get(packageName)
	if !exists {
		return PackagePageInfo{}, false
	}

	// Check if entry has expired
	if time.Since(info.LastUpdate) > c.ttl {
		c.privatePageCache.Remove(packageName)
		return PackagePageInfo{}, false
	}

	return info, true
}

// SetPublicPackagePage sets HTML content for a public package page
func (c *Cache) SetPublicPackagePage(packageName string, html []byte) {
	if !c.enabled {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	info := PackagePageInfo{
		HTML:       html,
		LastUpdate: time.Now(),
	}

	c.publicPageCache.Add(packageName, info)
}

// SetPrivatePackagePage sets HTML content for a private package page
func (c *Cache) SetPrivatePackagePage(packageName string, html []byte) {
	if !c.enabled {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	info := PackagePageInfo{
		HTML:       html,
		LastUpdate: time.Now(),
	}

	c.privatePageCache.Add(packageName, info)
}

// Clear clears all cached data
func (c *Cache) Clear() {
	if !c.enabled {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.publicCache.Purge()
	c.privateCache.Purge()
	c.publicPageCache.Purge()
	c.privatePageCache.Purge()
}

// IsEnabled returns whether the cache is enabled
func (c *Cache) IsEnabled() bool {
	return c.enabled
}

// GetStats returns cache statistics
func (c *Cache) GetStats() (int, int, int, int) {
	if !c.enabled {
		return 0, 0, 0, 0
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.publicCache.Len(), c.privateCache.Len(), c.publicPageCache.Len(), c.privatePageCache.Len()
}
