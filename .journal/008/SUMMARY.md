---
id: 008
title: Full five-channel release via meigma/release
date: 2026-08-22
status: complete
repos_touched: [incusos-builder, meigma/release, componere/pkgs, componere/homebrew-tap, componere/scoop-bucket]
related_sessions: [001, 004, 005]
---

## Goal
Configure the `componere` organization and `incusos-builder` to release across
the full supported spectrum of `~/code/meigma/release` (GitHub Release, GHCR
image, Homebrew, Scoop, native DEB/RPM/APK), then produce a fully published
release and prove it.

## Outcome
Goal met. **v0.2.0 is published on all five channels and every channel was
verified from the consumer side**: 26 release assets with a falsifier-checked
Cosign bundle and attestations; a signed, attested multi-arch image at
`ghcr.io/componere/incusos-builder` (tags `0.2.0`/`0.2`/`0`/`latest`); a cask
installed and run via `brew install --cask componere/tap/incusos-builder` on
this machine; a Scoop manifest validated on `windows-2025` and
`windows-11-arm`; and `apt-get`/`dnf`/`apk` installs from
`https://pkgs.componere.dev` in fresh Debian/Fedora/Alpine containers, covering
both architectures. macOS binaries are Developer ID-signed and notarized
(Gatekeeper: `Notarized Developer ID`).

The org was built out in the same session: App renamed `componere-release` and
narrowed to four selected repos, org-level credential and five `MACOS_*`
secrets at selected visibility, `componere/{pkgs,homebrew-tap,scoop-bucket}`
created with rulesets, R2 bucket `componere-packages` + `pkgs.componere.dev`,
and fresh aggregate/producer signing keys (stored in 1Password `Development`).
Everything pins release unit `0dee66ff6c4cc7e28d7bb65e97a37d701e0eff4a`
(v0.1.17), which this session released by merging meigma/release PR #64.

## Key Decisions
- Reuse the existing App (renamed `componere-release`) -> permissions already
  exactly matched and the tag-ruleset bypass keys on integration id `4551177`,
  which a rename does not change.
- Own `componere/pkgs` + `pkgs.componere.dev` instead of sharing `meigma/pkgs`
  -> `componere.dev` already lives in the same Cloudflare account, and sharing
  would force one App to be installable across both orgs.
- Componere-scoped Apple credentials in the existing Team -> Apple trust is
  per-Team, so a second Developer ID cert changes nothing user-visible but
  keeps the two orgs independently revocable.
- Single publishing run instead of the documented rehearse-then-retag flow ->
  every publisher is independently re-runnable at the same tag, so a failure
  costs a job re-run, not tag surgery; rehearsal value was recovered locally
  (full offline goreleaser run, signed packages with throwaway keys,
  image-local build matching the verifier's requirements).
- Enable all five publishers in one commit -> `publish-package-repository`
  requires `publish-release` in the same run and `github-release` requires the
  digest-pinned image, so partial enables fail closed or ship inconsistent
  channels.
- Fix the native-package blocker at the root (local receiver workflow) instead
  of relocating secrets or forking -> GitHub does not deliver caller
  environment secrets to a reusable workflow owned by another org; a local job
  selecting `packages-production` restores env-only aggregate keys, and a fork
  would break every `checksum_identity`/`attestation_signer` and the CLI's
  attestation-verified download.
- Leave the v0.1.0 draft release untouched -> its publication gate exists for
  human inspection; v0.2.0 supersedes it.

## Changes
- Org/infra: App renamed + selected install (verified via App JWT: exactly
  `incusos-builder`, `pkgs`, `homebrew-tap`, `scoop-bucket`); org variable
  `COMPONERE_RELEASE_APP_CLIENT_ID` + secret `COMPONERE_RELEASE_APP_PRIVATE_KEY`
  (selected: incusos-builder; repo-level duplicates deleted); five `MACOS_*`
  org secrets; GHCR package public; R2 bucket + custom domain; four signing
  keys in two new 1Password items; producer signing secrets on incusos-builder.
- `componere/pkgs` — policy (`checksum_identity`/`attestation_signer` pinned),
  public keys, `packages-production` env (required reviewer, 5 secrets),
  receiver now a **local** workflow mirroring the upstream reusable one.
- `componere/homebrew-tap`, `componere/scoop-bucket` — `release-cli init`
  scaffolds; rulesets created via API with required-check contexts registered
  before first run; first cask/manifest merged.
- `incusos-builder` PR #39 — bespoke `release.yml`/`attest.yml`/dry-run
  retired; caller pinned to the release unit; new `.goreleaser.yaml`;
  melange/apko rewritten to package the staged GoReleaser binary
  (`application`); `internal/update` Windows support (`space_windows.go` with
  `GetDiskFreeSpaceEx`; `space_unix.go`; `space_other.go` tag now
  `!linux && !windows`); `ghd.toml` on archive assets + shared signer;
  `security-scan.yml` stages the binary; docs (new `install.md`, trust-model,
  automation, run-in-ci) and repo skills rewritten; PR #41 enabled the five
  publishers; PR #40 tagged v0.2.0.
- `meigma/release` — merged #64 (released v0.1.17 with the cross-org #63 fix)
  and #68 (docs: cross-org receiver shape in the how-to + same-org constraint
  in the reference).

