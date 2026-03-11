package aiverify

import (
	"context"

	"github.com/jonesrussell/north-cloud/infrastructure/provider/anthropic"
)

// LLMVerifier calls Claude to verify records.
type LLMVerifier struct {
	client *anthropic.Client
}

// NewLLMVerifier creates a new LLM-backed verifier.
func NewLLMVerifier(client *anthropic.Client) *LLMVerifier {
	return &LLMVerifier{client: client}
}

// Verify sends a record to Claude and parses the response.
func (v *LLMVerifier) Verify(ctx context.Context, input VerifyInput) (*VerifyResult, error) {
	resp, err := v.client.Generate(ctx, anthropic.GenerateRequest{
		SystemPrompt: SystemPrompt,
		UserPrompt:   BuildUserPrompt(input),
	})
	if err != nil {
		return nil, err
	}
	return ParseVerifyResponse(resp.Content)
}
