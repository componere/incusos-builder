# `incusos-builder` release/install inventory

## Direct answer: delete-list and critical impact

The tracked source tree has **six files whose contents mention `ghd` case-insensitively**, plus the `ghd.toml` artifact itself:

1. `ghd.toml` — delete.
2. `.github/scripts/stage_ghd_release_assets.py` — delete or replace with a non-`ghd` release-asset staging/validation mechanism.
3. `.github/scripts/test_stage_ghd_release_assets.py` — delete or replace with tests for the new final asset contract.
4. Remove `ghd.toml` from `moon.yml`’s `releaseConfig` inputs.
5. Replace the `ghd` install/download section in `README.md`.
6. Replace the `ghd` staging step and summary command in `.github/workflows/release.yml`.
7. Replace the `ghd` staging rehearsal in `.github/workflows/release-dry-run.yml`.

Deleting only the named `ghd` files will break both release workflows: the staging script currently does more than validate `ghd.toml`; it copies GoReleaser artifacts into `dist/release-assets`, enforces the nine-file release contract, sets executable bits, and verifies `checksums.txt` (`.github/scripts/stage_ghd_release_assets.py:184-268`). Downstream release steps smoke-test and upload from that directory and attest its checksum file (`.github/workflows/release.yml:130-168`). A replacement asset-staging contract, or a clean cutover to uploading GoReleaser outputs directly, is therefore required.

**Windows verdict:** the codebase does **not** currently look buildable for `GOOS=windows`. `internal/update/cache.go` unconditionally imports `golang.org/x/sys/unix` and calls `unix.Statfs` (`internal/update/cache.go:3-16,274-287`). Worse, the intended non-Linux companion is tagged `//go:build !linux`, which includes Windows, but still imports `golang.org/x/sys/unix` and uses `unix.Statfs_t` (`internal/update/space_other.go:1-10`). No build was run, as required.

---

## 1. Exhaustive tracked `ghd` inventory

### `README.md`

Relevant text is entirely the current end-user GitHub-release install method (`README.md:5-21`):

```text
### From a GitHub release

[ghd](https://github.com/meigma/ghd) installs the release binary and verifies its build provenance attestation. Install `ghd` from that project's ... guide, then:

    ghd install componere/incusos-builder/incusos-builder \
      --store-dir "$HOME/.local/share/ghd/store" \
      --bin-dir "$HOME/.local/bin"
...
    ghd download componere/incusos-builder/incusos-builder@x.y.z --output "$PWD/out"
```

Purpose: makes `ghd` the only documented native-release installer/downloader and delegates provenance verification to it. Removing `ghd` makes this entire subsection unusable and requires replacement by Nix/Homebrew/Scoop/installer-script/direct-download instructions.

### `moon.yml`

`ghd.toml` participates in the release-config file group (`moon.yml:31-36`):

```yaml
releaseConfig:
  - '.goreleaser.yaml'
  - 'ghd.toml'
  - 'release-please-config.json'
  - '.release-please-manifest.json'
```

That group is an input to `root:check` (`moon.yml:109-118`). Deleting `ghd.toml` without removing this input leaves a dead file dependency.

### `ghd.toml`

The file does not spell the token `ghd` inside its contents, but it is the package manifest consumed by the staging script. It binds provenance to the isolated workflow and maps exactly four raw binary names (`ghd.toml:1-32`):

```toml
[provenance]
signer_workflow = "componere/incusos-builder/.github/workflows/attest.yml"

[[packages]]
name = "incusos-builder"
tag_pattern = "v${version}"
...
pattern = "incusos-builder_${version}_darwin_amd64"
...
pattern = "incusos-builder_${version}_darwin_arm64"
...
pattern = "incusos-builder_${version}_linux_amd64"
...
pattern = "incusos-builder_${version}_linux_arm64"

[[packages.binaries]]
path = "incusos-builder"
```

Deleting it directly breaks the staging script’s default `--config` and all of its manifest validation (`.github/scripts/stage_ghd_release_assets.py:32-36,59-78,88-103,125-181`).

### `.github/scripts/stage_ghd_release_assets.py`

This is both a `ghd.toml` validator and the release asset assembler:

