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

## 2026-08-16 13:05 — Plan composed and filed

Surveyed the public surfaces directly before delegating: `README.md`,
`SECURITY.md`, `docs/docs/index.md`, the four reference pages (`cli`,
`automation`, `configuration`, `cache`), `explanation/trust-model.md`,
`how-to/verify-boot-acceptance.md`, `how-to/run-in-ci.md`,
`how-to/recover-interrupted-build.md`, `moon.yml`, and `ghd.toml`. That
inventory went into the planner brief so the agent worked from a fixed
surface list rather than discovering scope on its own.

Spawned one `planner` subagent (`FunctionalTestPlanner`, 24 min). Its
first yield was a JSON stub instead of the document; recovered the real
Markdown by messaging it to write `'/Users/josh/.omp/agent/sessions/-code-imgoci-go/2026-08-16T19-18-48-228Z_01a00c03-26a4-7000-a48e-105ed855e845/local/functional-test-plan.md'`
(133,833 bytes).

Reviewed the result before filing:
- 51 cases across nine suites (A install/provenance, B CLI contract,
  C config/SOPS, D acquisition/cache/trust, E build/publication,
  F offline media, G docs-as-written, H release/supply chain,
  I boot acceptance). Verified every case ID referenced by the coverage
  ledger actually exists as a heading.
- Three-gate verdict model: pre-tag, draft-release, published-release.
  I-01 (the Incus boot gate) is mandatory pre-tag and is `Blocked`, not
  waivable, without an x86_64 Linux Incus host.
- Found one ledger gap: H-08 (security-scan workflow) existed as a case
  with no promise row. Added the row rather than bouncing the document.

Filed as `.journal/006/FUNCTIONAL-TEST-PLAN.md`.

Next: present to the developer for review.
