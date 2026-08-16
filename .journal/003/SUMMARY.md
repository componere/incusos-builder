---
id: 003
title: Complete Phase 5 E2E and boot acceptance
date: 2026-08-16
status: complete
repos_touched: [incusos-builder]
related_sessions: [001, 002]
---

## Goal
Continue the phased implementation plan from the Phase 5 boundary: add the opt-in live T3 suite, execute the required Linux boot-acceptance experiment, and establish the boot gate for releases.

## Outcome
Goal met. Phase 5 was implemented, reviewed, verified against the live IncusOS server, and squash-merged as PR #13. The Linux boot probe completed but did not observe seed consumption; recovery acceptance was therefore unreachable. The release gate is a documented manual Incus checklist that runs before every release tag until a CI boot gate succeeds.

## Key Decisions
- Keep `root:e2e` opt-in with `INCUSOS_BUILDER_E2E=1`, `runInCI: false`, and outside `root:check` -> the suite downloads and builds multi-gigabyte live artifacts and must not make ordinary CI depend on the external update server.
- Classify the Linux boot probe as negative -> Secure Boot enrollment succeeded, but the blank target disk did not change; guest network frames were diagnostic only and did not prove seed consumption.
- Use target-disk growth, not source-overlay growth or guest traffic, as the automated seed-consumption oracle -> firmware writes alter the source overlay and normal guest startup emits network traffic independently of seed processing.
- Fall back to a manual Incus release checklist -> no environment has yet demonstrated seed consumption plus recovery-path acceptance, so a merge-gating CI assertion would be unsound.
- Read FAT metadata using its reported logical size plus `io.ReadFull` -> go-diskfs v1.9.4 can return final-cluster zero slack to `io.ReadAll` even though the FAT directory entry records the correct size.
- Remove the temporary branch-only probe workflow and harness after the recorded run -> the experiment was one-shot scaffolding; the committed JSON evidence and run note are the durable artifacts.

## Changes
- `internal/cli/e2e_test.go` and `e2e_helpers_test.go` — live tests for versions parsing, smallest-image build, all eleven seed sections, and exact ISO/FAT32 rescue-media read-back.
- `moon.yml` — added opt-in `root:e2e`, excluded from `root:check` and CI.
- `docs/notes/phase-5-boot-evidence.json` — committed machine evidence from the Linux probe.
- `docs/notes/phase-5-boot-probe.md` — recorded run topology, measurements, negative result, and gate decision.
- `docs/docs/how-to/verify-boot-acceptance.md` — added the manual x86_64 Linux Incus pre-release checklist.
- `docs/mkdocs.yml` — exposed the checklist under How-to guides.
- PR #13 was squash-merged to `master` as `3fa587273e1d65bd69bd7ed5f55db81f25dcf9d6`; the local default branch was fast-forwarded and the feature worktree removed.

## Open Threads
- Phase 6 remains: complete the Diátaxis documentation set, release verification, repository settings, release workflow polish, and v1 readiness.
- Execute the manual boot-acceptance checklist before every release tag until a CI boot gate succeeds.
- IncusOS seed consumption, `RESCUE_DATA` detection, and signed recovery-metadata acceptance remain unobserved in an end-to-end boot.
- The seed golden is still compared with the vendored upstream `writeSeed` copy rather than an upstream-built customizer binary.
- When next touching Release Please, migrate `actions/create-github-app-token` from `app-id` to `client-id`; repository settings and open Dependabot PRs also remain.

## Lessons
- A reviewer reproduced the raw FAT mismatch and showed the writer was correct: `io.ReadAll` triggered a go-diskfs partial-cluster read bug. Fixing the writer would have corrupted valid media to satisfy a faulty test.
- Guest-originated DHCP/ARP/IPv6 traffic is not evidence that a seed was consumed. Acceptance oracles must be tied to the target state the seed changes.
- One-shot remote probes should distinguish harness failure from a clean negative result and preserve machine-readable evidence before their scaffolding is removed.

## References
- PR: https://github.com/componere/incusos-builder/pull/13
- Linux boot probe: https://github.com/componere/incusos-builder/actions/runs/31958439711
- Evidence: `docs/notes/phase-5-boot-evidence.json`
- Experiment record: `docs/notes/phase-5-boot-probe.md`
- Manual release gate: `docs/docs/how-to/verify-boot-acceptance.md`
- Plan and architecture: `.journal/001/PLAN.md`, `.journal/001/ARCHITECTURE.md`
- Prior implementation summary: `.journal/002/SUMMARY.md`
