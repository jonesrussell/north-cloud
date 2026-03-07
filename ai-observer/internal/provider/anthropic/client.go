// Package anthropic implements the LLMProvider interface using the Anthropic API.
package anthropic

import (
	"context"
	"fmt"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/provider"
)

const providerName = "anthropic"

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
func (c *Client) Generate(ctx context.Context, req provider.GenerateRequest) (provider.GenerateResponse, error) {
	params := buildParams(c.model, req)

	msg, err := c.inner.Messages.New(ctx, params)
	if err != nil {
		return provider.GenerateResponse{}, fmt.Errorf("anthropic generate: %w", err)
	}

	text := extractText(msg.Content)

	return provider.GenerateResponse{
		Content:      text,
		InputTokens:  int(msg.Usage.InputTokens),
		OutputTokens: int(msg.Usage.OutputTokens),
	}, nil
}

// buildParams constructs MessageNewParams from a GenerateRequest.
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

	if req.SystemPrompt != "" {
		params.System = []anthropicsdk.TextBlockParam{
			{Text: req.SystemPrompt},
		}
	}

	return params
}

// extractText returns the concatenated text from all text content blocks.
func extractText(blocks []anthropicsdk.ContentBlockUnion) string {
	var text string
	for _, block := range blocks {
		if block.Type == "text" {
			text += block.Text
		}
	}
	return text
}
