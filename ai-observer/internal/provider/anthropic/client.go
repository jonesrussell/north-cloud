// Package anthropic adapts the shared infrastructure Anthropic client
// to the ai-observer's LLMProvider interface.
package anthropic

import (
	"context"
	"fmt"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/provider"
	infraanthropic "github.com/jonesrussell/north-cloud/infrastructure/provider/anthropic"
)

const providerName = "anthropic"

// Client wraps the shared infrastructure Anthropic client
// and adapts it to the ai-observer's LLMProvider interface.
type Client struct {
	inner *infraanthropic.Client
}

// New creates a new Anthropic Client with the given API key and model name.
func New(apiKey, model string) *Client {
	return &Client{
		inner: infraanthropic.New(apiKey, model),
	}
}

// Name returns the provider name.
func (c *Client) Name() string {
	return providerName
}

// Generate sends a prompt to the Anthropic Messages API and returns the response.
func (c *Client) Generate(ctx context.Context, req provider.GenerateRequest) (provider.GenerateResponse, error) {
	systemPrompt := buildSystemPrompt(req.SystemPrompt, req.JSONSchema)

	resp, err := c.inner.Generate(ctx, infraanthropic.GenerateRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   req.UserPrompt,
		MaxTokens:    int64(req.MaxTokens),
	})
	if err != nil {
		return provider.GenerateResponse{}, fmt.Errorf("anthropic generate: %w", err)
	}

	return provider.GenerateResponse{
		Content:      resp.Content,
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
	}, nil
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
