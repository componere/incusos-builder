# Release-automation map: `template-go` and `template-go-api`

## Direct answer

The two repositories implement the same release architecture, with project-name and smoke-test differences:

1. Release Please runs on `master`, creates a protected `v*` tag and **draft** GitHub release through a GitHub App token.
2. The tag triggers `release.yml`; a resolver validates the tag and waits for the draft release to become visible.
3. GoReleaser builds four raw binaries (`darwin|linux` × `amd64|arm64`), four binary SBOMs, and `checksums.txt`, but does not publish.
4. A Python helper validates `ghd.toml`, stages exactly nine release assets, and verifies checksums; the workflow smoke-tests the host binary and uploads the staged files to the draft release.
5. Binary checksums are passed as a one-day Actions artifact to isolated `attest.yml`, which generates GitHub-hosted provenance.
6. Native amd64/arm64 melange jobs build separately signed Wolfi APKs; apko publishes one multi-arch GHCR image; the workflow validates and smoke-tests the manifest, signs the digest with keyless Cosign, and attests a Syft SBOM.
7. The image digest is passed to the same isolated reusable workflow for provenance; the final job writes manual draft-inspection and verification commands rather than publishing the GitHub release.

The job graph is identical in both repositories: `resolve-release → binary-release-assets`; from `binary-release-assets`, `attest-binaries` and `melange-build` can proceed; `resolve-release + binary-release-assets + melange-build → container-image-release → attest-image`; the final summary waits for `resolve-release`, both artifact release jobs, and both provenance jobs. See `template-go/.github/workflows/release.yml:26-80,159-175,179-186,260-273,395-418` and `template-go-api/.github/workflows/release.yml:26-80,159-175,179-186,256-269,387-410`.

**Critical inconsistency:** `template-go-api` has no root `ghd.toml` (the local path is absent, and `https://raw.githubusercontent.com/meigma/template-go-api/master/ghd.toml` returns 404), while its release workflow, dry run, Moon inputs, README, bootstrap guide, and Python helper all require it. The helper explicitly raises `StageError("missing ghd config …")`; therefore the current API-template binary release and release dry run cannot pass their ghd validation as checked out. Callers: `template-go-api/.github/workflows/release.yml:112-115`, `template-go-api/.github/workflows/release-dry-run.yml:58-101`; failure: `template-go-api/.github/scripts/stage_ghd_release_assets.py:92-102`; other stale references: `template-go-api/moon.yml:51-55`, `template-go-api/README.md:700-716`, `template-go-api/DELETE_ME.md:24-25,61-64,80,133-158`.

---

## 1. GoReleaser configuration

### Shared shape

The files are structurally identical except for `project_name`, build ID, main package, and binary name: `template-go/.goreleaser.yaml:1-54`; `template-go-api/.goreleaser.yaml:1-54`.

Key build block, verbatim apart from the project name:

```yaml
before:
  hooks:
    - go test ./...

builds:
  - id: template-go
    main: ./cmd/template-go
    binary: template-go
    env:
      - CGO_ENABLED=0
    goos:
      - darwin
      - linux
    goarch:
      - amd64
      - arm64
    flags:
      - -trimpath
    ldflags:
      - -s -w -buildid=
      - -X main.version={{ .Version }}
      - -X main.commit={{ .FullCommit }}
      - -X main.date={{ .CommitDate }}
    mod_timestamp: '{{ .CommitTimestamp }}'
```

`template-go`: `template-go/.goreleaser.yaml:5-31`. API substitutions: `id: template-go-api`, `main: ./cmd/template-go-api`, `binary: template-go-api` at `template-go-api/.goreleaser.yaml:9-13`; all other build settings are the same at `template-go-api/.goreleaser.yaml:14-31`.

Consequences:

- Four binaries only: Darwin/Linux and amd64/arm64; there is no Windows build. `template-go/.goreleaser.yaml:14-22`; `template-go-api/.goreleaser.yaml:14-22`.
- The binaries are pure-Go (`CGO_ENABLED=0`), trim paths, remove symbol/debug data and the build ID, and stamp `main.version`, `main.commit`, and `main.date` from GoReleaser’s version, full commit, and commit date; file modification time uses the commit timestamp. `template-go/.goreleaser.yaml:14-31`; `template-go-api/.goreleaser.yaml:14-31`.
- Every GoReleaser invocation first runs `go test ./...`. `template-go/.goreleaser.yaml:5-7`; `template-go-api/.goreleaser.yaml:5-7`.

Archive/output block, verbatim:

```yaml
archives:
  - id: template-go
    ids:
      - template-go
    formats:
      - binary
    name_template: >-
      {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}

checksum:
  name_template: checksums.txt

sboms:
  - id: binary
    artifacts: binary
```

`template-go/.goreleaser.yaml:33-48`; API project IDs at `template-go-api/.goreleaser.yaml:33-48`.

Thus the “archives” are **uncompressed raw binaries** (`formats: binary`), named `<project>_<version>_<os>_<arch>`. GoReleaser emits `checksums.txt` and one SBOM per binary. The workflows install Syft immediately before GoReleaser: `template-go/.github/workflows/release.yml:100-108`; `template-go-api/.github/workflows/release.yml:100-108`.

Release control, verbatim in both:

```yaml
changelog:
  disable: true

release:
  disable: true
```

`template-go/.goreleaser.yaml:50-54`; `template-go-api/.goreleaser.yaml:50-54`. Release Please owns changelog/release creation, and `release.yml` uploads assets itself.

### Explicitly absent

There are **no** GoReleaser `signs`, `brews`, `homebrew_casks`, `scoops`, `nix`, `nfpms`, `publishers`, or Docker publisher blocks in either `.goreleaser.yaml` (full files: `template-go/.goreleaser.yaml:1-54`; `template-go-api/.goreleaser.yaml:1-54`). Binary artifacts are not Cosign-signed by GoReleaser; instead their checksums receive GitHub provenance in `attest.yml`. Container signing is a separate `cosign sign --yes "$IMAGE_REF"` step: `template-go/.github/workflows/release.yml:376-392`; `template-go-api/.github/workflows/release.yml:368-384`.

---

## 2. Production release workflow

### Trigger and draft-release rendezvous

Both workflows trigger on pushed `v*` tags and manual dispatch with a required `tag` input; workflow-level permissions are empty. The image names are `ghcr.io/meigma/template-go` and `ghcr.io/meigma/template-go-api`: `template-go/.github/workflows/release.yml:8-24`; `template-go-api/.github/workflows/release.yml:8-24`.

The resolver chooses the dispatch input or `GITHUB_REF_NAME`, accepts only `vX.Y.Z` or a SemVer-like prerelease, strips the leading `v`, and exposes both values:

```sh
if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z][0-9A-Za-z.-]*)?$ ]]; then
  echo "Release tag must match vX.Y.Z or vX.Y.Z-prerelease: $tag" >&2
  exit 1
fi
version="${tag#v}"
```

`template-go/.github/workflows/release.yml:26-56`; `template-go-api/.github/workflows/release.yml:26-56`.

Release Please creates the draft; `release.yml` does not. To handle tag/release eventual consistency, the resolver polls up to 30 times with ten-second sleeps and accepts only `isDraft == true`:

```sh
for attempt in $(seq 1 30); do
  if gh release view "$RELEASE_TAG" --repo "$GITHUB_REPOSITORY" --json isDraft --jq '.isDraft' 2>/dev/null | grep -qx true; then
    exit 0
  fi
  echo "Draft release $RELEASE_TAG is not ready yet; retrying..."
  sleep 10
done
```

`template-go/.github/workflows/release.yml:58-72`; `template-go-api/.github/workflows/release.yml:58-72`.

### Per-job permissions

Both workflows start with `permissions: {}` and grant only per job:

| Job | Needs | Permissions |
|---|---|---|
| `resolve-release` | none | `contents: write` |
| `binary-release-assets` | resolver | `contents: write` |
| `attest-binaries` reusable caller | binary assets | `id-token: write`, `attestations: write`, `contents: read`, `packages: write` |
| `melange-build` matrix | resolver + binary assets | `contents: read` |
| `container-image-release` | resolver + binary assets + melange | `contents: read`, `packages: write`, `id-token: write`, `attestations: write`, `artifact-metadata: write` |
| `attest-image` reusable caller | container release | `id-token: write`, `attestations: write`, `packages: write`, `contents: read` |
| `release-inspection-summary` | resolver + binary assets + container + both attest jobs | `{}` |

`template-go/.github/workflows/release.yml:22-30,74-80,159-175,179-186,260-273,395-418`; `template-go-api/.github/workflows/release.yml:22-30,74-80,159-175,179-186,256-269,387-410`. `attest-binaries` must grant the otherwise-unused `packages: write` because the shared reusable job declares that permission for its image path and reusable workflows cannot elevate beyond the caller: `template-go/.github/workflows/release.yml:162-171`; `template-go-api/.github/workflows/release.yml:162-171`.

### Binary assets

After checkout (full history), manual dispatch checks out the requested tag, setup-go reads `go.mod`, and Syft is installed. GoReleaser runs:

