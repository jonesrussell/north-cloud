# WP02 — `ml-modules/` and `ml-framework/` triage

**Mission**: `audit-dead-code-removal-01KQG57Z`
**Issue**: #698
**Spec refs**: FR-003, FR-005, FR-007
**Dependencies**: WP01

## Outcome: documented, not deleted

The 2026-04-30 audit's "no consumers" hypothesis for `ml-modules/` and `ml-framework/` was **incorrect**. Both directories are live, production code. WP02 instead takes the spec FR-003 conditional path: *"If populated with intent, document and either fold into ml-sidecars/ or open a follow-up mission."* This WP adds two READMEs documenting the architecture and closes #698 as "audit was wrong; keep with docs". No directory is deleted.

## Evidence the audit was wrong

| Claim in audit | Reality |
|---|---|
| "Sibling directories to the active `ml-sidecars/`" | True, but they are **collaborators**, not duplicates. Three-tier architecture: framework + modules + sidecars. |
| "No README, no documented purpose" | True — addressed by this WP. |
| "Not referenced in classifier client code" | Technically true (classifier reaches sidecars via HTTP), but irrelevant — `ml-sidecars/` itself **imports `nc_ml` from `ml-framework/`** in every sidecar's `module.py`. |

Verifying greps performed in this WP's worktree (origin/main HEAD `091fe4d9`):

```bash
grep -rln 'from nc_ml' ml-sidecars/
# → ml-sidecars/{coforge,crime,entertainment,indigenous,mining}-ml/module.py

grep -lE 'ml-modules|ml-framework' docker-compose*.yml
# → docker-compose.dev.yml

grep -B1 -A3 'ml-modules\|ml-framework' docker-compose.dev.yml
# → 5 ml-* sidecar services each mount:
#     - ./ml-modules/<topic>:/opt/module
#     - ./ml-framework:/opt/ml-framework
```

`ml-framework/pyproject.toml` declares the package as `nc-ml v1.0.0 — North Cloud ML Sidecar Framework`. The `nc_ml.main` entry point dynamically imports the topic module from `/opt/module` at sidecar startup. Each `ml-modules/{topic}/module.py` subclasses `nc_ml.module.ClassifierModule`. Each `ml-sidecars/{topic}-ml/module.py` is a near-identical copy (production build artifact).

## Architecture (now documented in the new READMEs)

```
ml-framework/        → shared FastAPI framework + module protocol (nc_ml package)
ml-modules/{topic}/  → per-topic source-of-truth ClassifierModule implementations
ml-sidecars/{topic}-ml/ → production Docker build contexts (embed module code)
```

In dev: `docker-compose.dev.yml` mounts `ml-modules/{topic}` and `ml-framework` as volumes; live edit, no rebuild.
In prod: `ml-sidecars/{topic}-ml/Dockerfile` copies module code into image; `nc_ml` installed as a dependency.

5 active topics (1:1 mapping): `coforge`, `crime`, `entertainment`, `indigenous`, `mining` — matching publisher routing layers L8/L3/L6/L7/L5 respectively.

## Changes in this WP

| File | Action |
|---|---|
| `ml-framework/README.md` | **New** — documents the `nc_ml` framework, the module protocol, and the dev-vs-prod loading mechanism. |
| `ml-modules/README.md` | **New** — documents the per-topic source-of-truth pattern, subdirectory layout, the dev mount pattern, and a drift note about `ml-sidecars/{topic}-ml/module.py` mirroring. |

No code changed. No directories deleted. No compose files edited.

## Out of scope (possible follow-ups)

- **Drift between `ml-modules/{topic}/module.py` and `ml-sidecars/{topic}-ml/module.py`** — these should mirror, but no automation enforces it. A follow-up could either auto-sync them at build time or pick one as the canonical source.
- **Folding `ml-modules/` and `ml-framework/` into `ml-sidecars/`** — explicitly evaluated and rejected: would lose the dev live-edit ergonomics and require Dockerfile changes per sidecar. The 3-tier architecture is intentional.
- **Updating Mission A's `spec.md`/`research.md`** to retract the audit's claim — left as historical record of what the audit said. This WP02 doc is the corrected record.
- **Updating root `ARCHITECTURE.md`** to reference the new READMEs — out of scope per DIR-004 (no while-I'm-here).

## Verification

- `task drift:check` — green (no spec touched)
- `task layers:check` — green (no Go imports changed)
- `task ports:check` — green (no compose changed)
- No Go code touched → no `task lint` / `task test` deltas

## Acceptance

- Both new READMEs land on `main`.
- `ml-modules/` and `ml-framework/` remain unchanged in code.
- Issue #698 closed via `Closes #698`, with body explaining the audit revision.

## Conclusion

Spec FR-003's "document the keepers" path is the right call. `ml-modules/` and `ml-framework/` are an active 3-tier ML architecture; the audit just hadn't seen the dev compose mounts or the `nc_ml` framework's role. Adding the two READMEs prevents the next code-smell pass from making the same mistake.
