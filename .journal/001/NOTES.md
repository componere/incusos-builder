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
