# Testing Guide for Source Manager

This document describes the testing strategy and setup for the source-manager microservice.

## Overview

The source-manager service uses modern Go 1.25 testing practices with a focus on:
- **Unit tests**: Fast, isolated tests that don't require external dependencies
- **Integration tests**: Tests that use real database connections (can be skipped with `-short` flag)
- **Test helpers**: Reusable utilities for common test scenarios

## Running Tests

### All Tests

```bash
# Run all tests
task test

# Or directly with go
go test -mod=mod ./...
```

### Unit Tests Only (Fast)

```bash
# Run only unit tests (skips integration tests)
go test -mod=mod -short ./...
```

### Coverage

```bash
# Generate coverage report
task test:coverage

# This generates:
# - coverage.out (raw coverage data)
# - coverage.html (HTML report)
```

### Race Detection

```bash
# Run tests with race detector
task test:race
```

## Test Structure

### Test Packages

Tests are organized alongside the code they test:

```
internal/
├── models/
│   ├── source.go
│   └── source_test.go         # Model tests
├── config/
│   ├── config.go
│   └── config_test.go         # Config tests
├── repository/
│   ├── source.go
│   └── source_test.go         # Repository integration tests
├── handlers/
│   ├── source.go
│   └── source_test.go         # Handler tests (requires interface refactoring)
└── testhelpers/
    ├── logger.go              # Test logger utilities
    └── database.go            # Database test helpers
```

## Test Categories

### Unit Tests

Unit tests are fast and don't require external dependencies:

- **Models** (`internal/models/`): Test data structures, JSON marshaling, validation
- **Config** (`internal/config/`): Test configuration loading, validation, environment overrides

These tests run in all environments and are included when using `-short`.

### Integration Tests

Integration tests require a database connection:

- **Repository** (`internal/repository/`): Test database operations with real PostgreSQL

**Note**: Integration tests are skipped when running with `-short` flag. They require:
- A PostgreSQL database accessible at `localhost:5432`
- Test database named `gosources_test`
- User `postgres` with password `postgres` (or modify connection string in tests)

To run integration tests:

```bash
# Create test database (one-time setup)
createdb -U postgres gosources_test

# Run integration tests
go test -mod=mod ./internal/repository/...
```

### Handler Tests

Handler tests are currently structured but require refactoring the handler to use interfaces for proper mocking. The test structure demonstrates the approach:

- Uses `testify/mock` for repository mocking
- Tests HTTP request/response handling
- Validates status codes and response bodies

**Future Work**: Refactor handlers to accept repository interface for better testability.

## Test Helpers

### Logger Helper

`testhelpers.NewTestLogger()` creates a logger that discards output, suitable for tests:

```go
import "github.com/jonesrussell/gosources/internal/testhelpers"

logger := testhelpers.NewTestLogger()
```

### Database Helper

`testhelpers.RunMigrations()` runs database migrations for test database setup:

```go
import "github.com/jonesrussell/gosources/internal/testhelpers"

ctx := context.Background()
err := testhelpers.RunMigrations(ctx, db, logger)
```

## Modern Go 1.25 Testing Features

### Table-Driven Tests

All tests use table-driven test patterns for multiple scenarios:

```go
tests := []struct {
    name           string
    input          string
    expectedOutput string
    wantErr        bool
}{
    {
        name:           "valid input",
        input:          "test",
        expectedOutput: "TEST",
        wantErr:        false,
    },
    // ... more test cases
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        // Test implementation
    })
}
```

### Subtests with t.Run

Tests use subtests for better organization and selective running:

```bash
# Run specific subtest
go test -run TestStringArray_Value/valid_array
```

### Testify for Assertions

We use `testify` for cleaner assertions:

```go
import (
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// Use require for assertions that should stop the test on failure
require.NoError(t, err)

// Use assert for assertions that allow test to continue
assert.Equal(t, expected, actual)
```

## Best Practices

1. **Keep tests fast**: Unit tests should run in milliseconds
2. **Use -short flag**: Integration tests should check `testing.Short()` and skip when appropriate
3. **Clean up resources**: Use `defer` or `t.Cleanup()` for cleanup
4. **Meaningful test names**: Use descriptive names like `TestRepository_Create_WithValidSource`
5. **Table-driven tests**: Use for multiple scenarios
6. **Test error cases**: Test both success and failure paths
7. **Isolate tests**: Tests should not depend on each other or shared state

## Coverage Goals

- **Models**: 90%+ coverage (pure logic, easy to test)
- **Config**: 85%+ coverage (configuration loading)
- **Repository**: 80%+ coverage (database operations)
- **Handlers**: TBD (requires interface refactoring)

Current coverage can be checked with:

```bash
task test:coverage
open coverage.html
```

## CI/CD Integration

Tests should run in CI/CD with:

```yaml
# Example GitHub Actions
- name: Run unit tests
  run: go test -mod=mod -short ./...

- name: Run integration tests
  run: |
    # Setup test database
    # Run integration tests
    go test -mod=mod ./internal/repository/...

- name: Generate coverage
  run: go test -mod=mod -coverprofile=coverage.out ./...
```

## Future Improvements

1. **Testcontainers Integration**: Add `testcontainers-go` for automatic PostgreSQL setup in CI/CD
2. **Handler Interface Refactoring**: Refactor handlers to use repository interface for proper mocking
3. **Golden Files**: Use golden files for complex JSON response testing
4. **Property-Based Testing**: Consider `testing/quick` for property-based tests
5. **Benchmark Tests**: Add benchmarks for performance-critical paths
6. **Fuzzing**: Add fuzz tests for input validation

## Troubleshooting

### Tests Skip with Database Connection Error

Integration tests skip when database is unavailable. To run them:

1. Ensure PostgreSQL is running
2. Create test database: `createdb -U postgres gosources_test`
3. Run migrations manually or let tests handle it
4. Ensure connection string matches your setup

### Vendor Directory Issues

If you see vendor-related errors, use `-mod=mod`:

```bash
go test -mod=mod ./...
```

Or sync vendor:

```bash
go mod vendor
```

### Race Detector Finds Issues

Race conditions indicate shared mutable state. Review the code to:
- Use proper synchronization (mutexes, channels)
- Avoid shared state where possible
- Pass data by value instead of sharing pointers

