## Pre-Merge Checklist

- [x] `task lint:force` — clean (`2026-05-07T14:12:39Z`, base commit `e243979a`)
- [x] `task test` — clean (`2026-05-07T14:12:39Z`)
- [x] alert-crawler integration-tag compile gate via `GOWORK=off go test -tags integration ./...` (service root) — clean (`2026-05-07T14:12:39Z`)
- [x] `task drift:check` — clean (`2026-05-07T14:12:39Z`)
- [x] `task ports:check` — clean (`2026-05-07T14:12:39Z`)
- [x] `task layers:check` — clean with existing warning only (`signal-producer` unmapped `config` package; no violations) (`2026-05-07T14:12:39Z`)
- [x] `lefthook install` + `lefthook run pre-commit` + `lefthook run pre-push` — clean (`2026-05-07T14:12:39Z`)
- [ ] Manual staging smoke (`docker compose run --rm alert-crawler`, ES `_count`, Redis lifecycle subscribe) — pending human-run on staging host
- [x] Deferred migration issue filed: [#717](https://github.com/jonesrussell/north-cloud/issues/717)