- Fixed platforms and fixed asset count: four Darwin/Linux × amd64/arm64 tuples and `EXPECTED_ASSET_COUNT = 9` (`:17-23`).
- Defaults to `dist/artifacts.json`, `ghd.toml`, and `dist/release-assets` (`:29-36`).
- Requires `GITHUB_REPOSITORY`, parses `ghd.toml`, validates it, stages GoReleaser artifacts, checks the final set, verifies checksums, and prints staged paths (`:59-78`).
- Requires the manifest signer to be `<repo>/.github/workflows/attest.yml`, one matching package, `tag_pattern = "v${version}"`, the binary path, and all four raw asset patterns (`:125-181`).
- Accepts only GoReleaser artifact types `Binary`, `SBOM`, and the `Checksum` named `checksums.txt`; it ignores archives (`:184-231`).
- Converts each binary to mode `0755`, expects raw names `incusos-builder_<version>_<os>_<arch>`, expects sibling `<binary>.sbom.json`, and rejects any count other than nine (`:234-268`).
- Parses SHA-256 lines, requires at least every expected binary in the checksum map, requires every checksum entry to point to a staged file, and verifies every listed digest (`:271-314`).

Deleting this script breaks the calls in both release workflows. Retaining it while deleting only its `ghd` validation is insufficient for Windows/archive work because its artifact-type filter, platform set, raw names, and count remain fixed.

### `.github/scripts/test_stage_ghd_release_assets.py`

This standalone unittest imports the staging script by filename (`:14-19`), duplicates the four-platform tuple (`:22-27`), and asserts the exact nine assets (`:53-77`):

```python
[
    "checksums.txt",
    "incusos-builder_1.2.3_darwin_amd64",
    "incusos-builder_1.2.3_darwin_amd64.sbom.json",
    "incusos-builder_1.2.3_darwin_arm64",
    "incusos-builder_1.2.3_darwin_arm64.sbom.json",
    "incusos-builder_1.2.3_linux_amd64",
    "incusos-builder_1.2.3_linux_amd64.sbom.json",
    "incusos-builder_1.2.3_linux_arm64",
    "incusos-builder_1.2.3_linux_arm64.sbom.json",
]
```

It also asserts missing checksum, digest mismatch, wrong signer, missing Linux/arm64, and a ten-file unexpected-count failure (`:79-114`). Its fixture generates raw artifacts, raw-only checksum entries, and an inline `ghd.toml` clone (`:117-238`). Deleting the implementation makes the import fail. The ordinary Moon/CI test path does **not** invoke this file: `check-upstream` discovers only `test_check_upstream_closure.py` (`moon.yml:81-88`), and CI runs `moon ci` (`.github/workflows/ci.yml:82-83`). The release dry run is the active workflow-level exercise of the staging implementation (`.github/workflows/release-dry-run.yml:74-85`).

### `.github/workflows/release.yml`

Two content references:

```yaml
- name: Stage and validate ghd release assets
  ...
  run: python3 .github/scripts/stage_ghd_release_assets.py --tag "$RELEASE_TAG"
```

(`.github/workflows/release.yml:125-128`)

and the inspection-summary command:

```sh
echo "ghd download \"$GITHUB_REPOSITORY/incusos-builder@${RELEASE_VERSION}\" --output \"\$(mktemp -d)\""
```

(`.github/workflows/release.yml:474-477`). The former hard-fails if the script/config is deleted. The latter only prints an obsolete consumer command, so it would mislead without failing the job.

### `.github/workflows/release-dry-run.yml`

The rehearsal explicitly invokes the same staging implementation and checks that `attest.yml` exists because `ghd.toml` points to it (`.github/workflows/release-dry-run.yml:74-85`):

```yaml
- name: Stage and validate ghd release assets
  run: |
    ...
    python3 .github/scripts/stage_ghd_release_assets.py --tag "$DRY_RUN_TAG"
    ...
    test -f .github/workflows/attest.yml
```

Deleting either the script or `ghd.toml` fails release-PR/manual rehearsals.

### Explicit negative results

A case-insensitive tracked-tree search found no `ghd` content in:

- `docs/**`, including both files under `docs/notes/**`;
- `SECURITY.md` (`SECURITY.md:1-21` contains only support/reporting policy);
- `CONTRIBUTING.md` (`CONTRIBUTING.md:1-76` contains source setup and Release Please guidance, not `ghd`);
- `CHANGELOG.md` (`CHANGELOG.md:1-21` contains only 0.1.0 features/fixes);
- any `*.txtar` testscript;
- any Go source or `*_test.go` file.

The only test containing `ghd` is the Python unittest above.

---

## 2. `.goreleaser.yaml` and every hardcoded release-asset dependency

### Current GoReleaser contract

The complete relevant configuration is (`.goreleaser.yaml:1-48`):

