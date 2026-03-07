// Package provider defines the LLMProvider interface for AI callouts.
package provider

import "context"

// GenerateRequest is the input to an LLM call.
type GenerateRequest struct {
	SystemPrompt string
	UserPrompt   string
	MaxTokens    int
	// JSONSchema is an optional JSON schema string to enforce structured output.
	// Leave empty if the provider does not support structured output.
	JSONSchema string
}

// GenerateResponse is the output from an LLM call.
type GenerateResponse struct {
	Content      string
	InputTokens  int
	OutputTokens int
}

// LLMProvider is the interface all AI provider implementations must satisfy.
type LLMProvider interface {
	// Name returns the provider name (e.g. "anthropic", "openai").
	Name() string
	// Generate sends a prompt and returns a response.
	Generate(ctx context.Context, req GenerateRequest) (GenerateResponse, error)
}
