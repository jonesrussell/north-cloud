package main

import (
	"encoding/json"
	"errors"
	"os"
	"sync"
)

// ErrCacheEntryNotFound is returned when a cache entry doesn't exist.
var ErrCacheEntryNotFound = errors.New("cache entry not found")

// CacheSource indicates where a cached entry was found.
type CacheSource string

const (
	SourceNone     CacheSource = "none"
	SourceFixtures CacheSource = "fixtures"
	SourceCache    CacheSource = "cache"
)

// Cache manages cached HTTP responses.
type Cache struct {
	fixturesDir string
	cacheDir    string
	mu          sync.RWMutex
}

// NewCache creates a new Cache instance.
func NewCache(fixturesDir, cacheDir string) *Cache {
	return &Cache{
		fixturesDir: fixturesDir,
		cacheDir:    cacheDir,
	}
}

// Lookup searches for a cached entry. Fixtures take priority over cache.
// Returns (entry, source, error). Entry is nil on cache miss.
func (c *Cache) Lookup(domain, cacheKey string) (*CacheEntry, CacheSource, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Check fixtures first (priority)
	entry, err := c.loadEntry(c.fixturesDir, domain, cacheKey)
	if err == nil {
		return entry, SourceFixtures, nil
	}
	if !errors.Is(err, ErrCacheEntryNotFound) {
		return nil, SourceNone, err
	}

	// Check cache
	entry, err = c.loadEntry(c.cacheDir, domain, cacheKey)
	if err == nil {
		return entry, SourceCache, nil
	}
	if !errors.Is(err, ErrCacheEntryNotFound) {
		return nil, SourceNone, err
	}

	return nil, SourceNone, nil
}

// loadEntry attempts to load a cache entry from a directory.
// Returns ErrCacheEntryNotFound if the entry doesn't exist.
func (c *Cache) loadEntry(baseDir, domain, cacheKey string) (*CacheEntry, error) {
	entry := &CacheEntry{
		Domain:   domain,
		CacheKey: cacheKey,
		BaseDir:  baseDir,
	}

	// Read metadata
	metaPath := entry.MetadataPath()
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrCacheEntryNotFound
		}
		return nil, err
	}

	var metadata CacheEntryMetadata
	if unmarshalErr := json.Unmarshal(metaData, &metadata); unmarshalErr != nil {
		return nil, unmarshalErr
	}
	entry.Metadata = &metadata

	// Read body
	bodyPath := entry.BodyPath()
	bodyData, err := os.ReadFile(bodyPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Metadata exists but body missing - treat as miss
			return nil, ErrCacheEntryNotFound
		}
		return nil, err
	}
	entry.Body = bodyData

	return entry, nil
}

// FixturesDir returns the fixtures directory path.
func (c *Cache) FixturesDir() string {
	return c.fixturesDir
}

// CacheDir returns the cache directory path.
func (c *Cache) CacheDir() string {
	return c.cacheDir
}
