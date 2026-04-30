# Enrichment Service

The enrichment service accepts Waaseyaa enrichment requests, validates the API contract, and hands accepted work to asynchronous enrichment orchestration.

## Boundaries

- Do not modify Waaseyaa from this service.
- Treat `callback_url` and `callback_api_key` as request-local external contract values.
- Do not log raw callback API keys.
- Keep service imports local to `enrichment/`, standard library packages, and approved shared `infrastructure/` helpers when needed.

## Local Commands

```bash
task build
task test
task lint
task vuln
```

`golangci-lint` and `govulncheck` may need to be installed locally before lint or vulnerability checks can run.