```yaml
with:
  distribution: goreleaser
  version: '~> v2'
  args: release --clean --skip=publish
```

`template-go/.github/workflows/release.yml:82-110`; `template-go-api/.github/workflows/release.yml:82-110`. The action versions differ; see “Differences” below.

The next step calls the Python staging helper:

```yaml
- name: Stage and validate ghd release assets
  env:
    RELEASE_TAG: ${{ needs.resolve-release.outputs.tag }}
  run: python3 .github/scripts/stage_ghd_release_assets.py --tag "$RELEASE_TAG"
```

Both: `.github/workflows/release.yml:112-115`.

The smoke test computes the host artifact path under `dist/release-assets`, requires it to be executable, runs `--version`, and requires output containing both the binary name and resolved version. `template-go` uses `binary_name="template-go"`; API uses `binary_name="template-go-api"`: `template-go/.github/workflows/release.yml:117-141`; `template-go-api/.github/workflows/release.yml:117-141`.

All staged files are uploaded with clobber semantics:

```sh
gh release upload "$RELEASE_TAG" dist/release-assets/* --clobber
```

Both: `.github/workflows/release.yml:143-147`.

The exact staged checksum file is also handed to the isolated workflow as a one-day Actions artifact:

```yaml
name: release-checksums
path: dist/release-assets/checksums.txt
if-no-files-found: error
retention-days: 1
```

Both: `.github/workflows/release.yml:149-155`; the caller passes `checksums-artifact: release-checksums` at lines 172-175.

### melange → apko → GHCR

The matrix is native, not emulated:

```yaml
include:
  - arch: amd64
    runner: ubuntu-24.04
    apkdir: x86_64
  - arch: arm64
    runner: ubuntu-24.04-arm
    apkdir: aarch64
```

Both: `.github/workflows/release.yml:187-196`.

Each job resolves metadata and writes `melange-vars.yaml`:

```sh
echo "version=${RELEASE_VERSION}" >> "$GITHUB_OUTPUT"
echo "commit=$(git rev-parse HEAD)" >> "$GITHUB_OUTPUT"
echo "date=$(git show -s --format=%cI HEAD)" >> "$GITHUB_OUTPUT"
printf 'version: "%s"\ncommit: "%s"\ndate: "%s"\n' "$VERSION" "$COMMIT" "$DATE" > melange-vars.yaml
```

`template-go/.github/workflows/release.yml:220-236`; `template-go-api/.github/workflows/release.yml:216-232`. The values feed melange’s load-bearing linker flags:

```yaml
ldflags: "-buildid= -X main.version=${{vars.version}} -X main.commit=${{vars.commit}} -X main.date=${{vars.date}}"
```

`template-go/melange.yaml:24-38`; `template-go-api/melange.yaml:24-39`.

Each architecture mints its own ephemeral key, builds with Docker runner, and uploads only the APK tree and public key:

```sh
melange keygen "melange-${{ matrix.arch }}.rsa"
melange build melange.yaml \
  --arch ${{ matrix.arch }} \
  --runner docker \
  --signing-key "melange-${{ matrix.arch }}.rsa" \
  --source-dir . \
  --vars-file melange-vars.yaml
```

`template-go/.github/workflows/release.yml:238-258`; `template-go-api/.github/workflows/release.yml:234-254`.

The container job downloads both artifacts, logs into GHCR, pre-creates `sbom/`, and publishes:

```sh
mkdir -p sbom
apko publish apko.yaml "$IMAGE_TAG" \
  --arch amd64,arm64 \
  --keyring-append ./melange-amd64.rsa.pub \
  --keyring-append ./melange-arm64.rsa.pub \
  --sbom-path ./sbom \
  | tee apko-out.txt
```

`template-go/.github/workflows/release.yml:299-343`; `template-go-api/.github/workflows/release.yml:291-335`. It extracts the last `sha256:<64 hex>` from apko output and exports name, digest, tag, and digest-qualified ref: same cited ranges.

The workflow then requires the manifest’s relevant platforms to equal exactly `linux/amd64,linux/arm64`: `template-go/.github/workflows/release.yml:345-365`; `template-go-api/.github/workflows/release.yml:337-357`.

Smoke tests differ:

- CLI template: `docker run --rm "$IMAGE_REF" --version` and `--message "hello from container"`. `template-go/.github/workflows/release.yml:367-374`.
- API template: `--version`, then `openapi | grep -Fq "openapi: 3.0.3"`. `template-go-api/.github/workflows/release.yml:359-366`.

