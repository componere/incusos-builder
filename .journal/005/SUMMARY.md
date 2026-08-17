---
id: 005
title: Functional test campaign, remediation, and the v0.1.0 release
date: 2026-08-16
status: complete
repos_touched: [incusos-builder]
related_sessions: [001, 002, 003, 004]
---

## Goal
Compose a manual, real-world functional test plan proving incusos-builder delivers on its public promises and is ready to release. The session then continued past the plan: execute it, remediate what it found, close the one unproven claim, and cut the first release.

## Outcome
Goal met and substantially exceeded. Ran into 2026-08-17.

A 135-case plan was written, then **115 cases were executed by hand** across three waves. The campaign produced 93 distinct findings and two outright failures, both since closed. The project's central unproven claim — that a built image actually seeds an IncusOS install — was **observed for the first time anywhere** on a rented nested-KVM host. v0.1.0 was tagged, built, signed, attested, and verified as a consumer.

**The release is not published.** `release.yml` creates the GitHub Release as a draft for manual inspection; that gate was deliberately not stepped through. The container image is already public because GHCR has no draft state, so the two artifact classes are currently live at different times.

## Key Decisions
- Split the plan into detached waves rather than one ordered list -> a 75-case fast lane runs in ≤5 min per case on a laptop, so the expensive multi-gigabyte and boot cases never gate the cheap ones.
- Execute with parallel `functional-tester` agents capped at 4 -> matched the developer's instruction and kept shared-resource contention (bandwidth, one warm cache) manageable.
- Correct the BOOT-06 oracle *before* renting any hardware -> the gate hashed the **source** installer media, which `CleanupPostInstall` never modifies. It could not have passed on any host at any price. Renting first would have burned a paid three-hour window on an unpassable step.
- Rent Semaphore Cloud rather than buy or borrow an x86_64 machine -> the whole boot acceptance cost ~$0.25 of a $15 recurring credit, against the alternative of acquiring hardware.
- Write the gate as an **unattended pipeline** instead of an attended SSH session -> `sem debug job` allocates an ephemeral agent that dies with the session, an unrecoverable failure mode for a three-hour run. A captured console log is better evidence than a human watching a VGA console, and every assertion string traces to pinned upstream source.
- Leave O4 (recovery payload) as `ACCEPTED-NOT-ADJUDICATED` for the developer to judge -> the guide forbids inventing a success criterion, and the entire campaign turned on refusing inferred success.
- Reject 1.0.0 for the first release -> the developer is not ready to promise a stable API. `bump-minor-pre-major` stays `true` so breaking changes bump the minor and the project stays in 0.x.
- Set `initial-version: "0.1.0"` rather than only resetting the manifest -> a `0.0.0` manifest alone yields **1.0.0**, the exact version being rejected. See Lessons.
- Apply the eight Dependabot updates as one batch -> all eight were raised against an older master and none could merge without a rebase; serial merges would have cost eight rebase-and-CI rounds.
- Merge the README pass only after the release existed -> it documents install paths that require a published release.
- Do not publish the draft release -> the pipeline gates publication on human inspection, and stepping through that quietly would defeat its purpose.