```yaml
version: 2
project_name: incusos-builder

before:
  hooks:
    - go test ./...

builds:
  - id: incusos-builder
    main: ./cmd/incusos-builder
    binary: incusos-builder
    env:
      - CGO_ENABLED=0
    goos: [darwin, linux]
    goarch: [amd64, arm64]
    flags: [-trimpath]
    ldflags:
      - -s -w -buildid=
      - -X main.version={{ .Version }}
      - -X main.commit={{ .FullCommit }}
      - -X main.date={{ .CommitDate }}
    mod_timestamp: '{{ .CommitTimestamp }}'

archives:
  - id: incusos-builder
    ids: [incusos-builder]
    formats: [binary]
    name_template: '{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}'

checksum:
  name_template: checksums.txt

sboms:
  - id: binary
    artifacts: binary

changelog:
  disable: true

release:
  disable: true
```

Consequences:

- `archives.formats: [binary]` ships **raw executables**, not `.tar.gz`/`.zip` archives (`.goreleaser.yaml:29-37`).
- Four builds result from Darwin/Linux × amd64/arm64 (`:8-18`).
- The raw release name template is `incusos-builder_<version>_<os>_<arch>` (`:29-37`).
- `checksums.txt` is the checksum filename (`:39-40`).
- SBOMs are generated for binary artifacts (`:42-44`); the staging contract expects each final basename to be `<binary>.sbom.json` (`.github/scripts/stage_ghd_release_assets.py:234-240`).
- `release.disable: true` means GoReleaser does not publish the GitHub release (`.goreleaser.yaml:47-48`). The workflow also invokes `release --clean --skip=publish` (`.github/workflows/release.yml:116-123`) and performs the publication itself with `gh release upload` (`:156-160`).

### Hardcoded names/shape

| Location | Hardcoded dependency |
|---|---|
| `.goreleaser.yaml:29-44` | `binary` format, basename template, `checksums.txt`, binary SBOM source |
| `ghd.toml:10-32` | Four exact raw asset patterns with no extension; binary path |
| `.github/scripts/stage_ghd_release_assets.py:17-23` | Four platforms; nine final files |
| `.github/scripts/stage_ghd_release_assets.py:184-231` | Accepts `Binary`/`SBOM`/`Checksum`, not `Archive`; hardcodes `checksums.txt` |
| `.github/scripts/stage_ghd_release_assets.py:234-268` | `<name>_<version>_<os>_<arch>` and `.sbom.json`; exact count |
| `.github/scripts/stage_ghd_release_assets.py:271-314` | `checksums.txt` parser and staged-file consistency |
| `.github/scripts/test_stage_ghd_release_assets.py:53-114` | Exact nine filenames and raw/executable/count/error assertions |
| `.github/scripts/test_stage_ghd_release_assets.py:117-238` | Fixture duplicates four platforms, names, SBOM suffix, checksum filename, manifest patterns |
| `.github/workflows/release.yml:130-153` | Host smoke binary at `dist/release-assets/incusos-builder_<version>_<host_os>_<host_arch>` with no extension |
| `.github/workflows/release.yml:156-168` | Uploads `dist/release-assets/*`; separately uploads `dist/release-assets/checksums.txt` as workflow artifact `release-checksums` |
| `.github/workflows/release.yml:474-477` | Summary reconstructs the raw basename without extension |
| `.github/workflows/release-dry-run.yml:87-110` | Same raw host-binary path and executable-bit assumption |
| `.github/workflows/attest.yml:22-27,53-66` | Input described as an artifact containing `checksums.txt`; downloads it and attests `subject-checksums: checksums.txt` |

No static page under `docs/docs/**` hardcodes a release asset basename, `checksums.txt`, or `.sbom.json`. `README.md` avoids names only because `ghd` resolves them (`README.md:7-20`).

### Archive-format impact

Changing `formats: [binary]` to `tar.gz`/`zip` without a broader migration will fail:

1. The staging loop ignores GoReleaser `Archive` artifacts (`.github/scripts/stage_ghd_release_assets.py:198-213`).
2. The final-set validator still demands four raw executables and four sibling `.sbom.json` files (`:234-268`).
3. Both smoke tests execute an unarchived, extensionless path (`.github/workflows/release.yml:130-153`; `.github/workflows/release-dry-run.yml:87-110`).
4. The Python test asserts exactly the raw names and executable mode (`.github/scripts/test_stage_ghd_release_assets.py:53-77`).
5. The release summary reconstructs a raw asset name (`.github/workflows/release.yml:474-476`).
6. The old `ghd.toml` patterns have no archive extension (`ghd.toml:10-29`).

`gh release upload dist/release-assets/*` itself is format-neutral once the staging directory contains the intended final assets (`.github/workflows/release.yml:156-160`), and checksum-based attestation is format-neutral if the replacement `checksums.txt` covers those final assets (`.github/workflows/attest.yml:53-66`).

