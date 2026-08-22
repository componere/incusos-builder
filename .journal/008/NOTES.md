---
id: 008
title: New session
started: 2026-08-22
---

## 2026-08-22 08:42 — Kickoff
Goal for the session: not yet stated. The developer asked only to create a new
session; the actual request follows.

Current state of the world:
- `master` at `065b9e8` ("docs: rewrite the README for a released tool (#31)"),
  clean, in sync with `origin/master`.
- All seven plan phases from `.journal/001/PLAN.md` are merged. v0.1.0 was
  tagged and built on 2026-08-17; its artifacts are signed, attested, and
  consumer-verified.
- Open thread carried from session 005: the **v0.1.0 GitHub Release is still a
  draft** (`gh release edit v0.1.0 --draft=false` publishes it), while the GHCR
  image is already public. The README `ghd` install path does not work for
  unauthenticated users until the release is published.
- Other carried threads: `N-MEDIA-3` (NUL-padded ISO volume identifier
  untested), `fix-after-v1` deferrals F-CFG-1 / N-ART-5 / N-APUB-2, nine
  `repository-settings.toml` keys needing manual GitHub UI follow-up, and
  product follow-ups in issues #14–#18.
- Sessions 006 and 007 remain `in-progress` and are not part of this session.
  Session 007 left `PLAN.md` plus research on install channels
  (ghd/tap/scoop/nix) in `.journal/007/`.

Plan: wait for the developer's stated goal, then scope the work, load any
task-relevant skills beyond `git` and `worktrunk`, and branch off the fetched
default branch in a fresh Worktrunk worktree under `.wt/` before touching code.

## 2026-08-22 09:05 — Goal and org-state review
Goal: configure the `componere` org (and later `incusos-builder`) to release
across the full supported spectrum using `meigma/release`. Today: org setup only,
review then propose a plan.

Reviewed `~/code/meigma/release`:
- Release unit = one reviewed full `meigma/release` commit SHA covering the
  reusable workflows, setup action, and `release-cli`. Destinations: GitHub
  Release assets, GHCR multi-arch image, Homebrew cask PR, Scoop manifest PR,
  and DEB/RPM/APK in Cloudflare R2 via a central receiver repo.
- Authoritative org guide: `docs/how-to/prepare-your-github-organization.md`;
  destinations in `add-homebrew-and-scoop.md` and
  `operate-a-native-package-repository.md`; contracts in
  `docs/reference/release-system.md`.
- Local HEAD `2d524ae` ("fix(pkgrepo): support cross-organization release
  workflows", #63) is **one commit past the latest release v0.1.16** and is the
  commit that makes cross-org adoption sound (explicit `checksum_identity` /
  `attestation_signer` instead of deriving signer identity from the producer
  repo). componere adoption depends on it.

Observed live state (via `gh`):
- `componere` org: free plan (no org rulesets), Actions `allowed_actions: all`,
  `sha_pinning_required: false`, default workflow token `read`. **No org-level
  Actions variables or secrets exist.**
- App `componere-release-please` (integration id `4551177`) is installed on
  **all** componere repos with exactly the required permissions: contents:write,
  issues:write, pull_requests:write, metadata:read.
- `COMPONERE_RELEASE_APP_CLIENT_ID` / `COMPONERE_RELEASE_APP_PRIVATE_KEY` exist
  only as **repo-level** values on `incusos-builder`.
- `incusos-builder` rulesets: branch (`pull_request`, `required_status_checks`,
  `required_signatures`, linear history, no force push/delete) and tag
  (`creation`/`update`/`deletion`/`required_signatures`/`non_fast_forward`) with
  bypass for RepositoryRole 5 and Integration 4551177 on tags.
- **Finding: GHCR package `componere/incusos-builder` is `private`.** TECH_NOTES
  asserted the image goes public on push; it did not. The README container
  install path is broken for anonymous users today, as is the still-draft
  v0.1.0 GitHub Release.
- No `homebrew-*`, `scoop-*`, or central package repo exists in componere.

`meigma` reference layout (also free plan, so all of this is achievable):
- App `meigma-release` (selected repos), org var `MEIGMA_RELEASE_APP_CLIENT_ID`
  and org secret `MEIGMA_RELEASE_APP_PRIVATE_KEY`, both `selected` visibility
  (`meigma/release`, `meigma/pkgs`) — selected-repo visibility works on free.
- Five `MACOS_*` org secrets (selected: `meigma/release`).
- `meigma/homebrew-tap`, `meigma/scoop-bucket`, and `meigma/pkgs` (central
  receiver: `.config/package-repository.yaml`, `.config/keys/`, receiver
  workflow pinned to v0.1.16, `packages-production` environment with R2 +
  aggregate signing secrets, vars `CLOUDFLARE_ACCOUNT_ID` /
  `PACKAGE_REPOSITORY_R2_BUCKET`, bucket `meigma-packages`, origin
  `https://pkgs.meigma.dev`).
- `meigma/pkgs` policy still uses the pre-#63 `checksum_workflow` /
  `attestation_workflow` keys, so it must be updated when the release unit moves
  past v0.1.16.

Next: propose the org plan and get decisions on App reuse, package-repository
tenancy (own componere R2/domain vs reuse `meigma/pkgs`), and macOS signing.
