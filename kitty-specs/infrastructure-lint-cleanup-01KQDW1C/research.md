# Infrastructure Lint Cleanup Research

## Decisions

### D-001: Keep the mission scoped to `infrastructure/`

The mission should treat `infrastructure/` as the primary cleanup surface. Root
or service changes are allowed only when an infrastructure API/config cleanup
requires caller updates. This preserves the issue scope and avoids turning #646
into a monorepo-wide lint rewrite.

### D-002: Validate with module-local commands first

The canonical local commands are:

```bash
cd infrastructure
GOWORK=off go test ./...
GOWORK=off golangci-lint run --config ../.golangci.yml ./...
```

`go test` is available but currently stops before tests run because
`infrastructure/go.mod` needs tidy. `golangci-lint` is not installed on PATH in
this environment; the repo pins `golangci-lint 2.10.1` in `.tool-versions` and
has a root `install:tools` task that installs the same version.

### D-003: Known hotspots from static inspection

Initial source inspection found the issue-backed hotspots:

- `infrastructure/profiling/pprof.go` reads `ENABLE_PROFILING` and
  `PPROF_PORT` directly and starts an `http.ListenAndServe` pprof server with
  no explicit timeouts.
- `infrastructure/profiling/pyroscope.go` reads continuous profiling settings
  directly from env.
- `infrastructure/sse/broker.go` owns broker lifecycle context/cancel behavior.
- `infrastructure/retry/retry.go`, `infrastructure/config/types.go`,
  `infrastructure/sse/options.go`, and related tests contain duration/count
  literals likely to be `mnd` targets.
- `infrastructure/config/loader.go` legitimately reads env for generic config
  loading; this may need a narrow exception or should remain outside the
  profiling-specific smell unless lint reports otherwise.

## Evidence

| Source | Finding |
| --- | --- |
| `infrastructure/go.mod` | Module is isolated; validation should run with `GOWORK=off`. |
| `GOWORK=off go test ./...` from `infrastructure/` | Fails immediately with `go: updates to go.mod needed; to update it: go mod tidy`. |
| `Get-Command golangci-lint` | No installed `golangci-lint` found in PATH. |
| `.tool-versions` | Pins `golangci-lint 2.10.1`. |
| `Taskfile.yml` | `install:tools` installs `golangci-lint/v2@v2.10.1`, `goimports`, `migrate`, and `govulncheck`. |
| `rg` hotspot scan | Found direct profiling env reads, pprof server startup, SSE broker cancel paths, and likely magic-number duration/count literals. |

## Open Questions / Risks

- The full lint inventory cannot be regenerated until `golangci-lint` is
  installed locally.
- Running `go mod tidy` may change `infrastructure/go.mod` and `go.sum`; that
  should be captured deliberately in an implementation WP if needed for tests.
- Test package moves in WP03 may require small exported seams. Those seams must
  stay narrow and avoid public API expansion unless necessary.
