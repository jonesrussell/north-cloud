package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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

// Store saves a cache entry to the cache directory.
func (c *Cache) Store(entry *CacheEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Ensure domain directory exists
	domainDir := filepath.Join(c.cacheDir, entry.Domain)
	if err := os.MkdirAll(domainDir, 0755); err != nil {
		return err
	}

	// Update entry base dir to cache dir
	entry.BaseDir = c.cacheDir

	// Write metadata
	metaData, err := json.MarshalIndent(entry.Metadata, "", "  ")
	if err != nil {
		return err
	}
	if writeErr := os.WriteFile(entry.MetadataPath(), metaData, 0600); writeErr != nil {
		return writeErr
	}

	// Write body
	if writeErr := os.WriteFile(entry.BodyPath(), entry.Body, 0600); writeErr != nil {
		return writeErr
	}

	return nil
}

// CacheStats holds statistics about the cache.
type CacheStats struct {
	FixturesCount int      `json:"fixtures_count"`
	CacheCount    int      `json:"cache_count"`
	Domains       []string `json:"domains"`
}

// Stats returns statistics about cached entries.
func (c *Cache) Stats() *CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := &CacheStats{
		Domains: make([]string, 0),
	}

	domainSet := make(map[string]bool)

	// Count fixtures
	stats.FixturesCount = c.countEntries(c.fixturesDir, domainSet)

	// Count cache
	stats.CacheCount = c.countEntries(c.cacheDir, domainSet)

	for domain := range domainSet {
		stats.Domains = append(stats.Domains, domain)
	}

	return stats
}

func (c *Cache) countEntries(baseDir string, domainSet map[string]bool) int {
	count := 0

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return 0
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		domain := entry.Name()
		domainSet[domain] = true

		domainPath := filepath.Join(baseDir, domain)
		files, readErr := os.ReadDir(domainPath)
		if readErr != nil {
			continue
		}

		for _, file := range files {
			if filepath.Ext(file.Name()) == ".json" {
				count++
			}
		}
	}

	return count
}
