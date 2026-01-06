---
description: Lint all services in parallel
---

# Lint All Services

Runs golangci-lint across all North Cloud services in parallel.

## Usage

This command will:
1. Navigate to the project root
2. Run linting for all services using Task runner
3. Execute linters in parallel for speed
4. Report issues across all services

## Command

```bash
cd /home/jones/dev/north-cloud && task lint
```

## What Gets Checked

- Code style and formatting (gofmt, goimports)
- Potential bugs and errors
- Code complexity (gocyclo)
- Unused code (unused, deadcode)
- Security issues (gosec)
- Performance issues
- Error handling patterns

## Services Linted

- Crawler
- Source Manager
- Classifier
- Publisher
- Index Manager
- Search
- Auth

## Output

Shows linting results for each service with:
- Number of issues found
- Issue severity (warning, error)
- File and line number for each issue

## Before Committing

Run this command to ensure code quality before committing:
```bash
task lint && task test
```

## Fixing Issues

Most formatting issues can be auto-fixed:
```bash
cd /home/jones/dev/north-cloud && task fmt
```