## Open Threads
- The **v0.1.0 GitHub Release is still a draft**, now superseded by v0.2.0;
  publish or delete it deliberately.
- `meigma/release#67`: feature-sized fix (`release-cli init
  package-repository` scaffold) still open; `componere/pkgs`'s local receiver
  must be re-diffed against the upstream receiver on every release-unit bump
  until it ships.
- Upstream issues filed this session, all open: #65 (native key PEM encodings
  undocumented, fails post-tag), #66 (rulesets API beats the UI
  chicken-and-egg; `init` should emit ruleset JSON), #67 (cross-org env-secret
  boundary), meigma/pkgs#12 (pre-#63 policy fields break on next pin bump).
  Also unfiled: `examples/go-release/melange.yaml` sets `vendor`/`homepage`/
  `maintainer`, which melange rejects — the example cannot build as written.
- Sessions 006 and 007 remain in-progress and untouched; incusos-builder
  issues #14–#18 and session 005's N-MEDIA-3 / fix-after-v1 deferrals carry.
- Six Dependabot PRs (#32–#38) open on incusos-builder, not session work.

## Lessons
- **Environment secrets do not cross the organization boundary into reusable
  workflows**, and they fail as silently-empty strings, not errors.
  `secrets: inherit` reaches only repo/org scopes. Diagnosis required a probe
  job (same env, direct job: secrets resolve) plus pinning our caller to the
  exact SHA that works intra-org elsewhere (still empty) — isolating org
  ownership as the only variable.
- **`--skip=sign` in GoReleaser also strips nfpm package signatures**, not
  just Cosign steps; a validation run with that flag looks like a signing
  config defect.
- **`gomod.proxy: true` makes local stage validation impossible against an
  unpushed tag** — GoReleaser fetches the module from the Go proxy at the tag.
- Release Please does not rebase its release branch for commit types hidden
  from the changelog: the release PR's head still had `publish-*: false`, and
  only the squash-merge onto master carried the enablement into the tagged
  tree. Verify the workflow **at the tag** before trusting a merged release PR.
- A ruleset requiring checks from deleted workflows blocks every PR silently;
  required-check lists must be updated in the same change that deletes a
  workflow.
- `openssl pkcs12` needs `-legacy` for Keychain exports under OpenSSL 3, and
  nfpm needs traditional (`Proc-Type: 4,ENCRYPTED`) RSA PEMs, while
  `abuild-sign` needs the aggregate APK key unencrypted — none of it guessable,
  all verified by decrypting with the consumers' own primitives before use.

## References
- PRs: incusos-builder #39/#40/#41; meigma/release #64/#68; homebrew-tap #1;
  scoop-bucket #1.
- Release: https://github.com/componere/incusos-builder/releases/tag/v0.2.0
  (tag `37a4619`); image index `sha256:4f19971c…`; publication run
  componere/pkgs `32589946603` (`36/36 uploaded`).
- Issues: meigma/release #65, #66, #67; meigma/pkgs #12.
- Release unit: `meigma/release@0dee66ff6c4cc7e28d7bb65e97a37d701e0eff4a`
  (v0.1.17).
- Prior sessions: `.journal/005/SUMMARY.md` (v0.1.0), `.journal/001/PLAN.md`.
