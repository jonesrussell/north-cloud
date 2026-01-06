---
description: Lint a service (Go or Vue.js/TypeScript)
variables:
  - name: SERVICE
    description: Service name (Go: crawler, source-manager, classifier, publisher, index-manager, search, auth | Frontend: dashboard, search-frontend)
    default: crawler
---

# Lint Service

Runs linting on a specific North Cloud service to check code quality and style. Automatically detects Go services vs Vue.js/TypeScript frontends and runs the appropriate linter.

## Usage

This command will:
1. Navigate to the service directory
2. Detect the project type (Go or Node.js/Vue)
3. Run the appropriate linter:
   - **Go services**: `task lint` (golangci-lint)
   - **Frontend projects**: `npm run lint` (ESLint)
4. Report any linting issues or style violations

## Lintable Services

### Go Services (golangci-lint)

- `crawler`
- `source-manager`
- `classifier`
- `publisher`
- `index-manager`
- `search`
- `auth`

### Frontend Projects (ESLint)

- `dashboard`
- `search-frontend`

## Command

```bash
cd /home/jones/dev/north-cloud/$SERVICE && if [ -f "package.json" ]; then npm run lint; else task lint; fi
```

## Example

```bash
# Lint a Go service
SERVICE=publisher

# Lint the dashboard frontend
SERVICE=dashboard

# Lint the search frontend
SERVICE=search-frontend
```

## What Gets Checked

### Go Services
- Code style and formatting
- Potential bugs and errors
- Code complexity
- Unused code
- Security issues
- Performance issues

### Frontend Projects
- Vue.js best practices
- TypeScript type errors
- ESLint style rules
- Accessibility issues
- Unused imports/variables

## Related Commands

- Use `lint-all.md` to lint all services at once
