---
id: 005
title: Compose manual functional test plan for release readiness
started: 2026-08-16
---

## 2026-08-16 11:44 — Kickoff
Goal for the session: produce a manual, real-world functional testing plan that
exercises the public surfaces of incusos-builder and demonstrates the project is
ready to release and delivers on all documented promises. The plan document is
delivered in this session folder for developer review.

Current state of the world:
- All seven plan phases (0–6) are merged to `master` (PRs #7–#9, #11–#13, #19).
  Implementation and release-readiness work are complete; no release tag exists.
- Public surfaces: `incusos-builder` CLI (`build`, `validate`, `versions`,
  `init`), exit taxonomy 0–6, `--json`, config/SOPS input, rescue media output,
  the melange apk + apko image, and the release/attestation workflows.
- Known unproven area: IncusOS seed consumption and recovery acceptance in a real
  boot (`docs/docs/how-to/verify-boot-acceptance.md` is the standing manual gate).
- Automated coverage already exists: `root:check`, testscript suite, and opt-in
  `root:e2e` (`INCUSOS_BUILDER_E2E=1`). The requested plan is manual and
  human-run, not a new automated suite.

Plan: spawn a planner agent to survey the repo's public surfaces, documented
promises, and existing test coverage, then compose the functional test plan.
Write the final document to `.journal/005/FUNCTIONAL_TEST_PLAN.md` and present it
for review.

## 2026-08-16 12:35 — Functional test plan drafted

The `planner` agent type is rate-limited at the provider (three spawns, all HTTP
429, retry-after ~11 h). `completion()` on all model tiers and general `task`
subagents work, so the planning work was fanned out to six parallel `task`
agents, one per public surface (CLI, config/SOPS, server+artifacts, rescue
media, supply chain, docs+boot). Each returned a grounded promise inventory plus
draft manual cases; I composed and deduplicated the final document.

Deliverable: `.journal/005/FUNCTIONAL_TEST_PLAN.md` — 135 manual cases
(PRE 7, CLI 20, CFG 19, ART 21, MED 17, SUP 23, DOC 18, BOOT 10), a
promise→case traceability table, a cannot-verify section, exit criteria split
into blockers vs documented caveats, and a results log.

Findings the surveys produced while grounding the plan (all reproducible at
5337e7e, all recorded in §5 of the plan):
- Contract: unknown commands/operands exit 1 not 2; `init -o ./-` ignores the
  clean-to-dash sentinel and creates a file named `-`; empty
  `INCUSOS_BUILDER_CACHE_DIR` does not override the default; interactive `init`
  with offline=yes emits a config that fails `validate`.
- Docs: the tutorial's `go run` alias cannot produce the exit codes it asserts;
  `sha256sum` used in two macOS-advertising guides; `run-in-ci.md` prints an
  error message no code path emits; the overwrite prefix is quoted two ways;
  the SOPS how-to's `SOPS_AGE_KEY_FILE` claim is wrong.
- Repo state: private vulnerability reporting is disabled (SECURITY.md's
  reporting link is dead); no rulesets exist (tags unprotected); seven
  repository settings still unapplied; the open Release Please PR proposes
  0.1.2, not 1.0.0, and its changelog ends with a stray `## Changelog` heading.

Next: present the plan to the developer for review.

## 2026-08-16 13:10 — Split the plan into detached waves

Developer asked for everything over five minutes to move into a secondary wave
detached from the rest. Applied to `.journal/005/FUNCTIONAL_TEST_PLAN.md`:

- Wave 1 (75 cases incl. the cheap half of DOC-14): every case ≤5 min. Runs from
  a fresh checkout, <500 MB disk, no Docker, no image download.
- Wave 2 (48 cases + the DOC-14 mirror half): every case >5 min plus every cheap
  case that can only run against a Wave 2 artifact. Three tracks — A live build
  chain/media/doc walkthroughs, B container image + release rehearsal, C boot
  acceptance on a Linux host.
- Wave P (5 cases): post-tag signature/attestation/SBOM/checksum verification.
- Detachment: PRE-07 split into PRE-07-W1 (Wave 1 scaffold) and PRE-07-W2
  (Wave 2 bootstrap: own `$WORK2`, own cache, `root:e2e`, and an early dispatch
  of the GitHub rehearsal). `root:e2e` moved out of the shared preflight because
  it downloads multi-GB assets. Neither wave reads the other's artifacts.
- Every case now carries a `- Wave:` field; four split cases (CLI-20, DOC-11,
  DOC-13, DOC-14) name which half belongs where.
- §4.0 added: wave membership, per-track dependency ordering (`→` hard dep,
  `‖` concurrent), and a table of the fifteen >5-minute cases with time/disk.
- §7 exit criteria regrouped by originating wave so a wave can be signed off
  independently; §8 results log regrouped into shared preflight / Wave 1 /
  Wave 2 tracks A,B,C / Wave P.
