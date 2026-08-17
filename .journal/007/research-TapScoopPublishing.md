# Meigma Homebrew and Scoop publishing report

## Direct reproduction answer

Create these two **public, non-template** repositories in `componere`, both with default branch `main`:

1. `componere/homebrew-tap` — generated formulas under `Formula/`, specifically `Formula/incusos-builder.rb` for this project. The Meigma canonical repository is `meigma/homebrew-tap`, public, non-template, default branch `main`; the publishing configs explicitly use `directory: Formula` (`whzbox/.goreleaser.yaml:72-88`, `blob-cli/.goreleaser.yaml:90-105`, `blobber/.goreleaser.yaml:90-105`).
2. `componere/scoop-bucket` — generated manifests at repository root, specifically `incusos-builder.json`. All three Meigma `scoops:` blocks omit `directory`, and the observed manifests are root files (`whzbox.json`, `blob.json`, and others). GoReleaser explicitly says Scoop manifests are generally best left at root because `scoop bucket list` reports zero manifests when they are in a subdirectory: [GoReleaser Scoop documentation](https://goreleaser.com/customization/scoop/).

Add these repository secrets to `componere/incusos-builder`:

- `HOMEBREW_TAP_TOKEN`, authorized to write repository contents in **only** `componere/homebrew-tap`.
- `SCOOP_BUCKET_TOKEN`, authorized to write repository contents in **only** `componere/scoop-bucket`.

The actual Meigma secret type is not exposed and is not documented in the inspected repositories. GoReleaser requires a separate cross-repository token with content-write privileges because the source repository's default Actions token cannot publish to another repository: [GoReleaser Homebrew documentation, GitHub Actions section](https://goreleaser.com/customization/homebrew_formulas/#github-actions). If using fine-grained PATs or GitHub App installation tokens, grant target-repository **Contents: read and write** only. If using a classic PAT, GitHub documents the `repo` scope for creating/updating repository contents; `workflow` is additionally needed only when modifying `.github/workflows`, which neither publishing repository contains: [GitHub Contents API](https://docs.github.com/en/rest/repos/contents?apiVersion=2022-11-28#create-or-update-file-contents).

[INFERENCE] To reproduce the hardened Whzbox arrangement rather than the older Blob arrangement, explicitly pin `repository.branch: main` in both publishers, retain `directory: Formula` only for Homebrew, keep the Scoop manifest at root, and use the Whzbox draft handoff described below.

---

## 1. Exact GoReleaser publisher blocks

### Whzbox

Verbatim from `/Users/josh/code/meigma/whzbox/.goreleaser.yaml:72-103`:

```yaml
brews:
  - repository:
      owner: meigma
      name: homebrew-tap
      branch: main
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    directory: Formula
    homepage: "https://github.com/meigma/whzbox"
    description: "A small Go CLI for managing Whizlabs cloud sandboxes"
    license: "MIT OR Apache-2.0"
    skip_upload: auto
    commit_msg_template: "brew: update {{ .ProjectName }} to {{ .Tag }}"
    test: |
      system "#{bin}/whzbox", "version"
    install: |
      bin.install "whzbox"

scoops:
  - repository:
      owner: meigma
      name: scoop-bucket
      branch: main
      token: "{{ .Env.SCOOP_BUCKET_TOKEN }}"
    homepage: "https://github.com/meigma/whzbox"
    description: "A small Go CLI for managing Whizlabs cloud sandboxes"
    license: "MIT OR Apache-2.0"
    skip_upload: auto
    commit_msg_template: "scoop: update {{ .ProjectName }} to {{ .Tag }}"
```

There is **no Scoop `directory` field**; the manifest is generated at repository root (`scoop-bucket/origin/main:whzbox.json:1-15`).

### Blob CLI

Verbatim from `/Users/josh/code/meigma/blob-cli/.goreleaser.yaml:90-115`:

```yaml
brews:
  - repository:
      owner: meigma
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    directory: Formula
    homepage: "https://github.com/meigma/blob-cli"
    description: "A CLI for working with blob archives in OCI registries"
    license: "MIT"
    skip_upload: auto
    commit_msg_template: "brew: update {{ .ProjectName }} to {{ .Tag }}"
    test: |
      system "#{bin}/blob", "version"
    install: |
      bin.install "blob"

scoops:
  - repository:
      owner: meigma
      name: scoop-bucket
      token: "{{ .Env.SCOOP_BUCKET_TOKEN }}"
    homepage: "https://github.com/meigma/blob-cli"
    description: "A CLI for working with blob archives in OCI registries"
    license: "MIT"
    skip_upload: auto
    commit_msg_template: "scoop: update {{ .ProjectName }} to {{ .Tag }}"
```

Both `repository.branch` fields and the Scoop `directory` field are absent. The target repositories' default branch is `main`, so these publishers currently rely on default-branch discovery.

### Blobber

Verbatim from `/Users/josh/code/meigma/blobber/.goreleaser.yaml:90-115`:

```yaml
brews:
  - repository:
      owner: meigma
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    directory: Formula
    homepage: "https://github.com/meigma/blobber"
    description: "Push and pull files to OCI registries"
    license: "MIT"
    skip_upload: auto
    commit_msg_template: "brew: update {{ .ProjectName }} to {{ .Tag }}"
    test: |
      system "#{bin}/blobber", "version"
    install: |
      bin.install "blobber"

scoops:
  - repository:
      owner: meigma
      name: scoop-bucket
      token: "{{ .Env.SCOOP_BUCKET_TOKEN }}"
    homepage: "https://github.com/meigma/blobber"
    description: "Push and pull files to OCI registries"
    license: "MIT"
    skip_upload: auto
    commit_msg_template: "scoop: update {{ .ProjectName }} to {{ .Tag }}"
```

Again, both `repository.branch` fields and the Scoop `directory` field are absent.

### Differences among the three

| Field | Whzbox | Blob CLI | Blobber |
|---|---|---|---|
| Brew repository | `meigma/homebrew-tap`, `branch: main` | Same repository; branch omitted | Same repository; branch omitted |
| Scoop repository | `meigma/scoop-bucket`, `branch: main` | Same repository; branch omitted | Same repository; branch omitted |
| Brew directory | `Formula` | `Formula` | `Formula` |
| Scoop directory | absent/root | absent/root | absent/root |
| Homepage | `meigma/whzbox` | `meigma/blob-cli` | `meigma/blobber` |
| Description | Whizlabs sandbox text | Blob archive text | Push/pull OCI text |
| License | `MIT OR Apache-2.0` | `MIT` | `MIT` |
| Brew test | `whzbox version` | `blob version` | `blobber version` |
| Brew install | `bin.install "whzbox"` | `bin.install "blob"` | `bin.install "blobber"` |
| Commit templates | Same `brew:` / `scoop:` templates | Same | Same |
| `skip_upload` | `auto` for both | `auto` for both | `auto` for both |

Source ranges: `whzbox/.goreleaser.yaml:72-103`; `blob-cli/.goreleaser.yaml:90-115`; `blobber/.goreleaser.yaml:90-115`.

---

## 2. Build matrices and real archives

All three build the same five release targets:

- `linux/amd64`
- `linux/arm64`
- `darwin/amd64`
- `darwin/arm64`
- `windows/amd64`

Each declares `goos: [linux, darwin, windows]` and `goarch: [amd64, arm64]`, then ignores `windows/arm64`. Each ships real archives: `tar.gz` by default and a Windows `zip` via `format_overrides`. Thus Homebrew consumes four Darwin/Linux `.tar.gz` archives, while Scoop consumes the Windows AMD64 `.zip`.

Whzbox, verbatim (`/Users/josh/code/meigma/whzbox/.goreleaser.yaml:16-55`):

```yaml
builds:
  - id: whzbox
    main: ./cmd/whzbox
    binary: whzbox
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ignore:
      - goos: windows
        goarch: arm64
    flags:
      - -trimpath
    ldflags:
      - -s -w -buildid=
      - -X github.com/meigma/whzbox/internal/cli.Version={{ if .IsSnapshot }}{{ .Version }}{{ else }}{{ .Tag }}{{ end }}
      - -X github.com/meigma/whzbox/internal/cli.Commit={{ .FullCommit }}
      - -X github.com/meigma/whzbox/internal/cli.BuildTime={{ .Date }}
    mod_timestamp: '{{ .CommitTimestamp }}'

archives:
  - id: whzbox
    ids:
      - whzbox
    formats:
      - tar.gz
    format_overrides:
      - goos: windows
        formats:
          - zip
    files:
      - README.md
      - LICENSE*
      - SECURITY.md
```

Blob CLI uses the same matrix and archive formats, but its archive has `id: default`, an explicit `name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"`, and includes `LICENSE*` plus `README*` (`/Users/josh/code/meigma/blob-cli/.goreleaser.yaml:10-45`). Blobber is identical to Blob CLI except for build ID, main package, binary, and ldflags (`/Users/josh/code/meigma/blobber/.goreleaser.yaml:10-45`).

Whzbox's dry-run workflow independently asserts that exactly five archives exist and specifically checks for the Windows AMD64 ZIP and its SBOM (`/Users/josh/code/meigma/whzbox/.github/workflows/release-dry-run.yml:44-78`). Its opening comment states that snapshot mode does not publish release assets or update the tap/bucket (`release-dry-run.yml:1-3`).

---

## 3. Release workflows and draft handoff

### Whzbox: the publishing-enabled GoReleaser invocation

The actual publishing step is `/Users/josh/code/meigma/whzbox/.github/workflows/release.yml:101-111`:

```yaml
      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@f06c13b6b1a9625abc9e6e439d9c05a8f2190e94 # v7.2.3
        with:
          distribution: goreleaser
          version: v2.14.3
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
          SCOOP_BUCKET_TOKEN: ${{ secrets.SCOOP_BUCKET_TOKEN }}
          GORELEASER_CURRENT_TAG: ${{ needs.resolve-release.outputs.tag }}
```

Workflow-level permissions are deny-all (`permissions: {}` at `release.yml:14`). Job permissions are:

- `resolve-release`: `contents: write` (`release.yml:21-25`).
- `release`: `contents: write`, `id-token: write` (`release.yml:67-73`).
- `attest-binaries`: `contents: read`, `id-token: write`, `attestations: write` (`release.yml:152-161`).
- `publish-draft-release`: `contents: write` (`release.yml:163-177`).

Draft flow:

1. Release Please mints a GitHub App token from `vars.RELEASE_APP_CLIENT_ID` and `secrets.RELEASE_APP_PRIVATE_KEY` (`/Users/josh/code/meigma/whzbox/.github/workflows/release-please.yml:24-29`). Its inline comment says Release Please owns the PR, tag, and initial draft release; the tag-triggered workflow uploads, attests, then publishes it (`release-please.yml:31-40`). `release-please-config.json:3-4` sets `"draft": true` and `"force-tag-creation": true`.
2. `resolve-release` validates the tag and polls `gh release view ... --repo "$GITHUB_REPOSITORY"` until it finds a draft, with up to 30 ten-second attempts (`release.yml:20-64`).
3. GoReleaser is configured to reuse and preserve that draft (`/Users/josh/code/meigma/whzbox/.goreleaser.yaml:115-120`):

   ```yaml
   release:
     draft: true
     prerelease: auto
     use_existing_draft: true
     replace_existing_artifacts: true
     mode: keep-existing
   ```

4. After GoReleaser and the isolated attestation job succeed, the checkout-less final job runs (`release.yml:163-177`):

   ```yaml
   run: gh release edit "$RELEASE_TAG" --repo "$GITHUB_REPOSITORY" --draft=false
   ```

### Blob CLI differs materially

Blob CLI has one publishing job and grants `contents: write` plus `id-token: write` at workflow level (`/Users/josh/code/meigma/blob-cli/.github/workflows/release.yml:14-21`). Its publishing invocation is (`release.yml:45-54`):

```yaml
      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@e435ccd777264be153ace6237001ef4d979d3a7a # v6.4.0
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
          SCOOP_BUCKET_TOKEN: ${{ secrets.SCOOP_BUCKET_TOKEN }}
```

It has no draft resolver, no `GORELEASER_CURRENT_TAG`, no isolated attestation handoff, and no final draft-publication job. Its GoReleaser release settings say the Release Please release is appended to and is not a draft (`/Users/josh/code/meigma/blob-cli/.goreleaser.yaml:117-125`):

```yaml
release:
  github:
    owner: meigma
    name: blob-cli
  # Append to the release created by release-please
  mode: append
  draft: false
  prerelease: auto
  name_template: "v{{.Version}}"
```

### Two release landmines already fixed in Whzbox

#### `8a20583ad1667ba3a5c67f5733eaac77abe26111` — draft discovery

`git show 8a20583ad1667ba3a5c67f5733eaac77abe26111` changes only:

```diff
     permissions:
-      contents: read
+      contents: write
```

The `resolve-release` token had only read permission, which did not give it push-equivalent access to draft releases. Consequently the resolver could not discover the existing `v1.1.0` draft. PR #42 states both the failure and the fix explicitly: [meigma/whzbox#42](https://github.com/meigma/whzbox/pull/42). The current permission is `contents: write` at `whzbox/.github/workflows/release.yml:24-25`.

**Avoidance:** any job that queries the pre-existing draft must retain `contents: write`; a workflow-level `contents: read` is insufficient for this draft-discovery path.

#### `0eb46d7ec9ede8cef62538857e2aa24d23f86ade` — checkout-less publication

`git show 0eb46d7ec9ede8cef62538857e2aa24d23f86ade` changes only:

```diff
-        run: gh release edit "$RELEASE_TAG" --draft=false
+        run: gh release edit "$RELEASE_TAG" --repo "$GITHUB_REPOSITORY" --draft=false
```

The final publication job has no checkout, so `gh` had no local repository context. The release assets and attestation had already succeeded, but the final `gh release edit` failed in run `31662069467`. PR #44 states this directly: [meigma/whzbox#44](https://github.com/meigma/whzbox/pull/44). The current checkout-less job and explicit target are at `whzbox/.github/workflows/release.yml:163-177`.

**Avoidance:** every `gh release` call in a checkout-less job must pass `--repo "$GITHUB_REPOSITORY"` (or equivalently provide explicit repository context).

---

## 4. Publishing token provisioning evidence

Authenticated CLI identity was confirmed as `jmgilman` with `gh auth status`.

Verbatim `gh secret list --repo meigma/whzbox`:

```text
HOMEBREW_TAP_TOKEN       2026-04-13T03:40:54Z
RELEASE_APP_PRIVATE_KEY  2026-04-12T15:40:48Z
SCOOP_BUCKET_TOKEN       2026-04-13T03:41:34Z
```

Verbatim `gh variable list --repo meigma/whzbox`:

```text
RELEASE_APP_CLIENT_ID  Iv23liZp61F0vaCah14R  2026-04-12T15:55:39Z
```

The release GitHub App credentials are used only by the Release Please workflow (`whzbox/.github/workflows/release-please.yml:24-40`). GoReleaser receives the two dedicated publishing secrets (`whzbox/.github/workflows/release.yml:101-111`), which the publisher configs consume as `.Env.HOMEBREW_TAP_TOKEN` and `.Env.SCOOP_BUCKET_TOKEN` (`whzbox/.goreleaser.yaml:73-94`).

Recent generated commits in both target repositories are authored/committed as `goreleaserbot <bot@goreleaser.com>` and resolve to the GitHub user `goreleaserbot`; examples are tap commit `df640cb5b25dee44993bbf7dea3cc4afa2c66c51` and bucket commit `b7e07edc7815338ed4b1e7e673a18843772b42c2`: [tap commit](https://github.com/meigma/homebrew-tap/commit/df640cb5b25dee44993bbf7dea3cc4afa2c66c51), [bucket commit](https://github.com/meigma/scoop-bucket/commit/b7e07edc7815338ed4b1e7e673a18843772b42c2). This commit identity does not prove whether the stored credential is a classic PAT, fine-grained PAT, or App token.

Negative finding: targeted searches across Markdown, workflows, and release configs in `whzbox`, `blob-cli`, `blobber`, `tap`, and `scoop-bucket` found no 1Password item/reference, PAT provisioning instructions, token scope documentation, or GitHub App reference connected to `HOMEBREW_TAP_TOKEN` or `SCOOP_BUCKET_TOKEN`. PR #8 only says the tokens were wired into the release workflow; it does not document their creation or type: [meigma/whzbox#8](https://github.com/meigma/whzbox/pull/8).

---

## 5. Canonical repository names and the `tap` discrepancy

Verbatim results from the requested `gh repo view` calls:

```json
// gh repo view meigma/tap --json name,description,visibility,isTemplate,defaultBranchRef
{"defaultBranchRef":{"name":"main"},"description":"Homebrew tap for Meigma applications","isTemplate":false,"name":"homebrew-tap","visibility":"PUBLIC"}

// gh repo view meigma/homebrew-tap --json name,description,visibility,isTemplate,defaultBranchRef
{"defaultBranchRef":{"name":"main"},"description":"Homebrew tap for Meigma applications","isTemplate":false,"name":"homebrew-tap","visibility":"PUBLIC"}

// gh repo view meigma/scoop-bucket --json name,description,visibility,isTemplate,defaultBranchRef
{"defaultBranchRef":{"name":"main"},"description":"Scoop bucket for Meigma applications","isTemplate":false,"name":"scoop-bucket","visibility":"PUBLIC"}
```

Both `gh repo view meigma/tap` and `gh api repos/meigma/tap` resolve to repository ID `1130154523`, canonical name/full name `homebrew-tap` / `meigma/homebrew-tap`, and canonical URL [https://github.com/meigma/homebrew-tap](https://github.com/meigma/homebrew-tap). Thus `meigma/tap` is a retained old-name alias/redirect to the renamed canonical repository, not a second repository.

The local clone still records the legacy URL:

```text
origin  git@github.com:meigma/tap.git (fetch)
origin  git@github.com:meigma/tap.git (push)
```

The GoReleaser configs correctly target the canonical `name: homebrew-tap` (`whzbox/.goreleaser.yaml:73-77`; `blob-cli/.goreleaser.yaml:91-94`; `blobber/.goreleaser.yaml:91-94`). Both canonical publishing repositories use default branch `main`; Whzbox additionally pins it explicitly.

---

## 6. Generated artifacts and repository layouts

### Important local-tap staleness

The requested local `/Users/josh/code/meigma/tap` checkout and its local `origin/main` are both at `415df000b4986cce195d15cf96b0be5256ac7b44` (`2026-04-26`, `chore: add cask directory`). The canonical GitHub repository has newer commits through August. Therefore the full local `whzbox.rb` below is v1.0.3, while canonical GitHub `main` currently contains v1.1.0: [current canonical whzbox formula](https://github.com/meigma/homebrew-tap/blob/main/Formula/whzbox.rb). The local `blob.rb` already matches the canonical v1.1.0 file.

### Full local `tap/Formula/whzbox.rb`

`/Users/josh/code/meigma/tap/Formula/whzbox.rb:1-50`:

```ruby
# typed: false
# frozen_string_literal: true

# This file was generated by GoReleaser. DO NOT EDIT.
class Whzbox < Formula
  desc "A small Go CLI for managing Whizlabs cloud sandboxes"
  homepage "https://github.com/meigma/whzbox"
  version "1.0.3"
  license "MIT OR Apache-2.0"

  on_macos do
    if Hardware::CPU.intel?
      url "https://github.com/meigma/whzbox/releases/download/v1.0.3/whzbox_1.0.3_darwin_amd64.tar.gz"
      sha256 "e21bc503e4eaf5fccaad11a07b0992791fb6710366217344241997c728ecfc7d"

      define_method(:install) do
        bin.install "whzbox"
      end
    end
    if Hardware::CPU.arm?
      url "https://github.com/meigma/whzbox/releases/download/v1.0.3/whzbox_1.0.3_darwin_arm64.tar.gz"
      sha256 "b5aadda21bb55418161cd0e4a44a8058f786ed5ec7f8365f607b60aeeeee95f6"

      define_method(:install) do
        bin.install "whzbox"
      end
    end
  end

  on_linux do
    if Hardware::CPU.intel? && Hardware::CPU.is_64_bit?
      url "https://github.com/meigma/whzbox/releases/download/v1.0.3/whzbox_1.0.3_linux_amd64.tar.gz"
      sha256 "ebefe3d13b2a96cc8af6e5f4685e651e621f394c88366c1236c814065fb50ad7"
      define_method(:install) do
        bin.install "whzbox"
      end
    end
    if Hardware::CPU.arm? && Hardware::CPU.is_64_bit?
      url "https://github.com/meigma/whzbox/releases/download/v1.0.3/whzbox_1.0.3_linux_arm64.tar.gz"
      sha256 "78d34037ec0f33bea2a874df9c562e4b9754398abf44cb11a4185543c5190b1c"
      define_method(:install) do
        bin.install "whzbox"
      end
    end
  end

  test do
    system "#{bin}/whzbox", "version"
  end
end
```

### Full local `tap/Formula/blob.rb`

`/Users/josh/code/meigma/tap/Formula/blob.rb:1-50`:

```ruby
# typed: false
# frozen_string_literal: true

# This file was generated by GoReleaser. DO NOT EDIT.
class Blob < Formula
  desc "A CLI for working with blob archives in OCI registries"
  homepage "https://github.com/meigma/blob-cli"
  version "1.1.0"
  license "MIT"

  on_macos do
    if Hardware::CPU.intel?
      url "https://github.com/meigma/blob-cli/releases/download/v1.1.0/blob_1.1.0_darwin_amd64.tar.gz"
      sha256 "812018d1aa11b1ab551ddfc1306263cf36c4036e0b6f39996f5264b2cb97b749"

      def install
        bin.install "blob"
      end
    end
    if Hardware::CPU.arm?
      url "https://github.com/meigma/blob-cli/releases/download/v1.1.0/blob_1.1.0_darwin_arm64.tar.gz"
      sha256 "5103a014935d7dba4226526f6313745012146ec7dd59a7a0252596d121e51ec1"

      def install
        bin.install "blob"
      end
    end
  end

  on_linux do
    if Hardware::CPU.intel? && Hardware::CPU.is_64_bit?
      url "https://github.com/meigma/blob-cli/releases/download/v1.1.0/blob_1.1.0_linux_amd64.tar.gz"
      sha256 "8c8aa82b7b2188f341c56588dd43571fe75de075f4858af9f0af897971911848"
      def install
        bin.install "blob"
      end
    end
    if Hardware::CPU.arm? && Hardware::CPU.is_64_bit?
      url "https://github.com/meigma/blob-cli/releases/download/v1.1.0/blob_1.1.0_linux_arm64.tar.gz"
      sha256 "61c0897ffc1cab7ba7326d2ed462352988c551ceb6eadcc0472d19923a0a4fbc"
      def install
        bin.install "blob"
      end
    end
  end

  test do
    system "#{bin}/blob", "version"
  end
end
```

### `Casks/.gitkeep`

The placeholder is not empty. `/Users/josh/code/meigma/tap/Casks/.gitkeep:1` contains:

```text
# This directory is populated by release automation.
```

On canonical GitHub `main`, this placeholder coexists with `Casks/ghd.rb`; it is no longer the only Casks file: [canonical tap tree](https://api.github.com/repos/meigma/homebrew-tap/git/trees/main?recursive=1).

### Full `scoop-bucket/whzbox.json` from `origin/main`

`/Users/josh/code/meigma/scoop-bucket` has no local commits, so this was read exactly with `git show origin/main:whzbox.json` (`origin/main:whzbox.json:1-15`):

```json
{
    "version": "1.1.0",
    "architecture": {
        "64bit": {
            "url": "https://github.com/meigma/whzbox/releases/download/v1.1.0/whzbox_1.1.0_windows_amd64.zip",
            "bin": [
                "whzbox.exe"
            ],
            "hash": "46725e6742a2daeee574a20a11bf81d7a95edaec30491143280a0a89aafc3b80"
        }
    },
    "homepage": "https://github.com/meigma/whzbox",
    "license": "MIT OR Apache-2.0",
    "description": "A small Go CLI for managing Whizlabs cloud sandboxes"
}
```

### Full `scoop-bucket/blob.json` from `origin/main`

`git show origin/main:blob.json` (`origin/main:blob.json:1-15`):

```json
{
    "version": "1.1.0",
    "architecture": {
        "64bit": {
            "url": "https://github.com/meigma/blob-cli/releases/download/v1.1.0/blob_1.1.0_windows_amd64.zip",
            "bin": [
                "blob.exe"
            ],
            "hash": "6b0d693260d5d2cc896f0da969b05370b1c530ba8442206a03ff0932b7fe49ca"
        }
    },
    "homepage": "https://github.com/meigma/blob-cli",
    "license": "MIT",
    "description": "A CLI for working with blob archives in OCI registries"
}
```

### Complete current canonical layouts

`GET repos/meigma/homebrew-tap/git/trees/main?recursive=1` returns:

```text
Casks/
Casks/.gitkeep
Casks/ghd.rb
Formula/
Formula/blob.rb
Formula/blobber.rb
Formula/cuefn.rb
Formula/whzbox.rb
```

Source: [canonical tap tree](https://api.github.com/repos/meigma/homebrew-tap/git/trees/main?recursive=1).

`GET repos/meigma/scoop-bucket/git/trees/main?recursive=1` returns:

```text
blob.json
blobber.json
cuefn.json
whzbox.json
```

Source: [canonical bucket tree](https://api.github.com/repos/meigma/scoop-bucket/git/trees/main?recursive=1).

Neither repository has a README: both requested README endpoints returned `404 Not Found`. Neither has `.github/workflows`; both requested contents endpoints returned `404 Not Found`, and the recursive trees contain no `.github` path. Sources: [tap README endpoint](https://api.github.com/repos/meigma/homebrew-tap/readme), [bucket README endpoint](https://api.github.com/repos/meigma/scoop-bucket/readme), [tap workflows endpoint](https://api.github.com/repos/meigma/homebrew-tap/contents/.github/workflows), [bucket workflows endpoint](https://api.github.com/repos/meigma/scoop-bucket/contents/.github/workflows).

Historical bootstrap evidence is also minimal: the first tap commit created `Formula/blobber.rb` directly (`d297a52`), and the first bucket commit created `blobber.json` directly (`a9fcb9ef8645`). No README or CI scaffold preceded the generated files: [tap history](https://github.com/meigma/homebrew-tap/commits/main), [bucket history](https://github.com/meigma/scoop-bucket/commits/main).

---

## 7. Rulesets and branch protection

Requested ruleset calls returned empty arrays for both canonical repositories:

```text
gh api repos/meigma/tap/rulesets            -> []
gh api repos/meigma/homebrew-tap/rulesets   -> []
gh api repos/meigma/scoop-bucket/rulesets   -> []
```

Sources: [tap rulesets](https://api.github.com/repos/meigma/homebrew-tap/rulesets), [bucket rulesets](https://api.github.com/repos/meigma/scoop-bucket/rulesets).

The legacy branch-protection endpoint also returned `404 Branch not protected` for `main` in both repositories:

```text
gh api repos/meigma/homebrew-tap/branches/main/protection -> 404 Branch not protected
gh api repos/meigma/scoop-bucket/branches/main/protection -> 404 Branch not protected
```

Sources: [tap branch protection](https://api.github.com/repos/meigma/homebrew-tap/branches/main/protection), [bucket branch protection](https://api.github.com/repos/meigma/scoop-bucket/branches/main/protection).

Therefore there is no observed ruleset, required PR, status check, or bypass actor involved in the current direct bot commits. The publishing credential still needs ordinary write access to the target repository.

---

## 8. Existing `componere` repositories

Verbatim `gh repo list componere --limit 100 --json name,visibility,description` result, formatted:

| Name | Visibility | Description |
|---|---|---|
| `incusos-builder` | PUBLIC | `Build seeded IncusOS installation media from a YAML config.` |
| `incusos-spire` | PUBLIC | empty |
| `imgoci` | PRIVATE | empty |
| `incus-vm-oci` | PRIVATE | empty |
| `incus-bootc` | PUBLIC | empty |

No repository named `homebrew-tap`, `tap`, `scoop-bucket`, or any other tap/bucket-like name exists in the returned organization inventory. All five existing repositories currently use default branch `master`; the two new publishing repositories should nevertheless use `main` to match the pinned GoReleaser target and Meigma publishing repositories.

Source organization listing: [GitHub API — componere repositories](https://api.github.com/orgs/componere/repos?per_page=100).

---

## Unknowns and caveats

1. **Actual credential type is not discoverable.** GitHub exposes secret names and update timestamps, not secret values or types. No inspected documentation identifies the two publishing secrets as classic PATs, fine-grained PATs, App tokens, or a named 1Password entry.
2. **The local tap clone is stale.** Its local `origin/main` is April 2026 and does not reflect the current canonical GitHub tree; this is why local `Formula/whzbox.rb` is v1.0.3 while the current repository is v1.1.0.
3. **GoReleaser now labels `brews:` deprecated as of v2.10 in favor of Homebrew Casks**, although the three inspected projects still use `brews:` successfully: [GoReleaser Homebrew Formulas documentation](https://goreleaser.com/customization/homebrew_formulas/). This report reproduces the requested Meigma formula pattern rather than changing that design.
4. No builds, formatters, linters, or tests were run, per the investigation-only constraints.