// Package mcp is tested here to assert internal handler behavior (routeToolCall, handlePromptsList, etc.).
//
//nolint:testpackage // we need to call unexported routeToolCall and handle* methods for unit tests
package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestRouteToolCall_UnknownTool_ReturnsMethodNotFound(t *testing.T) {
	t.Helper()
	s := NewServer("local", nil, nil, nil, nil, nil, nil, nil)
	ctx := context.Background()
	id := "test-id"
	toolName := "nonexistent_tool"
	arguments := json.RawMessage(`{}`)

	resp := s.routeToolCall(ctx, id, toolName, arguments)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Error == nil {
		t.Fatal("expected error response for unknown tool")
	}
	if resp.Error.Code != MethodNotFound {
		t.Errorf("expected error code %d (MethodNotFound), got %d", MethodNotFound, resp.Error.Code)
	}
	if !strings.Contains(resp.Error.Message, "Unknown tool") {
		t.Errorf("expected message containing 'Unknown tool', got %q", resp.Error.Message)
	}
}

func TestHandleInitialize_IncludesCapabilities(t *testing.T) {
	t.Helper()
	s := NewServer("local", nil, nil, nil, nil, nil, nil, nil)
	req := &Request{JSONRPC: "2.0", ID: "1", Method: "initialize", Params: json.RawMessage(`{}`)}
	resp := s.HandleRequestWithContext(context.Background(), req)
	if resp == nil || resp.Result == nil {
		t.Fatal("expected non-nil result")
	}
	var result struct {
		Capabilities map[string]any `json:"capabilities"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Capabilities == nil {
		t.Fatal("expected capabilities in result")
	}
	if _, ok := result.Capabilities["tools"]; !ok {
		t.Error("expected capabilities.tools")
	}
	prompts, _ := result.Capabilities["prompts"].(map[string]any)
	if prompts == nil {
		t.Error("expected capabilities.prompts")
	}
	resources, _ := result.Capabilities["resources"].(map[string]any)
	if resources == nil {
		t.Error("expected capabilities.resources")
	}
}

func TestHandlePromptsList_ReturnsPrompts(t *testing.T) {
	t.Helper()
	s := NewServer("local", nil, nil, nil, nil, nil, nil, nil)
	req := &Request{JSONRPC: "2.0", ID: "1", Method: "prompts/list", Params: json.RawMessage(`{}`)}
	resp := s.handlePromptsList(req, "1")
	if resp == nil || resp.Result == nil {
		t.Fatal("expected non-nil result")
	}
	var result struct {
		Prompts []Prompt `json:"prompts"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	const expectedPrompts = 4
	if n := len(result.Prompts); n != expectedPrompts {
		t.Errorf("expected %d prompts, got %d", expectedPrompts, n)
	}
}

func TestHandlePromptsGet_ValidName_ReturnsMessages(t *testing.T) {
	t.Helper()
	s := NewServer("local", nil, nil, nil, nil, nil, nil, nil)
	params := `{"name":"debug_crawl_job","arguments":{"job_id":"test-job-123"}}`
	req := &Request{JSONRPC: "2.0", ID: "1", Method: "prompts/get", Params: json.RawMessage(params)}
	resp := s.handlePromptsGet(req, "1")
	if resp == nil || resp.Result == nil {
		t.Fatal("expected non-nil result")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	var result struct {
		Messages []PromptMessage `json:"messages"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.Messages) == 0 {
		t.Error("expected at least one message")
	}
}

func TestHandlePromptsGet_UnknownName_ReturnsInvalidParams(t *testing.T) {
	t.Helper()
	s := NewServer("local", nil, nil, nil, nil, nil, nil, nil)
	params := `{"name":"nonexistent_prompt","arguments":{}}`
	req := &Request{JSONRPC: "2.0", ID: "1", Method: "prompts/get", Params: json.RawMessage(params)}
	resp := s.handlePromptsGet(req, "1")
	if resp == nil || resp.Error == nil {
		t.Fatal("expected error for unknown prompt name")
	}
	if resp.Error.Code != InvalidParams {
		t.Errorf("expected InvalidParams, got %d", resp.Error.Code)
	}
}

func TestHandlePromptsGet_MissingRequiredArgs_ReturnsInvalidParams(t *testing.T) {
	t.Helper()
	s := NewServer("local", nil, nil, nil, nil, nil, nil, nil)
	params := `{"name":"debug_crawl_job","arguments":{}}`
	req := &Request{JSONRPC: "2.0", ID: "1", Method: "prompts/get", Params: json.RawMessage(params)}
	resp := s.handlePromptsGet(req, "1")
	if resp == nil || resp.Error == nil {
		t.Fatal("expected error when required argument job_id is missing")
	}
	if resp.Error.Code != InvalidParams {
		t.Errorf("expected InvalidParams, got %d", resp.Error.Code)
	}
	if !strings.Contains(resp.Error.Message, "missing required") {
		t.Errorf("expected message to mention missing required, got %q", resp.Error.Message)
	}
}

func TestHandleResourcesList_ReturnsResources(t *testing.T) {
	t.Helper()
	s := NewServer("local", nil, nil, nil, nil, nil, nil, nil)
	req := &Request{JSONRPC: "2.0", ID: "1", Method: "resources/list", Params: json.RawMessage(`{}`)}
	resp := s.handleResourcesList(req, "1")
	if resp == nil || resp.Result == nil {
		t.Fatal("expected non-nil result")
	}
	var result struct {
		Resources []ResourceListItem `json:"resources"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.Resources) == 0 {
		t.Error("expected at least one resource")
	}
}

func TestHandleResourcesRead_ValidURI_ReturnsContents(t *testing.T) {
	t.Helper()
	s := NewServer("local", nil, nil, nil, nil, nil, nil, nil)
	params := `{"uri":"northcloud://docs/pipeline"}`
	req := &Request{JSONRPC: "2.0", ID: "1", Method: "resources/read", Params: json.RawMessage(params)}
	resp := s.handleResourcesRead(req, "1")
	if resp == nil || resp.Result == nil {
		t.Fatal("expected non-nil result")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	var result struct {
		Contents []ResourceContent `json:"contents"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.Contents) == 0 || result.Contents[0].Text == "" {
		t.Error("expected non-empty content text")
	}
}

func TestHandleResourcesRead_UnknownURI_ReturnsResourceNotFound(t *testing.T) {
	t.Helper()
	s := NewServer("local", nil, nil, nil, nil, nil, nil, nil)
	params := `{"uri":"northcloud://docs/nonexistent"}`
	req := &Request{JSONRPC: "2.0", ID: "1", Method: "resources/read", Params: json.RawMessage(params)}
	resp := s.handleResourcesRead(req, "1")
	if resp == nil || resp.Error == nil {
		t.Fatal("expected error for unknown URI")
	}
	if resp.Error.Code != ResourceNotFound {
		t.Errorf("expected ResourceNotFound (%d), got %d", ResourceNotFound, resp.Error.Code)
	}
}
