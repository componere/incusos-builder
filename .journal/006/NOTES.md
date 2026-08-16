---
id: 006
title: Compose manual functional test plan for incusos-builder release readiness
started: 2026-08-16
---

## 2026-08-16 12:20 — Kickoff
Goal for the session: spawn a planner agent to compose a manual, real-world
functional test plan for the incusos-builder project. The plan must exercise the
public surfaces, demonstrate that the project delivers on every promise it makes
(README, docs, CLI contracts, release artifacts), and show that the project is
ready to be released. The final plan document lands in this session folder and
is presented to the developer for review.

Current state of the world:
- All seven implementation-plan phases are merged on `master`; no release has
  been cut (see `.journal/004/SUMMARY.md`).
- `root:check` gates format, lint, build, tests, docs, and the upstream-closure
  check. `root:e2e` is opt-in behind `INCUSOS_BUILDER_E2E=1` and is excluded
  from CI.
- Known unproven areas carried forward: IncusOS seed consumption on a real boot,
  `RESCUE_DATA` detection, and signed recovery-metadata acceptance. The manual
  boot-acceptance checklist (`docs/docs/how-to/verify-boot-acceptance.md`) is a
  pre-tag gate until a boot run succeeds.
- Release plumbing is rehearsed but non-publishing; repository settings in
  `.github/repository-settings.toml` remain unapplied.

Plan:
1. Survey the repo's public surfaces (CLI commands/flags/exit codes, docs
   promises, README claims, release/artifact promises, config schema).
2. Spawn a planner agent with that surface inventory to compose the manual
   functional test plan.
3. Review the plan for coverage against the promises, then write it into this
   session folder as `FUNCTIONAL-TEST-PLAN.md`.
4. Present it to the developer for review.
