// Package archive provides HTML archiving functionality using MinIO object storage.
package archive

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jonesrussell/north-cloud/crawler/internal/config/minio"
	"github.com/jonesrussell/north-cloud/crawler/internal/logger"
	miniogo "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Archiver handles HTML archiving to MinIO object storage.
type Archiver struct {
	client       *miniogo.Client
	config       *minio.Config
	logger       logger.Interface
	uploadChan   chan *UploadTask
	uploadWorker *UploadWorker
}

const (
	// defaultUploadQueueSize is the default size for the async upload queue.
	defaultUploadQueueSize = 100
)

// NewArchiver creates a new HTML archiver.
func NewArchiver(cfg *minio.Config, log logger.Interface) (*Archiver, error) {
	if cfg == nil {
		return nil, errors.New("minio config is nil")
	}

	archiver := &Archiver{
		config: cfg,
		logger: log,
	}

	// If disabled, return early
	if !cfg.Enabled {
		log.Info("MinIO archiving disabled")
		return archiver, nil
	}

	// Initialize MinIO client
	client, err := miniogo.New(cfg.Endpoint, &miniogo.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		if cfg.FailSilently {
			log.Warn("Failed to create MinIO client, continuing without archiving", "error", err)
			return archiver, nil
		}
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	archiver.client = client

	// Start async upload worker if enabled
	if cfg.UploadAsync {
		archiver.uploadChan = make(chan *UploadTask, defaultUploadQueueSize)
		archiver.uploadWorker = NewUploadWorker(archiver, log)
		archiver.uploadWorker.Start()
		log.Info("Started async MinIO upload worker", "queue_size", defaultUploadQueueSize)
	}

	log.Info("MinIO archiver initialized",
		"endpoint", cfg.Endpoint,
		"bucket", cfg.Bucket,
		"async", cfg.UploadAsync)

	return archiver, nil
}

// Archive archives HTML to MinIO (synchronous or asynchronous based on config).
func (a *Archiver) Archive(ctx context.Context, task *UploadTask) error {
	if !a.config.Enabled || a.client == nil {
		return nil // Archiving disabled or client not initialized
	}

	if task == nil {
		return errors.New("upload task is nil")
	}

	// Async mode: send to worker queue
	if a.config.UploadAsync {
		select {
		case a.uploadChan <- task:
			return nil
		default:
			a.logger.Warn("Upload queue full, dropping task", "url", task.URL)
			if !a.config.FailSilently {
				return errors.New("upload queue full")
			}
			return nil
		}
	}

	// Sync mode: upload immediately
	return a.uploadHTML(ctx, task)
}

// uploadHTML uploads HTML and metadata to MinIO.
func (a *Archiver) uploadHTML(ctx context.Context, task *UploadTask) error {
	// Generate object key
	objectKey := a.generateObjectKey(task)

	// Create metadata
	metadata := a.createMetadata(task, objectKey)

	// Upload HTML file
	htmlReader := bytes.NewReader(task.HTML)
	_, err := a.client.PutObject(
		ctx,
		a.config.Bucket,
		objectKey,
		htmlReader,
		int64(len(task.HTML)),
		miniogo.PutObjectOptions{
			ContentType: "text/html; charset=utf-8",
			UserMetadata: map[string]string{
				"url":         task.URL,
				"source":      task.SourceName,
				"crawled-at":  task.Timestamp.Format(time.RFC3339),
				"status-code": strconv.Itoa(task.StatusCode),
			},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to upload HTML: %w", err)
	}

	a.logger.Debug("Uploaded HTML to MinIO",
		"object_key", objectKey,
		"size", len(task.HTML),
		"url", task.URL)

	// Upload metadata file
	if err2 := a.uploadMetadata(ctx, metadata); err2 != nil {
		a.logger.Warn("Failed to upload metadata, continuing",
			"error", err2,
			"url", task.URL)
		// Don't fail the whole operation if metadata upload fails
	}

	return nil
}

// uploadMetadata uploads metadata JSON to MinIO.
func (a *Archiver) uploadMetadata(ctx context.Context, metadata *HTMLMetadata) error {
	// Generate metadata object key
	metadataKey := strings.Replace(metadata.ObjectKey, ".html", ".meta.json", 1)
	metadataKey = strings.Replace(metadataKey, a.config.Bucket, a.config.MetadataBucket, 1)

	// Serialize metadata to JSON
	jsonData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Upload metadata file
	metadataReader := bytes.NewReader(jsonData)
	_, err = a.client.PutObject(
		ctx,
		a.config.MetadataBucket,
		metadataKey,
		metadataReader,
		int64(len(jsonData)),
		miniogo.PutObjectOptions{
			ContentType: "application/json",
		},
	)
	if err != nil {
		return fmt.Errorf("failed to upload metadata: %w", err)
	}

	a.logger.Debug("Uploaded metadata to MinIO",
		"object_key", metadataKey,
		"size", len(jsonData))

	return nil
}

// generateObjectKey generates a structured object key for the HTML file.
// Format: live/{source_name}/{year}/{month}/{day}/{url_hash}_{timestamp}.html
func (a *Archiver) generateObjectKey(task *UploadTask) string {
	urlHash := hashURL(task.URL)
	timestamp := task.Timestamp.Format("20060102150405")
	year := task.Timestamp.Format("2006")
	month := task.Timestamp.Format("01")
	day := task.Timestamp.Format("02")
	sourceName := sanitizeSourceName(task.SourceName)

	return fmt.Sprintf("live/%s/%s/%s/%s/%s_%s.html",
		sourceName, year, month, day, urlHash, timestamp)
}

// createMetadata creates metadata for the archived HTML.
func (a *Archiver) createMetadata(task *UploadTask, objectKey string) *HTMLMetadata {
	urlHash := hashURL(task.URL)
	contentType := ""
	if ct, ok := task.Headers["Content-Type"]; ok {
		contentType = ct
	}

	return &HTMLMetadata{
		URL:           task.URL,
		URLHash:       urlHash,
		SourceName:    task.SourceName,
		CrawledAt:     task.Timestamp,
		StatusCode:    task.StatusCode,
		ContentType:   contentType,
		ContentLength: int64(len(task.HTML)),
		Headers:       task.Headers,
		ObjectKey:     objectKey,
		// ESIndex and ESDocumentID can be populated by caller if needed
	}
}

// hashURL generates a SHA-256 hash of the URL (first 8 characters).
func hashURL(url string) string {
	h := sha256.Sum256([]byte(url))
	return hex.EncodeToString(h[:])[:8]
}

var (
	// invalidObjectNameChars matches characters that are problematic in MinIO/S3 object names.
	// MinIO/S3 allows most characters, but these can cause issues: control chars, \, ?, *, |, <, >, :, "
	invalidObjectNameChars = regexp.MustCompile(`[\\?*|<>:"\x00-\x1F]`)
	// consecutiveUnderscores matches two or more consecutive underscores.
	consecutiveUnderscores = regexp.MustCompile(`_{2,}`)
)

// sanitizeSourceName sanitizes the source name for use in MinIO object keys.
// MinIO/S3 object names should avoid control characters and certain special characters.
// This function:
// 1. Converts to lowercase
// 2. Replaces problematic characters with underscores
// 3. Replaces dots and spaces with underscores (for consistency)
// 4. Collapses consecutive underscores
// 5. Removes leading/trailing underscores
func sanitizeSourceName(sourceName string) string {
	if sourceName == "" {
		return "unknown"
	}

	// Convert to lowercase first
	normalized := strings.ToLower(sourceName)

	// Replace problematic characters with underscores
	normalized = invalidObjectNameChars.ReplaceAllString(normalized, "_")

	// Replace dots, spaces, and slashes with underscores (for consistency in object keys)
	normalized = strings.ReplaceAll(normalized, ".", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	normalized = strings.ReplaceAll(normalized, "/", "_")

	// Remove any remaining control characters (safety check)
	normalized = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '_'
		}
		return r
	}, normalized)

	// Collapse consecutive underscores into a single underscore
	normalized = consecutiveUnderscores.ReplaceAllString(normalized, "_")

	// Remove leading and trailing underscores
	normalized = strings.Trim(normalized, "_")

	// Handle edge case: if all characters were invalid, return fallback
	if normalized == "" {
		return "unknown"
	}

	return normalized
}

// HealthCheck verifies MinIO connectivity.
func (a *Archiver) HealthCheck(ctx context.Context) error {
	if !a.config.Enabled || a.client == nil {
		return nil // Not enabled, skip health check
	}

	// Check if bucket exists
	exists, err := a.client.BucketExists(ctx, a.config.Bucket)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	if !exists {
		return fmt.Errorf("bucket %s does not exist", a.config.Bucket)
	}

	return nil
}

// Close gracefully shuts down the archiver.
func (a *Archiver) Close() error {
	if a.uploadWorker != nil {
		a.logger.Info("Shutting down MinIO upload worker")
		a.uploadWorker.Stop()
	}
	return nil
}