---

## 3. `.github/workflows/release.yml`: full release surface

### Trigger, serialization, and graph

- Trigger: `push` of `v*` or manual `workflow_dispatch` with a required `tag` (`.github/workflows/release.yml:9-18`).
- Default permissions are empty (`:20`).
- Runs for the same workflow/tag serialize and are never cancelled in flight (`:22-26`).
- Fixed image name is `ghcr.io/componere/incusos-builder` (`:28-29`).

Graph:

```text
resolve-release
  └─ binary-release-assets
       ├─ attest-binaries ──────────────────────┐
       └─ melange-build[amd64,arm64]             │
             └─ container-image-release          │
                    └─ attest-image              │
                                                  └─ release-inspection-summary
```

More exactly, `melange-build` needs both `resolve-release` and `binary-release-assets` (`:195-202`); `container-image-release` needs resolve, binary assets, and melange (`:279-294`); the summary needs resolve, binary assets, container image, and both attestations (`:448-457`).

### `resolve-release` (`:32-81`)

- Permission: `contents: write`, because draft releases require push access to list (`:35-38`).
- Resolves dispatched tag or `GITHUB_REF_NAME`, validates SemVer-shaped `vX.Y.Z[-prerelease]`, emits `tag` and `version` (`:43-64`).
- Polls up to 30 times at 10-second intervals until `gh release view ... .isDraft` is exactly true; otherwise fails (`:66-80`).

### `binary-release-assets` (`:82-168`)

Permission: `contents: write` (`:82-88`). Every step:

1. Checkout with full history and no persisted credentials (`:90-95`).
2. On manual dispatch, detach at the requested tag (`:96-101`).
3. Setup Go from `go.mod` with cache (`:102-106`).
4. Install pinned Syft `v1.43.0` (`:107-114`).
5. Run GoReleaser v2 as `release --clean --skip=publish` (`:116-123`).
6. Stage/validate through the `ghd` script (`:125-128`).
7. Smoke-test the host raw binary and require output beginning `incusos-builder <version> ` (`:130-153`).
8. Upload every staged file to the draft release with `gh release upload ... --clobber` (`:156-160`).
9. Upload `checksums.txt` as the one-day `release-checksums` workflow artifact for isolated attestation (`:162-168`).

### `attest-binaries` (`:170-191`)

Calls `./.github/workflows/attest.yml` with `checksums-artifact: release-checksums` (`:172-191`). It grants `id-token`, `attestations`, `packages`, and `artifact-metadata` write plus `contents: read`; comments explain the extra package/artifact scopes are needed because the reusable workflow is shared with image attestation (`:175-187`).

### `melange-build` matrix (`:192-277`)

- Native `amd64` on `ubuntu-24.04` and `arm64` on `ubuntu-24.04-arm`, with apk dirs `x86_64`/`aarch64` (`:195-212`).
- Permission: `contents: read` (`:201-202`).
- Checkout, optional tag checkout, setup pinned mise with publishing-cache disabled (`:214-235`).
- Resolve version, full commit, and UTC commit date; write `melange-vars.yaml` (`:236-255`).
- Generate per-arch ephemeral signing key, run `melange build` with Docker runner and vars, then upload the apk tree and public key as one-day `apk-<arch>` artifacts (`:257-277`).

### `container-image-release` (`:279-430`)

Permissions: `contents: read`; write `packages`, `id-token`, `attestations`, and `artifact-metadata` (`:286-291`). Every step:

1. Checkout and optional requested-tag checkout (`:296-307`).
2. Setup pinned mise with cache disabled (`:308-317`).
3. Install pinned Syft (`:318-322`).
4. Download/merge `apk-*` artifacts (`:323-329`).
5. Login to GHCR with `github.token` (`:330-336`).
6. `apko publish` a two-arch image, append both ephemeral public keys, emit SBOMs, parse the final `sha256:` digest, and expose name/digest/ref outputs (`:337-363`).
7. Inspect the digest (not the tag) and require exactly `linux/amd64,linux/arm64` (`:365-387`).
8. Run `--version`, require the release version prefix, and require OCI user `65532` (`:389-412`).
9. Keyless-sign the digest with `cosign sign --yes` (`:414-417`).
10. Generate `image.spdx.json` with Syft (`:419-422`).
11. Attest the image SBOM and push the attestation to the registry (`:424-430`).

### `attest-image` (`:432-446`)

Calls the same isolated `attest.yml`, passing the final image name/digest and `push-to-registry: true`; grants the reusable workflow the required write scopes (`:433-446`).

