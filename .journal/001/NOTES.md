---
id: 001
title: Bootstrap incusos-builder repository
started: 2026-08-15
---

## 2026-08-15 09:11 — Kickoff
Goal for the session: bootstrap the new `componere/incusos-builder` repository, freshly created from `meigma/template-go`, and begin project work.
Current state of the world: public repo created from the template and cloned to `~/code/componere/incusos-builder` (default branch `master`, initial commit `424e764`). Session journal system set up: `journal/jmgilman` branch created and pushed, journal worktree at `.wt/journal-jmgilman`, scaffold `.journal/` files committed. Template placeholders (`template-go`, module path, binary name, release config) not yet renamed; `DELETE_ME.md` checklist not yet started.
Plan: await the user's direction; likely first steps are the template rename pass (module path, `cmd/` binary, Moon metadata, release config) per `DELETE_ME.md`, then defining what incusos-builder actually builds.

## 2026-08-15 09:14 — Located and cloned customizer source
The IncusOS customizer website (https://incusos-customizer.linuxcontainers.org/ui/) is part of lxc/incus-os. Its source lives at `incus-osd/cmd/image-customizer` (Go server: main.go, oidc.go, certificate.go; static UI under `html/`), with the API types in `incus-osd/api/customizer`.
Shallow-cloned lxc/incus-os to `reference/incus-os` in the repo root; added `/reference/` to `.gitignore` (committed to master).
Next: study the customizer to inform what incusos-builder will do.

## 2026-08-15 10:05 — Architecture designed via architect/reviewer loop
Ran software-architect ↔ architecture-reviewer loop, capped at 3 review rounds, then one final architect pass to close the last review's findings (no 4th review). Result committed as ARCHITECTURE.md on master.
Key grounded findings from reference/incus-os:
- Seed injection = uncompressed tar spliced at fixed byte offset 2,148,532,224 (2049 MiB) into the decompressed base image (customizer main.go sendOSImage/writeSeed); seed-data partition capped at 100 MiB (mkosi.repart/01-seed-data.conf) — pre-publication size invariant.
- Base images come from update-server index (https://images.linuxcontainers.org/os), assets filtered by channel/version/arch/type, SHA-256 verified; update.json/update.sjson appended literally for offline/rescue media (update.sjson is the trusted, signature-verified metadata at recovery time).
- Web API injects 9 seed sections; incus-osd consumes 11 — kernel and security seeds are CLI-exclusive capability. GetSecurity rejects non-empty encryption recovery keys (must be a config-validation rejection).
- Offline/rescue two-artifact flow (image + RESCUE_DATA media) is a hard v1 requirement, not descope-able.
- Design: hexagonal; ports ImageSource/Decrypter/RescueWriter/ProgressSink etc.; pinned upstream seed types for byte-compatible YAML; SOPS via getsops decrypt in-memory (any top-level `sops` key → decrypt path, failures = ErrDecrypt exit 4); charm log/lipgloss/huh with --interactive/--color/--progress=auto|always|never; digest-keyed download cache; no-clobber atomic two-artifact publication.
Review trail (rounds 1-3) and design versions live in the omp session local:// artifacts.
Next: template rename pass, then prototype spikes named in ARCHITECTURE.md open questions (splice + boot test first).
