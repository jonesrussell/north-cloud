// Package domain provides domain models used across the application.
package domain

// Type represents the type of content being processed.
type Type string

// Type constants are defined as needed by consumers.
// The Type type itself is used by content.Item to represent content types.

// These types are defined as any to avoid import cycles.
// They will be used by other packages that need these types.
type (
	// Config represents the configuration interface.
	Config any

	// Storage represents the storage interface.
	Storage any
)
