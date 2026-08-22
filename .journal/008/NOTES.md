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

## 2026-08-22 10:35 — Handoffs closed, upstream issues filed
Filed three issues rather than keeping the findings local:
- `meigma/release#65` — the PEM encoding required for each native signing key is
  undocumented, and neither wrong encoding fails before a tag exists.
- `meigma/release#66` — required destination checks can be created through the
  rulesets API before their first run, so the how-to's "enable the publisher for
  one real release first" ordering needlessly leaves the destination unprotected.
  Includes both created ruleset IDs and the context-string derivation.
- `meigma/pkgs#12` — that policy still uses pre-#63 `checksum_workflow` /
  `attestation_workflow` and will fail at dispatch time once its receiver pin
  moves past v0.1.16.

R2 credentials (`Componere Packages R2 Production Credentials`) set as
`R2_ACCESS_KEY_ID` / `R2_SECRET_ACCESS_KEY` in `packages-production`; all five
environment secrets now present. Verified with a SigV4 list against the live
bucket: `http=200`, `<Name>componere-packages</Name>`, `KeyCount 0`. Write was
deliberately **not** probed — the token has no delete permission, so any test
object would be permanent in the production bucket.

Apple: developer created `Development/Apple Componere` (raw `Componere.p12`,
`App Store Connect Auth Key.p8`, `passphrase`, `connect_key_id`,
`connect_issuer_id`) rather than copying the meigma credentials — the
recommended option. Pre-flight verified before wiring anything:
- certificate is `Developer ID Application: Joshua Gilman (7MN6B2QY4W)`, issued
  by `Developer ID Certification Authority G2`, valid to 2031-08-23, private key
  present. This is the correct type; an `Apple Development` or
  `Developer ID Installer` certificate would have failed at notarization.
- `openssl pkcs12` needs **`-legacy`** to read the Keychain export under
  OpenSSL 3; without it the extraction silently produces nothing.
- ASC key parses as a 256-bit EC private key (ES256, as Apple requires);
  `connect_key_id` is 10 chars and `connect_issuer_id` is a 36-char UUID.

Five `MACOS_*` org secrets set at `selected` visibility scoped to
`componere/incusos-builder` only. `MACOS_SIGN_P12` and `MACOS_NOTARY_KEY` are
base64 of the raw files (this item stores raw files, unlike
`meigma-ghd-apple`, which stores pre-encoded `.txt` blobs). Temp Apple material
removed.

App changes confirmed: slug `componere-release`, installation `selected`,
permissions unchanged, integration id still `4551177`, so the `incusos-builder`
tag-ruleset bypass still resolves. REST cannot enumerate an installation's
selected repositories without app auth, so tap/bucket/pkgs coverage is unproven
until the first token mint against each.

**Still open: the GHCR package is private.** Reported twice as done, but
`orgs/componere/packages/container/incusos-builder` reports
`visibility: private` with `updated_at` unchanged at 2026-08-17T03:06:46Z, and
an anonymous pull fails: a `ghcr.io/token` scope request followed by a manifest
GET returns **401**. That is not an API cache artifact. The control is on the
package page's Danger Zone, not in org package settings.

Notarization is load-bearing for the cask path, not optional polish:
`meigma/release` sets `sign-and-notarize-macos: true` for its own releases, and
the generated cask carries no `quarantine: false` and no `xattr` postflight.

## 2026-08-22 10:45 — GHCR correction
Correcting the two earlier entries that reported the GHCR package as private:
it is now **public**. The earlier failures were real, not caching — the change
had been applied at the **org** level (package-creation policy) rather than to
the package itself. Applying it in the package's own Danger Zone took effect
immediately: `visibility: public`, `updated_at` 2026-08-22T16:31:53Z.

Verified anonymously, with only a `ghcr.io/token` anonymous pull token:
- `GET /v2/componere/incusos-builder/tags/list` returns
  `["v0.1.0", "sha256-e3bfe748…"]` (the second is the Cosign signature tag).
- `GET /v2/componere/incusos-builder/manifests/v0.1.0` returns **200** with
  `docker-content-digest: sha256:e3bfe74884acbbd707bccca58e89d66ec542c63f365d0f5f73af45a6284b37de`,
  matching the digest recorded in session 005, media type
  `application/vnd.oci.image.index.v1+json`, platforms `linux/amd64` and
  `linux/arm64`.

