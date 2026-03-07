package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	readability "github.com/go-shiori/go-readability"
)

const (
	fetchTimeout            = 30 * time.Second
	maxBodyBytes            = 5 * 1024 * 1024 // 5 MB
	ollamaMaxBodyChars      = 8000            // truncate body before sending to LLM
	llmResponsePreviewChars = 200             // chars shown in parse error messages
)

// handleFetchURL implements the fetch_url tool: fetches a URL, extracts readable content,
// and optionally applies JS rendering or LLM schema extraction.
func (s *Server) handleFetchURL(ctx context.Context, id any, arguments json.RawMessage) *Response {
	var args struct {
		URL           string            `json:"url"`
		JSRender      bool              `json:"js_render"`
		ExtractSchema map[string]string `json:"extract_schema"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return s.errorResponse(id, InvalidParams, "invalid arguments: "+err.Error())
	}
	if args.URL == "" {
		return s.errorResponse(id, InvalidParams, "url is required")
	}
	if _, err := url.ParseRequestURI(args.URL); err != nil {
		return s.errorResponse(id, InvalidParams, "invalid url: "+err.Error())
	}

	// Fetch HTML — via renderer sidecar or plain HTTP.
	var html string
	var fetchErr error
	if args.JSRender {
		if s.rendererURL == "" {
			return s.errorResponse(id, InternalError,
				"js_render requires RENDERER_URL to be configured (Playwright renderer sidecar not available)")
		}
		html, fetchErr = s.fetchRendered(ctx, args.URL)
	} else {
		html, fetchErr = fetchPlain(ctx, args.URL)
	}
	if fetchErr != nil {
		return s.errorResponse(id, InternalError, "fetch failed: "+fetchErr.Error())
	}

	// Extract readable content via go-readability.
	parsedURL, _ := url.Parse(args.URL) // error impossible: URL already validated by url.ParseRequestURI above
	article, readErr := readability.FromReader(strings.NewReader(html), parsedURL)
	if readErr != nil {
		// Fallback: return raw body truncated
		article.Title = args.URL
		article.TextContent = truncateString(html, ollamaMaxBodyChars)
	}

	result := map[string]any{
		"url":        args.URL,
		"title":      strings.TrimSpace(article.Title),
		"body":       strings.TrimSpace(article.TextContent),
		"byline":     article.Byline,
		"excerpt":    article.Excerpt,
		"site_name":  article.SiteName,
		"rendered":   args.JSRender,
		"word_count": len(strings.Fields(article.TextContent)),
	}

	// Schema extraction via Ollama.
	if len(args.ExtractSchema) > 0 {
		if s.ollamaURL == "" {
			return s.errorResponse(id, InternalError,
				"extract_schema requires OLLAMA_URL to be configured")
		}
		extracted, extractErr := s.extractWithOllama(ctx, article.TextContent, args.ExtractSchema)
		if extractErr != nil {
			result["extract_schema_error"] = extractErr.Error()
		} else {
			result["extracted"] = extracted
		}
	}

	return s.successResponse(id, result)
}

// fetchPlain fetches a URL using a plain HTTP GET and returns the response body as a string.
func fetchPlain(ctx context.Context, targetURL string) (string, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, targetURL, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; NorthCloud/1.0; +https://northcloud.one)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	return string(body), nil
}

// fetchRendered calls the Playwright renderer sidecar and returns the rendered HTML.
func (s *Server) fetchRendered(ctx context.Context, targetURL string) (string, error) {
	payload, err := json.Marshal(map[string]any{
		"url":      targetURL,
		"wait_for": "networkidle",
	})
	if err != nil {
		return "", fmt.Errorf("marshal render request: %w", err)
	}

	renderCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(renderCtx, http.MethodPost, s.rendererURL+"/render", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build renderer request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("renderer request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		HTML  string `json:"html"`
		Error string `json:"error"`
	}
	if decodeErr := json.NewDecoder(resp.Body).Decode(&result); decodeErr != nil {
		return "", fmt.Errorf("decode renderer response: %w", decodeErr)
	}
	if result.Error != "" {
		return "", fmt.Errorf("renderer error: %s", result.Error)
	}
	return result.HTML, nil
}

// extractWithOllama calls the Ollama API to extract structured fields from page body text.
func (s *Server) extractWithOllama(ctx context.Context, bodyText string, schema map[string]string) (map[string]any, error) {
	schemaJSON, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal schema: %w", err)
	}

	truncatedBody := truncateString(bodyText, ollamaMaxBodyChars)

	prompt := fmt.Sprintf(`Extract the following fields from the web page content below.
Return ONLY valid JSON with the requested fields. Use null for fields not found.

Fields to extract:
%s

Web page content:
%s

Respond with ONLY the JSON object, no explanation.`, string(schemaJSON), truncatedBody)

	requestBody, err := json.Marshal(map[string]any{
		"model":  s.ollamaModel,
		"prompt": prompt,
		"stream": false,
		"options": map[string]any{
			"temperature": 0,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal ollama request: %w", err)
	}

	ollamaCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ollamaCtx, http.MethodPost, s.ollamaURL+"/api/generate", bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("build ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()

	var ollamaResp struct {
		Response string `json:"response"`
		Error    string `json:"error"`
	}
	if decodeErr := json.NewDecoder(resp.Body).Decode(&ollamaResp); decodeErr != nil {
		return nil, fmt.Errorf("decode ollama response: %w", decodeErr)
	}
	if ollamaResp.Error != "" {
		return nil, fmt.Errorf("ollama error: %s", ollamaResp.Error)
	}

	// Parse the JSON response from the LLM.
	responseText := strings.TrimSpace(ollamaResp.Response)
	// Strip markdown code fences if present.
	responseText = strings.TrimPrefix(responseText, "```json")
	responseText = strings.TrimPrefix(responseText, "```")
	responseText = strings.TrimSuffix(responseText, "```")
	responseText = strings.TrimSpace(responseText)

	var extracted map[string]any
	if parseErr := json.Unmarshal([]byte(responseText), &extracted); parseErr != nil {
		return nil, fmt.Errorf("parse llm json response: %w (response was: %s)", parseErr, truncateString(responseText, llmResponsePreviewChars))
	}
	return extracted, nil
}

// truncateString truncates s to at most maxChars characters.
func truncateString(s string, maxChars int) string {
	runes := []rune(s)
	if len(runes) <= maxChars {
		return s
	}
	return string(runes[:maxChars]) + "…"
}
