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

## 2026-08-22 11:55 — v0.2.0 published; native packages blocked
Release run `32586815661` succeeded in **every** job. v0.2.0 is public.

Proven, not assumed:
- **Assets**: 26 assets. Downloaded four across three formats and both OS
  families; all match `checksums.txt`. `cosign verify-blob` on
  `checksums.txt.sigstore.json` returns `Verified OK` against identity
  `https://github.com/meigma/release/.github/workflows/go-pre-publish.yml@0dee66ff…`.
  Paired falsifier (claiming our own `release.yml` as signer) fails and names
  the real SAN — so the signature genuinely binds to the pinned shared workflow.
- **Attestation**: `gh attestation verify --signer-workflow
  meigma/release/.github/workflows/publish-github-release.yml` gives
  `buildSignerURI=…publish-github-release.yml@0dee66ff…`,
  `sourceRepositoryURI=https://github.com/componere/incusos-builder`,
  `sourceRepositoryRef=refs/tags/v0.2.0`,
  `sourceRepositoryDigest=37a4619`. Two falsifiers (foreign signer workflow,
  wrong `--source-ref`) both fail. Bare success prints nothing, as expected.
- **macOS**: the darwin/arm64 binary reports
  `Authority=Developer ID Application: Joshua Gilman (7MN6B2QY4W)`, hardened
  runtime, timestamped, and `spctl -a -t install` says
  `source=Notarized Developer ID`. `codesign --test-requirement="=notarized"`
  passes. Apple notarization works with the new componere credentials.
- **Image**: anonymous pull of `0.2.0` → index
  `sha256:4f19971c…`, `linux/amd64` + `linux/arm64`, annotations carry
  `revision=37a4619` and `version=0.2.0` (both injected, as designed). Channel
  tags `0.2.0`, `0.2`, `0`, `latest` all present. `docker run` prints
  `incusos-builder 0.2.0 (37a4619…)`; config user `65532`, entrypoint
  `/usr/bin/incusos-builder`. `cosign verify` returns both a
  `cosign/sign/v1` and a `slsa.dev/provenance/v1` claim (recursive signing).
  Index provenance and the amd64 platform SPDX SBOM attestation both verify
  against the pinned `publish-oci-image.yml`.
- **Homebrew**: `brew tap` + `brew install --cask componere/tap/incusos-builder`
  installed 0.2.0 on this machine and `incusos-builder --version` ran. Notable:
  the installed binary carries `com.apple.quarantine` and still executes —
  which is only true because it is notarized. That settles the earlier question
  about whether notarization was load-bearing for the cask path: it is.
  Uninstalled and untapped afterward.
- **Destinations**: tap PR #1 and bucket PR #1 both passed validation on real
  runners (`macos-26`, `windows-2025`, `windows-11-arm`) and were merged. The
  required check contexts registered before they had ever run reported exactly
  as predicted, which closes the loop on `meigma/release#66`.

**Blocked: native packages.** Three receiver runs in `componere/pkgs` fail at
`setup-package-repository` with `GPG_PASSPHRASE is empty`; the GPG and APK key
secrets are empty too. Diagnosis, in order:
1. A temporary direct job in `componere/pkgs` selecting the same environment
   printed `gpg_passphrase_len=32`, `gpg_private_key_len=4756`,
   `apk_private_key_len=4324`, `r2_key_id_len=32`. **The secrets are correct at
   rest**; the reusable workflow is not receiving them.
2. Removing `secrets: inherit` (matching the doc template) did not help.
   Restoring it did not help. `meigma/pkgs` commit `3193b8a5`
   ("fix(release): inherit publication secrets") proves it is required there.
3. Adding a dummy repo-level secret, to rule out an empty inherited set, did not
   help.
4. The two callers differ by **one line**: meigma pins v0.1.16 and publishes
   successfully; we pin v0.1.17. `git diff v0.1.16 v0.1.17` over
   `publish-package-repository.yml` and `setup-package-repository/` is empty —
   byte-identical files.

So it is either a v0.1.17 regression outside those files or a GitHub platform
change today. `meigma/pkgs` has a `repository_dispatch` run from 15:59Z still
`waiting`; approving it settles which, but that publishes into the other org, so
it is the developer's call. Filed as `meigma/release#67` with all of the above.

Workaround available but not taken unilaterally: move the five values to
repository-level secrets in `componere/pkgs` so `secrets: inherit` delivers
them. The `packages-production` required reviewer still applies because the
called job selects the environment regardless; what is lost is that the
aggregate keys become readable by any workflow in that repo, contradicting the
guide's stated boundary. Needs a decision, and `op read` needs Touch ID
approval (it timed out twice with `authorization timeout`).

Leftovers in `componere/pkgs`: the probe workflow is removed; the diagnostic
secret `PACKAGE_REPOSITORY_INHERIT_MARKER` and the revert commits stay until
this is resolved.

## 2026-08-22 12:00 — Correction: the package blocker is cross-organization
Correcting the previous entry's two candidate causes. Both are wrong.

Pinned `componere/pkgs` temporarily to **v0.1.16** — the exact SHA `meigma/pkgs`
published with successfully at 04:22Z today — and it failed identically with
empty `GPG_PRIVATE_KEY` / `GPG_PASSPHRASE` / `APK_PRIVATE_KEY` (run
`32588946064`). Pin reverted. So it is neither a v0.1.17 regression nor a
platform change, and approving meigma's pending run is unnecessary.

The distinguishing factor is the only remaining difference between the callers:

| Caller | Callee | Same org | Environment secrets |
|---|---|---|---|
| `meigma/pkgs` | `meigma/release` | yes | delivered |
| `componere/pkgs` | `meigma/release` | **no** | **empty** |

