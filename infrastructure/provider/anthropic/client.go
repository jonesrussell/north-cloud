// Package anthropic provides a shared Anthropic SDK wrapper for Claude API calls.
package anthropic

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const (
	maxRetries       = 3
	baseRetryWait    = 250 * time.Millisecond
	defaultMaxTokens = 1024
	// retryJitterDivisor controls max jitter as a fraction of base delay.
	retryJitterDivisor = 2
)

// GenerateRequest holds the prompt data for a Claude API call.
type GenerateRequest struct {
	SystemPrompt string
	UserPrompt   string
	MaxTokens    int64 // 0 uses defaultMaxTokens
}

// GenerateResponse holds the parsed response from Claude.
type GenerateResponse struct {
	Content      string
	InputTokens  int
	OutputTokens int
}

// Client wraps the Anthropic SDK for Claude API calls.
type Client struct {
	inner anthropicsdk.Client
	model anthropicsdk.Model
}

// New creates a new Anthropic client.
func New(apiKey, model string) *Client {
	return &Client{
		inner: anthropicsdk.NewClient(option.WithAPIKey(apiKey)),
		model: anthropicsdk.Model(model),
	}
}

// Generate sends a prompt to Claude and returns the response text.
// Retries up to maxRetries times on transient errors with exponential backoff.
func (c *Client) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}

	params := anthropicsdk.MessageNewParams{
		Model:     c.model,
		MaxTokens: maxTokens,
		System: []anthropicsdk.TextBlockParam{
			{Text: req.SystemPrompt},
		},
		Messages: []anthropicsdk.MessageParam{
			anthropicsdk.NewUserMessage(
				anthropicsdk.NewTextBlock(req.UserPrompt),
			),
		},
	}

	var lastErr error
	for attempt := range maxRetries {
		if attempt > 0 {
			wait := baseRetryWait * time.Duration(1<<attempt)
			jitter := time.Duration(rand.Int64N(int64(wait / retryJitterDivisor)))
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			case <-time.After(wait + jitter):
			}
		}

		resp, err := c.inner.Messages.New(ctx, params)
		if err == nil {
			if len(resp.Content) == 0 {
				return nil, errors.New("empty response from Claude")
			}
			return &GenerateResponse{
				Content:      resp.Content[0].Text,
				InputTokens:  int(resp.Usage.InputTokens),
				OutputTokens: int(resp.Usage.OutputTokens),
			}, nil
		}

		lastErr = err
	}

	return nil, fmt.Errorf("claude API failed after %d retries: %w", maxRetries, lastErr)
}
