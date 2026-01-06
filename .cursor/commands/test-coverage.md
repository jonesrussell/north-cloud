---
description: Run tests with coverage report
variables:
  - name: SERVICE
    description: Service name (crawler, source-manager, classifier, publisher, index-manager, search, auth)
    default: crawler
---

# Test with Coverage

Runs the full test suite with code coverage analysis and generates an HTML report.

## Usage

This command will:
1. Navigate to the service directory
2. Run all tests with coverage tracking
3. Generate coverage percentage
4. Create HTML coverage report
5. Open report in browser (if supported)

## Command

```bash
cd /home/jones/dev/north-cloud/$SERVICE && task test:coverage
```

## Example

```bash
# Get coverage report for classifier
SERVICE=classifier
```

## Output

- Console: Coverage percentage by package
- File: `coverage.out` (coverage data)
- File: `coverage.html` (visual report)

## Coverage Report Shows

- Which lines of code are covered by tests
- Which functions lack test coverage
- Branch coverage for conditionals
- Overall package coverage percentage

## Target Coverage

The project aims for 80%+ test coverage across all services.

## Related Commands

- Use `test-service.md` for quick test runs without coverage
- Use `test-all.md` to check coverage across all services
