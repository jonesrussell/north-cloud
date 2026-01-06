---
description: Lint a service with golangci-lint
variables:
  - name: SERVICE
    description: Service name (crawler, source-manager, classifier, publisher, index-manager, search, auth)
    default: crawler
---

# Lint Service

Runs golangci-lint on a specific North Cloud service to check code quality and style.

## Usage

This command will:
1. Navigate to the service directory
2. Run golangci-lint with project configuration
3. Report any linting issues or style violations

## Lintable Services

- `crawler`
- `source-manager`
- `classifier`
- `publisher`
- `index-manager`
- `search`
- `auth`

## Command

```bash
cd /home/jones/dev/north-cloud/$SERVICE && task lint
```

## Example

```bash
# Lint the publisher service
SERVICE=publisher
```

## What Gets Checked

- Code style and formatting
- Potential bugs and errors
- Code complexity
- Unused code
- Security issues
- Performance issues

## Related Commands

- Use `lint-all.md` to lint all services at once
