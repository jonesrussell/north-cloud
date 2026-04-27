package mapper

// This file exports unexported helpers for use by tests in the mapper_test
// package. Only compiled during `go test`.

// StringFromPath is the test-only alias for stringFromPath.
var StringFromPath = stringFromPath

// FirstStringInSlice is the test-only alias for firstStringInSlice.
var FirstStringInSlice = firstStringInSlice

// OptionalStringFromPath is the test-only alias for optionalStringFromPath.
var OptionalStringFromPath = optionalStringFromPath

// IntFromPath is the test-only alias for intFromPath.
var IntFromPath = intFromPath
