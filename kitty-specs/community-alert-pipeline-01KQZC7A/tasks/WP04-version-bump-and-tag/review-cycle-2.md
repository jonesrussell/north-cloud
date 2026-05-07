---
affected_files: []
cycle_number: 2
mission_slug: community-alert-pipeline-01KQZC7A
reproduction_command:
reviewed_at: '2026-05-06T21:58:50Z'
reviewer_agent: unknown
verdict: rejected
wp_id: WP04
---

# WP04 Review — Cycle 1

**Reviewer:** claude:opus:reviewer:reviewer
**Verdict:** REJECT (request changes)
**Date:** 2026-05-06

## Summary

The Go side of the v1.1.0 release is clean and verifiable: tag is pushed, proxy.golang.org has indexed it, a fresh-module pin (`go get github.com/jonesrussell/indigenous-taxonomy@v1.1.0`) succeeds, and all 14 Go tests PASS. CHANGELOG follows Keep a Changelog format and accurately enumerates WP01-03 changes. The release commit `c4fb24d` touches only the four expected files (CHANGELOG.md, version.go, version.py, TaxonomyVersion.php).

However, **one tracked version file was missed**: `generated/python/pyproject.toml` still declares `version = "1.0.0"`. This is the canonical Python package version used by pip/setuptools/PyPI, and it must move in lockstep with `version.py` for the Python release to be coherent. T013 ("Version is 1.1.0 across all version files") is therefore not fully satisfied.

## Verification Results

| Check | Result |
|-------|--------|
| HEAD on `indigenous-taxonomy` main is `c4fb24d` | PASS |
| `v1.1.0` tag exists | PASS |
| Release commit touches only CHANGELOG + 3 version files | PASS |
| CHANGELOG `[v1.1.0]` section present, Keep a Changelog format | PASS |
| `version.go` = `1.1.0` | PASS |
| `version.py` = `1.1.0` | PASS |
| `TaxonomyVersion.php` = `1.1.0` | PASS |
| **`pyproject.toml` = `1.1.0`** | **FAIL — still `1.0.0`** |
| 14 Go tests PASS | PASS |
| `go get @v1.1.0` from fresh module resolves via proxy | PASS |
| No leftover `"1.0.0"` in tracked source files (excluding `build/` artifacts) | FAIL (pyproject.toml) |

The `generated/python/build/lib/indigenous_taxonomy/version.py` hit is a build artifact and is ignorable (not tracked by git, regenerated on `python -m build`).

## Required Changes

1. Bump `generated/python/pyproject.toml` `version` from `"1.0.0"` to `"1.1.0"`.
2. Either:
   - **(preferred)** Add the change to a new commit on `main`, then move the `v1.1.0` tag forward (`git tag -d v1.1.0 && git tag v1.1.0 <new-sha> && git push --force origin v1.1.0`). Note: this re-tag requires care because proxy.golang.org may have already cached the old tag SHA. If retag is risky, see option (b).
   - **(alternative)** Land the pyproject fix as a follow-up commit and ship `v1.1.1` for the Python package only. Keep `v1.1.0` as the Go-coherent tag. Document the discrepancy in CHANGELOG.
3. Confirm `proxy.golang.org` still resolves `v1.1.0` (re-run the fresh-module pin test).

## Notes for the Implementer

- The Go release is solid — WP19 (Go consumer pin) is unblocked even without this fix.
- The fix matters for Python coherence and any future Python consumer (PyPI publish, pip install via VCS, etc.).
- Recommend option (a) since no consumer has yet pinned `v1.1.0` and the Go module is content-addressed (the proxy will accept the same content under the same tag if SHA changes).
