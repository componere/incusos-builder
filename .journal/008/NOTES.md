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

## 2026-08-22 09:10 — Decisions
Developer chose: reuse `componere-release-please` and rename it to
`componere-release`; own `componere/pkgs` + `componere-packages` R2 bucket +
`pkgs.componere.dev`; enable macOS signing/notarization now.

## 2026-08-22 10:05 — Org setup executed
Release unit pinned: **`0dee66ff6c4cc7e28d7bb65e97a37d701e0eff4a` (v0.1.17)**.
Merged `meigma/release` PR #64 (`chore(main): release 0.1.17`) as `0dee66f`;
the Release workflow completed successfully, so #63's cross-org support is now
released. Every componere pin uses that one SHA.

Done and verified:
- `release-cli` 0.1.17 installed via `mise x github:meigma/release@0.1.17`;
  mise verified the checksum and GitHub artifact attestations, and
  `release-cli version --json` reports `commit=0dee66ff…`, matching the pin.
- Cloudflare: bucket `componere-packages` created (WNAM, standard), custom
  domain `pkgs.componere.dev` attached to the bucket root with minTLS 1.2 in
  zone `5a708e9af40df8e2c7bec7d6e8a96732`. `curl https://pkgs.componere.dev/`
  returns 404 with `ssl_verify_result=0`, so DNS and TLS are live against an
  empty bucket.
- **Cloudflare API-token management is out of scope for the local token**
  (`9109 Unauthorized to access requested resource` on both
  `/accounts/{id}/tokens/permission_groups` and `/user/tokens/...`), so the
  bucket-scoped R2 S3 access key must be minted in the dashboard.
- Signing keys generated locally in a temp `GNUPGHOME`, then **format-verified
  by decrypting each one with the same primitives the pipeline uses** rather
  than assumed:
  - aggregate OpenPGP RSA-4096 `46ED937D27753B5F0180F94DDF46DA94E742C7F9`;
  - aggregate APK RSA-4096 as an **unencrypted traditional PEM**, because
    `repogen.generateAPK` runs `abuild-sign -k` inside Alpine;
  - producer RPM OpenPGP RSA-4096 `9149E2C3CC98D5C162C107F00B8DDE3EA1AE1A9D`;
  - producer APK RSA-4096 as a **legacy-encrypted PEM** (`Proc-Type: 4,ENCRYPTED`
    / `DEK-Info: AES-256-CBC`), because nfpm v2.47.0
    `internal/sign/rsa.go` uses `x509.DecryptPEMBlock` + `ParsePKCS1PrivateKey`.
    An OpenSSL-3 default PKCS#8 `ENCRYPTED PRIVATE KEY` would not have worked;
    `openssl rsa -traditional -aes256` is required.
- 1Password `Development`: new items `Componere Packages Aggregate Signing Keys`
  and `Componere incusos-builder Package Signing Keys` hold every private key,
  public key, and passphrase. 1Password assignment syntax rejects field labels
  containing unescaped periods, so attachment labels use underscores.
- `componere/pkgs` created (public): policy with `origin:
  https://pkgs.componere.dev`, producer `componere/incusos-builder` / package
  `incusos-builder`, post-#63 `checksum_identity` +
  `attestation_signer` fields, four public keys under `.config/keys/`,
  receiver `.github/workflows/publish.yml` pinned to the release unit with
  `secrets: inherit` (mirroring meigma's live receiver, not the doc template),
  CODEOWNERS, and `docs/operations.md`. Vars `CLOUDFLARE_ACCOUNT_ID` and
  `PACKAGE_REPOSITORY_R2_BUCKET` set; environment `packages-production` created
  with required reviewer `jmgilman` (id 2308444), protected-branch policy, and
  the three aggregate signing secrets.
- `componere/incusos-builder` producer secrets set: `RPM_SIGNING_KEY`,
  `RPM_SIGNING_PASSPHRASE`, `APK_SIGNING_KEY`, `APK_SIGNING_PASSPHRASE`.
- Org variable `COMPONERE_RELEASE_APP_CLIENT_ID` and org secret
  `COMPONERE_RELEASE_APP_PRIVATE_KEY` created at `selected` visibility scoped to
  `componere/incusos-builder` only (the tap, bucket, and receiver never need the
  key). Repo-level duplicates deleted.
- **Credential migration proven live**: dispatched `release-please.yml` in
  `incusos-builder` (run `32583533995`); `Create release app token` succeeded
  against the org-level values and no release PR was opened, so the migration
  had no side effects.
- `componere/homebrew-tap` and `componere/scoop-bucket` created from
  `release-cli init`; both generated CI workflows pin
  `meigma/release@0dee66ff…`. Active `Default branch` rulesets on each: PR
  required (0 approvals, squash only), required check
  `casks / Homebrew cask validation` / `manifests / Scoop manifest validation`
  (integration 15368), block deletion, block non-fast-forward, admin bypass.
  **The rulesets API accepted the check contexts before they had ever run**,
  which sidesteps the chicken-and-egg the how-to describes for the UI. The
  context strings match `meigma/homebrew-tap`'s live classic protection exactly.
- Actions policy verified: org `allowed_actions: all`, `sha_pinning_required:
  false`, default token `read` on the org and all four repos. Nothing needed
  changing. Do **not** enable `sha_pinning_required`: the copied caller uses a
  tag-pinned `googleapis/release-please-action`.

Blocked or handed off:
- Five `MACOS_*` org secrets: **stopped before reading any Apple value.** The
  candidates are meigma-scoped (`Development/meigma-ghd-apple` has
  `MACOS_SIGN_P12.txt`, `cert_password`, `MACOS_NOTARY_KEY.txt`,
  `app_store_connect_key_id`, `app_store_connect_issuer_id`;
  `Development/Apple Meigma` holds the account-level pair). Copying either into
  componere spreads one Apple credential set across two orgs; a
  componere-specific Developer ID cert avoids that. Awaiting the developer.
- `R2_ACCESS_KEY_ID` / `R2_SECRET_ACCESS_KEY` in `packages-production`: needs a
  dashboard-minted, bucket-scoped R2 S3 token (list/read/write, no delete).
- UI-only: rename the App to `componere-release`, narrow its installation from
  *all repositories* to `incusos-builder`, `homebrew-tap`, `scoop-bucket`,
  `pkgs` (all four are currently covered only because the install is org-wide),
  and flip the GHCR package `componere/incusos-builder` to public.
- No standalone policy-validation command exists in `release-cli` 0.1.17
  (`init`, `stage`, `image`, `plan`, `publish`, `verify`, `version`), so
  `.config/package-repository.yaml` is first validated by a real receiver run.
