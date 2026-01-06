---
description: Run tests for a service
variables:
  - name: SERVICE
    description: Service name (crawler, source-manager, classifier, publisher, index-manager, search, auth)
    default: crawler
---

# Test Service

Runs the test suite for a specific North Cloud service using the Task runner.

## Usage

This command will:
1. Navigate to the service directory
2. Run all unit tests using `task test`
3. Display test results and coverage summary

## Testable Services

- `crawler`
- `source-manager`
- `classifier`
- `publisher`
- `index-manager`
- `search`
- `auth`

## Command

```bash
cd /home/jones/dev/north-cloud/$SERVICE && task test
```

## Example

```bash
# Test the crawler service
SERVICE=crawler
```

## Related Commands

- Use `test-coverage.md` for detailed coverage reports
- Use `test-all.md` to run tests across all services
