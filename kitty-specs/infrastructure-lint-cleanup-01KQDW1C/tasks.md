# Infrastructure Lint Cleanup Tasks

## WP01 — Constants and Low-Risk Lint

Extract named constants for `mnd` findings in config, monitoring, health, and retry code. Preserve values and behavior.

## WP02 — Profiling Config Cleanup

Remove direct profiling-package `os.Getenv` usage by routing values through config structs/loaders or narrow call-site inputs.

## WP03 — Test-Package Hygiene

Move affected tests to external `_test` packages and add the smallest necessary exported test seams.

## WP04 — Security and Complexity Fixes

Fix gosec findings, nil/nil returns, shadowing, exhaustive switches, nested control flow, and overly complex tests/functions. Verify full infrastructure lint is clean.
