package mcp

import "encoding/json"

// Request represents an MCP JSON-RPC request
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents an MCP JSON-RPC response
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *ErrorObject    `json:"error,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      any         `json:"id"`
	Error   ErrorObject `json:"error"`
}

// ErrorObject represents an error in the response
type ErrorObject struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Error codes
const (
	ParseError       = -32700
	InvalidRequest   = -32600
	MethodNotFound   = -32601
	InvalidParams    = -32602
	InternalError    = -32603
	ResourceNotFound = -32002
)

// Tool represents an MCP tool
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	Scope       ToolScope      `json:"-"`
}

// ToolScope controls which environment a tool is available in.
type ToolScope string

const (
	// ScopeShared tools are available in both local and production.
	ScopeShared ToolScope = "shared"
	// ScopeLocal tools are only available in local development.
	ScopeLocal ToolScope = "local"
	// ScopeProd tools are only available in production.
	ScopeProd ToolScope = "prod"
)

// Environment constants.
const (
	EnvLocal = "local"
	EnvProd  = "prod"
)

// IsAllowed returns true if this scope is permitted in the given environment.
func (s ToolScope) IsAllowed(env string) bool {
	if s == ScopeShared {
		return true
	}
	return string(s) == env
}

// ToolCallParams represents parameters for a tool call
type ToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// Prompt represents an MCP prompt template (for prompts/list and prompts/get).
type Prompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument describes a single argument for a prompt.
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
	Type        string `json:"type,omitempty"` // e.g. "string", "number"
}

// PromptMessage is one message in a prompt (user or assistant).
type PromptMessage struct {
	Role    string          `json:"role"` // "user" or "assistant"
	Content []PromptContent `json:"content"`
}

// PromptContent is one content item (e.g. text block).
type PromptContent struct {
	Type string `json:"type"` // "text"
	Text string `json:"text"`
}

// ResourceListItem is a resource entry for resources/list.
type ResourceListItem struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ResourceContent is one content item for resources/read.
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text,omitempty"`
}
