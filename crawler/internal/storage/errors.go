package storage

import "errors"

var (
	// ErrInvalidIndexHealth indicates the index health is invalid
	ErrInvalidIndexHealth = errors.New("invalid index health format")
	// ErrInvalidDocCount indicates the index document count is invalid
	ErrInvalidDocCount = errors.New("invalid index document count format")
	// ErrIndexNotFound indicates the requested index does not exist
	ErrIndexNotFound = errors.New("index not found")
)