### `release-inspection-summary` (`:448-491`)

Runs only after both artifact classes and both provenance calls succeed (`:451-457`). It prints:

- draft release inspection via `gh release view`;
- the raw host binary basename;
- binary `gh attestation verify` bound to `attest.yml` and the release tag;
- the `ghd download` command that must be removed;
- GHCR login/pull/run;
- image `gh attestation verify` by digest;
- `cosign verify` bound to `release.yml` identity;
- the explicit instruction to publish/reject the GitHub draft manually, with a warning that the GHCR image is already public because GHCR has no draft state (`:459-490`).

No job publishes the draft GitHub release. It remains a human decision after the summary (`:487-490`).

---

## 4. `.github/workflows/release-dry-run.yml`: full rehearsal surface

### Trigger and gating

The header says it intentionally omits GitHub release upload, image push, signing, and attestation, while invoking the same real commands up to those boundaries (`.github/workflows/release-dry-run.yml:1-12`). It triggers on every PR targeting `master` and manual dispatch (`:16-20`), has empty default permissions (`:22`), and cancels superseded runs for the same workflow/ref (`:24-26`).

Both root jobs execute only for manual dispatch or a PR whose head starts `release-please--` (`:29-36,113-118`). The container job uses the same condition under `always()` (`:185-194`). Therefore **all three jobs report skipped on ordinary PRs**. This is intentional: `.github/repository-settings.toml` documents that the broad trigger preserves required contexts and that skipped jobs report success (`.github/repository-settings.toml:80-92`).

### `binary-release-dry-run` (`:29-110`)

Every rehearsed step:

1. Checkout full history (`:38-43`).
2. Setup Go (`:44-48`).
3. Install and print pinned Syft (`:49-59`).
4. Create a synthetic tag `v0.0.0-dryrun.<run_id>.<attempt>` (`:35-36,61-62`).
5. Run the same GoReleaser `release --clean --skip=publish` with `GORELEASER_CURRENT_TAG` (`:64-72`).
6. Run the real `ghd` staging script and verify `attest.yml` still exists (`:74-85`).
7. Execute the host raw artifact and validate the version prefix (`:87-110`).

### `melange-build-dry-run` (`:113-183`)

Uses the same native amd64/arm64 matrix and apk-dir mapping as release (`:119-128`), then checkout, mise setup, synthetic metadata, vars file, ephemeral keys, `melange build`, and one-day `apk-<arch>` artifact upload (`:130-183`).

### `container-image-dry-run` (`:185-314`)

Needs the melange matrix and first explicitly fails unless its result was `success` (`:185-205`). Then:

1. Checkout and setup mise (`:207-218`).
2. Install/print Syft (`:219-225`).
3. Download/merge the apk artifacts (`:227-233`).
4. Use `apko build`, not publish, to create/load `image.tar` for amd64+arm64 and require generated SBOM files (`:234-255`).
5. Inspect each loaded per-arch image and require Linux/amd64 and Linux/arm64 (`:257-276`).
6. Execute the amd64 image, require synthetic version output, and require user `65532` (`:278-302`).
7. Generate a nonempty Syft SPDX JSON SBOM without attesting it (`:304-314`).

There is no Windows-runner execution smoke test, and the dry run deliberately does not exercise Homebrew/Scoop/Nix/installer-script publication today.

---

## 5. Attestation, Release Please, and repository rules

### `.github/workflows/attest.yml`

This is a reusable, isolated provenance workflow; its comments state the isolation/SLSA-L3 intent and exact `gh attestation verify --signer-workflow .../attest.yml` shape (`.github/workflows/attest.yml:1-18`). Inputs cover either a checksum workflow artifact or image name/digest plus registry push (`:20-42`). The single `Attest` job grants OIDC/attestation/artifact-metadata/package writes and contents read (`:46-56`). It conditionally:

- downloads the named checksum artifact and runs `actions/attest` with `subject-checksums: checksums.txt` (`:58-66`);
- logs in to GHCR and uses `actions/attest-build-provenance` for an image digest (`:68-86`).

Removing `ghd` does **not** require changing the isolated signer path. Changing final asset names/formats requires ensuring the uploaded `checksums.txt` names the new final assets because that file is the binary provenance subject (`:58-66`).

### `.github/workflows/release-please.yml`

The header documents required repository variable/secret, protected-tag bypass, and the intended lifecycle: release PR → merge creates protected `v*` tag and draft release → `release.yml` fills draft → human publishes (`.github/workflows/release-please.yml:1-17`). It runs on pushes to `master` or manual dispatch, serializes without cancellation, grants only its job contents/PR/issues write, creates a GitHub App token, and runs Release Please with the repository config and manifest (`:19-57`).