My first anonymous probe returned 404 because I requested tag `0.1.0`; the
published tag is `v0.1.0`. A 404 there means "no such tag", not "not public" —
the 401 in the previous entry was the real private-state signal.

This closes the README's container install path for anonymous users. The v0.1.0
GitHub Release is still a draft, so the `ghd` path remains broken until that is
published or superseded by the first meigma/release-driven release.

Org setup is complete: all 15 items done, nothing blocked. The one thing REST
cannot confirm is which repositories the `componere-release` installation
selects; that surfaces as a token-mint failure on the first tap, bucket, or
dispatch publication rather than as silent breakage.

## 2026-08-22 11:10 — Producer migration merged, v0.2.0 tagged
Migration PR #39 and publisher-enablement PR #41 merged; release PR #40 merged,
producing tag **v0.2.0** at `37a4619`. Release run `32586815661` is building.

Sequencing that mattered: release-please did **not** rebase its release branch
onto the publisher-enablement commit (a `ci:` commit is hidden from the
changelog, so its 17:06 run saw nothing releasable, and PR #40's head still had
`publish-*: false`). Squash merge applied only the changelog and manifest diff
onto master, so the tagged tree carries `publish-*: true`. Verified directly at
the tag before trusting it:
`gh api '.../contents/.github/workflows/release.yml?ref=v0.2.0'` shows all five
inputs `true`. Merging the release PR without checking would have been a coin
flip.

Chose a single publishing run over the documented rehearse-then-publish flow.
Rationale: a rehearsal at v0.2.0 would then need the tag moved onto the
enable-publishers commit, and every publisher is independently re-runnable at
the same tag, so a failure costs a job re-run rather than a tag move. The
rehearsal value was recovered locally instead (see below).

Local pre-flight, all before pushing anything:
- All six `GOOS`/`GOARCH` targets compile. **Windows did not compile at all**:
  `internal/update` used `unix.Statfs` for the cache free-space check, and the
  release contract requires Windows amd64/arm64 for Scoop. Split into
  `space_unix.go` (statfs) and `space_windows.go`
  (`GetDiskFreeSpaceEx`, whose quota-adjusted result needs no block
  arithmetic); `space_other.go`'s tag became `!linux && !windows`.
- `goreleaser check` passes; a full offline `goreleaser release --skip=publish`
  produced 6 archives, 6 native packages, 12 SBOMs, checksums, the cask, and
  the Scoop manifest.
- **`--skip=sign` also clears nfpm package signatures**, not just the Cosign
  checksum signature (`internal/pipe/nfpm/nfpm.go`: `if skips.Any(ctx,
  skips.Sign) { info.APK.Signature = nfpm.APKSignature{} ... }`). My first
  validation run therefore produced unsigned apks and briefly looked like a
  config defect. Re-running without that skip produced
  `.SIGN.RSA.incusos-builder-apk-001.rsa.pub`, matching `apk_key.published` in
  the `componere/pkgs` policy, plus an RSA-signed RPM. Compared against
  meigma's published v0.1.16 apk, which has the same three-segment layout.
- `mise run image-local` built the image from the rewritten melange/apko;
  `docker run --rm incusos-builder:dev --version` printed the banner and
  `docker inspect` reported user `65532` and entrypoint
  `/usr/bin/incusos-builder` — the two values `image verify` enforces.
- **App installation scope closed.** Minted an App JWT from the 1Password PEM
  and listed `/installation/repositories`: exactly 4 —
  `incusos-builder`, `pkgs`, `homebrew-tap`, `scoop-bucket`. This was the one
  item REST could not answer earlier, and it gates every publisher's token.

Config findings worth keeping:
- melange rejects `vendor`, `homepage`, and `maintainer` under `package:`. The
  maintained `examples/go-release/melange.yaml` sets all three, so the example
  cannot build as written; meigma's own `melange.yaml` omits them.
- `gomod.proxy: true` makes GoReleaser fetch the module from the Go proxy at the
  tag, so a local stage against an unpushed tag fails with
  `invalid version: unknown revision`. Local validation needs `proxy: false`.
- The live branch ruleset still required `Binary Release Dry Run` and
  `Container Image Dry Run` after those workflows were deleted, which blocked
  PR #39 permanently. `configure_github_repo.py apply` failed with
  `PATCH ... 404`, so the ruleset was converged with a direct `PUT`. The
  declared file and live state now agree on `ci` + `GitHub Pages`.
- `security-scan.yml` also built the image from source and was rewritten to
  stage `application` the same way the release does.
