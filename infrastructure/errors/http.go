// Package errors provides shared error handling utilities for the North Cloud microservices.
package errors

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	// MinErrorStatusCode is the minimum HTTP status code considered an error
	MinErrorStatusCode = 400
)

// HTTPError represents an HTTP API error response
type HTTPError struct {
	StatusCode int
	Status     string
	Body       string
	Message    string
}

// Error implements the error interface
func (e *HTTPError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("HTTP error (%d %s): %s", e.StatusCode, e.Status, e.Message)
	}
	return fmt.Sprintf("HTTP error: %d %s", e.StatusCode, e.Status)
}

// ParseHTTPError parses an HTTP error response into a structured error.
// It reads the response body and attempts to extract error information.
func ParseHTTPError(resp *http.Response) error {
	if resp.StatusCode < MinErrorStatusCode {
		return nil
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return &HTTPError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Message:    fmt.Sprintf("failed to read error response body: %v", err),
		}
	}

	bodyStr := string(bodyBytes)

	// Try to parse as JSON error response
	var jsonErr struct {
		Error   string `json:"error"`
		Message string `json:"message"`
		Errors  []struct {
			Status string `json:"status"`
			Title  string `json:"title"`
			Detail string `json:"detail"`
		} `json:"errors"`
	}

	if json.Unmarshal(bodyBytes, &jsonErr) == nil {
		// Check if we have a simple error/message format
		if jsonErr.Error != "" || jsonErr.Message != "" {
			msg := jsonErr.Error
			if msg == "" {
				msg = jsonErr.Message
			}
			return &HTTPError{
				StatusCode: resp.StatusCode,
				Status:     resp.Status,
				Body:       bodyStr,
				Message:    msg,
			}
		}

		// Check if we have a JSON:API errors array
		if len(jsonErr.Errors) > 0 {
			errorDetails := make([]string, len(jsonErr.Errors))
			for i, err := range jsonErr.Errors {
				if err.Detail != "" {
					errorDetails[i] = fmt.Sprintf("%s: %s", err.Title, err.Detail)
				} else {
					errorDetails[i] = err.Title
				}
			}
			allErrors := strings.Join(errorDetails, "; ")
			return &HTTPError{
				StatusCode: resp.StatusCode,
				Status:     resp.Status,
				Body:       bodyStr,
				Message:    allErrors,
			}
		}
	}

	// Fallback: return generic error with status code and body
	return &HTTPError{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Body:       bodyStr,
		Message:    bodyStr,
	}
}

// WrapHTTPError wraps an HTTP error with additional context.
func WrapHTTPError(err error, context string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", context, err)
}

// IsHTTPError checks if an error is an HTTPError
func IsHTTPError(err error) bool {
	_, ok := err.(*HTTPError)
	return ok
}

// GetHTTPStatusCode extracts the HTTP status code from an error if it's an HTTPError
func GetHTTPStatusCode(err error) (int, bool) {
	if httpErr, ok := err.(*HTTPError); ok {
		return httpErr.StatusCode, true
	}
	return 0, false
}