### `release-please-config.json` and manifest

Current release policy (`release-please-config.json:1-25`):

```json
"release-type": "go",
"include-v-in-tag": true,
"include-component-in-tag": false,
"force-tag-creation": true,
"draft": true,
"initial-version": "0.1.0",
"bump-minor-pre-major": true,
...
"extra-files": ["melange.yaml", "apko.yaml"]
```

It updates `CHANGELOG.md`, `melange.yaml`, and `apko.yaml`; it does not currently version any install-channel metadata (`release-please-config.json:11-17`). The manifest baseline is `0.1.0` (`.release-please-manifest.json:1-3`).

### `.github/repository-settings.toml`

Key release/change-planning constraints:

- Default branch `master`; immutable releases enabled (`.github/repository-settings.toml:14-17`).
- Squash-only, linear-history repository posture (`:23-33,50-72`).
- Default-branch required checks are exactly `ci`, `GitHub Pages`, `Binary Release Dry Run`, and `Container Image Dry Run` (`:74-92`).
- `Melange Build Dry Run (<arch>)` is not a separately required context; the required container dry-run checks its matrix result (`.github/workflows/release-dry-run.yml:185-205`).
- Renaming either required dry-run job or adding a new PR publication/rehearsal check that must block merges requires updating the `contexts` list (`.github/repository-settings.toml:74-92`). Merely adding a tag-triggered publishing job does not automatically add a required PR context.
- Tags are protected against create/update/delete/force-push; only repository admins and the `componere-release-please` app bypass, and tag status checks are disabled (`:94-111`).
- There are no Homebrew/Scoop/Nix/installer-specific settings, rulesets, secrets, or variables declared in this file (`:1-127`).

---

## 6. User-facing installation/acquisition documentation

### Pages that currently document acquiring this product

1. **`README.md:5-44` — must be rewritten.** It has three methods:
   - GitHub release through `ghd` (`:7-21`);
   - container `docker pull`/`docker run`, pinned to `vX.Y.Z`, explicitly no `latest` (`:23-30`);
   - source build through mise/Moon, with Go 1.26.4 and output `bin/incusos-builder` (`:32-44`).

2. **`docs/docs/tutorials/first-seeded-iso.md:16-37` — source-acquisition prerequisite must be updated.** It currently says:

   ```text
   - A clone of this repository and mise installed. The project is not released,
     so we will build the CLI from source.
   ...
   mise install
   mise x -- moon run root:build
   IOB="$PWD/bin/incusos-builder"
   ```

   The “project is not released” statement conflicts with the current 0.1.0 release history (`CHANGELOG.md:1-21`) and should not survive an install-channel rewrite.

3. **`CONTRIBUTING.md:33-66`** is contributor setup, not end-user installation. It uses `mise install`, documents all Moon commands, and explains the local melange/apko container build. It contains no `ghd`; it is affected only if source-development tooling or task names change.

A targeted search of every page under `docs/docs/**` found **no other page that tells users how to obtain the `incusos-builder` binary or container image**. Other uses of “download,” “install,” “release,” “image,” and “verify” refer to IncusOS input-image acquisition, the generated installer media, or boot/recovery verification—not acquisition of this CLI. Examples include the update-download trust model (`docs/docs/explanation/trust-model.md:7-18,77-94`), local mirror input layout (`docs/docs/how-to/use-local-mirror.md:117-127`), and the boot acceptance release gate (`docs/docs/how-to/verify-boot-acceptance.md:1-35`). Those are not install-channel pages.

There is currently no static manual page giving manual binary checksum/SBOM/attestation verification. The only native-release provenance promise is the `ghd` sentence in README (`README.md:7-9`); exact manual verification commands exist only in the release run summary (`.github/workflows/release.yml:459-490`).

### MkDocs/Diátaxis placement

`docs/mkdocs.yml` defines a clear Diátaxis nav: Home, Tutorials, How-to guides, Reference, Explanation (`docs/mkdocs.yml:31-51`). There is no installation page in any quadrant. A task-oriented “install with Nix/Homebrew/Scoop/script/direct download” page belongs under **How-to guides** [INFERENCE based on the repository’s existing Diátaxis structure], and adding it requires a nav entry around `docs/mkdocs.yml:35-42`. The homepage currently links the tutorial, all existing how-tos, reference pages, and explanations (`docs/docs/index.md:7-31`); a new installation guide should also be linked there [INFERENCE].

### Static documentation impact list

