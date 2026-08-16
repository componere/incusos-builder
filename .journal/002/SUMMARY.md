---
id: 002
title: Implement phases 0-4 of incusos-builder
date: 2026-08-16
status: complete
repos_touched: [incusos-builder]
related_sessions: [001]
---

## Goal
Execute the session-001 implementation plan from Phase 0 (template rename) as far as possible, using an orchestrator + parallel-programmer + tight-reviewer agent pattern with one PR per phase.

## Outcome
Goal exceeded: Phases 0 through 4 all merged (PRs #7, #8, #9, #11, #12). The tool is end-to-end functional — a live-server smoke built a real seeded aarch64 image in 17 s with digest-verified output and a byte-correct seed tar at offset 2,148,532,224. Remaining: Phase 5 (T3 live suite + boot gate) and Phase 6 (docs/release/v1).

## Key Decisions
- Upstream pin `github.com/lxc/incus-os/incus-osd@v0.0.0-20260815030500-0f5b8057f2fc` (commit, not tag — tag `202608102114` lacks `api/seed.Services`).
- Hand-parse GPT with 512/2048/4096 sector probe → go-diskfs read path defaults to 512 for regular files and ISO layouts put the GPT at byte 2048.
- Rescue media: go-diskfs for both formats, RR-only ISO (its Joliet tree presents unusable names); mkisofs fallback not needed.
- Boot gate (1.E): OPEN — macOS cannot diagnose (Secure Boot hard gate; shipped UKIs configure no console). Interim = release checklist; Phase 5.2 gets one time-boxed Linux run (attempt-C command line recorded in docs/notes/spike-1e-boot.md).
- Sentinels consolidated into `internal/errdefs` leaf with §6-named re-exports → broke build↔config/update cycles; restored `ErrConfig` for oversized seed tars.
- Server documents plain-decoded (not strict): integrity from digests/caps/three-way binding; strictness would break builds on additive upstream fields.
- Plain `http://` `--server` = usage error (exit 2); adapter https-only (incl. redirect-downgrade guard on final response URL) stays defense-in-depth.
- Metadata size cap 1 MiB (measured 11.9/14.3 KiB live; ~4,800-entry ceiling).
- `SeedRenderFunc` explicit injection into `build.Build` (no package-level state); seed↔build cycle resolved.

## Changes
- Phase 0 (#7): full template rename to `componere/incusos-builder`; demo bits deleted; mockery pinned; release-please app credentials applied from 1Password (`COMPONERE_RELEASE_APP_ID` var + private-key secret); Release Please + release dry-run proven green post-rename.
- Phase 1 (#8): five spike findings docs under `docs/notes/` + quarantined spike modules under `spikes/`; adversarial review corrected 12 findings incl. two P0s (false swtpm premise; my own rooted-path ReadDir misuse).
- Phase 2 (#9): `internal/build` (§6 ports verbatim, Resolve, GPT probe, streamed splice), `internal/seed` (writeSeed-byte-identical renderer + goldens), `internal/config` (SOPS in-memory, strict decode, §4 validation), mockery mocks, `root:check-upstream` type-only closure gate in CI.
- Phase 3 (#11): `internal/update` (allowlist-first HTTPS+local sources, content-addressed cache, S/MIME sjson validation, retry around fetch+admit), `internal/media` (RR-only ISO + GPT/FAT32 raw, proportional FAT sizing), `internal/ux` (palette, fancy/plain reporters), `internal/errdefs`.
- Phase 4 (#12): `internal/cli` publisher (§3 lifecycle exact) + build/validate/versions/init commands + policy/exit mapping; `cmd/incusos-builder` wiring with runtime pin lookup; `internal/testfixture` sparse mirror; 9-script testscript e2e suite (exit codes 2–6, --json, `-o -`, SOPS stdin; 828 MiB peak RSS after review fix from 15.4 GB).

## Open Threads
- Phase 5: T3 live suite (env-gated `INCUSOS_BUILDER_E2E=1`, moon `root:e2e`) + boot acceptance gate — one time-boxed Linux attempt (QEMU+swtpm+OVMF secure varstore port of the recorded attempt-C command line, or `incus launch`) decides CI gate vs release checklist. Rescue-media recovery detection rides the same run; spike-1B format decision stays provisional until then.
- Phase 6: Diátaxis docs, README rewrite, release verification, v1. License still template-default.
- Nine-section seed golden is pinned to a vendored writeSeed copy; comparison against an upstream-BUILT customizer binary still open (see `internal/seed/testdata/README`).
- `actions/create-github-app-token` deprecates `app-id` for `client-id`; the 1Password item already carries `client_id`.
- Repository settings (`.github/repository-settings.toml`) not yet applied; dependabot PRs #1–#6 still open.

## Lessons
- Recurring agent failure mode: writing read-tool preview output (line numbers/hash headers) into source files — corrupted 3 files across phases; always rebuild the package after writes.
- Two agents wrote into the main checkout instead of the worktree; an explicit "verify cwd" line in agent contexts stopped it.
- Local `golangci-lint` (global 2.11.4) shadowed the mise-pinned 2.12.2 → local-green/CI-red on goconst; use `mise x --` for gate runs.
- `.gitignore` unrooted `build/` silently ignored `internal/build/` — a base commit shipped without its core package until a devops agent caught it.
- The tight-reviewer pattern paid for itself every phase: caught a P1 https-redirect downgrade, FAT32 sizing that failed live assets, a 15.4 GB test OOM, and two upstream-semantics divergences.

## References
- PRs: componere/incusos-builder #7 (rename), #8 (spikes), #9 (domain core), #11 (adapters), #12 (CLI).
- Plan/architecture: `.journal/001/PLAN.md`, `.journal/001/ARCHITECTURE.md`.
- Spike findings: `docs/notes/spike-1{a,b,c,d,e}-*.md` on master.
- Boot-gate command lines: `docs/notes/spike-1e-boot.md` (attempt C).