Both then sign the digest-qualified image with keyless Cosign, generate `image.spdx.json` with Syft, and use `actions/attest-sbom` with `push-to-registry: true`: `template-go/.github/workflows/release.yml:376-392`; `template-go-api/.github/workflows/release.yml:368-384`. Image provenance is a separate call to `attest.yml` with the name, digest, and registry-push flag: `template-go/.github/workflows/release.yml:395-407`; `template-go-api/.github/workflows/release.yml:387-399`.

The corresponding declarative image files are also release inputs:

- `melange.yaml` defines package name/version, Go 1.26 build environment, default metadata vars, output path, and linker flags: `template-go/melange.yaml:1-38`; `template-go-api/melange.yaml:1-39`.
- `apko.yaml` consumes `@local ./packages`, installs the just-built package, runs as uid/gid 65532, declares amd64+arm64, and carries Release-Please-managed OCI version annotation: `template-go/apko.yaml:1-42`; `template-go-api/apko.yaml:1-42`.

### Final summary—not final publication

The summary writes:

- the draft tag;
- `gh release view`;
- binary `gh attestation verify` using `attest.yml` as signer;
- a `ghd download` command;
- image pull/run, GitHub attestation verification, and Cosign verification;
- “Publish or reject the draft release manually after inspection,” plus a warning that GHCR is already live because GHCR has no draft state.

`template-go/.github/workflows/release.yml:409-451`; `template-go-api/.github/workflows/release.yml:401-443`. The ghd command itself is at `template-go/.github/workflows/release.yml:438` and `template-go-api/.github/workflows/release.yml:430`.

---

## 3. Release dry run

### Trigger and conditions

Both files use a broad PR trigger against `master` plus manual dispatch, with empty workflow permissions. Each expensive job runs only on manual dispatch or a PR whose head begins `release-please--`:

```yaml
on:
  pull_request:
    branches:
      - master
  workflow_dispatch:

permissions: {}
```

and:

```yaml
if: ${{ github.event_name == 'workflow_dispatch' || startsWith(github.head_ref, 'release-please--') }}
```

Both: `.github/workflows/release-dry-run.yml:11-25,133-138`. `container-image-dry-run` uses `always()` so it can explicitly inspect the melange job result, but still applies the same event/head condition: both at lines 202-209.

The repository settings require `Binary Release Dry Run` and `Container Image Dry Run` alongside `ci`, and protect tags while allowing the `meigma-release-please` app to bypass tag creation restrictions: `template-go/.github/repository-settings.toml:71-104`; `template-go-api/.github/repository-settings.toml:71-104`. Both also set `immutable_releases = true`: each `.github/repository-settings.toml:13-17`.

### Binary rehearsal

The binary job reuses production checkout, setup-go, Syft installation, and the same per-repository GoReleaser action/config. It creates `v0.0.0-dryrun.<run_id>.<run_attempt>` both as a local Git tag and as `GORELEASER_CURRENT_TAG`, then runs `release --clean --skip=publish`: both `.github/workflows/release-dry-run.yml:27-56`.

It **does not reuse the Python staging helper**. Instead, an inline “Validate ghd-compatible dry-run artifacts” step:

- reads and checks `ghd.toml` signer, package name, tag pattern, binary path, and four asset patterns;
- requires `dist/artifacts.json` to describe four binaries, four SBOMs, and one checksum artifact;
- requires the four exact binary and SBOM names for Darwin/Linux × amd64/arm64;
- requires a 64-hex checksum line for each binary.

Both: `.github/workflows/release-dry-run.yml:58-104`. It then finds the host binary through `dist/artifacts.json`, runs `--version`, and checks binary name plus synthetic version: both lines 106-129.

### Container rehearsal

It reuses the production native runner matrix, mise setup, metadata/vars-file generation, per-arch ephemeral key, melange invocation, and APK/public-key artifact handoff: both `.github/workflows/release-dry-run.yml:131-200`.

The final job fails unless the matrix result is `success`, downloads both artifacts, but performs only a single-arch amd64 `apko build`, Docker load/tag, and smoke test. It deliberately does **not** push, sign, generate/attest SBOMs, or invoke provenance. `template-go/.github/workflows/release-dry-run.yml:202-250`; `template-go-api/.github/workflows/release-dry-run.yml:202-250`. Smoke tests mirror production: template `--message`; API `openapi | grep` at each file’s lines 247-250.

---

## 4. Isolated attestation workflow

The two reusable workflows have identical interface and topology. Four optional inputs are defined:

