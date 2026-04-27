package client

// This file exports unexported identifiers for use by tests in the
// client_test package. Build-tag *_test.go files are only compiled during
// `go test`, so these symbols are not exported in the production binary.

// ErrClient is the test-only alias for the unexported errClient sentinel.
var ErrClient = errClient

// ErrServer is the test-only alias for the unexported errServer sentinel.
var ErrServer = errServer

// Retry is the test-only alias for the unexported retry helper.
var Retry = retry

// RetryOp is the test-only alias for the unexported retryOp type.
type RetryOp = retryOp