GitHub does not surface a caller's **environment** secrets to a reusable
workflow owned by a different organization. Only secrets that cross the
boundary explicitly — passed by name, or inherited from the caller's repository
and organization scopes — arrive. That explains every observation: the same
secrets resolve in a direct job in the same repo, `secrets: inherit` is
necessary but insufficient because environment scope is not something `inherit`
can reach, and the receiver's own files are irrelevant.

This is a genuine gap in the release unit, not an adopter mistake: #63 shipped
cross-org support, but `publish-package-repository.yml` reads its aggregate keys
from `secrets.*` while declaring `environment: packages-production`, which only
works intra-org. The guide's stated boundary — aggregate private keys only in
the central protected environment — is **not achievable cross-organization**
with the current receiver. Reported on `meigma/release#67` with a suggested fix
(declare them as `workflow_call` secrets, or document the trade-off).

Consequence for componere: the only way to publish native packages today is to
hold the five values in `componere/pkgs` repository or organization secrets so
`secrets: inherit` delivers them. The `packages-production` required reviewer
still gates publication because the called job selects the environment
regardless; what is lost is that the aggregate keys become readable by any
workflow in that repository. Awaiting the developer's decision plus a Touch ID
approval for `op read`.

## 2026-08-22 12:20 — Native packages published; all five channels live
The developer chose to fix the root cause rather than relocate secrets. The
root fix needed no fork:

- The receiver's own setup actions (`setup-release-cli`,
  `setup-package-repository`) already accept `owner/repo/path@ref` references.
- `componere/pkgs/.github/workflows/publish.yml` is now a **local** workflow
  whose job selects `packages-production` itself, mirroring the upstream
  receiver at `0dee66ff…` step for step. Same-repo job → environment secrets
  resolve natively; aggregate keys stay environment-only, preserving the trust
  boundary the reusable path could only achieve intra-org.
- `local-build: always` is intra-repo-only, so the local receiver uses the
  default acquisition mode: it downloads the stamped v0.1.17 `release-cli`
  from the meigma/release GitHub Release with checksum + attestation
  verification. Stronger provenance than a runner-local source build.
- Forking meigma/release was considered and rejected: the producer side works
  cross-org already (explicit named secrets), every `checksum_identity` /
  `attestation_signer` names meigma/release, and a fork has no releases for
  `setup-release-cli` to verify. Only the receiver had the defect.

First publication through the local receiver: run `32589946603`, envelope
`{"state":"published","artifacts":36,"uploaded":36}`. All ten public roots and
keys return 200.

Client proof, each in a fresh container following the shipped install doc:
- Debian (`apt-get install incusos-builder`, signed-by aggregate key) → 0.2.0
- Fedora (`dnf install`, `gpgcheck=1` + `repo_gpgcheck=1`, both keys in
  `gpgkey=`) → 0.2.0; dnf's "Verify package files" gate passed, so the
  producer RPM signature verified
- Alpine (`apk add`, both published keys in `/etc/apk/keys`) → 0.2.0; this one
  pulled the **amd64** package under emulation, so both architectures were
  exercised across the three tests

Cleanup complete: `PACKAGE_REPOSITORY_INHERIT_MARKER` deleted, probe workflow
and temporary pin already removed, `docs/operations.md` in componere/pkgs now
describes the local receiver. Upstream `meigma/release#67` updated with the
working shape and a suggestion to ship it as `release-cli init
package-repository`, matching the tap/bucket initializer pattern.

**v0.2.0 is fully published across all five channels: GitHub Release, GHCR
image, Homebrew cask, Scoop manifest, and APT/DNF/APK at pkgs.componere.dev —
each verified from the consumer side.**

## 2026-08-22 12:25 — Cross-org receiver documented upstream
The developer asked whether the local-receiver finding should go upstream.
Answer: yes, as docs, not just the #67 comment — the how-to is what the next
cross-org adopter reads before their first publication fails. Merged
`meigma/release#68` ("docs(packages): document the cross-organization receiver
shape"):
- `operate-a-native-package-repository.md` now has two receiver variants:
  the reusable call (same-org only) and the step-for-step local workflow
  (cross-org), with the `local-build: always` limitation called out.
- `release-system.md` states the same-organization constraint on
  `publish-package-repository.yml` next to its interface table.
The feature-sized follow-up (`release-cli init package-repository` emitting the
scaffold) remains tracked on `meigma/release#67`. Worktree pruned; branch
deletion needed `-D` because squash-merge leaves the branch unmerged in git's
eyes.

## 2026-08-22 12:40 — Close
Session closed. All work is merged and every local default branch is
fast-forwarded:
- componere/incusos-builder: PRs #39 (producer migration), #41 (enable
  publishers), #40 (release 0.2.0 → tag v0.2.0 at `37a4619`); master at
  `37a4619`.
- meigma/release: PRs #64 (release v0.1.17 = release unit `0dee66ff…`), #68
  (cross-org receiver docs); main at `a05761a`.
- componere/homebrew-tap #1 and componere/scoop-bucket #1 merged;
  componere/pkgs received direct pushes (policy repo, local receiver) and its
  publication run `32589946603` uploaded 36/36 artifacts.
Session worktrees removed; only master and journal/jmgilman remain.

Handoff state: **v0.2.0 fully published and consumer-verified on all five
channels.** Open items live in SUMMARY.md's Open Threads — chiefly the v0.1.0
draft release decision, meigma/release#65–#67 + meigma/pkgs#12, the unfiled
melange example defect, and the local-receiver re-diff duty on release-unit
bumps. SUMMARY.md written; INDEX.md row set to complete; TECH_NOTES.md revised
(release unit, native packages, GoReleaser gotchas, new signer identities,
ruleset lesson).