```yaml
checksums-artifact:  # uploaded artifact containing checksums.txt
subject-name:        # fully qualified image name
subject-digest:      # sha256:... image digest
push-to-registry:    # boolean, default false
```

`template-go/.github/workflows/attest.yml:18-39`; `template-go-api/.github/workflows/attest.yml:18-39`.

The reusable job has `id-token: write`, `attestations: write`, `contents: read`, and `packages: write`: both `.github/workflows/attest.yml:41-50`.

- Binary path: conditionally download the named artifact and call `actions/attest` with `subject-checksums: checksums.txt`. `template-go/.github/workflows/attest.yml:51-62`; `template-go-api/.github/workflows/attest.yml:51-62`.
- Image path: conditionally log into GHCR on the isolated runner, then call `actions/attest-build-provenance` with name, digest, and registry-push setting. `template-go/.github/workflows/attest.yml:64-83`; `template-go-api/.github/workflows/attest.yml:64-83`.

The isolation rationale is explicit: a reusable workflow has its own execution context and OIDC identity; caller build steps cannot reach its signing material or inject steps/secrets. The comments identify that separation as the SLSA Build L3 isolation property while noting the build is not thereby hermetic. Both `.github/workflows/attest.yml:1-15`.

---

## 5. Release Please

Both `.github/workflows/release-please.yml` files are identical. They run on pushes to `master` and manual dispatch, default to no permissions, and give their only job `contents`, `pull-requests`, and `issues` write access. They mint a token with `actions/create-github-app-token` from `vars.MEIGMA_RELEASE_APP_ID` and `secrets.MEIGMA_RELEASE_APP_PRIVATE_KEY`, then pass it to Release Please with the manifest/config paths: both `.github/workflows/release-please.yml:1-38`.

Verbatim downstream prerequisites from the workflow header:

```yaml
# Required release app settings:
# - vars.MEIGMA_RELEASE_APP_ID
# - secrets.MEIGMA_RELEASE_APP_PRIVATE_KEY
# - protected-tag ruleset bypass for the release app, because this workflow
#   creates protected v* tags after the release PR is merged.
```

Both `.github/workflows/release-please.yml:1-5`; the corresponding bypass is in `.github/repository-settings.toml:84-99`.

The release configuration is identical except for package name:

```json
"release-type": "go",
"include-v-in-tag": true,
"include-component-in-tag": false,
"force-tag-creation": true,
"draft": true,
"bump-minor-pre-major": true,
"bump-patch-for-minor-pre-major": true
```

`template-go/release-please-config.json:2-9`; `template-go-api/release-please-config.json:2-9`.

Each package writes `CHANGELOG.md` and bumps both image-version files:

```json
"changelog-path": "CHANGELOG.md",
"extra-files": ["melange.yaml", "apko.yaml"]
```

`template-go/release-please-config.json:10-16`; `template-go-api/release-please-config.json:10-16`. The markers are at `template-go/melange.yaml:8-11`, `template-go/apko.yaml:37-42`, `template-go-api/melange.yaml:8-11`, and `template-go-api/apko.yaml:37-42`.

Changelog sections are visible for `feat`/Features, `fix`/Bug Fixes, `perf`/Performance, and `deps`/Dependencies; `docs` and `chore` are hidden: both `release-please-config.json:17-24`.

Manifest versions are:

```json
{ ".": "0.1.1" }
```

at `template-go/.release-please-manifest.json:1-3`, and:

```json
{ ".": "1.0.4" }
```

at `template-go-api/.release-please-manifest.json:1-3`. The corresponding changelog heads are `template-go/CHANGELOG.md:1-8` and `template-go-api/CHANGELOG.md:1-31`.

---

## 6. ghd manifest, staging helper, and deletion/replacement surface

### `template-go/ghd.toml`

The only real root manifest is `template-go/ghd.toml:1-36`:

```toml
version = 1

[provenance]
signer_workflow = "meigma/template-go/.github/workflows/attest.yml"

[[packages]]
name = "template-go"
description = "Meigma Go repository template starter CLI."
tag_pattern = "v${version}"
```

It declares four raw asset patterns:

```text
template-go_${version}_darwin_amd64
template-go_${version}_darwin_arm64
template-go_${version}_linux_amd64
template-go_${version}_linux_arm64
```

and installed binary path `template-go`: `template-go/ghd.toml:13-36`.

`template-go-api/ghd.toml` is absent despite the references listed above. Its test fixture shows the intended missing content—same four patterns with `template-go-api`, signer `meigma/template-go-api/.github/workflows/attest.yml`, and path `template-go-api`: `template-go-api/.github/scripts/test_stage_ghd_release_assets.py:180-217`.

