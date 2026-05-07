# Community Alert Pipeline - Spec Pointer

The authoritative specification for this subsystem lives in the Spec Kitty mission directory:

- **Spec**: [`kitty-specs/community-alert-pipeline-01KQZC7A/spec.md`](../../kitty-specs/community-alert-pipeline-01KQZC7A/spec.md)
- **Plan**: [`kitty-specs/community-alert-pipeline-01KQZC7A/plan.md`](../../kitty-specs/community-alert-pipeline-01KQZC7A/plan.md)
- **Research**: [`kitty-specs/community-alert-pipeline-01KQZC7A/research.md`](../../kitty-specs/community-alert-pipeline-01KQZC7A/research.md)
- **Data Model**: [`kitty-specs/community-alert-pipeline-01KQZC7A/data-model.md`](../../kitty-specs/community-alert-pipeline-01KQZC7A/data-model.md)
- **Contracts**: [`kitty-specs/community-alert-pipeline-01KQZC7A/contracts/`](../../kitty-specs/community-alert-pipeline-01KQZC7A/contracts/)

The `alert-crawler/` service is the implementation of this spec.

## Why this pointer exists

`task drift:check` (per repo `CLAUDE.md`) maps source-code paths to spec documents. Because the
authoritative spec lives in `kitty-specs/`, this pointer file makes the mapping explicit and
discoverable for engineers grepping `docs/specs/`.

## Update policy

This file is a pointer. Do NOT duplicate spec content here. Update the spec, and the pointer
remains valid.
