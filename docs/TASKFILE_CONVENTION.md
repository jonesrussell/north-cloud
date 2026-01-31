# Taskfile convention

Each service has its own `Taskfile.yml`. The root `Taskfile.yml` only delegates to those service tasks; it does not run inline logic (e.g. no direct `govulncheck` or custom build steps at root).

## Canonical service list

| Service         | Type    | Migrations |
|----------------|---------|------------|
| auth           | Go      | No         |
| classifier     | Go      | Yes        |
| crawler        | Go      | Yes        |
| dashboard      | Frontend| No         |
| index-manager  | Go      | Yes        |
| mcp-north-cloud | Go    | No         |
| publisher      | Go      | Yes        |
| search         | Go      | No         |
| search-frontend| Frontend| No         |
| source-manager | Go      | Yes        |

## Task contract by service type

### All services (Go and frontend)

- **default** – Show available tasks (e.g. `task --list`)
- **lint** – Lint the code
- **test** – Run tests
- **test:coverage** – Run tests with coverage (frontends may use a no-op)
- **test:race** – Run tests with race detector (frontends use a no-op)
- **build** – Build the service

### Go services only

- **vuln** – Run `govulncheck ./...`
- **lint** – Must use repo root config: `golangci-lint run --config ../.golangci.yml ./...`

### Services with a database

- **migrate:up** – Apply migrations
- **migrate:down** – Rollback last migration
- **migrate:version** – Show current version
- **migrate:force** – Force version (fix dirty state; requires `VERSION` env)

## Install tools

The root `install:tools` task is the canonical way to install dev tools (golangci-lint/v2, goimports, migrate, govulncheck, air). Service-level `install:tools` may match (e.g. use `golangci-lint/v2`) or be omitted.

## Root Taskfile

- Root tasks are named by operation and service: `lint:SERVICE`, `test:SERVICE`, `test:cover:SERVICE`, `test:race:SERVICE`, `vuln:SERVICE`, `migrate:SERVICE` (for services with DB).
- Aggregate tasks (`lint`, `test`, `test:cover`, `test:race`, `vuln`, `migrate:up`, etc.) run the corresponding per-service tasks.
- Root does not define `sources`/`generates` or run tools directly in service dirs; it always runs `task <name>` in the service directory.

## Adding a new service

1. Create `SERVICE/Taskfile.yml` and implement the contract:
   - All: `default`, `lint`, `test`, `test:coverage`, `test:race`, `build`
   - Go only: `vuln`; use `--config ../.golangci.yml` in `lint`
   - If the service has a DB: `migrate:up`, `migrate:down`, `migrate:version`, `migrate:force`
2. In the root `Taskfile.yml`:
   - Add `lint:SERVICE`, `test:SERVICE`, `test:cover:SERVICE`, `test:race:SERVICE` delegate tasks.
   - Include them in the aggregate `lint`, `test`, `test:cover`, `test:race` task lists.
   - If Go: add `vuln:SERVICE` and add it to the `vuln` aggregate.
   - If DB: add `migrate:SERVICE`, `migrate:down:SERVICE`, `migrate:version:SERVICE` (and `migrate:force:SERVICE` if applicable) and add them to the `migrate:up`, `migrate:down`, `migrate:version` aggregates.
3. Update this document with the new service in the canonical list.