### What the helper does

The helpers are line-for-line equivalent except `--binary-name` defaults to `template-go` versus `template-go-api`: each `.github/scripts/stage_ghd_release_assets.py:30-37`.

Flow:

1. Require a tag beginning with non-empty `v`, strip it, and require `GITHUB_REPOSITORY`. Lines 56-89.
2. Load `ghd.toml` and `dist/artifacts.json`, requiring valid TOML and a JSON array of objects. Lines 92-123.
3. Validate `[provenance].signer_workflow == "$GITHUB_REPOSITORY/.github/workflows/attest.yml"`; exactly one package named for the binary; `tag_pattern = "v${version}"`; a matching binary path; and all four Darwin/Linux × amd64/arm64 patterns. Lines 125-181.
4. Copy matching GoReleaser `Binary`, `SBOM`, and `Checksum` artifacts into `dist/release-assets`; reject missing paths and duplicate destination names; preserve metadata and chmod binaries `0755`. Lines 184-231.
5. Require exactly nine files: four raw binaries, four `<binary>.sbom.json` files, and `checksums.txt`; reject missing, non-executable, or unexpected files. Lines 234-268.
6. Parse checksum lines as 64-hex SHA-256 plus a safe basename; reject empty/invalid/duplicate/path-traversing entries; require every expected binary to have an entry, every checksum entry to refer to a staged file, and every digest to match. Lines 271-335.
7. Print the sorted staged file list. Lines 75-83.

The exact produced names are demonstrated by tests:

- `template-go`: `checksums.txt`, `template-go_1.2.3_{darwin,linux}_{amd64,arm64}`, and each binary name plus `.sbom.json`: `template-go/.github/scripts/test_stage_ghd_release_assets.py:54-77`.
- API: identical with `template-go-api_…`: `template-go-api/.github/scripts/test_stage_ghd_release_assets.py:54-77`.

Both test files cover success/executable bit and failures for missing checksum entry, checksum mismatch, wrong signer workflow, missing OS/arch binary, and unexpected tenth asset: each `.github/scripts/test_stage_ghd_release_assets.py:53-116`. A repository-wide caller search found no CI/Moon invocation of `test_stage_ghd_release_assets.py`; CI only runs `moon ci --summary minimal` (`template-go/.github/workflows/ci.yml:81-82`; `template-go-api/.github/workflows/ci.yml:81-82`), and neither Moon graph includes this Python unittest (`template-go/moon.yml:40-87`; `template-go-api/moon.yml:61-181`). The production helper itself is invoked only from `release.yml`.

### Complete ghd removal/replacement checklist

Scoped to source/template files (excluding local `.journal`, `.wt`, and cache data), all ghd-coupled surfaces are:

1. **Manifest:** delete `template-go/ghd.toml:1-36`; API has no manifest to delete.
2. **Production workflow:** replace “Stage and validate ghd release assets” and its script call; retain equivalent safe staging/checksum validation if assets are still uploaded. Remove/replace the `ghd download` inspection command. `template-go/.github/workflows/release.yml:112-115,438`; `template-go-api/.github/workflows/release.yml:112-115,430`.
3. **Dry run:** rename/replace “Validate ghd-compatible dry-run artifacts” and delete all `ghd.toml` signer/package/tag/binary/asset-pattern checks; the GoReleaser artifact-count/name/checksum assertions remain independently useful. Both `.github/workflows/release-dry-run.yml:58-104`.
4. **Helper:** either delete both `.github/scripts/stage_ghd_release_assets.py` files or refactor/rename them. The ghd-only pieces are the docstring, `--config`, `load_toml`, `validate_ghd_config`, `find_package`, and their calls/errors; staging, exact asset-set, executable-bit, and SHA-256 verification are not inherently ghd-specific. Each script: lines 2,30-37,56-80,92-102,125-181,184-335.
5. **Tests:** delete or rename/refactor both `test_stage_ghd_release_assets.py`. Remove the temporary `ghd.toml` fixture/writer and wrong-signer test; keep/adapt success, asset-set, executable, and checksum tests if the staging helper survives. Each test: lines 13-19,53-116,125-217.
6. **Moon inputs:** remove `ghd.toml` from `releaseConfig`: `template-go/moon.yml:32-37`; `template-go-api/moon.yml:50-55`.
7. **README:** remove rename/setup instructions and the ghd installation claim in template-go (`template-go/README.md:23-31,97-112`); remove stale release-layer references/claim in API (`template-go-api/README.md:700-716`).
8. **Bootstrap guide:** replace every ghd decision/rename/delete instruction: `template-go/DELETE_ME.md:20-21,57-60,76,89-114`; `template-go-api/DELETE_ME.md:24-25,61-64,80,133-158`.
9. **Tool pins:** nothing to remove—`ghd` does not appear in either `mise.toml` or `mise.lock`; no workflow installs or executes the ghd CLI. The only actual CLI command is emitted as summary text.
10. **GoReleaser:** its current raw-binary format and `<project>_<version>_<os>_<arch>` template are the asset contract that ghd consumes (`.goreleaser.yaml:33-41`), but GoReleaser itself is not ghd-specific. Any archive/publisher changes should deliberately replace that contract rather than delete the entire file.

