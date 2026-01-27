// Package triggers provides event trigger handlers for the V2 scheduler.
package triggers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/scheduler/v2/schedule"
)

const (
	// SignatureHeader is the HTTP header containing the HMAC signature.
	SignatureHeader = "X-Webhook-Signature"

	// TimestampHeader is the HTTP header containing the request timestamp.
	TimestampHeader = "X-Webhook-Timestamp"

	// maxRequestBodySize is the maximum size of webhook request body (1MB).
	maxRequestBodySize = 1 << 20

	// maxTimestampAge is the maximum age of a webhook timestamp (5 minutes).
	maxTimestampAge = 5 * time.Minute
)

var (
	// ErrInvalidSignature is returned when the webhook signature is invalid.
	ErrInvalidSignature = errors.New("invalid webhook signature")

	// ErrMissingSignature is returned when the webhook signature is missing.
	ErrMissingSignature = errors.New("missing webhook signature")

	// ErrTimestampExpired is returned when the webhook timestamp is too old.
	ErrTimestampExpired = errors.New("webhook timestamp expired")

	// ErrInvalidTimestamp is returned when the webhook timestamp is invalid.
	ErrInvalidTimestamp = errors.New("invalid webhook timestamp")

	// ErrRequestTooLarge is returned when the request body is too large.
	ErrRequestTooLarge = errors.New("webhook request body too large")
)

// WebhookPayload represents the payload of a webhook request.
type WebhookPayload struct {
	Event     string         `json:"event"`
	Source    string         `json:"source,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data,omitempty"`
}

// WebhookHandler handles incoming webhook requests.
type WebhookHandler struct {
	secret       []byte
	matcher      *schedule.EventMatcher
	eventHandler schedule.EventHandler
	verifyHMAC   bool
}

// WebhookConfig holds configuration for the webhook handler.
type WebhookConfig struct {
	// Secret is the HMAC secret for signature verification.
	Secret string

	// VerifyHMAC enables HMAC signature verification.
	VerifyHMAC bool
}

// NewWebhookHandler creates a new webhook handler.
func NewWebhookHandler(
	cfg WebhookConfig,
	matcher *schedule.EventMatcher,
	handler schedule.EventHandler,
) *WebhookHandler {
	return &WebhookHandler{
		secret:       []byte(cfg.Secret),
		matcher:      matcher,
		eventHandler: handler,
		verifyHMAC:   cfg.VerifyHMAC,
	}
}

// ServeHTTP implements http.Handler.
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Read and validate request body
	body, err := h.readBody(r)
	if err != nil {
		h.writeError(w, err)
		return
	}

	// Verify signature if enabled
	if h.verifyHMAC {
		if verifyErr := h.verifySignature(r, body); verifyErr != nil {
			h.writeError(w, verifyErr)
			return
		}
	}

	// Parse payload
	var payload WebhookPayload
	if unmarshalErr := json.Unmarshal(body, &payload); unmarshalErr != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Get webhook path
	webhookPath := r.URL.Path

	// Find matching jobs
	jobIDs := h.matcher.MatchWebhook(webhookPath)
	if len(jobIDs) == 0 {
		// No matching jobs, but acknowledge the webhook
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Trigger matched jobs
	var triggeredCount int
	for _, jobID := range jobIDs {
		event := schedule.Event{
			Type:    schedule.EventTypeWebhook,
			Source:  payload.Source,
			Pattern: webhookPath,
			Payload: payload.Data,
		}

		if triggerErr := h.eventHandler(ctx, jobID, event); triggerErr == nil {
			triggeredCount++
		}
	}

	// Write response
	h.writeSuccess(w, triggeredCount)
}

// HandleWebhook processes a webhook request programmatically.
func (h *WebhookHandler) HandleWebhook(
	ctx context.Context,
	path string,
	payload WebhookPayload,
) ([]string, error) {
	jobIDs := h.matcher.MatchWebhook(path)
	if len(jobIDs) == 0 {
		return nil, nil
	}

	triggered := make([]string, 0, len(jobIDs))
	for _, jobID := range jobIDs {
		event := schedule.Event{
			Type:    schedule.EventTypeWebhook,
			Source:  payload.Source,
			Pattern: path,
			Payload: payload.Data,
		}

		if err := h.eventHandler(ctx, jobID, event); err == nil {
			triggered = append(triggered, jobID)
		}
	}

	return triggered, nil
}

// readBody reads the request body with size limit.
func (h *WebhookHandler) readBody(r *http.Request) ([]byte, error) {
	if r.ContentLength > maxRequestBodySize {
		return nil, ErrRequestTooLarge
	}

	reader := io.LimitReader(r.Body, maxRequestBodySize+1)
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	if len(body) > maxRequestBodySize {
		return nil, ErrRequestTooLarge
	}

	return body, nil
}

// verifySignature verifies the HMAC signature of the request.
func (h *WebhookHandler) verifySignature(r *http.Request, body []byte) error {
	signature := r.Header.Get(SignatureHeader)
	if signature == "" {
		return ErrMissingSignature
	}

	// Verify timestamp if provided
	if timestamp := r.Header.Get(TimestampHeader); timestamp != "" {
		ts, parseErr := time.Parse(time.RFC3339, timestamp)
		if parseErr != nil {
			return ErrInvalidTimestamp
		}

		if time.Since(ts) > maxTimestampAge {
			return ErrTimestampExpired
		}
	}

	// Remove "sha256=" prefix if present
	signature = strings.TrimPrefix(signature, "sha256=")

	// Decode expected signature
	expectedSig, decodeErr := hex.DecodeString(signature)
	if decodeErr != nil {
		return ErrInvalidSignature
	}

	// Calculate actual signature
	mac := hmac.New(sha256.New, h.secret)
	mac.Write(body)
	actualSig := mac.Sum(nil)

	// Compare signatures
	if !hmac.Equal(expectedSig, actualSig) {
		return ErrInvalidSignature
	}

	return nil
}

// writeError writes an error response.
func (h *WebhookHandler) writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrMissingSignature), errors.Is(err, ErrInvalidSignature):
		http.Error(w, err.Error(), http.StatusUnauthorized)
	case errors.Is(err, ErrTimestampExpired), errors.Is(err, ErrInvalidTimestamp):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, ErrRequestTooLarge):
		http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
	default:
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// writeSuccess writes a success response.
func (h *WebhookHandler) writeSuccess(w http.ResponseWriter, triggeredCount int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]any{
		"status":    "ok",
		"triggered": triggeredCount,
	}

	if marshalErr := json.NewEncoder(w).Encode(response); marshalErr != nil {
		// Already wrote header, can't change status code
		return
	}
}

// GenerateSignature generates an HMAC signature for a payload.
func GenerateSignature(secret, payload []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