## Changes
- `.journal/005/FUNCTIONAL_TEST_PLAN.md` — 135 manual cases across 8 surfaces, wave-tagged, with a promise-to-test traceability table and an honest cannot-verify section. Corrected mid-session as execution disproved parts of it.
- `.journal/005/{WAVE1,WAVE2_TRACKA,WAVE2_TRACKB}_RESULTS.md` — per-case verdicts with verbatim observed evidence.
- `.journal/005/REMEDIATION_PLAN.md` — dispositions for all 93 findings, mechanically verified to cover every finding ID.
- `.journal/005/{BOOT_10_RECORD.md,TRACK_C_GATE_LOG.txt,TRACK_C_RUNBOOK.md,TRACK_C_OPTIONS.md}` — the boot-acceptance verdict, its 2,478-line raw log, the venue runbook, and the venue cost research.
- PR #21 — closed the actionable CLI, acquisition, docs, and CI findings; includes the `AllowEmptyEnv` fix and the live-suite isolation regression it caused.
- PR #24 — recorded the first observed boot acceptance across `trust-model.md`, `verify-boot-acceptance.md`, and the superseded Phase 5 probe note.
- PR #26 — release version baseline: manifest `0.0.0`, `initial-version: "0.1.0"`, `bump-patch-for-minor-pre-major: false`.
- PR #29 — emptied the `CHANGELOG.md` seed so the first entry has no stray `## Changelog`.
- PR #30 — eight dependency updates in one batch; Dependabot alerts went to zero.
- PR #31 — README rewritten for a released tool, plus the matching `SECURITY.md` correction.
- PR #10 — `chore(master): release 0.1.0`, which tagged `v0.1.0`.
- `.github/repository-settings.toml` applied to the live repository (six changes).

## Open Threads
- **The v0.1.0 GitHub Release is still a draft.** `gh release edit v0.1.0 --draft=false` publishes it. Until then the `ghd` install path documented in the README does not work for unauthenticated users, while the container image already does.
- `N-MEDIA-3`: the NUL-padded ISO volume identifier is still untested. The raw config matches the GPT partlabel and never reads it; one optional ISO gate run closes it.
- `fix-after-v1` deferrals: F-CFG-1, N-ART-5, N-APUB-2.
- Nine `repository-settings.toml` keys are not exposed by the REST API and remain manual GitHub UI follow-ups.
- Session 006 was open concurrently and is not part of this closeout.

## Lessons
- **Reading the right function is not the same as reading the function that runs.** I traced release-please's `determineReleaseType` correctly and concluded a `0.0.0` manifest would yield `0.0.1`. Wrong: `manifest.ts` excludes exactly `"0.0.0"` from the manifest backfill, so no `latestRelease` exists, `buildNewVersion()` never calls the versioning strategy, and `initialReleaseVersion()` returns **1.0.0**. My fix would have produced the one version the developer had just rejected. The developer's "are you certain?" caught it; the check cost two minutes.
- **Silent success is not evidence.** `gh attestation verify` on `gh` 2.97.0 prints nothing and exits 0. Every verification here was paired with a deliberate falsifier, and the failure messages proved more informative than the passes — the wrong-`--source-ref` error named the real attested ref, and the foreign-identity error named both real signing identities.
- **Verify the oracle before buying the venue.** The boot gate had been unpassable since it was written, for a reason visible in the pinned upstream source. Three researchers running in parallel found it in minutes, before a single dollar was spent.
- **A skipped required check reports success.** The release dry-run jobs gate on `workflow_dispatch || release-please--*`, so on an ordinary PR they report `skipping`, which counts as passing. A green PR would not have proven the `setup-go` major bump on the release path; a dispatched dry run did.
- **Wall-clock assertions in unit tests are latent CI flakes.** `TestGenerateSatisfiesUpdateAdapter` failed twice in one day on unrelated PRs. Fixed on master by #27 while this session ran.
- **Concurrent agents on shared machine state collide in ways file-level coordination misses.** Two testers shared a tmux socket and one ran `kill-server`, destroying the other's session mid-case. Per-agent sockets are now the standing policy.

## References
- PRs: #21, #24, #26, #29, #30, #31, and release PR #10.
- Boot acceptance: Semaphore pipeline `4c5cc805`, job `a8b16331`; record `.journal/005/BOOT_10_RECORD.md`.
- Release build: Actions run 31989614368; image index `sha256:e3bfe74884acbbd707bccca58e89d66ec542c63f365d0f5f73af45a6284b37de`.
- Release dry run proving the action bumps: Actions run 31988148676.
- Plan and prior sessions: `.journal/001/PLAN.md`, `.journal/004/SUMMARY.md`.
