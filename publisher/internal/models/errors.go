package models

import "errors"

// Common errors
var (
	// ErrNotFound is returned when a resource is not found
	ErrNotFound = errors.New("resource not found")

	// ErrAlreadyExists is returned when a resource already exists (e.g., duplicate name)
	ErrAlreadyExists = errors.New("resource already exists")

	// ErrNoFieldsToUpdate is returned when no fields are provided for an update
	ErrNoFieldsToUpdate = errors.New("no fields to update")

	// ErrInvalidUUID is returned when a UUID is invalid
	ErrInvalidUUID = errors.New("invalid UUID")

	// ErrDuplicateRoute is returned when a route with the same source/channel already exists
	ErrDuplicateRoute = errors.New("route with this source and channel already exists")

	// ErrInvalidQualityScore is returned when quality score is out of range (0-100)
	ErrInvalidQualityScore = errors.New("quality score must be between 0 and 100")
)
