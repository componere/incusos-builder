---
id: 007
title: New session
started: 2026-08-16
---

## 2026-08-16 20:27 — Kickoff
Goal for the session: not yet stated. The developer asked to start a new session
and has not given the actual request; this entry records the starting state so
the first real task can be logged against it.

Current state of the world:
- `master` at `065b9e8` ("docs: rewrite the README for a released tool (#31)"),
  clean, level with `origin/master`.
- All seven implementation-plan phases (`.journal/001/PLAN.md`) are merged.
- `v0.1.0` was tagged and built on 2026-08-17: binaries and the multi-arch image
  are signed, attested, and consumer-verified. **The GitHub Release is still a
  draft**; the container image is already public because GHCR has no draft state.
- Boot acceptance passed once (Semaphore Cloud, nested KVM) — seed consumption
  observed for the first time. Record: `.journal/005/BOOT_10_RECORD.md`.
- `.github/repository-settings.toml` has been applied; nine manifest keys are
  not REST-exposed and remain manual GitHub UI follow-ups.
- Known open threads carried in from session 005: publish the draft release
  (`gh release edit v0.1.0 --draft=false`), untested NUL-padded ISO volume
  identifier (`N-MEDIA-3`), and `fix-after-v1` deferrals F-CFG-1, N-ART-5,
  N-APUB-2. Follow-up product work is issues #14–#18.
- Session 006 is open concurrently and is not owned by this session.

Plan: wait for the developer's actual request, then work it inside a Worktrunk
worktree created from the fetched default branch, integrating via a GitHub PR and
squash merge.

## 2026-08-16 21:05 — Goal set: replace ghd with Nix/Homebrew/Scoop/installer
Developer's request: (1) remove `ghd` entirely, (2) add Nix, Homebrew, Scoop, and
a downloader bash script as install channels, (3) rewrite install instructions to
present the full set. Reference material: `~/code/meigma/template-go`,
`template-go-api` (release automation), `meigma/homebrew-tap` (canonically
`meigma/homebrew-tap`, old alias `meigma/tap`) and `meigma/scoop-bucket`
(publishing targets). #2 requires new `componere` tap and scoop repos plus the
automation to update them.

Explored with four parallel research agents. Reports saved to
`local://research-{TemplateGoRelease,TapScoopPublishing,IncusosGhdInventory,NixAndInstallScript}.md`
(session-local; also `agent://<name>`).

Findings that drive the plan:
- **Templates are NOT the reference for the new channels.** `template-go` and
  `template-go-api` still use ghd (`ghd.toml` + `.github/scripts/stage_ghd_release_assets.py`),
  ship raw binaries (`archives.formats: [binary]`), and have no brew/scoop/nix
  publishers. `template-go-api` is actually broken: its release/dry-run jobs call
  the staging script but `ghd.toml` does not exist on master.
- **`meigma/whzbox` is the real reference** for tap+scoop: GoReleaser `brews:` +
  `scoops:` with `HOMEBREW_TAP_TOKEN` / `SCOOP_BUCKET_TOKEN` repo secrets,
  `skip_upload: auto`, tar.gz + windows zip archives, `linux|darwin|windows` ×
  `amd64|arm64` minus windows/arm64. Two landmines already fixed there:
  draft discovery needs `contents: write` (PR #42), and `gh release edit` in a
  checkout-less job needs `--repo "$GITHUB_REPOSITORY"` (PR #44).
- **Windows is required by Scoop and does not compile today.** Measured with
  `GOOS=windows GOARCH=amd64 go build ./...`: exactly three errors, all in
  `internal/update` (`cache.go` `unix.Statfs`, `space_other.go` tagged `!linux`
  but importing `x/sys/unix`). Everything else, go-diskfs included, typechecks
  for Windows. Fix is a `space_windows.go` using `GetDiskFreeSpaceEx`.
- **Archive format must change** from raw binary to tar.gz (+ windows zip):
  Homebrew, Scoop, and GoReleaser's `nix:` generator all refuse raw binaries.
  That cascades into the staging script, both smoke tests, `checksums.txt`
  contents (the attestation subject), the release summary, and README.
- **Draft-release timing is the central design problem.** This repo deliberately
  leaves the GitHub Release as a draft for human inspection, but tap/scoop/nix
  metadata points at release asset URLs that 404 until the draft is published.
  whzbox solved it by auto-publishing the draft at the end of the release run.
- **`brews:` is deprecated in GoReleaser v2.10** in favour of `homebrew_casks:`,
  but casks are macOS-only, so a Linux-first CLI still needs a formula.
- **Nix**: GoReleaser's `nix:` is OSS (not Pro), needs `nix-hash` on the runner
  and a real archive, and writes `pkgs/<name>/default.nix` into a NUR-style repo.
  Alternative with no third repo: in-repo `flake.nix` with `buildGoModule`
  (`vendorHash` upkeep). The developer only named tap + scoop as new repos.
- `componere` has no tap/bucket repo yet; existing repos default to `master`,
  but the publishing repos should use `main` to match GoReleaser targets.

Next: planner agent produces the implementation plan.

## 2026-08-16 21:55 — Plan delivered
`InstallChannelPlanner` (planner agent) produced `.journal/007/PLAN.md`. The four
research reports are alongside it as `research-*.md`.

Two facts I established from GoReleaser v2.17.1 source and fed to the planner,
because they decide the whole shape of the change:
- `release.disable: true` disables only the release client, not the `brews`/
  `scoops`/`nix` pipes (`internal/pipe/brew/brew.go:54`). `--skip=publish` is what
  suppresses them, and this repo passes it today.
- With `release.disable: true`, `client.NewReleaseClient` returns
  `errURLTemplater` (`internal/client/client.go:115-135`), so every channel
  publisher must set an explicit `url_template` or the run fails with
  `ErrReleaseDisabled`. brew/scoop/nix each only consult the client when their own
  `url_template` is empty.

Plan shape: 7 phases / 6 implementation PRs. Phase 0 is a throwaway GoReleaser
spike to freeze the real `dist/` layout before writing validators. Then Windows
port (`internal/update/space_windows.go`), in-repo Nix flake, archive migration +
ghd deletion, `install.sh`, Homebrew/Scoop generation with post-publication
`publish-channels.yml`, docs. First real release with the new contract is v0.2.0;
v0.1.0's draft is left alone.

Decisions I directed rather than leaving open:
- Nix = in-repo `flake.nix` with `buildGoModule`, not a third `componere/nix`
  repo. Reversible, no extra credential, and the developer named only two new
  repos. Costs owned in the plan: `vendorHash` refresh procedure, a required `Nix`
  CI context, `doCheck = false`, and docs stating the Nix output is a source build
  and NOT the attested archive.
- Homebrew formula only, no cask: `brews:` is deprecated since GoReleaser v2.10
  but casks are macOS-only and this CLI is Linux-first.
- Channel metadata is generated during the tag run with `skip_upload: true`,
  retained as an Actions artifact, and committed by a `release: published`
  workflow. That keeps the human draft gate and avoids depending on a second
  build being byte-identical.

Awaiting developer decisions: create `componere/homebrew-tap` and
`componere/scoop-bucket` (public, default branch `main`) and provision
`HOMEBREW_TAP_TOKEN` / `SCOOP_BUCKET_TOKEN` (fine-grained PAT, Contents: write,
scoped to one repo each) — human-only prerequisites that block PR 5 only.