- Rewrite: `README.md:5-44`.
- Rewrite prerequisite/source setup: `docs/docs/tutorials/first-seeded-iso.md:16-37`.
- Update nav if an install page is added: `docs/mkdocs.yml:31-51`.
- Update home index if an install page is added: `docs/docs/index.md:7-31`.
- Review only, likely unchanged: `CONTRIBUTING.md:33-76`, `SECURITY.md:1-21`, `CHANGELOG.md:1-21`.

---

## 7. Build/test plumbing for Windows and archive changes

### `moon.yml`

- Go source inputs include `cmd/**/*.go`, `internal/**/*.go`, module files, and mise pins (`moon.yml:17-29`).
- Release-config inputs are `.goreleaser.yaml`, `ghd.toml`, Release Please config, and manifest (`:31-36`); remove `ghd.toml`, and add any new in-repo release/install scripts/configs that should invalidate `root:check` [INFERENCE].
- `build` is host-native `go build -o bin/incusos-builder ./cmd/incusos-builder`, with no Windows target or `.exe` output (`:64-70`).
- `test` is host-native `go test ./...` (`:72-77`).
- `root:check` depends on format, lint, build, test, upstream closure, and docs, and hashes the release config, but it does not run a cross-platform compile (`:108-118`).

### `mise.toml`

- Pins Go 1.26.4/Python 3.14.3 and dev/release tools (`mise.toml:16-36`). GoReleaser is not mise-pinned; release workflows obtain it via `goreleaser-action` (`.github/workflows/release.yml:116-123`).
- The lock-generation instruction lists only `linux-x64,linux-arm64,macos-x64,macos-arm64` (`mise.toml:11-13`), so the fail-closed `mise install` development environment is not declared for Windows (`:38-46`). This does not itself prevent GoReleaser from cross-compiling, but it means current documented source setup is not a Windows-supported toolchain surface [INFERENCE].
- `image-local` is a POSIX shell/Docker/melange/apko task using host `GOARCH`; it is for a Linux container image and is unrelated to producing a Windows CLI (`:48-73`).

### `melange.yaml` / `apko.yaml`

- Melange builds `./cmd/incusos-builder` with Go 1.26, `CGO_ENABLED=0`, and the same version/commit/date stamps as GoReleaser (`melange.yaml:1-39`). This produces the signed Wolfi apk, not native release binaries.
- apko installs the local apk, runs `/usr/bin/incusos-builder` as UID 65532, and declares only `amd64` and `arm64` Linux image arches (`apko.yaml:1-44`).
- Adding a Windows native artifact does not imply a Windows container target; the current image pipeline remains Linux-only (`apko.yaml:31-44`). Changing GoReleaser archive format is isolated from melange/apko unless the binary-release job/staging failure blocks their `needs` chain (`.github/workflows/release.yml:195-201,279-285`).

### Direct Windows portability evidence

Blocking source:

```go
// internal/update/cache.go
import "golang.org/x/sys/unix"
...
var st unix.Statfs_t
if err := unix.Statfs(dir, &st); err != nil { ... }
```

(`internal/update/cache.go:3-16,274-287`)

```go
//go:build !linux
package update
import "golang.org/x/sys/unix"
func statfsBlockSize(st *unix.Statfs_t) uint64 { ... }
```

(`internal/update/space_other.go:1-10`)

The direct module dependency is pinned at `golang.org/x/sys v0.47.0` (`go.mod:5-21`). A repository source search found no Windows-specific implementation of the free-space probe and no `//go:build windows` file. The pinned module cache likewise contains no `unix/*windows*` files. Therefore the current package cannot be treated as Windows-buildable.

Other platform-sensitive evidence, not by itself proven blocking:

- `cmd/incusos-builder/main.go` registers `syscall.SIGINT` and `syscall.SIGTERM` (`cmd/incusos-builder/main.go:3-8,25-27`).
- `internal/cli/versions.go` maps host CPU only (`amd64`→`x86_64`, `arm64`→`aarch64`) and does not gate host OS (`internal/cli/versions.go:213-225`).
- `/dev/ttyS0` occurrences are seed-data values or tests, not host filesystem access (`internal/cli/e2e_helpers_test.go:260-266`; `internal/seed/seed_test.go:389-394`).
- No production source search result used `/proc/`, `/sys/`, `unix.` outside the cache/statfs code, or hardcoded `/dev/` as a host path.

### Tests/verification that encode current platform or version behavior

