package main

import (
	"path/filepath"
	"time"
)

// CachedRequest represents the request portion of a cache entry.
type CachedRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

// CachedResponse represents the response portion of a cache entry.
type CachedResponse struct {
	Status        int               `json:"status"`
	Headers       map[string]string `json:"headers"`
	WasCompressed bool              `json:"was_compressed"`
}

// CacheEntryMetadata is the JSON metadata stored alongside cached responses.
type CacheEntryMetadata struct {
	Request    CachedRequest  `json:"request"`
	Response   CachedResponse `json:"response"`
	RecordedAt time.Time      `json:"recorded_at"`
	CacheKey   string         `json:"cache_key"`
}

// CacheEntry represents a cached request/response pair.
type CacheEntry struct {
	Domain   string
	CacheKey string
	BaseDir  string
	Metadata *CacheEntryMetadata
	Body     []byte
}

// MetadataPath returns the path to the .json metadata file.
func (e *CacheEntry) MetadataPath() string {
	return filepath.Join(e.BaseDir, e.Domain, e.CacheKey+".json")
}

// BodyPath returns the path to the .body file.
func (e *CacheEntry) BodyPath() string {
	return filepath.Join(e.BaseDir, e.Domain, e.CacheKey+".body")
}
