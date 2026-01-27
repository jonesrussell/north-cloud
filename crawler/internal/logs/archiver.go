package logs

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/config/minio"
	miniogo "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	logContentType = "application/gzip"
	logPathFormat  = "logs/%s/%d.log.gz" // logs/{job_id}/{execution_number}.log.gz
)

// logArchiver implements Archiver for MinIO log storage.
type logArchiver struct {
	client *miniogo.Client
	bucket string
	logger infralogger.Logger
}

// NewArchiver creates a new log archiver using the provided MinIO config.
func NewArchiver(cfg *minio.Config, bucket string, logger infralogger.Logger) (Archiver, error) {
	if cfg == nil || !cfg.Enabled {
		logger.Info("Log archiver disabled (MinIO not configured)")
		return &noopArchiver{}, nil
	}

	client, err := miniogo.New(cfg.Endpoint, &miniogo.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	// Use provided bucket or fallback to config bucket
	if bucket == "" {
		bucket = cfg.Bucket
	}

	logger.Info("Log archiver initialized",
		infralogger.String("endpoint", cfg.Endpoint),
		infralogger.String("bucket", bucket),
	)

	return &logArchiver{
		client: client,
		bucket: bucket,
		logger: logger,
	}, nil
}

// Archive uploads logs to MinIO and returns the object key.
func (a *logArchiver) Archive(ctx context.Context, task *ArchiveTask) (string, error) {
	// Generate object key
	objectKey := fmt.Sprintf(logPathFormat, task.JobID, task.ExecutionNumber)

	// Compress content if not already compressed
	compressed, err := a.compressIfNeeded(task.Content)
	if err != nil {
		return "", fmt.Errorf("failed to compress logs: %w", err)
	}

	// Upload to MinIO
	reader := bytes.NewReader(compressed)
	_, err = a.client.PutObject(
		ctx,
		a.bucket,
		objectKey,
		reader,
		int64(len(compressed)),
		miniogo.PutObjectOptions{
			ContentType: logContentType,
			UserMetadata: map[string]string{
				"job_id":           task.JobID,
				"execution_id":     task.ExecutionID,
				"execution_number": strconv.Itoa(task.ExecutionNumber),
				"line_count":       strconv.Itoa(task.LineCount),
				"started_at":       task.StartedAt.Format(time.RFC3339),
				"archived_at":      time.Now().Format(time.RFC3339),
			},
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to upload logs to MinIO: %w", err)
	}

	a.logger.Debug("Archived job logs",
		infralogger.String("object_key", objectKey),
		infralogger.Int("size_bytes", len(compressed)),
		infralogger.String("job_id", task.JobID),
		infralogger.Int("line_count", task.LineCount),
	)

	return objectKey, nil
}

// GetObject retrieves an archived log file from MinIO.
func (a *logArchiver) GetObject(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	object, err := a.client.GetObject(ctx, a.bucket, objectKey, miniogo.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object from MinIO: %w", err)
	}
	return object, nil
}

// Close is a no-op for the archiver.
func (a *logArchiver) Close() error {
	return nil
}

// compressIfNeeded gzips the content.
func (a *logArchiver) compressIfNeeded(content []byte) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(content); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// noopArchiver is a no-op implementation when MinIO is disabled.
type noopArchiver struct{}

func (a *noopArchiver) Archive(_ context.Context, _ *ArchiveTask) (string, error) {
	return "", nil
}

// ErrArchiverDisabled is returned when attempting to use a disabled archiver.
var ErrArchiverDisabled = errors.New("log archiver is disabled")

func (a *noopArchiver) GetObject(_ context.Context, _ string) (io.ReadCloser, error) {
	return nil, ErrArchiverDisabled
}

func (a *noopArchiver) Close() error {
	return nil
}
