# WP01 Cross-Repo Implementation Reference

Work package WP01 operates on the sibling repository `../indigenous-taxonomy/`.
Implementation was committed there at:

- **Repo**: `jonesrussell/indigenous-taxonomy`
- **Commit**: `98bf57c` — `feat(taxonomy): add Treaty namespace (WP01 of community-alert-pipeline-01KQZC7A)`
- **Branch**: `main`

## Files Changed in indigenous-taxonomy

- `schema/treaties.yaml` (new) — 11 numbered-treaty entries, treaty:1..treaty:11
- `scripts/generate.py` (modified) — added `gen_go_treaties()` function, wired into `main()`
- `generated/go/taxonomy/treaties.go` (new) — Treaty type, TreatyArea1..11 constants, AllTreaties, IsValidTreaty
- `generated/go/taxonomy/treaties_test.go` (new) — 4 unit tests, all passing
- `generated/go/taxonomy/version.go` (modified) — schema hash updated
- `generated/php/src/TaxonomyVersion.php` (modified) — schema hash updated
- `generated/python/indigenous_taxonomy/version.py` (modified) — schema hash updated

## Validation Results

- `go build ./generated/go/taxonomy/...` — PASS
- `go test ./generated/go/taxonomy/... -run TestTreaty` — PASS (4/4 tests)
- `gofmt -l generated/go/taxonomy/treaties.go` — clean (empty output)
- `go vet ./generated/go/taxonomy/...` — clean
