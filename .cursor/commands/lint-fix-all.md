---
description: Auto-fix linting issues across all Go services
---

# Fix Linting Issues Across All Services

Automatically fixes auto-fixable linting issues across all North Cloud Go services using golangci-lint.

## Usage

This command will:
1. Navigate to each service directory
2. Run `golangci-lint run --fix` in each service (golangci-lint must run from within each Go module)
3. Automatically fix all auto-fixable issues
4. Report remaining issues that require manual fixes

## Command

```bash
cd /home/jones/dev/north-cloud && for service in auth crawler classifier publisher search source-manager index-manager mcp-north-cloud; do echo "=== Fixing $service ===" && cd "$service" && golangci-lint run --fix && cd ..; done
```

## What Gets Fixed

The `--fix` flag automatically fixes:
- Formatting issues (goimports, gofmt)
- Import organization
- Whitespace issues
- Some style violations
- Unused imports (removed)

## What Requires Manual Fix

Some issues cannot be auto-fixed and require manual intervention:
- Logic errors
- Missing error handling
- Complex refactoring
- Security issues
- Performance improvements

## Services Processed

- Auth
- Crawler
- Classifier
- Publisher
- Search
- Source Manager
- Index Manager
- MCP North Cloud

## Workflow

1. **Run auto-fix**:
   ```bash
   cd /home/jones/dev/north-cloud && for service in auth crawler classifier publisher search source-manager index-manager mcp-north-cloud; do echo "=== Fixing $service ===" && cd "$service" && golangci-lint run --fix && cd ..; done
   ```

2. **Check remaining issues**:
   ```bash
   task lint
   ```

3. **Manually fix remaining issues** based on the lint output

4. **Verify all issues resolved**:
   ```bash
   task lint
   ```

## Notes

- Each service must be linted from within its own directory (where `go.mod` exists)
- The consolidated `.golangci.yml` at the root will be used automatically
- Some linting issues may require code changes beyond auto-fix capabilities
- Run tests after auto-fixing to ensure nothing was broken: `task test`

## Related Commands

- Use `lint-all.md` to check for linting issues without fixing
- Use `lint-service.md` to lint a specific service
- Use `test-all.md` to verify tests pass after fixes