---

## 7. Current installation documentation

There is no end-user binary installation section or executable installation command in either README or source `docs/` tree. Searches found no `ghd download`, `go install`, `brew install`, `scoop install`, Nix profile command, or `curl | sh` command in:

- `template-go/README.md` and `template-go/docs/docs/`;
- `template-go-api/README.md` and `template-go-api/docs/docs/`.

The only end-user claim is prose:

> “The root `ghd.toml` matches the default GoReleaser output so generated projects can be installed with `ghd` once the release workflow runs.”

`template-go/README.md:111-112`; API has the same sentence at `template-go-api/README.md:716` even though its manifest is missing.

`mise install` is documented only as contributor/toolchain bootstrap, not product-binary installation: `template-go/README.md:5-21`; `template-go-api/README.md:17-38`. Source docs have no release-install material: `template-go/docs/docs/index.md:1-16`; API docs’ search hits are HTTP request examples, not binary installation (`template-go-api/docs/docs/index.md:1-37`; `template-go-api/docs/docs/api.md` contains API reference material only).

The sole concrete ghd installation command appears in the release job’s generated inspection summary, not public docs: `template-go/.github/workflows/release.yml:438`; `template-go-api/.github/workflows/release.yml:430`.

---

## 8. Moon and mise release-path inputs/tasks

### Moon

Neither repo defines a release/publish task. Both define a `releaseConfig` file group and include it in the aggregate `check` task so release-config changes affect that task’s inputs:

`template-go/moon.yml:32-38,75-87`:

```yaml
releaseConfig:
  - '.goreleaser.yaml'
  - 'ghd.toml'
  - 'release-please-config.json'
  - '.release-please-manifest.json'
  - '.github/workflows.disabled/**/*.yml'
```

The referenced `.github/workflows.disabled/` directory does not exist; therefore template-go’s Moon input group does **not** cover its active release workflows.

`template-go-api/moon.yml:50-55,171-181`:

```yaml
releaseConfig:
  - '.goreleaser.yaml'
  - 'ghd.toml'
  - 'release-please-config.json'
  - '.release-please-manifest.json'
  - '.github/workflows/release*.yml'
```

The API glob covers `release.yml`, `release-dry-run.yml`, and `release-please.yml`, but not `attest.yml`; it also names the absent `ghd.toml`. Neither group includes the staging helper/tests, `melange.yaml`, or `apko.yaml`.

Both use `toolchains.default: system`; mise supplies the binaries. `mise.toml` and `mise.lock` are also included in Go/lint input groups, so pin changes invalidate Moon inputs: `template-go/moon.yml:10-31`; `template-go-api/moon.yml:10-49`.

### mise

Shared release/supply-chain pins:

```toml
go = "1.26.4"
python = "3.14.3"
"aqua:moonrepo/moon"          = "2.3.5"
"aqua:chainguard-dev/melange" = "0.54.0"
"aqua:chainguard-dev/apko"    = "1.2.19"
"aqua:sigstore/cosign"        = "3.1.1"
```

`template-go/mise.toml:18-36`; `template-go-api/mise.toml:18-39`. `GOTOOLCHAIN = "local"`, `lockfile = true`, and `locked = true` are at `template-go/mise.toml:38-47` and `template-go-api/mise.toml:41-50`; `mise.lock` supplies the resolved per-platform integrity data.

Both define `image-local`, the local analogue of the release melange/apko path. It deletes prior `packages`/`image.tar`, generates an ephemeral key, writes `dev` metadata, builds host-arch APK with Docker runner, builds the apko image using the public key, loads it, and removes the arch suffix by retagging: `template-go/mise.toml:49-70`; `template-go-api/mise.toml:52-73`.

The API template additionally defines `stack-up`, dependent on `image-local`, to run Docker Compose; it is local API-stack convenience rather than publication: `template-go-api/mise.toml:75-78`.

---

## 9. Differences and relative maturity

