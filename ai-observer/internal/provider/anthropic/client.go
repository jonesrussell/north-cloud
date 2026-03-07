// Package anthropic implements the LLMProvider interface using the Anthropic API.
package anthropic

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/provider"
)

const (
	providerName = "anthropic"
	// maxRetries is the number of attempts for rate-limited requests.
	maxRetries = 3
	// retryBaseDelay is the initial backoff delay before jitter.
	retryBaseDelay = 250 * time.Millisecond
	// rateLimitStatus is the HTTP status code for rate limiting.
	rateLimitStatus = 429
	// retryJitterDivisor controls max jitter as a fraction of base delay (base/2 = up to 50%).
	retryJitterDivisor = 2
)

// Client wraps the Anthropic SDK client and implements provider.LLMProvider.
type Client struct {
	inner anthropicsdk.Client
	model anthropicsdk.Model
}

// New creates a new Anthropic Client with the given API key and model name.
func New(apiKey, model string) *Client {
	return &Client{
		inner: anthropicsdk.NewClient(option.WithAPIKey(apiKey)),
		model: anthropicsdk.Model(model),
	}
}

// Name returns the provider name.
func (c *Client) Name() string {
	return providerName
}

// Generate sends a prompt to the Anthropic Messages API and returns the response.
// Retries up to maxRetries times on 429 rate-limit responses with exponential backoff and jitter.
func (c *Client) Generate(ctx context.Context, req provider.GenerateRequest) (provider.GenerateResponse, error) {
	params := buildParams(c.model, req)

	var lastErr error
	for attempt := range maxRetries {
		if attempt > 0 {
			delay := retryDelay(attempt)
			select {
			case <-ctx.Done():
				return provider.GenerateResponse{}, fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			case <-time.After(delay):
			}
		}

		msg, err := c.inner.Messages.New(ctx, params)
		if err == nil {
			return provider.GenerateResponse{
				Content:      extractText(msg.Content),
				InputTokens:  int(msg.Usage.InputTokens),
				OutputTokens: int(msg.Usage.OutputTokens),
			}, nil
		}

		if !isRateLimit(err) {
			return provider.GenerateResponse{}, fmt.Errorf("anthropic generate: %w", err)
		}
		lastErr = err
	}

	return provider.GenerateResponse{}, fmt.Errorf("anthropic generate: rate limited after %d attempts: %w", maxRetries, lastErr)
}

// retryDelay returns the backoff duration for the given attempt (1-indexed).
// Delay doubles each attempt: 250ms, 500ms, 1s — plus up to 50% random jitter.
func retryDelay(attempt int) time.Duration {
	base := retryBaseDelay * (1 << attempt)
	jitter := time.Duration(rand.Int64N(int64(base / retryJitterDivisor)))
	return base + jitter
}

// isRateLimit reports whether err is an Anthropic 429 rate-limit error.
func isRateLimit(err error) bool {
	var apiErr *anthropicsdk.Error
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == rateLimitStatus
	}
	return false
}

// buildParams constructs MessageNewParams from a GenerateRequest.
// If JSONSchema is set, it is appended to the system prompt so the model
// knows the exact output structure required.
func buildParams(model anthropicsdk.Model, req provider.GenerateRequest) anthropicsdk.MessageNewParams {
	params := anthropicsdk.MessageNewParams{
		Model:     model,
		MaxTokens: int64(req.MaxTokens),
		Messages: []anthropicsdk.MessageParam{
			anthropicsdk.NewUserMessage(
				anthropicsdk.NewTextBlock(req.UserPrompt),
			),
		},
	}

	systemText := buildSystemPrompt(req.SystemPrompt, req.JSONSchema)
	if systemText != "" {
		params.System = []anthropicsdk.TextBlockParam{
			{Text: systemText},
		}
	}

	return params
}

// buildSystemPrompt assembles the final system prompt, appending the JSON schema
// constraint when provided.
func buildSystemPrompt(base, jsonSchema string) string {
	if jsonSchema == "" {
		return base
	}
	if base == "" {
		return "Output must conform to the following JSON schema:\n" + jsonSchema
	}
	return base + "\n\nOutput must conform to the following JSON schema:\n" + jsonSchema
}

// extractText returns the concatenated text from all text content blocks.
func extractText(blocks []anthropicsdk.ContentBlockUnion) string {
	var sb strings.Builder
	for i := range blocks {
		if blocks[i].Type == "text" {
			sb.WriteString(blocks[i].Text)
		}
	}
	return sb.String()
}
