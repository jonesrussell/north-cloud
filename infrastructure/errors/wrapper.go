// Package errors provides shared error handling utilities for the North Cloud microservices.
package errors

import "fmt"

// WrapWithContext wraps an error with additional context information.
// This provides consistent error wrapping across services.
func WrapWithContext(err error, context string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", context, err)
}

// WrapWithContextf wraps an error with formatted context information.
func WrapWithContextf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	context := fmt.Sprintf(format, args...)
	return fmt.Errorf("%s: %w", context, err)
}