There is no single repository that is uniformly “newer”; the evidence is mixed.

### Areas where `template-go-api` is more current

- Newer `actions/setup-go`: v7.0.0 versus template-go v6.5.0. `template-go-api/.github/workflows/release.yml:93-98` and dry run lines 34-39; `template-go/.github/workflows/release.yml:93-98` and dry run lines 34-39.
- Newer `goreleaser-action`: v7.2.3 versus v7.2.2. API `release.yml:103-108`, `release-dry-run.yml:49-54`; template `release.yml:103-108`, `release-dry-run.yml:49-54`.
- Newer `docker/login-action`: v4.4.0 versus v4.2.0 in both production and isolated attestation. API `release.yml:303-308`, `attest.yml:68-74`; template `release.yml:311-316`, `attest.yml:68-74`.
- Newer binary `actions/attest`: v4.2.0 versus v4.1.0. API `attest.yml:57-62`; template `attest.yml:57-62`.
- Its Moon release-workflow input glob points at active `release*.yml`; template-go points at nonexistent `.github/workflows.disabled/**/*.yml`. `template-go-api/moon.yml:50-55`; `template-go/moon.yml:32-38`.
- It has application-specific API/OpenAPI smoke tests in both real and dry-run container jobs. API `release.yml:359-366`, `release-dry-run.yml:244-250`.

### Areas where `template-go` is more hardened or complete

- In both publishing jobs, template-go deliberately sets mise action cache to `false`, with comments explaining that publishing tools must be fetched fresh and reverified against `mise.lock`. `template-go/.github/workflows/release.yml:207-218,286-297`. API uses `cache: true` in both corresponding jobs: `template-go-api/.github/workflows/release.yml:207-214,282-289`.
- Template-go actually contains the `ghd.toml` its release machinery requires. API does not.
- Template-go README gives one additional post-clone instruction to update ghd provenance/package/asset/binary/image names; API ends after the stale installability claim. `template-go/README.md:111-112`; `template-go-api/README.md:716`.

### Project-specific differences only

- Names, commands, image refs, melange/apko package/entrypoint/annotations, and manifest versions are appropriately project-specific.
- API has additional mise dev tools (`sqlc`, `mockery`, `goose`) and `stack-up`, unrelated to publishing-channel capabilities: `template-go-api/mise.toml:25-28,75-78`.
- Neither template currently has Homebrew, Scoop, Nix, nfpm, installer-script, or GoReleaser signing/publisher configuration.

---

## File-copy inventory for a downstream repository

A downstream project reproducing the pipeline must copy and rename/configure at least:

- `.goreleaser.yaml` — binary matrix, raw output naming, checksums, per-binary SBOMs, linker metadata.
- `.github/workflows/release-please.yml`, `release-please-config.json`, `.release-please-manifest.json`, and `CHANGELOG.md` — release PR/tag/draft orchestration.
- `.github/workflows/release.yml` — draft rendezvous, binary staging/upload, APK/image publication, signing, SBOM, reusable provenance calls, summary.
- `.github/workflows/release-dry-run.yml` — release-PR/manual rehearsal.
- `.github/workflows/attest.yml` — isolated binary/image provenance.
- `.github/scripts/stage_ghd_release_assets.py` plus its test **only if retaining that staging design**; remove/refactor all ghd coupling for a ghd-free pipeline.
- `melange.yaml`, `apko.yaml`, `mise.toml`, and `mise.lock` — pinned tools plus signed APK/multi-arch image build.
- `moon.yml` — release config inputs/required aggregate check.
- `.github/repository-settings.toml` and `.github/scripts/configure_github_repo.py` — immutable-release setting, required dry-run check contexts, protected tag rules, and GitHub App bypass (`repository-settings.toml:13-17,71-104`; script applies immutable releases and rulesets at `configure_github_repo.py:523-526,615-634`).
- Repository/org settings not stored as secrets in source: `MEIGMA_RELEASE_APP_ID`, `MEIGMA_RELEASE_APP_PRIVATE_KEY`, and installation/ruleset bypass for the Release Please GitHub App (`.github/workflows/release-please.yml:1-5,25-38`).

## Unknowns / negative findings

- No Homebrew/Scoop/Nix/nfpm/installer-script publisher configuration exists in either repository.
- No end-user binary installation commands exist in either README or source docs tree.
- No ghd tool pin exists in either mise config/lock.
- No CI/Moon caller for the staging-helper unittest was found.
- `template-go-api/ghd.toml` is genuinely absent locally and from the current remote `master`; the intended manifest can only be reconstructed from its test fixture. There are no further unresolved factual unknowns.