- Staging unittest encodes exactly four Darwin/Linux artifacts and no Windows (`.github/scripts/test_stage_ghd_release_assets.py:22-27,53-114`).
- Real and dry-run workflows smoke-test only the Linux runner’s host binary and assume no `.exe` suffix (`.github/workflows/release.yml:130-153`; `.github/workflows/release-dry-run.yml:87-110`).
- Container workflows assert only Linux amd64/arm64 (`.github/workflows/release.yml:365-387`; `.github/workflows/release-dry-run.yml:257-276`).
- `TestVersionFlagPrintsBuildMetadata` requires exactly:

  ```text
  incusos-builder 0.1.0 (abc1234) built 2026-05-08T10:00:00Z
  incus-os API: v0.0.0-20260815030500-0f5b8057f2fc
  ```

  (`internal/cli/root_test.go:29-51`). Release smoke tests only check the first-line prefix (`.github/workflows/release.yml:147-152`; `.github/workflows/release-dry-run.yml:104-109`).
- No Go test or testscript asserts a supported host-OS list or Windows behavior. Product architecture validation is about IncusOS media architectures (`x86_64`/`aarch64`), not the host binary OS (`docs/docs/reference/cli.md:169-181`).

---

## 8. Recent history and design notes

Local `master` points to `065b9e83c047fc78ec8f2c2938e5fd88756d4307` (`.git/HEAD`; `.git/refs/heads/master`). The repository has only 23 commits, so `git log --oneline -30` yields all 23. Subjects below are grounded in the GitHub commits API for `master`: https://api.github.com/repos/componere/incusos-builder/commits?sha=master

```text
065b9e8 docs: rewrite the README for a released tool (#31)
03a84f5 chore(master): release 0.1.0 (#10)
2e14c9f chore(deps): apply the outstanding dependency updates as one batch (#30)
959f032 docs: move phase 1 spike findings to the session 002 journal (#28)
6369865 fix(release): empty the changelog seed so the first entry has no stray heading (#29)
2abe534 chore(release): reset the version baseline so the first release is 0.1.0 (#26)
11655f4 docs: remove plan lineage and trim comments across the codebase (#25)
cc34ef4 test(testfixture): drop the wall-clock budget assertion (#27)
e02dd1e docs: record the first observed boot acceptance (#24)
64cf0ee fix: close functional-campaign findings across CLI, acquisition, docs, and CI (#21)
59c268b docs(reference): describe every configuration property (#20)
5337e7e docs: complete phase 6 release readiness (#19)
3fa5872 test: add phase 5 end-to-end acceptance (#13)
c755426 feat: phase 4 CLI surface and output publisher (#12)
9d25964 feat: phase 3 adapters — update ImageSource, media RescueWriter, ux Reporter (#11)
e529e84 feat: phase 2 domain core — build, seed, config, type gate (#9)
900afef chore: phase 1 de-risking spikes — findings and quarantined spike code (#8)
673e247 chore: rename template-go scaffold to incusos-builder (#7)
86f7bf7 docs: remove architecture doc from master tracking
39214f3 docs: move architecture doc to session journal
cc1df8b docs: add incusos-builder architecture (3 review rounds)
2e45b83 chore: ignore local reference clones under /reference
424e764 Initial commit
```

`docs/notes/**` contains only the Phase 5 boot probe and its JSON evidence. The note explains why boot acceptance is a manual pre-tag checklist rather than CI (`docs/notes/phase-5-boot-probe.md:18-31,70-86`); it does **not** explain the raw-binary/`ghd`/draft-release packaging choice. The JSON is machine evidence for that boot experiment (`docs/notes/phase-5-boot-evidence.json:1`). No `ghd`, GoReleaser, Homebrew, Scoop, Nix, installer-script, checksum, SBOM, or release-asset rationale was found under `docs/notes/**`.

The rationale that does exist is embedded in workflow/config comments rather than `docs/notes`: draft publication is deliberately manual (`.github/workflows/release.yml:1-6,487-490`), rehearsal intentionally stops only at publish/sign/attest boundaries (`.github/workflows/release-dry-run.yml:1-12`), and attestation is isolated for the stated SLSA-L3 property (`.github/workflows/attest.yml:1-18`).

---

## Unknowns / decisions the implementation plan must resolve

1. The final per-channel asset contract—raw binaries versus `.tar.gz`/`.zip`, Windows `.exe` naming, archive contents, and whether SBOMs remain per binary or follow archives—is not defined by the current repository. The current fixed contract is documented above.
2. No Windows build was executed because the assignment forbids builds. The direct `x/sys/unix` imports are sufficient to reject a “currently Windows-buildable” verdict; additional transitive Windows incompatibilities may exist.
3. There is no current static install/verification manual beyond README, and no `docs/notes` packaging rationale. New documentation placement is therefore a design choice within the existing Diátaxis nav, not a migration of an existing install page.