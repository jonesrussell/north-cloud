package producer

// This file exports unexported identifiers for use by tests in the
// producer_test package. Only compiled during `go test`.

// CheckpointFileMode is the test-only alias for checkpointFileMode.
var CheckpointFileMode = checkpointFileMode

// BuildQuery is the test-only alias for buildQuery.
var BuildQuery = buildQuery
