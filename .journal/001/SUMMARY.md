---
id: 001
title: Bootstrap incusos-builder repository
date: 2026-08-15
status: complete
repos_touched: [incusos-builder]
related_sessions: []
---

## Goal
Bootstrap `componere/incusos-builder` from `meigma/template-go`, set up the session journal system, and produce a reviewed architecture plus an end-to-end implementation plan for a Go CLI that turns one YAML config into seeded IncusOS install media (local alternative to the IncusOS web customizer).

## Outcome
Goal met. Repo created and cloned; journal system live on `journal/jmgilman`; architecture written by a software-architect agent and hardened through 3 adversarial review rounds (plus one closing architect pass); full phased implementation plan composed by a planner agent. No feature code written yet — that starts next session at Phase 0 of the plan.

## Key Decisions
- Architecture/plan docs live in `.journal/001/` on the journal branch, not the repo root → owner's call; keeps master clean until implementation starts.
- Pin and import upstream `github.com/lxc/incus-os/incus-osd` seed/API types rather than mirroring them → byte-compatible YAML and strict decode against the exact pinned schema; feasibility spike (1.D) verifies the dependency closure.
- Offline/rescue media is in v1 scope → it is UI-visible in the web app, so descoping would violate the config-coverage hard requirement (round-1 review blocker).
- Reject `encryption_recovery_keys` at config validation → upstream `GetSecurity` fatally rejects it at boot; building such an image is a known-bad artifact.
- Spikes before build-out (splice+boot first) → owner's agile preference; the 2049 MiB splice and pure-Go rescue media are the two make-or-break mechanics.
- Bootstrap commits went straight to master (no PRs) → pre-protection bootstrap phase; normal PR flow starts with implementation.

## Changes
- Repo created from template; cloned to `~/code/componere/incusos-builder` (default branch `master`).
- `.gitignore` — added `/reference/` for local upstream clones.
- `reference/incus-os/` — shallow clone of `lxc/incus-os` (gitignored, read-only reference).
- `.journal/001/ARCHITECTURE.md` — final architecture (hexagonal; ports ImageSource/Decrypter/RescueWriter/Reporter; SOPS in-memory decrypt; exit taxonomy; charm UX with auto|always|never toggles; digest-keyed cache; atomic two-artifact publication).
- `.journal/001/PLAN.md` — 7-phase plan (0 rename → 1 spikes → 2 domain core → 3 parallel adapters → 4 CLI → 5 e2e/boot gate → 6 docs/release).
- Master history note: `ARCHITECTURE.md` briefly lived at repo root (commits cc1df8b→86f7bf7) before moving to the journal.

## Open Threads
- Template rename pass (Phase 0) not done: module still `github.com/meigma/template-go`, binary still `cmd/template-go`, `DELETE_ME.md` still present.
- Spike decisions pending (Phase 1): GPT reader choice (go-diskfs vs hand parse), upstream module pin, boot-gate CI viability (1.E), metadata size caps (1.C).
- Repository settings (branch protection, release credentials) not yet applied from `.github/repository-settings.toml`.
- Dependabot PR open: `actions/cache-6.1.0` branch on origin.
- License still template-default; needs a real license before publishing artifacts.

## References
- Architecture: `.journal/001/ARCHITECTURE.md` (same folder)
- Plan: `.journal/001/PLAN.md` (same folder)
- Upstream: https://github.com/lxc/incus-os — customizer at `incus-osd/cmd/image-customizer/`
- Web app being replaced: https://incusos-customizer.linuxcontainers.org/ui/
