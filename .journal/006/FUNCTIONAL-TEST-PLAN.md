# Manual functional release test plan

## 1. Purpose and verdict criteria

This plan proves that `incusos-builder` is ready for its first release by using the same surfaces an external user uses: the compiled CLI, YAML files, stdin and stdout, the live HTTPS update service, a local mirror, generated installer and rescue-media files, the documentation, GitHub Releases, `ghd`, GHCR, and a real IncusOS boot. It does not treat unit-test coverage or an implementation detail as evidence that a public promise works.

The release decision has three gates:

1. **Pre-tag gate.** All applicable cases in suites A–G, H-01 through H-03, and I-01 must pass. In particular, the boot-acceptance observations in I-01 are mandatory before a tag. A missing suitable Linux/Incus host is `Blocked`, not a waiver.
2. **Draft-release gate.** After Release Please creates the first tag and draft release, H-04, H-06, and H-07 must pass before a maintainer publishes the draft GitHub release. The GHCR tag is already public at this point; a failure blocks publication and requires correction in a new release candidate.
3. **Published-release gate.** H-05 and the post-publication portions of G-07 must pass against the public release. Only then is the verdict **GO**. The first release cannot prove anonymous `ghd` installation before it is published; the dry run and authenticated draft checks reduce but do not eliminate that first-publish risk.

Use these result classes:

| Result | Meaning | Release effect |
|---|---|---|
| `Pass` | Every command and observable assertion in the case matched. | None. |
| `Fail` | A documented command, exit code, stream rule, artifact property, integrity check, boot observation, or publication check differed. | Blocks the release. Do not relabel it as a limitation. |
| `Blocked` | A required host, permission, credential, network route, disk budget, or upstream artifact was unavailable. | Blocks the applicable gate until executed. |
| `Known limitation` | The project explicitly does not claim the property, or the check can only occur after the first publication boundary. | Record the residual risk. It does not excuse a failed claimed behavior. |
| `Not applicable` | The source explicitly limits the promise to a different platform or release phase. | Record the source and reason. Do not use this for lack of time or equipment. |

A **GO** requires:

- every promise-inventory row to have at least one `Pass` result;
- no `Fail` or `Blocked` result in a mandatory case;
- the x86_64 boot gate to show install completion, installer seed wipe, `RESCUE_DATA` detection, and acceptance/effect of the signed recovery payload;
- a clean release rehearsal, correct draft artifacts, signatures, SBOMs, and attestations;
- successful public `ghd install` and `ghd download` after publication;
- an evidence archive tied to the tested commit and, after tagging, the tag and immutable digests.

## 2. Scope

### In scope

- Source installation through the pinned `mise` toolchain and the documented build command.
- The released native binaries, `ghd` package mapping, and the GHCR image.
- The four CLI commands and every documented flag, default, environment override, exit code, JSON envelope, prompt rule, and stream contract.
- Plain and SOPS-encrypted configuration loading, strict decoding, defaults, validation, seed tar rendering, and size enforcement.
- Live HTTPS and local-mirror acquisition, cache behavior, trust checks, cancellation, and adversarial metadata or asset inputs.
- ISO and raw installers for `x86_64` and `aarch64`, compressed output, output streaming, overwrite/publication behavior, and seed splice invariants.
- Raw/FAT32 and ISO9660 offline rescue media and their staged metadata/application tree.
- Every tutorial and how-to guide, executed as written with placeholders resolved to test paths.
- Release Please, the release dry run, GoReleaser assets, melange/apko image publication, SBOMs, cosign, GitHub attestations, Pages, license and support documents, and repository settings needed by those workflows.
- The mandatory x86_64 Incus install-and-recovery boot checklist.

### Out of scope

- **Writing new automated tests or CI jobs.** This is a human functional plan; existing gates are invoked only as supporting evidence.
- **Code coverage and internal collaborator behavior.** Internal seams are not public user surfaces.
- **Refactoring or redesign.** A failed case is reported against the current contract; this plan does not prescribe an implementation.
- **Performance or throughput targets.** The project publishes no build-time or memory-performance promise.
- **Windows binaries or Windows execution.** `.goreleaser.yaml` and `ghd.toml` publish only Darwin and Linux.
- **Support for old releases.** `SECURITY.md` supports only the latest published release after publication.
- **Publisher authentication by the builder.** `docs/docs/explanation/trust-model.md` explicitly says build-time checks do not authenticate the IncusOS publisher or PKCS#7 chain; the booted OS owns that decision.
- **Exhaustive runtime effect of every upstream IncusOS seed field.** The builder promises strict acceptance and rendering of the pinned schema. IncusOS runtime semantics beyond the documented boot gate belong to upstream.
- **Aarch64 boot acceptance.** The required release gate in `docs/docs/how-to/verify-boot-acceptance.md` is x86_64. Aarch64 media creation is tested structurally and remains a recorded residual boot risk.

## 3. Promise inventory

The citations name the file and heading or configuration table that makes the promise.

| Promise | Source | Proved by |
|---|---|---|
| The tool builds seeded IncusOS installation media from YAML and is a local alternative to the web customizer. | `README.md` — opening paragraph; `docs/docs/index.md` — `# incusos-builder` | A-01, E-01, E-04 |
| Source setup uses `mise`, Go 1.26.4, `go run`, and `moon run root:build`. | `README.md` — `Install / From source`; `mise.toml` — `[tools]` and `[settings]` | A-01 |
| Before the first release, `ghd` and GHCR have nothing to fetch; after release, the documented source, `ghd install`, `ghd download`, and container paths work. | `README.md` — `Status`, `From a GitHub release`, `Container image` | A-02, H-05, H-06 |
| Quick start initializes, validates, and builds; `--server` defaults to `https://images.linuxcontainers.org/os`; offline configs also produce rescue media. | `README.md` — `Quick start` | E-01, F-01, G-01 |
| The first-ISO tutorial produces an x86_64 seeded ISO and stops short of boot acceptance. | `docs/docs/tutorials/first-seeded-iso.md` — all numbered steps and `Stop at the boot-acceptance boundary` | G-01, I-01 |
| SOPS file/stdin decryption is in-memory, selected by a top-level `sops` key, and failures map to exit 4 without secret leakage. | `docs/docs/how-to/sops-encryption.md` — steps 1–5 and `Troubleshooting`; `docs/docs/reference/configuration.md` — `Load pipeline`, `SOPS` | C-03, G-02 |
| Offline media publishes installer plus `RESCUE_DATA`, supports default/explicit resource paths, and preserves documented hash and overwrite semantics. | `docs/docs/how-to/build-offline-media.md` — steps 1–5 and `Overwrite behavior` | F-01, F-02, F-03, G-03 |
| CI can use one JSON document, no prompts, stable stderr, flag-over-env precedence, stdin config, exit branching, and explicit overwrite ownership. | `docs/docs/how-to/run-in-ci.md` — steps 1–7 | B-04, B-06, B-08, B-09, E-05, G-04 |
| A local mirror uses the HTTPS tree, existing-directory classification, path allowlist, required metadata, and digest admission. | `docs/docs/how-to/use-local-mirror.md` — steps 1–5 | D-01, D-04, D-05, G-05 |
| Interrupted `--force` builds leave classifiable finals, backups, and temps, and the documented recovery procedure restores or discards them safely. | `docs/docs/how-to/recover-interrupted-build.md` — `What --force does`, steps 1–7 | E-06, E-07, G-06 |
| Boot acceptance must observe install, seed wipe, `RESCUE_DATA`, and signed recovery effect before every release tag. | `docs/docs/how-to/verify-boot-acceptance.md` — opening gate and steps 1–8 | I-01 |
| Root, `build`, `validate`, `versions`, and `init` have no operands; help, version, all persistent flags, all command flags, and documented defaults behave as listed. | `docs/docs/reference/cli.md` — `incusos-builder`, `Persistent flags`, and four command headings | B-01 through B-05 |
| `--version` is exactly two lines and reports the pinned incus-osd pseudo-version. | `docs/docs/reference/cli.md` — `incusos-builder`; `docs/docs/explanation/upstream-version-coupling.md` — `Why a pseudo-version` | A-01, B-02, H-04 |
| Exit codes 0–6 are stable, help is not reprinted on failure, and wrapped errors retain their class. | `docs/docs/reference/automation.md` — `Exit codes` | B-09, C-03, C-05, D-08, E-05 |
| Error, build, validate, versions, and init JSON envelopes have the documented fields; JSON is one newline-terminated document. | `docs/docs/reference/automation.md` — `JSON envelopes` | B-03, B-04, B-09, E-01, F-01 |
| Flags beat environment values, including explicit false booleans; `CI`, `NO_COLOR`, `TERM=dumb`, `ACCESSIBLE`, and `SOPS_AGE_KEY` have their documented effects. | `docs/docs/reference/automation.md` — `Environment variables and precedence`, `TTY, prompts, and --no-input` | B-06, B-07, B-08, C-03 |
| stdout/stderr separation, quiet behavior, stream sentinel rules, TTY auto-resolution, and progress auto-resolution are stable. | `docs/docs/reference/automation.md` — `stdout and stderr`, `-`, `TTY, prompts, and --no-input` | B-07, B-08, E-02, E-05 |
| Only schema `version: 1` is accepted; strict unknown-field text names the exact upstream pin; quoted literals are redacted. | `docs/docs/reference/configuration.md` — `Schema pin`, `Strict decode`, `Load pipeline` | C-01, C-02 |
| Defaults and every validation-matrix row are applied exactly as documented. | `docs/docs/reference/configuration.md` — `Defaults`, `Validation` | C-01, C-04, F-03 |
| All eleven seed sections render in documented order, mode 0600, with default section version `"1"`; a tar larger than seed-data is exit 3. | `docs/docs/reference/configuration.md` — `seeds`, `Seed-data size` | C-04, C-05 |
| ISO/raw and x86_64/aarch64 are accepted; channel/release/offline fields have documented semantics. | `docs/docs/reference/configuration.md` — `image` | C-01, D-01, E-01 |
| Cache defaults differ on macOS and Linux; override, absolute content-addressed layout, immutable digest reuse, mode 0444, validation, cleanup, and low-space warning work. | `docs/docs/reference/cache.md` — `Location`, `Layout`, `Digest keys`, `Hit, miss, and success`, `Implemented cleanup` | D-02 through D-04, D-07, D-09 |
| Metadata validates version, filename, lowercase SHA-256, and `0 < size <= 8 GiB`; tampering uses the documented wording. | `docs/docs/reference/cache.md` — `Digest keys`; `docs/docs/how-to/use-local-mirror.md` — `Expect digest checks` | D-04, D-05 |
| Output supports ISO/raw, both architectures, `.gz` stored-byte digest, `--force`, prompt, backups, both-or-neither, and distinct cleaned paths. | `docs/docs/reference/cli.md` — `build`; `docs/docs/how-to/build-offline-media.md` — `Overwrite behavior` | E-01 through E-07 |
| Splicing changes only the tar-sized prefix of seed-data and preserves bytes before and after it. | `docs/docs/explanation/seed-injection.md` — `The splice invariant` | E-04 |
| Offline media forces update checks to `never`, requires applications, supports ISO9660 and GPT+FAT32 `RESCUE_DATA`, and stages metadata and application assets at documented paths. | `docs/docs/how-to/build-offline-media.md` — steps 1–4; `docs/docs/reference/configuration.md` — `Defaults`, `image.offline` | F-01 through F-04 |
| Network sources are HTTPS-only; plain HTTP and HTTPS-to-HTTP redirects fail. | `docs/docs/explanation/trust-model.md` — `Why HTTPS, and why a redirect still has to be HTTPS` | D-01, D-06 |
| Filename/size/digest admission and three-way selected-file binding are enforced; index is capped at 64 MiB and each metadata file at 1 MiB; unknown metadata fields are tolerated and trailing data is not. | `docs/docs/explanation/trust-model.md` — `Filename, size, and SHA-256`, JSON paragraph, size-cap paragraph | D-04, D-05, D-06 |
| `update.sjson` must be multipart/signed and structurally bound, but the builder does not authenticate PKCS#7. | `docs/docs/explanation/trust-model.md` — `What update.sjson is, and what parsing it does not do` | D-05, D-06, I-01 |
| Release Please creates a release PR with changelog and version-marker changes; merging creates a draft release and tag. | `CONTRIBUTING.md` — `Commits and releases`; `release-please-config.json`; `.github/workflows/release-please.yml` | H-02 |
| The non-publishing dry run executes the real GoReleaser, ghd staging, melange, apko, image, and SBOM paths without publishing. | `.github/workflows/release-dry-run.yml` — file header and jobs | H-03 |
| GoReleaser publishes exactly four mapped binaries, four binary SBOMs, and checksums that match `ghd.toml`. | `.goreleaser.yaml` — `builds`, `archives`, `sboms`; `ghd.toml` — asset rows; `.github/scripts/stage_ghd_release_assets.py` — `EXPECTED_ASSET_COUNT` | H-04, H-05 |
| GHCR contains amd64 and arm64, runs as UID 65532, carries correct version metadata, has an SBOM, and is keyless-signed. | `apko.yaml` — `accounts`, `archs`; `.github/workflows/release.yml` — `Container Image Release` | H-06, H-07 |
| Binary and image provenance are generated by isolated `attest.yml`; GitHub verification must bind signer workflow and source tag. | `.github/workflows/attest.yml` — file header and `Attest`; `.github/workflows/release.yml` — attestation jobs | H-07 |
| MkDocs builds strictly and Pages publishes the documented site. | `docs/mkdocs.yml` — `strict`, `site_url`; `.github/workflows/docs-pages.yml` | G-07 |
| Apache-2.0 or MIT licensing, latest-release security support, private reporting, and the contributor setup are accurate. | `README.md` — `License`; `SECURITY.md`; `CONTRIBUTING.md` — `Local setup` and `Pull requests` | G-07, H-01 |
| Desired repository settings include immutable releases, private vulnerability reporting, protected branches/tags, required checks, and Pages HTTPS. | `.github/repository-settings.toml` — `[repository]`, `[security]`, `[pages]`, `[rulesets.*]` | H-01 |
| The released container image is scanned and carries no unfixed HIGH/CRITICAL vulnerability; SARIF is uploaded even when the scan fails. | `.github/workflows/security-scan.yml` — build and scan jobs | H-08 |

## 4. Test environment and prerequisites

### Required hosts

1. **Primary:** macOS on Apple Silicon (`arm64`) with an interactive terminal. Reserve at least 50 GiB free after tool installation. Four decompressed installer outputs, multiple caches, rescue media, and corruption copies are multi-gigabyte.
2. **Boot host:** x86_64 Linux with hardware virtualization (`/dev/kvm`), Incus, a managed `default` storage pool, `incusbr0`, sudo access, and at least 80 GiB free in the pool plus space for installer/rescue working copies. It must have a SPICE viewer.
3. **Native released-binary/image checks:** use the macOS arm64 host for Darwin arm64 and Linux arm64 through Docker Desktop; use the x86_64 Linux host for Linux amd64 and the amd64 container image. Darwin amd64 execution needs Rosetta 2; if unavailable, inspect its Mach-O architecture and record the native-execution gap separately.

### Tools

Install or make available:

- `git`, `curl`, `openssl`, Bash, Python 3, `jq`, `yq`, `sha256sum`, `gzip`, GNU/BSD tar;
- `mise`, the repository-pinned tools from `mise install`, and `moon` through `mise`;
- `sops`, `age`/`age-keygen`;
- Docker Desktop on macOS and Docker/Buildx on Linux;
- `xorriso`, `mtools` (`mdir`, `mcopy`, `mlabel`), and `sgdisk`/`gdisk`;
- `gh`, `ghd`, `cosign`, `syft`, and `file`;
- on the boot host: `incus`, `losetup`, `lsblk`, `blockdev`, `dd`, `awk`, and `remote-viewer` or `spicy`.

On macOS, one suitable Homebrew command is:

```bash
brew install mise gh jq yq sops age cosign syft coreutils gnu-tar xorriso mtools gptfdisk bash
```

Install Docker Desktop and `ghd` using their published installers. Do not use a globally installed `golangci-lint` for project checks. Confirm the pinned version with `mise x -- golangci-lint version`; it must report `2.12.2` rather than the known shadowing global `2.11.4`.

### Network and credentials

- Allow HTTPS access to `github.com`, `api.github.com`, `ghcr.io`, Sigstore/Fulcio/Rekor endpoints, mise/aqua sources, Wolfi repositories, and `https://images.linuxcontainers.org/os`.
- Before release-side cases, `gh auth status` must show a maintainer identity with repository, Actions, release, package, and attestation read/write access as required by the workflow being dispatched.
- The repository must have `vars.COMPONERE_RELEASE_APP_CLIENT_ID` and `secrets.COMPONERE_RELEASE_APP_PRIVATE_KEY`; the Release Please app must be allowed to create protected tags.
- Authenticate Docker to GHCR for private/draft inspection if required. Anonymous pulls are used again after publication.
- The Linux executor needs sudo and Incus administration rights.

### Common workspace

Do not create fixtures in the repository. Start a Bash shell and run:

```bash
set -euo pipefail
export REPO=/Users/josh/code/componere/incusos-builder
export WORK="$(mktemp -d -t incusos-builder-functional.XXXXXX)"
export EVIDENCE="$WORK/evidence"
export CACHE="$WORK/cache"
export MIRROR="$WORK/mirror"
mkdir -p "$EVIDENCE" "$CACHE" "$MIRROR"
cd "$REPO"

git rev-parse HEAD | tee "$EVIDENCE/tested-commit.txt"
git status --short | tee "$EVIDENCE/initial-status.txt"
test ! -s "$EVIDENCE/initial-status.txt"

mise install
mise x -- moon run root:build
export IB="$REPO/bin/incusos-builder"
test -x "$IB"
```

Keep `$WORK` until the result record and required artifacts have been archived. Record free space before the large suites:

```bash
df -h "$WORK" | tee "$EVIDENCE/disk-before.txt"
```

### Local mirror setup

This setup intentionally copies the live update-server layout. It is test data, not a repository change.

```bash
set -euo pipefail
export UPDATE_BASE=https://images.linuxcontainers.org/os
curl --fail --location --proto '=https' --proto-redir '=https' \
  "$UPDATE_BASE/index.json" -o "$MIRROR/index.json"

export INCUSOS_VERSION="$(jq -r '
  [.updates[] | select(.channels | index("stable"))]
  | sort_by(.version) | last | .version
' "$MIRROR/index.json")"
test -n "$INCUSOS_VERSION"
test "$INCUSOS_VERSION" != null
printf '%s\n' "$INCUSOS_VERSION" | tee "$EVIDENCE/mirror-version.txt"

mirror_download() {
  local rel=$1
  mkdir -p "$MIRROR/$INCUSOS_VERSION/$(dirname "$rel")"
  curl --fail --location --proto '=https' --proto-redir '=https' \
    "$UPDATE_BASE/$INCUSOS_VERSION/$rel" \
    -o "$MIRROR/$INCUSOS_VERSION/$rel"
}

for arch in x86_64 aarch64; do
  for typ in image-raw image-iso; do
    rel="$(jq -r --arg v "$INCUSOS_VERSION" --arg a "$arch" --arg t "$typ" '
      .updates[] | select(.version == $v) | .files[]
      | select(.architecture == $a and .type == $t) | .filename
    ' "$MIRROR/index.json" | head -n 1)"
    test -n "$rel"
    mirror_download "$rel"
  done
done

export INCUS_APP_REL="$(jq -r --arg v "$INCUSOS_VERSION" '
  .updates[] | select(.version == $v) | .files[]
  | select(.architecture == "x86_64" and .type == "application"
           and .component == "incus" and (.filename | endswith("/incus.raw.gz")))
  | .filename
' "$MIRROR/index.json" | head -n 1)"
test -n "$INCUS_APP_REL"
mirror_download "$INCUS_APP_REL"

curl --fail --location --proto '=https' --proto-redir '=https' \
  "$UPDATE_BASE/$INCUSOS_VERSION/update.json" \
  -o "$MIRROR/$INCUSOS_VERSION/update.json"
curl --fail --location --proto '=https' --proto-redir '=https' \
  "$UPDATE_BASE/$INCUSOS_VERSION/update.sjson" \
  -o "$MIRROR/$INCUSOS_VERSION/update.sjson"

jq -r --arg v "$INCUSOS_VERSION" '
  .updates[] | select(.version == $v) | .files[]
  | [.sha256, (.size|tostring), .filename] | @tsv
' "$MIRROR/index.json" |
while IFS=$'\t' read -r digest size rel; do
  path="$MIRROR/$INCUSOS_VERSION/$rel"
  [ -f "$path" ] || continue
  test "$(wc -c < "$path" | tr -d ' ')" = "$size"
  printf '%s  %s\n' "$digest" "$path" | sha256sum --check --status
  printf 'verified %s\n' "$rel"
done | tee "$EVIDENCE/mirror-assets.txt"
```

The expected tree is `index.json`, four architecture/type installer files below `$INCUSOS_VERSION`, the x86_64 `incus.raw.gz`, and `update.json`/`update.sjson` directly below the version. A size or digest mismatch in setup is a blocked environment, not a product failure.

### Throwaway age key

```bash
mkdir -p "$WORK/sops"
age-keygen -o "$WORK/sops/age.key"
export AGE_RECIPIENT="$(age-keygen -y "$WORK/sops/age.key")"
export SOPS_AGE_KEY="$(grep '^AGE-SECRET-KEY-' "$WORK/sops/age.key")"
unset SOPS_AGE_KEY_FILE SOPS_AGE_KEY_CMD
printf '%s\n' "$AGE_RECIPIENT" > "$EVIDENCE/age-recipient.txt"
```

Never archive `age.key` or `SOPS_AGE_KEY` as release evidence.

## 5. Test suites and cases

Run suites in order unless a case states that it can run in parallel.

### Suite A — installation and provenance

#### A-01 — Source installation and pinned toolchain

**Promise and citation:** `README.md` — `Install / From source`; `CONTRIBUTING.md` — `Local setup`; `mise.toml` — `[tools]`, `[settings]`.

**Preconditions:** Fresh clone access; no previously activated repository tool environment.

**Commands:**

```bash
cd "$WORK"
git clone https://github.com/componere/incusos-builder.git source-install
cd source-install
mise install
mise x -- go version | tee "$EVIDENCE/source-go-version.txt"
mise x -- golangci-lint version | tee "$EVIDENCE/source-lint-version.txt"
go run ./cmd/incusos-builder --version | tee "$EVIDENCE/source-version.txt"
mise x -- moon run root:build
test -x bin/incusos-builder
bin/incusos-builder --version | tee "$EVIDENCE/source-built-version.txt"
```

**Expected observable result:** Go reports `go1.26.4`; golangci-lint reports `2.12.2`; both version invocations print exactly:

```text
incusos-builder dev (none) built unknown
incus-os API: v0.0.0-20260815030500-0f5b8057f2fc
```

`bin/incusos-builder` exists and is executable.

**Evidence:** The four captured version files, clone commit, and Moon task result.

**Pass/fail:** Pass only if the pinned tools, `go run`, and built binary all work with the exact two-line version output.

#### A-02 — Honest pre-release install boundary

**Promise and citation:** `README.md` — `Status`, `From a GitHub release`, `Container image`.

**Preconditions:** Run before the first tag.

**Commands:**

```bash
cd "$REPO"
test -z "$(git tag --list 'v*')"
test -z "$(gh release list --repo componere/incusos-builder --limit 1)"
set +e
ghd download componere/incusos-builder/incusos-builder@0.1.1 \
  --output "$WORK/pre-release-ghd" \
  >"$EVIDENCE/pre-release-ghd.stdout" 2>"$EVIDENCE/pre-release-ghd.stderr"
rc=$?
set -e
test "$rc" -ne 0
```

**Expected observable result:** No `v*` tag or GitHub release exists, and `ghd` cannot download the nonexistent release. The README directs the user to source installation instead.

**Evidence:** Tag/release listings, `ghd` stdout/stderr, and exit status.

**Pass/fail:** Pass if the repository state and documentation agree. After a release exists, mark the pre-release portion not applicable and use H-05/H-06.

#### A-03 — Existing repository and live gates

**Promise and citation:** `CONTRIBUTING.md` — `Local setup`; `moon.yml` — `check`, `e2e`.

**Preconditions:** Pinned toolchain installed; multi-GB network access for e2e.

**Commands:**

```bash
cd "$REPO"
mise x -- moon run root:check 2>&1 | tee "$EVIDENCE/root-check.txt"
mise x -- moon run root:e2e 2>&1 | tee "$EVIDENCE/root-e2e.txt"
```

**Expected observable result:** Both tasks exit 0. `root:check` runs format, lint, build, tests, upstream closure, and docs. `root:e2e` runs the opt-in live suite and does not skip for missing `INCUSOS_BUILDER_E2E` because the Moon task sets it.

**Evidence:** Complete task output and tested commit.

**Pass/fail:** Any failure blocks release. These gates supplement, but do not replace, the remaining public-surface cases.

### Suite B — CLI and automation contract

#### B-01 — Root, command discovery, help, flags, and operands

**Promise and citation:** `docs/docs/reference/cli.md` — entire page.

**Preconditions:** `$IB` built.

**Commands:**

```bash
set -euo pipefail
"$IB" >"$EVIDENCE/root-empty.stdout" 2>"$EVIDENCE/root-empty.stderr"
test ! -s "$EVIDENCE/root-empty.stdout"
test ! -s "$EVIDENCE/root-empty.stderr"

"$IB" --help >"$EVIDENCE/help-root.txt"
for cmd in build validate versions init; do
  "$IB" "$cmd" --help >"$EVIDENCE/help-$cmd.txt"
done

grep -F 'build' "$EVIDENCE/help-root.txt"
grep -F 'validate' "$EVIDENCE/help-root.txt"
grep -F 'versions' "$EVIDENCE/help-root.txt"
grep -F 'init' "$EVIDENCE/help-root.txt"
for flag in color progress no-input verbose quiet server cache-dir json; do
  grep -F -- "--$flag" "$EVIDENCE/help-root.txt"
done
for flag in config output resources-output force; do
  grep -F -- "--$flag" "$EVIDENCE/help-build.txt"
done
grep -F -- '--config' "$EVIDENCE/help-validate.txt"
grep -F -- '--channel' "$EVIDENCE/help-versions.txt"
grep -F -- '--architecture' "$EVIDENCE/help-versions.txt"
grep -F -- '--output' "$EVIDENCE/help-init.txt"

for cmd in build validate versions init; do
  set +e
  "$IB" "$cmd" unexpected-operand \
    >"$EVIDENCE/$cmd-operand.stdout" 2>"$EVIDENCE/$cmd-operand.stderr"
  rc=$?
  set -e
  test "$rc" -eq 2
  grep -F 'usage error' "$EVIDENCE/$cmd-operand.stderr"
done
```

**Expected observable result:** Root with no command exits 0 and writes nothing. Help names exactly four registered commands and all documented flags. Every positional operand exits 2; failure output does not append Cobra help text.

**Evidence:** All help and operand files.

**Pass/fail:** Pass only if no command/flag is missing and all four commands reject operands.

#### B-02 — Exact source version output

**Promise and citation:** `docs/docs/reference/cli.md` — `incusos-builder`; `docs/docs/explanation/upstream-version-coupling.md` — `Why a pseudo-version`.

**Commands:**

```bash
"$IB" --version >"$EVIDENCE/version.txt"
cat >"$WORK/expected-version.txt" <<'EOF'
incusos-builder dev (none) built unknown
incus-os API: v0.0.0-20260815030500-0f5b8057f2fc
EOF
diff -u "$WORK/expected-version.txt" "$EVIDENCE/version.txt"
test "$(wc -l < "$EVIDENCE/version.txt" | tr -d ' ')" -eq 2
```

**Expected observable result:** Exact diff match and two newline-terminated lines.

**Evidence:** `version.txt`.

**Pass/fail:** Any text, line-count, or pin difference fails.

#### B-03 — `init` file, stdout, JSON, quiet, and refusal

**Promise and citation:** `docs/docs/reference/cli.md` — `init`; `docs/docs/reference/automation.md` — `init`, `-`, `stdout and stderr`.

**Commands:**

```bash
mkdir -p "$WORK/init"
"$IB" init --no-input --color never -o "$WORK/init/config.yaml" \
  >"$EVIDENCE/init-human.stdout" 2>"$EVIDENCE/init-human.stderr"
grep -Fx "wrote $WORK/init/config.yaml" "$EVIDENCE/init-human.stdout"
grep -F 'version: 1' "$WORK/init/config.yaml"
grep -F 'type: iso' "$WORK/init/config.yaml"
grep -F 'architecture: x86_64' "$WORK/init/config.yaml"
grep -F 'channel: stable' "$WORK/init/config.yaml"
grep -F 'offline: false' "$WORK/init/config.yaml"
for section in applications incus install migration-manager network operations-center provider services update kernel security; do
  grep -F "#  $section: {}" "$WORK/init/config.yaml"
done
"$IB" validate -f "$WORK/init/config.yaml" --color never

"$IB" init --no-input -o - >"$EVIDENCE/init-stream.yaml"
! grep -F 'wrote ' "$EVIDENCE/init-stream.yaml"
"$IB" validate -f "$EVIDENCE/init-stream.yaml" --color never

"$IB" init --json --no-input -o "$WORK/init/json.yaml" \
  >"$EVIDENCE/init-json.stdout" 2>"$EVIDENCE/init-json.stderr"
test "$(wc -l < "$EVIDENCE/init-json.stdout" | tr -d ' ')" -eq 1
jq -e --arg p "$WORK/init/json.yaml" '.result.output == $p' "$EVIDENCE/init-json.stdout"

"$IB" init -q --no-input -o "$WORK/init/quiet.yaml" \
  >"$EVIDENCE/init-quiet.stdout" 2>"$EVIDENCE/init-quiet.stderr"
test ! -s "$EVIDENCE/init-quiet.stdout"

set +e
"$IB" init --no-input -o "$WORK/init/config.yaml" \
  >"$EVIDENCE/init-existing.stdout" 2>"$EVIDENCE/init-existing.stderr"
rc=$?
set -e
test "$rc" -eq 2
grep -F 'refusing to overwrite existing file' "$EVIDENCE/init-existing.stderr"

set +e
"$IB" init --json --no-input -o - \
  >"$EVIDENCE/init-json-stream.stdout" 2>"$EVIDENCE/init-json-stream.stderr"
rc=$?
set -e
test "$rc" -eq 2
jq -e '.error.code == 2' "$EVIDENCE/init-json-stream.stdout"
grep -F -- '--json cannot be combined with -o -' "$EVIDENCE/init-json-stream.stderr"
```

**Expected observable result:** Deterministic config, all eleven commented seed keys, valid YAML, raw YAML only on `-o -`, exact file success line, exact init JSON shape, quiet file write with no stdout, and exit-2 refusals. `init` never offers `--force`.

**Evidence:** Generated YAML and all captured streams.

**Pass/fail:** Every assertion must pass.

#### B-04 — `validate` and `versions` human and JSON contracts

**Promise and citation:** `docs/docs/reference/cli.md` — `validate`, `versions`; `docs/docs/reference/automation.md` — `validate`, `versions`.

**Commands:**

```bash
cat >"$WORK/valid.yaml" <<'EOF'
version: 1
image:
  type: iso
  architecture: x86_64
EOF

"$IB" validate -f "$WORK/valid.yaml" --color never \
  >"$EVIDENCE/validate-human.stdout" 2>"$EVIDENCE/validate-human.stderr"
grep -Fx 'configuration valid' "$EVIDENCE/validate-human.stdout"
"$IB" validate --json -f "$WORK/valid.yaml" \
  >"$EVIDENCE/validate-json.stdout"
jq -e '.result == {"valid":true,"type":"iso","architecture":"x86_64","offline":false}' \
  "$EVIDENCE/validate-json.stdout"

"$IB" versions --server "$MIRROR" --cache-dir "$CACHE" \
  --architecture x86_64 --color never >"$EVIDENCE/versions-human.stdout"
for heading in Version Channel Architecture Type; do
  grep -F "$heading" "$EVIDENCE/versions-human.stdout"
done
grep -F "$INCUSOS_VERSION" "$EVIDENCE/versions-human.stdout"
grep -F 'x86_64' "$EVIDENCE/versions-human.stdout"

"$IB" versions --json --server "$MIRROR" --cache-dir "$CACHE" \
  --architecture x86_64 >"$EVIDENCE/versions-json.stdout"
test "$(wc -l < "$EVIDENCE/versions-json.stdout" | tr -d ' ')" -eq 1
jq -e --arg v "$INCUSOS_VERSION" '
  (.result.versions | type == "array") and
  (any(.result.versions[]; .version == $v and
    (.channels | type == "array") and
    (.architectures | index("x86_64")) and
    (.published_at | fromdateiso8601 > 0)))
' "$EVIDENCE/versions-json.stdout"

"$IB" versions --json --channel definitely-unknown --server "$MIRROR" \
  --cache-dir "$CACHE" >"$EVIDENCE/versions-empty.stdout"
jq -e '.result.versions == []' "$EVIDENCE/versions-empty.stdout"

"$IB" versions --json --channel= --architecture= --server "$MIRROR" \
  --cache-dir "$CACHE" >"$EVIDENCE/versions-all.stdout"
jq -e '(.result.versions | length) > 0 and
       any(.result.versions[].architectures[]; . == "x86_64") and
       any(.result.versions[].architectures[]; . == "aarch64")' \
  "$EVIDENCE/versions-all.stdout"
```

**Expected observable result:** Validate is exact. Versions human output has the four columns; JSON arrays are never null; unknown channel is an empty success; empty channel means stable; empty architecture retains both architectures. Per-image type appears in the table, not the JSON entries.

**Evidence:** All captured outputs.

**Pass/fail:** Every shape and filtering assertion must pass.

#### B-05 — Usage-error matrix

**Promise and citation:** `docs/docs/reference/cli.md` — persistent flags and all command usage paragraphs; `docs/docs/reference/automation.md` — exit 2 row.

**Commands:**

```bash
cat >"$WORK/online.yaml" <<EOF
version: 1
image:
  type: raw
  architecture: x86_64
  release: "$INCUSOS_VERSION"
EOF
cat >"$WORK/offline.yaml" <<EOF
version: 1
image:
  type: raw
  architecture: x86_64
  release: "$INCUSOS_VERSION"
  offline: true
seeds:
  applications:
    applications:
      - name: incus
EOF

expect_usage() {
  local name=$1 needle=$2
  shift 2
  set +e
  "$@" >"$EVIDENCE/$name.stdout" 2>"$EVIDENCE/$name.stderr"
  local rc=$?
  set -e
  test "$rc" -eq 2
  grep -F -- "$needle" "$EVIDENCE/$name.stderr"
}

expect_usage missing-build-config '-f/--config is required' \
  "$IB" build -o "$WORK/x.img"
expect_usage missing-build-output '-o/--output is required' \
  "$IB" build -f "$WORK/online.yaml"
expect_usage missing-validate-config '-f/--config is required' \
  "$IB" validate
expect_usage unknown-flag 'unknown flag' \
  "$IB" --definitely-unknown
expect_usage bad-color 'invalid --color' \
  "$IB" --color ultraviolet
expect_usage bad-progress 'invalid --progress' \
  "$IB" --progress sometimes
expect_usage verbose-quiet '--verbose and --quiet cannot be used together' \
  "$IB" --verbose -q
expect_usage json-build-stream '--json cannot be combined with -o -' \
  "$IB" build --json -f "$WORK/online.yaml" -o -
expect_usage offline-stream 'offline builds cannot use -o -' \
  "$IB" build -f "$WORK/offline.yaml" -o -
expect_usage resources-online '--resources-output requires offline: true' \
  "$IB" build -f "$WORK/online.yaml" -o "$WORK/x.img" --resources-output "$WORK/r.img"
expect_usage resources-stdout 'resources path cannot be -' \
  "$IB" build -f "$WORK/offline.yaml" -o "$WORK/x.img" --resources-output -
expect_usage same-cleaned-path 'image and resources paths must be distinct' \
  "$IB" build -f "$WORK/offline.yaml" -o "$WORK/same.img" \
  --resources-output "$WORK/dir/../same.img"
expect_usage plain-http 'plain http is not supported; use https or a local mirror directory' \
  "$IB" versions --server http://example.invalid/os --cache-dir "$CACHE"
expect_usage missing-server-path 'is neither an https URL nor an existing directory' \
  "$IB" versions --server /definitely-not-a-mirror --cache-dir "$CACHE"
```

**Expected observable result:** Every invocation exits 2, contains the stated diagnostic on stderr, and does not print help after the error.

**Evidence:** One stdout/stderr pair per row.

**Pass/fail:** Any different code, missing diagnostic, prompt, fetch, or help dump fails.

#### B-06 — Environment precedence and explicit false flags

**Promise and citation:** `docs/docs/reference/automation.md` — `Environment variables and precedence`.

**Commands:**

```bash
INCUSOS_BUILDER_JSON=true "$IB" validate --json=false -f "$WORK/valid.yaml" \
  >"$EVIDENCE/json-false.stdout"
grep -Fx 'configuration valid' "$EVIDENCE/json-false.stdout"
! jq -e . "$EVIDENCE/json-false.stdout" >/dev/null 2>&1

INCUSOS_BUILDER_JSON=true "$IB" validate -f "$WORK/valid.yaml" \
  >"$EVIDENCE/json-env.stdout"
jq -e '.result.valid == true' "$EVIDENCE/json-env.stdout"

INCUSOS_BUILDER_SERVER=/definitely-not-a-mirror \
  "$IB" versions --server "$MIRROR" --cache-dir "$CACHE" --json \
  >"$EVIDENCE/server-flag-wins.stdout"
jq -e '(.result.versions | length) > 0' "$EVIDENCE/server-flag-wins.stdout"

INCUSOS_BUILDER_SERVER="$MIRROR" INCUSOS_BUILDER_CACHE_DIR="$CACHE" \
  "$IB" versions --json >"$EVIDENCE/server-cache-env.stdout"
jq -e '(.result.versions | length) > 0' "$EVIDENCE/server-cache-env.stdout"

INCUSOS_BUILDER_PROGRESS=always "$IB" versions --server "$MIRROR" \
  --cache-dir "$CACHE" --color never --json \
  >"$EVIDENCE/progress-env.stdout" 2>"$EVIDENCE/progress-env.stderr"
grep -F 'progress 100%' "$EVIDENCE/progress-env.stderr"

INCUSOS_BUILDER_COLOR=always "$IB" versions --server "$MIRROR" \
  --cache-dir "$CACHE" >"$EVIDENCE/color-env.stdout"
grep $'\033\[' "$EVIDENCE/color-env.stdout"
```

Run the no-input boolean override in a real terminal:

```bash
env -u CI ACCESSIBLE=1 INCUSOS_BUILDER_NO_INPUT=true \
  "$IB" init --no-input=false -o "$WORK/init/env-false-interactive.yaml"
```

Answer `raw`, `aarch64`, `testing`, and `yes`.

**Expected observable result:** Explicit `--json=false` beats the true environment value. Environment values apply when no flag is parsed. Server and cache flags beat bad environment values. Progress/color environment values act like flags. In the TTY, `--no-input=false` beats the true environment value and the ACCESSIBLE form asks all four questions; the YAML contains the selected values.

**Evidence:** Captured streams and interactive transcript/YAML.

**Pass/fail:** Every precedence assertion must hold. The explicit no-input false case is blocked, not skipped, if no TTY is available.

#### B-07 — TTY, `CI`, non-TTY, accessible prompts, and cancellation

**Promise and citation:** `docs/docs/reference/automation.md` — `TTY, prompts, and --no-input`; `docs/docs/reference/cli.md` — `init`.

**Commands and actions:**

1. In a real terminal with `CI` unset, run:

   ```bash
   env -u CI ACCESSIBLE=1 "$IB" init --no-input=false -o "$WORK/init/accessible.yaml"
   ```

   Confirm the line-oriented form asks `Image type`, `Architecture`, `Channel`, and `Offline install?`. Choose `iso`, `x86_64`, empty channel, and `no`. Confirm channel becomes `stable`.

2. Start a second interactive init and press Ctrl+C:

   ```bash
   env -u CI ACCESSIBLE=1 "$IB" init -o "$WORK/init/cancelled.yaml"
   ```

3. Test CI and non-TTY auto-on:

   ```bash
   CI=1 "$IB" init --no-input=false -o "$WORK/init/ci.yaml" \
     >"$EVIDENCE/init-ci.stdout" 2>"$EVIDENCE/init-ci.stderr"
   "$IB" init --no-input=false -o "$WORK/init/non-tty.yaml" \
     >"$EVIDENCE/init-non-tty.stdout" 2>"$EVIDENCE/init-non-tty.stderr"
   grep -F 'type: iso' "$WORK/init/ci.yaml"
   grep -F 'type: iso' "$WORK/init/non-tty.yaml"
   ```

**Expected observable result:** Accessible prompts go to stderr. Empty interactive channel becomes stable. Ctrl+C exits 2 with `init cancelled` and leaves no output file. `CI=1` and redirected stdout disable prompts even with `--no-input=false`, producing the deterministic example.

**Evidence:** Terminal transcript, exit status for cancellation, generated files, and captured streams.

**Pass/fail:** All four resolution paths must match.

#### B-08 — stdout/stderr, quiet, progress, verbose, and color

**Promise and citation:** `docs/docs/reference/automation.md` — `stdout and stderr`; `docs/docs/reference/cli.md` — persistent flags and build verbose paragraph.

**Commands:**

```bash
"$IB" validate -q -f "$WORK/valid.yaml" \
  >"$EVIDENCE/quiet.stdout" 2>"$EVIDENCE/quiet.stderr"
test ! -s "$EVIDENCE/quiet.stdout"

"$IB" validate -q --json -f "$WORK/valid.yaml" \
  >"$EVIDENCE/quiet-json.stdout" 2>"$EVIDENCE/quiet-json.stderr"
jq -e '.result.valid == true' "$EVIDENCE/quiet-json.stdout"

"$IB" versions --json --progress auto --color never --server "$MIRROR" \
  --cache-dir "$CACHE" >"$EVIDENCE/progress-auto.stdout" 2>"$EVIDENCE/progress-auto.stderr"
! grep -F 'progress ' "$EVIDENCE/progress-auto.stderr"

"$IB" versions --json --progress always --color never --server "$MIRROR" \
  --cache-dir "$CACHE" >"$EVIDENCE/progress-always.stdout" 2>"$EVIDENCE/progress-always.stderr"
grep -F 'progress 100%' "$EVIDENCE/progress-always.stderr"

"$IB" versions --color always --server "$MIRROR" --cache-dir "$CACHE" \
  >"$EVIDENCE/color-always.stdout" 2>"$EVIDENCE/color-always.stderr"
grep $'\033\[' "$EVIDENCE/color-always.stdout"

"$IB" versions --color never --server "$MIRROR" --cache-dir "$CACHE" \
  >"$EVIDENCE/color-never.stdout"
! grep $'\033\[' "$EVIDENCE/color-never.stdout"

NO_COLOR=1 script -q "$EVIDENCE/no-color.tty" \
  "$IB" versions --server "$MIRROR" --cache-dir "$CACHE"
! grep $'\033\[' "$EVIDENCE/no-color.tty"
TERM=dumb script -q "$EVIDENCE/term-dumb.tty" \
  "$IB" versions --server "$MIRROR" --cache-dir "$CACHE"
! grep $'\033\[' "$EVIDENCE/term-dumb.tty"

"$IB" build --verbose --json -f "$WORK/online.yaml" -o "$WORK/verbose.img" \
  --server "$MIRROR" --cache-dir "$CACHE" --color never --progress never \
  >"$EVIDENCE/verbose.stdout" 2>"$EVIDENCE/verbose.stderr"
jq -e '.result.sha256 | test("^[0-9a-f]{64}$")' "$EVIDENCE/verbose.stdout"
grep -F 'resolved version' "$EVIDENCE/verbose.stderr"
grep -F 'output paths' "$EVIDENCE/verbose.stderr"
```

On Linux, `script` may require `script -q -c '<command>' <file>`; use that equivalent without changing the asserted environment.

**Expected observable result:** Quiet suppresses only human success output; JSON remains. Auto progress disappears under JSON/non-TTY; explicit always prints progress only to stderr. Color always emits CSI, never/`NO_COLOR`/`TERM=dumb` do not. Verbose adds debug plan records to stderr without contaminating JSON stdout.

**Evidence:** Captured streams and TTY recordings.

**Pass/fail:** Any stream contamination or policy mismatch fails.

#### B-09 — Exit codes 0 through 6 and JSON error envelope

**Promise and citation:** `docs/docs/reference/automation.md` — `Exit codes`, `Error`.

**Commands:**

```bash
cat >"$WORK/bad-type.yaml" <<'EOF'
version: 1
image:
  type: disk
  architecture: x86_64
EOF
cat >"$WORK/stray-sops.yaml" <<'EOF'
version: 1
image:
  type: iso
  architecture: x86_64
sops: {}
EOF

check_rc() {
  local want=$1 name=$2
  shift 2
  set +e
  "$@" >"$EVIDENCE/$name.stdout" 2>"$EVIDENCE/$name.stderr"
  local got=$?
  set -e
  test "$got" -eq "$want"
}

check_rc 0 exit-0 "$IB"
check_rc 2 exit-2 "$IB" build -o "$WORK/no-config.img"
check_rc 3 exit-3 "$IB" validate -f "$WORK/bad-type.yaml"
check_rc 4 exit-4 env SOPS_AGE_KEY= "$IB" validate -f "$WORK/stray-sops.yaml"
check_rc 5 exit-5 "$IB" build -f "$WORK/online.yaml" -o "$WORK/no-cache.img" \
  --server "$MIRROR" --cache-dir=
rm -rf "$WORK/no-such-parent"
check_rc 6 exit-6 "$IB" build -f "$WORK/online.yaml" \
  -o "$WORK/no-such-parent/out.img" --server "$MIRROR" --cache-dir "$CACHE"

set +e
"$IB" validate -f "$WORK/bad-type.yaml" 2>&-
rc=$?
set -e
test "$rc" -eq 1
printf '%s\n' "$rc" >"$EVIDENCE/exit-1.txt"

set +e
"$IB" validate --json -f "$WORK/bad-type.yaml" \
  >"$EVIDENCE/error-envelope.stdout" 2>"$EVIDENCE/error-envelope.stderr"
rc=$?
set -e
test "$rc" -eq 3
test "$(wc -l < "$EVIDENCE/error-envelope.stdout" | tr -d ' ')" -eq 1
jq -e '.error.code == 3 and (.error.message | contains("invalid config"))' \
  "$EVIDENCE/error-envelope.stdout"
test "$(jq -r '.error.message' "$EVIDENCE/error-envelope.stdout")" = \
     "$(cat "$EVIDENCE/error-envelope.stderr")"
```

**Expected observable result:** Codes 0–6 are all deliberately observed. Exit 1 is caused by a failure to write the returned error to closed stderr. JSON failure is one line; its code equals the process code; stderr repeats the same message; help is absent.

**Evidence:** All exit streams and `exit-1.txt`.

**Pass/fail:** Every exact code and envelope assertion must pass.

### Suite C — configuration and SOPS

#### C-01 — Schema gate, defaults, and validation matrix

**Promise and citation:** `docs/docs/reference/configuration.md` — `Schema pin`, `Defaults`, `Validation`, `version`, `image`.

**Preconditions:** `$IB` and a writable fixture directory.

**Commands:**

```bash
mkdir -p "$WORK/config-matrix"
expect_config3() {
  local name=$1 needle=$2 body=$3
  printf '%s' "$body" >"$WORK/config-matrix/$name.yaml"
  set +e
  "$IB" validate -f "$WORK/config-matrix/$name.yaml" \
    >"$EVIDENCE/config-$name.stdout" 2>"$EVIDENCE/config-$name.stderr"
  local rc=$?
  set -e
  test "$rc" -eq 3
  grep -F -- "$needle" "$EVIDENCE/config-$name.stderr"
}

expect_config3 missing-version 'version: required' \
  $'image:\n  type: iso\n  architecture: x86_64\n'
expect_config3 version-2 'version: unsupported schema version; a newer CLI is required' \
  $'version: 2\nimage:\n  type: iso\n  architecture: x86_64\n'
expect_config3 bad-type 'image.type: must be iso or raw' \
  $'version: 1\nimage:\n  type: disk\n  architecture: x86_64\n'
expect_config3 bad-arch 'image.architecture: must be x86_64 or aarch64' \
  $'version: 1\nimage:\n  type: iso\n  architecture: amd64\n'
expect_config3 bad-sort 'seeds.install.target.sort_order: must be empty, smallest, or largest' \
  $'version: 1\nimage:\n  type: iso\n  architecture: x86_64\nseeds:\n  install:\n    target:\n      sort_order: medium\n'
expect_config3 offline-no-apps 'seeds.applications: required when image.offline is true' \
  $'version: 1\nimage:\n  type: iso\n  architecture: x86_64\n  offline: true\n'
expect_config3 offline-empty-apps 'seeds.applications: required when image.offline is true' \
  $'version: 1\nimage:\n  type: iso\n  architecture: x86_64\n  offline: true\nseeds:\n  applications:\n    applications: []\n'
expect_config3 recovery-key 'seeds.security.encryption_recovery_keys' \
  $'version: 1\nimage:\n  type: iso\n  architecture: x86_64\nseeds:\n  security:\n    encryption_recovery_keys:\n      - super-secret-recovery-key\n'
! grep -F 'super-secret-recovery-key' "$EVIDENCE/config-recovery-key.stderr"
grep -F 'it is not possible to set encryption recovery key(s) via the security seed' \
  "$EVIDENCE/config-recovery-key.stderr"

for pair in 'iso x86_64' 'raw aarch64'; do
  set -- $pair
  cat >"$WORK/config-matrix/good-$1-$2.yaml" <<EOF
version: 1
image:
  type: $1
  architecture: $2
EOF
  "$IB" validate -f "$WORK/config-matrix/good-$1-$2.yaml" --color never
 done

for order in '' smallest Smallest largest LARGEST; do
  cat >"$WORK/config-matrix/sort.yaml" <<EOF
version: 1
image:
  type: iso
  architecture: x86_64
seeds:
  install:
    target:
      sort_order: "$order"
EOF
  "$IB" validate -f "$WORK/config-matrix/sort.yaml" --color never
 done

cat >"$WORK/config-matrix/recovery-empty.yaml" <<'EOF'
version: 1
image:
  type: iso
  architecture: x86_64
seeds:
  security:
    encryption_recovery_keys: []
EOF
"$IB" validate -f "$WORK/config-matrix/recovery-empty.yaml" --color never
```

**Expected observable result:** All invalid rows exit 3 with the stated field path/text and no secret. Both valid type/architecture pairs, all case-insensitive sort-order forms, and an empty recovery-key list exit 0. Channel omission is subsequently observed as `stable` in E-01; section/offline defaults are observed in C-04/F-03.

**Evidence:** Matrix files and stdout/stderr for each invalid row.

**Pass/fail:** Every documented matrix row must match.

#### C-02 — Strict decode, exact pin wording, metadata redaction

**Promise and citation:** `docs/docs/reference/configuration.md` — `Strict decode`, `Load pipeline`.

**Commands:**

```bash
cat >"$WORK/config-matrix/unknown.yaml" <<'EOF'
version: 1
image:
  type: iso
  architecture: x86_64
  flavor: desktop
EOF
set +e
"$IB" validate -f "$WORK/config-matrix/unknown.yaml" \
  >"$EVIDENCE/unknown.stdout" 2>"$EVIDENCE/unknown.stderr"
rc=$?
set -e
test "$rc" -eq 3
grep -F 'image.flavor: unknown to incus-os v0.0.0-20260815030500-0f5b8057f2fc; a newer incusos-builder may accept this' \
  "$EVIDENCE/unknown.stderr"

cat >"$WORK/config-matrix/quoted-secret.yaml" <<'EOF'
version: "DO-NOT-LEAK-THIS-LITERAL"
image:
  type: iso
  architecture: x86_64
EOF
set +e
"$IB" validate -f "$WORK/config-matrix/quoted-secret.yaml" \
  >"$EVIDENCE/redaction.stdout" 2>"$EVIDENCE/redaction.stderr"
rc=$?
set -e
test "$rc" -eq 3
grep -F '<value>' "$EVIDENCE/redaction.stderr"
! grep -F 'DO-NOT-LEAK-THIS-LITERAL' "$EVIDENCE/redaction.stderr"
```

**Expected observable result:** Exact dotted path and pin hint; quoted secret replaced by `<value>`.

**Evidence:** Input files and diagnostics.

**Pass/fail:** Any accepted unknown field or leaked literal fails.

#### C-03 — SOPS selection, file/stdin success, and exit-4 failures

**Promise and citation:** `docs/docs/how-to/sops-encryption.md`; `docs/docs/reference/configuration.md` — `SOPS`, `Errors`.

**Commands:**

```bash
cat >"$WORK/sops/plain.yaml" <<'EOF'
version: 1
image:
  type: iso
  architecture: x86_64
seeds:
  applications:
    applications:
      - name: incus
EOF
sops --age "$AGE_RECIPIENT" -e "$WORK/sops/plain.yaml" >"$WORK/sops/encrypted.yaml"
grep -F 'sops:' "$WORK/sops/encrypted.yaml"
grep -F 'ENC[' "$WORK/sops/encrypted.yaml"

"$IB" validate -f "$WORK/sops/encrypted.yaml" --color never \
  >"$EVIDENCE/sops-file.stdout"
grep -Fx 'configuration valid' "$EVIDENCE/sops-file.stdout"
"$IB" validate -f - --color never <"$WORK/sops/encrypted.yaml" \
  >"$EVIDENCE/sops-stdin.stdout"
grep -Fx 'configuration valid' "$EVIDENCE/sops-stdin.stdout"

for kind in no-key stray tampered; do
  case "$kind" in
    no-key) input="$WORK/sops/encrypted.yaml" ;;
    stray)
      cat >"$WORK/sops/stray.yaml" <<'EOF'
version: 1
image:
  type: iso
  architecture: x86_64
sops: {}
EOF
      input="$WORK/sops/stray.yaml" ;;
    tampered)
      cp "$WORK/sops/encrypted.yaml" "$WORK/sops/tampered.yaml"
      python3 - "$WORK/sops/tampered.yaml" <<'PY'
from pathlib import Path
p = Path(__import__('sys').argv[1])
s = p.read_text()
pos = s.find('ENC[')
assert pos >= 0
p.write_text(s[:pos+4] + ('A' if s[pos+4] != 'A' else 'B') + s[pos+5:])
PY
      input="$WORK/sops/tampered.yaml" ;;
  esac
  set +e
  env SOPS_AGE_KEY= "$IB" validate --json -f "$input" \
    >"$EVIDENCE/sops-$kind.stdout" 2>"$EVIDENCE/sops-$kind.stderr"
  rc=$?
  set -e
  test "$rc" -eq 4
  jq -e '.error.code == 4 and (.error.message | contains("decryption failed"))' \
    "$EVIDENCE/sops-$kind.stdout"
  grep -F 'decryption failed' "$EVIDENCE/sops-$kind.stderr"
done
```

For the tampered case, repeat once with the real `SOPS_AGE_KEY` to specifically exercise a MAC/ciphertext failure; it must still exit 4.

**Expected observable result:** Encrypted file and stdin validate at exit 0. Presence of `sops` alone selects decryption. Missing key, stray metadata, and tampering all exit 4 and never fall through to schema exit 3. No decrypted plaintext file appears.

**Evidence:** Encrypted input, redacted diagnostics, and directory listing before/after. Do not archive the private key.

**Pass/fail:** Every selection and error mapping must match.

#### C-04 — All eleven seed members, order, mode, and defaults

**Promise and citation:** `docs/docs/reference/configuration.md` — `Defaults`, `seeds`; `docs/docs/explanation/seed-injection.md` — `The splice invariant`.

**Commands:**

```bash
cat >"$WORK/all-sections.yaml" <<EOF
version: 1
image:
  type: raw
  architecture: x86_64
  release: "$INCUSOS_VERSION"
seeds:
  applications:
    applications:
      - name: incus
  incus:
    apply_defaults: true
  operations-center:
    apply_defaults: true
  migration-manager:
    apply_defaults: true
  install:
    force_reboot: true
    target:
      min_size: 50GiB
      sort_order: largest
  network:
    confirmation_timeout: 30s
  provider:
    name: images
    config:
      server: https://images.linuxcontainers.org/os
  services:
    tailscale:
      enabled: false
  update:
    auto_reboot: false
    check_frequency: 6h
  kernel:
    console:
      - device: /dev/ttyS0
        baud_rate: 115200
  security:
    custom_ca_certs:
      - dummy-ca
    encryption_recovery_keys: []
EOF
"$IB" validate -f "$WORK/all-sections.yaml" --color never
"$IB" build --json -f "$WORK/all-sections.yaml" -o "$WORK/all-sections.img" \
  --server "$MIRROR" --cache-dir "$CACHE" --color never --progress never \
  >"$EVIDENCE/all-sections-build.json" 2>"$EVIDENCE/all-sections-build.stderr"

mkdir -p "$WORK/seed-members"
python3 - "$WORK/all-sections.img" "$WORK/seed-members" <<'PY'
import pathlib, sys, tarfile
image, out = pathlib.Path(sys.argv[1]), pathlib.Path(sys.argv[2])
expected = [
 "applications.yaml", "incus.yaml", "operations-center.yaml",
 "migration-manager.yaml", "install.yaml", "network.yaml", "provider.yaml",
 "services.yaml", "update.yaml", "kernel.yaml", "security.yaml",
]
with image.open('rb') as f:
    f.seek(2148532224)
    with tarfile.open(fileobj=f, mode='r|') as tf:
        members = []
        for m in tf:
            members.append(m.name)
            assert m.mode == 0o600, (m.name, oct(m.mode))
            data = tf.extractfile(m).read()
            (out / m.name).write_bytes(data)
assert members == expected, members
print('\n'.join(members))
PY

for member in applications incus operations-center migration-manager install network provider services update kernel security; do
  grep -F 'version: "1"' "$WORK/seed-members/$member.yaml"
done
```

**Expected observable result:** Build JSON reports `seed_bytes > 0`; tar members are exactly the documented order; every header mode is 0600; every omitted section version rendered as `"1"`.

**Evidence:** Build envelope, member listing, and extracted YAML.

**Pass/fail:** Any missing/extra/reordered member, mode difference, or absent default fails.

#### C-05 — Seed tar size limit

**Promise and citation:** `docs/docs/reference/configuration.md` — `Seed-data size`, `Errors`; `docs/docs/reference/automation.md` — exit 3 includes oversized tar.

**Commands:**

```bash
python3 - "$WORK/oversized.yaml" "$INCUSOS_VERSION" <<'PY'
import sys
path, version = sys.argv[1:]
with open(path, 'w') as f:
    f.write(f'''version: 1
image:
  type: raw
  architecture: x86_64
  release: "{version}"
seeds:
  security:
    custom_ca_certs:
      - ''')
    f.write('A' * 105_000_000)
    f.write('\n')
PY
set +e
"$IB" build --json -f "$WORK/oversized.yaml" -o "$WORK/oversized.img" \
  --server "$MIRROR" --cache-dir "$CACHE" --color never --progress never \
  >"$EVIDENCE/oversized.stdout" 2>"$EVIDENCE/oversized.stderr"
rc=$?
set -e
test "$rc" -eq 3
jq -e '.error.code == 3 and
       (.error.message | test("seed tar is [0-9]+ bytes, seed-data partition holds 104857600"))' \
  "$EVIDENCE/oversized.stdout"
test ! -e "$WORK/oversized.img"
```

**Expected observable result:** Exit 3 with actual tar size and `104857600`; no final output.

**Evidence:** Envelope, stderr, and final-directory listing.

**Pass/fail:** Any output publication or other exit class fails.

### Suite D — acquisition, cache, mirror, and trust

#### D-01 — Live HTTPS, local mirror, server default and classification

**Promise and citation:** `README.md` — Quick start server default; `docs/docs/how-to/use-local-mirror.md` — steps 1–4; `docs/docs/reference/cli.md` — `versions`.

**Commands:**

```bash
"$IB" versions --json --architecture x86_64 --cache-dir "$WORK/live-cache" \
  >"$EVIDENCE/live-versions.json" 2>"$EVIDENCE/live-versions.stderr"
jq -e '(.result.versions | length) > 0' "$EVIDENCE/live-versions.json"

"$IB" versions --json --server "$MIRROR" --cache-dir "$CACHE" \
  --architecture x86_64 >"$EVIDENCE/mirror-versions.json"
jq -e --arg v "$INCUSOS_VERSION" 'any(.result.versions[]; .version == $v)' \
  "$EVIDENCE/mirror-versions.json"

mkdir -p "$WORK/relative-parent"
(
  cd "$WORK/relative-parent"
  "$IB" versions --json --server ../mirror --cache-dir ../relative-cache \
    --architecture x86_64
) >"$EVIDENCE/relative-mirror.json"
jq -e '(.result.versions | length) > 0' "$EVIDENCE/relative-mirror.json"
```

Plain HTTP and missing-path rejection are already asserted in B-05.

**Expected observable result:** Default HTTPS and absolute/relative existing directories list releases. A regular file passed as `--server` must also be rejected as neither HTTPS nor directory.

**Evidence:** Version envelopes and server-classification diagnostics.

**Pass/fail:** All source-selection paths must match.

#### D-02 — Default cache location and override on macOS and Linux

**Promise and citation:** `docs/docs/reference/cache.md` — `Location`, `Override`.

**Commands on macOS:**

```bash
mkdir -p "$WORK/mac-home"
HOME="$WORK/mac-home" "$IB" build --json -f "$WORK/online.yaml" \
  -o "$WORK/mac-default-cache.img" --server "$MIRROR" \
  --color never --progress never >"$EVIDENCE/mac-default-cache.json"
test -d "$WORK/mac-home/Library/Caches/incusos-builder/sha256"
test ! -e "$WORK/mac-home/.cache/incusos-builder"
```

**Commands on Linux:**

```bash
mkdir -p "$WORK/linux-home" "$WORK/linux-xdg"
HOME="$WORK/linux-home" XDG_CACHE_HOME="$WORK/linux-xdg" \
  "$IB" build --json -f "$WORK/online.yaml" \
  -o "$WORK/linux-default-cache.img" --server "$MIRROR" \
  --color never --progress never >"$EVIDENCE/linux-default-cache.json"
test -d "$WORK/linux-xdg/incusos-builder/sha256"
```

Override on either host:

```bash
rm -rf "$WORK/override-cache"
INCUSOS_BUILDER_CACHE_DIR=/definitely/wrong \
  "$IB" build --json -f "$WORK/online.yaml" -o "$WORK/cache-override.img" \
  --server "$MIRROR" --cache-dir "$WORK/override-cache" \
  --color never --progress never >"$EVIDENCE/cache-override.json"
test -d "$WORK/override-cache/sha256"
```

**Expected observable result:** macOS uses `~/Library/Caches`, ignoring XDG; Linux honors XDG; explicit flag beats environment.

**Evidence:** Envelopes and directory trees.

**Pass/fail:** Both host-specific locations and override must pass.

#### D-03 — Content-addressed layout and no-fetch reuse

**Promise and citation:** `docs/docs/reference/cache.md` — `Layout`, `Digest keys`, `Hit, miss, and success`.

**Commands:**

```bash
rm -rf "$WORK/reuse-cache"
cp "$WORK/online.yaml" "$WORK/reuse.yaml"
"$IB" build --json -f "$WORK/reuse.yaml" -o "$WORK/reuse-1.img" \
  --server "$MIRROR" --cache-dir "$WORK/reuse-cache" \
  --color never --progress never >"$EVIDENCE/reuse-1.json"

IMAGE_REL="$(jq -r --arg v "$INCUSOS_VERSION" '
  .updates[] | select(.version == $v) | .files[]
  | select(.architecture == "x86_64" and .type == "image-raw") | .filename
' "$MIRROR/index.json" | head -n1)"
IMAGE_DIGEST="$(jq -r --arg v "$INCUSOS_VERSION" --arg f "$IMAGE_REL" '
  .updates[] | select(.version == $v) | .files[]
  | select(.filename == $f) | .sha256
' "$MIRROR/index.json")"
CACHE_BLOB="$WORK/reuse-cache/sha256/$IMAGE_DIGEST"
test -f "$CACHE_BLOB"
python3 - "$CACHE_BLOB" <<'PY'
import os, pathlib, sys
p = pathlib.Path(sys.argv[1])
assert (p.stat().st_mode & 0o777) == 0o444, oct(p.stat().st_mode & 0o777)
assert p.name.isascii() and len(p.name) == 64 and p.name == p.name.lower()
PY

mv "$MIRROR/$INCUSOS_VERSION/$IMAGE_REL" "$MIRROR/$INCUSOS_VERSION/$IMAGE_REL.hidden"
"$IB" build --json -f "$WORK/reuse.yaml" -o "$WORK/reuse-2.img" \
  --server "$MIRROR" --cache-dir "$WORK/reuse-cache" \
  --color never --progress never >"$EVIDENCE/reuse-2.json"
mv "$MIRROR/$INCUSOS_VERSION/$IMAGE_REL.hidden" "$MIRROR/$INCUSOS_VERSION/$IMAGE_REL"

sha256sum "$WORK/reuse-1.img" "$WORK/reuse-2.img" | tee "$EVIDENCE/reuse-output-sha256.txt"
test "$(jq -r '.result.sha256' "$EVIDENCE/reuse-1.json")" = \
     "$(jq -r '.result.sha256' "$EVIDENCE/reuse-2.json")"
```

**Expected observable result:** Cache path is `<cache>/sha256/<metadata digest>`, mode 0444. Second build succeeds while source asset is absent and produces the same output digest, proving verified cache reuse and no fetch.

**Evidence:** Cache tree, modes, envelopes, output hashes.

**Pass/fail:** Any source open on the second build, mutable mode, or digest change fails.

#### D-04 — Truncated/tampered asset, corrupt cache, and admission cleanup

**Promise and citation:** `docs/docs/reference/cache.md` — `Hit, miss, and success`, `Implemented cleanup`; `docs/docs/how-to/use-local-mirror.md` — `Expect digest checks on every asset`.

**Commands:**

```bash
cp -a "$MIRROR" "$WORK/truncated-mirror"
TRUNC="$WORK/truncated-mirror/$INCUSOS_VERSION/$IMAGE_REL"
python3 - "$TRUNC" <<'PY'
import os, sys
p=sys.argv[1]
with open(p,'r+b') as f: f.truncate(os.path.getsize(p)-1)
PY
rm -rf "$WORK/truncated-cache"
set +e
"$IB" build --json -f "$WORK/online.yaml" -o "$WORK/truncated.img" \
  --server "$WORK/truncated-mirror" --cache-dir "$WORK/truncated-cache" \
  --color never --progress never \
  >"$EVIDENCE/truncated.stdout" 2>"$EVIDENCE/truncated.stderr"
rc=$?
set -e
test "$rc" -eq 5
grep -F 'asset failed size/digest admission; untrusted metadata; possible tampering' \
  "$EVIDENCE/truncated.stderr"
test -z "$(find "$WORK/truncated-cache" -name '.fetch-*' -print)"
test ! -e "$WORK/truncated.img"

chmod u+w "$CACHE_BLOB"
printf corrupt >"$CACHE_BLOB"
"$IB" build --json -f "$WORK/online.yaml" -o "$WORK/cache-repaired.img" \
  --server "$MIRROR" --cache-dir "$WORK/reuse-cache" \
  --color never --progress never >"$EVIDENCE/cache-repaired.json"
printf '%s  %s\n' "$IMAGE_DIGEST" "$CACHE_BLOB" | sha256sum --check --status
python3 - "$CACHE_BLOB" <<'PY'
import pathlib, sys
assert pathlib.Path(sys.argv[1]).stat().st_mode & 0o777 == 0o444
PY

chmod u+w "$CACHE_BLOB"
printf corrupt-again >"$CACHE_BLOB"
mv "$MIRROR/$INCUSOS_VERSION/$IMAGE_REL" "$MIRROR/$INCUSOS_VERSION/$IMAGE_REL.hidden"
set +e
"$IB" build -f "$WORK/online.yaml" -o "$WORK/cache-reject.img" \
  --server "$MIRROR" --cache-dir "$WORK/reuse-cache" \
  >"$EVIDENCE/cache-reject.stdout" 2>"$EVIDENCE/cache-reject.stderr"
rc=$?
set -e
mv "$MIRROR/$INCUSOS_VERSION/$IMAGE_REL.hidden" "$MIRROR/$INCUSOS_VERSION/$IMAGE_REL"
test "$rc" -eq 5
grep -F "open $IMAGE_REL" "$EVIDENCE/cache-reject.stderr"
```

**Expected observable result:** Truncation is exit 5 with exact admission wording and no `.fetch-*`/digest/final. A corrupt cached entry is never issued; with source present it is replaced by verified mode-0444 bytes, and with source absent the build fails acquisition.

**Evidence:** Diagnostics, cache listings, repaired digest/mode.

**Pass/fail:** Any use of corrupt/truncated bytes or leftover admission temp fails.

#### D-05 — Untrusted metadata field validation and three-way binding

**Promise and citation:** `docs/docs/reference/cache.md` — `Digest keys`; `docs/docs/how-to/use-local-mirror.md` — steps 1 and 5; `docs/docs/explanation/trust-model.md` — `Filename, size, and SHA-256`, `What update.sjson is`.

**Commands:** Create one mirror copy and fresh cache per mutation.

```bash
metadata_case() {
  local name=$1 jq_filter=$2 needle=$3
  local dir="$WORK/meta-$name"
  cp -a "$MIRROR" "$dir"
  jq --arg v "$INCUSOS_VERSION" "$jq_filter" "$dir/index.json" >"$dir/index.new"
  mv "$dir/index.new" "$dir/index.json"
  rm -rf "$WORK/cache-meta-$name"
  set +e
  "$IB" build --json -f "$WORK/online.yaml" -o "$WORK/meta-$name.img" \
    --server "$dir" --cache-dir "$WORK/cache-meta-$name" \
    --color never --progress never \
    >"$EVIDENCE/meta-$name.stdout" 2>"$EVIDENCE/meta-$name.stderr"
  local rc=$?
  set -e
  test "$rc" -eq 5
  grep -F "$needle" "$EVIDENCE/meta-$name.stderr"
  test ! -e "$WORK/meta-$name.img"
}

metadata_case filename \
  '(.updates[] | select(.version==$v) | .files[] | select(.architecture=="x86_64" and .type=="image-raw") | .filename) = "../evil"' \
  'untrusted metadata; possible tampering'
metadata_case digest \
  '(.updates[] | select(.version==$v) | .files[] | select(.architecture=="x86_64" and .type=="image-raw") | .sha256) = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"' \
  'untrusted metadata; possible tampering'
metadata_case zero-size \
  '(.updates[] | select(.version==$v) | .files[] | select(.architecture=="x86_64" and .type=="image-raw") | .size) = 0' \
  'untrusted metadata; possible tampering'
metadata_case over-8g \
  '(.updates[] | select(.version==$v) | .files[] | select(.architecture=="x86_64" and .type=="image-raw") | .size) = 8589934593' \
  'untrusted metadata; possible tampering'

cp -a "$MIRROR" "$WORK/meta-trailing"
printf '{}\n' >>"$WORK/meta-trailing/index.json"
set +e
"$IB" versions --json --server "$WORK/meta-trailing" --cache-dir "$WORK/cache-trailing" \
  >"$EVIDENCE/meta-trailing.stdout" 2>"$EVIDENCE/meta-trailing.stderr"
rc=$?
set -e
test "$rc" -eq 5
grep -F 'trailing data in index.json; untrusted metadata; possible tampering' \
  "$EVIDENCE/meta-trailing.stderr"

cp -a "$MIRROR" "$WORK/meta-version"
jq --arg v "$INCUSOS_VERSION" '(.updates[] | select(.version==$v) | .version)=".."' \
  "$WORK/meta-version/index.json" >"$WORK/meta-version/index.new"
mv "$WORK/meta-version/index.new" "$WORK/meta-version/index.json"
sed 's/release: ".*"/release: ".."/' "$WORK/online.yaml" >"$WORK/meta-version.yaml"
set +e
"$IB" build -f "$WORK/meta-version.yaml" -o "$WORK/meta-version.img" \
  --server "$WORK/meta-version" --cache-dir "$WORK/cache-meta-version" \
  >"$EVIDENCE/meta-version.stdout" 2>"$EVIDENCE/meta-version.stderr"
rc=$?
set -e
test "$rc" -eq 5
grep -F 'untrusted metadata; possible tampering' "$EVIDENCE/meta-version.stderr"
```

For three-way binding, replace `update.sjson` in an offline mirror with a structurally valid but unbound multipart message:

```bash
cp -a "$MIRROR" "$WORK/unbound-mirror"
cat >"$WORK/unbound-mirror/$INCUSOS_VERSION/update.sjson" <<EOF
MIME-Version: 1.0
Content-Type: multipart/signed; boundary="BOUNDARY"; protocol="application/x-pkcs7-signature"; micalg=sha-256

--BOUNDARY
Content-Type: application/json

{"version":"$INCUSOS_VERSION","files":[]}
--BOUNDARY
Content-Type: application/x-pkcs7-signature

not-authenticated-by-builder
--BOUNDARY--
EOF
set +e
"$IB" build --json -f "$WORK/offline.yaml" -o "$WORK/unbound.img" \
  --server "$WORK/unbound-mirror" --cache-dir "$WORK/unbound-cache" \
  --color never --progress never \
  >"$EVIDENCE/unbound.stdout" 2>"$EVIDENCE/unbound.stderr"
rc=$?
set -e
test "$rc" -eq 5
grep -F 'update.sjson missing selected file' "$EVIDENCE/unbound.stderr"
grep -F 'untrusted metadata; possible tampering' "$EVIDENCE/unbound.stderr"
```

**Expected observable result:** Invalid version/path/digest/size and trailing JSON all fail before trusted use with exit 5 and tamper wording. A multipart message that omits the selected filename+digest pair fails the three-way binding.

**Evidence:** Mutated indices/messages and diagnostics.

**Pass/fail:** Every adversarial input must be rejected with no final artifact.

#### D-06 — HTTPS redirect downgrade, size caps, tolerant plain decode, and S/MIME structure

**Promise and citation:** `docs/docs/explanation/trust-model.md` — HTTPS heading, JSON rationale, size-cap paragraph, and `What update.sjson is`.

**Redirect commands:**

```bash
mkdir -p "$WORK/https-fixture"
openssl req -x509 -newkey rsa:2048 -nodes \
  -keyout "$WORK/https-fixture/key.pem" -out "$WORK/https-fixture/cert.pem" \
  -days 1 -subj '/CN=localhost' -addext 'subjectAltName=DNS:localhost'
cat >"$WORK/https-fixture/downgrade.py" <<'PY'
import http.server, ssl, threading
class Redirect(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(302)
        self.send_header('Location', 'http://localhost:18080/index.json')
        self.end_headers()
    def log_message(self, fmt, *args): pass
class Plain(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        body=b'{"format":"1.0","updates":[]}'
        self.send_response(200); self.send_header('Content-Length',str(len(body)))
        self.end_headers(); self.wfile.write(body)
    def log_message(self, fmt, *args): pass
plain=http.server.ThreadingHTTPServer(('localhost',18080),Plain)
threading.Thread(target=plain.serve_forever,daemon=True).start()
tls=http.server.ThreadingHTTPServer(('localhost',18443),Redirect)
ctx=ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
ctx.load_cert_chain(__import__('sys').argv[1], __import__('sys').argv[2])
tls.socket=ctx.wrap_socket(tls.socket,server_side=True)
tls.serve_forever()
PY
python3 "$WORK/https-fixture/downgrade.py" \
  "$WORK/https-fixture/cert.pem" "$WORK/https-fixture/key.pem" \
  >"$EVIDENCE/downgrade-server.log" 2>&1 &
DOWNGRADE_PID=$!
sleep 1
set +e
SSL_CERT_FILE="$WORK/https-fixture/cert.pem" "$IB" versions --json \
  --server https://localhost:18443 --cache-dir "$WORK/downgrade-cache" \
  >"$EVIDENCE/downgrade.stdout" 2>"$EVIDENCE/downgrade.stderr"
rc=$?
set -e
kill "$DOWNGRADE_PID" 2>/dev/null || true
wait "$DOWNGRADE_PID" 2>/dev/null || true
test "$rc" -eq 5
grep -F 'redirected to non-https URL' "$EVIDENCE/downgrade.stderr"
```

**Cap and decode commands:**

```bash
cp -a "$MIRROR" "$WORK/cap-index"
python3 - "$WORK/cap-index/index.json" <<'PY'
import sys
with open(sys.argv[1], 'r+b') as f: f.truncate(67_108_865)
PY
set +e
"$IB" versions --server "$WORK/cap-index" --cache-dir "$WORK/cap-cache" \
  >"$EVIDENCE/cap-index.stdout" 2>"$EVIDENCE/cap-index.stderr"
rc=$?
set -e
test "$rc" -eq 5
grep -F 'exceeds 67108864-byte cap; untrusted metadata; possible tampering' \
  "$EVIDENCE/cap-index.stderr"

cp -a "$MIRROR" "$WORK/cap-meta"
python3 - "$WORK/cap-meta/$INCUSOS_VERSION/update.json" <<'PY'
import sys
with open(sys.argv[1], 'r+b') as f: f.truncate(1_048_577)
PY
set +e
"$IB" build -f "$WORK/offline.yaml" -o "$WORK/cap-meta.img" \
  --server "$WORK/cap-meta" --cache-dir "$WORK/cap-meta-cache" \
  >"$EVIDENCE/cap-meta.stdout" 2>"$EVIDENCE/cap-meta.stderr"
rc=$?
set -e
test "$rc" -eq 5
grep -F 'exceeds 1048576-byte cap; untrusted metadata; possible tampering' \
  "$EVIDENCE/cap-meta.stderr"

cp -a "$MIRROR" "$WORK/unknown-metadata"
jq '.future_index_field={"accepted":true} | .updates[0].future_update_field=42' \
  "$WORK/unknown-metadata/index.json" >"$WORK/unknown-metadata/index.new"
mv "$WORK/unknown-metadata/index.new" "$WORK/unknown-metadata/index.json"
jq '.future_metadata_field=true' \
  "$WORK/unknown-metadata/$INCUSOS_VERSION/update.json" \
  >"$WORK/unknown-metadata/$INCUSOS_VERSION/update.new"
mv "$WORK/unknown-metadata/$INCUSOS_VERSION/update.new" \
  "$WORK/unknown-metadata/$INCUSOS_VERSION/update.json"
"$IB" build --json -f "$WORK/offline.yaml" -o "$WORK/unknown-metadata.img" \
  --server "$WORK/unknown-metadata" --cache-dir "$WORK/unknown-metadata-cache" \
  --color never --progress never >"$EVIDENCE/unknown-metadata.json"
```

Replace `update.sjson` with `{}` and repeat the offline build with a fresh cache; expect exit 5 and `update.sjson is not multipart/signed; untrusted metadata; possible tampering`.

**Expected observable result:** The downgrade fails even though the initial URL is HTTPS and its test CA is trusted. Caps are exact. Unknown fields do not fail decoding. Non-multipart `update.sjson` fails structurally.

**Evidence:** Server script/log, mutated files, envelopes/diagnostics.

**Pass/fail:** All checks must match. This case proves structural acceptance/rejection, not PKCS#7 authentication.

#### D-07 — Low-free-space warning

**Promise and citation:** `docs/docs/reference/cache.md` — final paragraph of `Hit, miss, and success`.

**Preconditions:** macOS; `hdiutil`; valid large mirror asset.

**Commands:**

```bash
hdiutil create -quiet -size 64m -fs APFS -volname INCUSOS_LOWSPACE \
  "$WORK/lowspace.dmg"
LOWSPACE_MOUNT="$(hdiutil attach -nobrowse "$WORK/lowspace.dmg" | awk 'END {print $NF}')"
set +e
"$IB" build --json -f "$WORK/online.yaml" -o "$WORK/lowspace.img" \
  --server "$MIRROR" --cache-dir "$LOWSPACE_MOUNT/cache" \
  --color never --progress never \
  >"$EVIDENCE/lowspace.stdout" 2>"$EVIDENCE/lowspace.stderr"
rc=$?
set -e
test "$rc" -eq 5
grep -F 'warning: cache free space below asset size' "$EVIDENCE/lowspace.stderr"
test -z "$(find "$LOWSPACE_MOUNT/cache" -name '.fetch-*' -print 2>/dev/null)"
hdiutil detach "$LOWSPACE_MOUNT"
```

**Expected observable result:** The warning is emitted before the eventual no-space acquisition failure; low-space detection itself is not the error; admission temp is removed.

**Evidence:** stderr ordering and mounted cache listing.

**Pass/fail:** Missing warning or leftover temp fails. The expected later disk-full failure is not a product failure.

#### D-08 — Acquisition cancellation and cleanup

**Promise and citation:** `docs/docs/reference/automation.md` — exit 5 canceled fetch; `docs/docs/reference/cache.md` — aborted-admit cleanup.

**Commands:**

```bash
rm -rf "$WORK/cancel-cache" "$WORK/cancel.img"
set +e
"$IB" build --json -f "$WORK/online.yaml" -o "$WORK/cancel.img" \
  --cache-dir "$WORK/cancel-cache" --color never --progress always \
  >"$EVIDENCE/cancel.stdout" 2>"$EVIDENCE/cancel.stderr" &
pid=$!
for i in $(seq 1 100); do
  find "$WORK/cancel-cache" -name '.fetch-*' -print -quit 2>/dev/null | grep -q . && break
  sleep 0.1
done
kill -INT "$pid"
wait "$pid"
rc=$?
set -e
test "$rc" -eq 5
jq -e '.error.code == 5 and (.error.message | contains("context canceled"))' \
  "$EVIDENCE/cancel.stdout"
test ! -e "$WORK/cancel.img"
test -z "$(find "$WORK/cancel-cache" -name '.fetch-*' -print 2>/dev/null)"
```

**Expected observable result:** SIGINT during live acquisition maps to exit 5, JSON contains `acquisition failed: context canceled`, no final is published, and no admission temp remains.

**Evidence:** Envelope, stderr/progress, cache and output listing.

**Pass/fail:** Any other code or leftover final/temp fails.

#### D-09 — HTTPS retries and one clean re-download

**Promise and citation:** `docs/docs/reference/cache.md` — final paragraph of `Implemented cleanup`.

**Preconditions:** Trusted local TLS fixture from D-06, mirror asset available.

Create a small HTTPS fixture server that returns 503 twice for `index.json`, then succeeds, and returns a one-byte installer body once before the full body. The following server logs exact request counts:

```bash
cat >"$WORK/https-fixture/retry.py" <<'PY'
import http.server, pathlib, ssl, sys, urllib.parse
root=pathlib.Path(sys.argv[1]).resolve(); asset='/' + sys.argv[2]
counts={}
class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        p=urllib.parse.unquote(self.path.split('?',1)[0]); counts[p]=counts.get(p,0)+1
        print(f'COUNT {p} {counts[p]}', flush=True)
        if p=='/index.json' and counts[p] < 3:
            self.send_response(503); self.end_headers(); return
        file=(root / p.lstrip('/')).resolve()
        if root not in file.parents and file != root:
            self.send_response(404); self.end_headers(); return
        if p==asset and counts[p] == 1:
            body=file.read_bytes()[:1]
        else:
            body=file.read_bytes()
        self.send_response(200); self.send_header('Content-Length',str(len(body)))
        self.end_headers(); self.wfile.write(body)
    def log_message(self, fmt, *args): pass
srv=http.server.ThreadingHTTPServer(('localhost',18444),H)
ctx=ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER); ctx.load_cert_chain(sys.argv[3],sys.argv[4])
srv.socket=ctx.wrap_socket(srv.socket,server_side=True); srv.serve_forever()
PY
RETRY_ASSET="$INCUSOS_VERSION/$IMAGE_REL"
python3 "$WORK/https-fixture/retry.py" "$MIRROR" "$RETRY_ASSET" \
  "$WORK/https-fixture/cert.pem" "$WORK/https-fixture/key.pem" \
  >"$EVIDENCE/retry-server.log" 2>&1 &
RETRY_PID=$!
sleep 1
rm -rf "$WORK/retry-cache"
SSL_CERT_FILE="$WORK/https-fixture/cert.pem" "$IB" build --json \
  -f "$WORK/online.yaml" -o "$WORK/retry.img" \
  --server https://localhost:18444 --cache-dir "$WORK/retry-cache" \
  --color never --progress never >"$EVIDENCE/retry.json" 2>"$EVIDENCE/retry.stderr"
kill "$RETRY_PID" 2>/dev/null || true
wait "$RETRY_PID" 2>/dev/null || true
test "$(grep -c 'COUNT /index.json ' "$EVIDENCE/retry-server.log")" -eq 3
test "$(grep -c "COUNT /$RETRY_ASSET " "$EVIDENCE/retry-server.log")" -eq 2
jq -e '.result.sha256 | test("^[0-9a-f]{64}$")' "$EVIDENCE/retry.json"
```

**Expected observable result:** Two transient 503s are retried, the third index succeeds, the mismatched one-byte asset gets exactly one clean re-download, and the build succeeds from the second asset response.

**Evidence:** Server log, final envelope and hash.

**Pass/fail:** Different request counts, unverified success, or a leftover temp fails.

### Suite E — build outputs, splice, streaming, and publication

#### E-01 — ISO/raw and x86_64/aarch64 build matrix

**Promise and citation:** `README.md` — Quick start; `docs/docs/reference/cli.md` — `build`; `docs/docs/reference/configuration.md` — `image.type`, `image.architecture`.

**Commands:**

```bash
mkdir -p "$WORK/matrix"
for arch in x86_64 aarch64; do
  for typ in raw iso; do
    ext=img; [ "$typ" = iso ] && ext=iso
    cfg="$WORK/matrix/$typ-$arch.yaml"
    out="$WORK/matrix/$typ-$arch.$ext"
    cat >"$cfg" <<EOF
version: 1
image:
  type: $typ
  architecture: $arch
  release: "$INCUSOS_VERSION"
seeds:
  applications:
    applications:
      - name: incus
EOF
    "$IB" build --json -f "$cfg" -o "$out" \
      --server "$MIRROR" --cache-dir "$CACHE" --color never --progress never \
      >"$EVIDENCE/matrix-$typ-$arch.json" \
      2>"$EVIDENCE/matrix-$typ-$arch.stderr"
    jq -e --arg o "$out" --arg t "$typ" --arg a "$arch" --arg v "$INCUSOS_VERSION" '
      .result.output==$o and .result.type==$t and .result.architecture==$a and
      .result.version==$v and .result.channel=="stable" and
      .result.seed_bytes>0 and (.result.sha256|test("^[0-9a-f]{64}$")) and
      (.result|has("resources_output")|not)
    ' "$EVIDENCE/matrix-$typ-$arch.json"
    test "$(sha256sum "$out" | awk '{print $1}')" = \
         "$(jq -r '.result.sha256' "$EVIDENCE/matrix-$typ-$arch.json")"
  done
done
file "$WORK/matrix"/* | tee "$EVIDENCE/matrix-file-types.txt"
grep -F 'ISO 9660' "$EVIDENCE/matrix-file-types.txt"
```

Run one build without JSON to verify the human summary begins `summary` and includes `output`, `type`, `architecture`, `version`, `channel`, `seed_bytes`, and a lowercase 64-character `sha256`.

**Expected observable result:** Four successful files and matching stored-byte hashes; ISO files identify as ISO9660; raw files are disk images; online builds have no resources output.

**Evidence:** Four configs, envelopes, hashes, file output, human summary.

**Pass/fail:** All four combinations are mandatory.

#### E-02 — `.gz` stored-byte digest

**Promise and citation:** `docs/docs/reference/cli.md` — `.gz` paragraph under `build`; `docs/docs/reference/automation.md` — build `sha256`.

**Commands:**

```bash
"$IB" build --json -f "$WORK/matrix/raw-x86_64.yaml" \
  -o "$WORK/compressed.img.gz" --server "$MIRROR" --cache-dir "$CACHE" \
  --color never --progress never >"$EVIDENCE/compressed.json"
gzip --test "$WORK/compressed.img.gz"
test "$(sha256sum "$WORK/compressed.img.gz" | awk '{print $1}')" = \
     "$(jq -r '.result.sha256' "$EVIDENCE/compressed.json")"
gzip -dc "$WORK/compressed.img.gz" | sha256sum >"$EVIDENCE/compressed-unpacked.sha256"
sha256sum "$WORK/matrix/raw-x86_64.img" >"$EVIDENCE/plain.sha256"
test "$(awk '{print $1}' "$EVIDENCE/compressed-unpacked.sha256")" = \
     "$(awk '{print $1}' "$EVIDENCE/plain.sha256")"
```

**Expected observable result:** Envelope digest matches compressed stored bytes including footer; decompressed content matches the equivalent plain output.

**Evidence:** Envelope and three hashes.

**Pass/fail:** Both stored and unpacked comparisons must pass.

#### E-03 — `-f -`, `-o -`, and mid-stream output failure

**Promise and citation:** `docs/docs/reference/automation.md` — `-`, stdout/stderr table, mid-stream JSON rule; `docs/docs/reference/cli.md` — `build`.

**Commands:**

```bash
cat "$WORK/matrix/raw-x86_64.yaml" | "$IB" build -f - -o - \
  --server "$MIRROR" --cache-dir "$CACHE" --color never --progress never \
  >"$WORK/streamed.img" 2>"$EVIDENCE/streamed.stderr"
test ! -s "$EVIDENCE/streamed.stderr" || ! grep -F 'summary' "$EVIDENCE/streamed.stderr"
test "$(sha256sum "$WORK/streamed.img" | awk '{print $1}')" = \
     "$(sha256sum "$WORK/matrix/raw-x86_64.img" | awk '{print $1}')"
```

On Linux, provoke a write failure after streaming starts:

```bash
set +e
"$IB" build -f "$WORK/matrix/raw-x86_64.yaml" -o - \
  --server "$MIRROR" --cache-dir "$CACHE" --color never --progress never \
  >/dev/full 2>"$EVIDENCE/mid-stream.stderr"
rc=$?
set -e
test "$rc" -eq 6
grep -F 'output write failed' "$EVIDENCE/mid-stream.stderr"
```

**Expected observable result:** Streamed bytes alone occupy stdout and equal the file build. `/dev/full` maps write failure to exit 6; no JSON/error document is appended to artifact stdout.

**Evidence:** Stream hash and Linux diagnostic.

**Pass/fail:** Stream contamination, digest mismatch, or wrong failure code fails.

#### E-04 — Splice preserves prefix and suffix

**Promise and citation:** `docs/docs/explanation/seed-injection.md` — `The splice invariant`.

**Commands:**

```bash
SEED_BYTES="$(jq -r '.result.seed_bytes' "$EVIDENCE/matrix-raw-x86_64.json")"
python3 - "$MIRROR/$INCUSOS_VERSION/$IMAGE_REL" \
  "$WORK/matrix/raw-x86_64.img" "$SEED_BYTES" <<'PY' | tee "$EVIDENCE/splice-regions.txt"
import gzip, hashlib, sys
source, output, seed_len = sys.argv[1], sys.argv[2], int(sys.argv[3])
offset=2148532224

def readn(f,n):
    chunks=[]; left=n
    while left:
        b=f.read(min(left,4*1024*1024))
        assert b, f'short read with {left} bytes left'
        chunks.append(b); left-=len(b)
    return b''.join(chunks)
def hash_n(f,n):
    h=hashlib.sha256(); left=n
    while left:
        b=f.read(min(left,4*1024*1024)); assert b
        h.update(b); left-=len(b)
    return h.hexdigest()
def hash_rest(f):
    h=hashlib.sha256(); total=0
    for b in iter(lambda:f.read(4*1024*1024),b''):
        h.update(b); total+=len(b)
    return h.hexdigest(),total
with gzip.open(source,'rb') as a, open(output,'rb') as b:
    pa,pb=hash_n(a,offset),hash_n(b,offset); assert pa==pb
    sa,sb=readn(a,seed_len),readn(b,seed_len); assert sa!=sb
    ha,na=hash_rest(a); hb,nb=hash_rest(b); assert ha==hb and na==nb
print('prefix_sha256',pa)
print('seed_changed_bytes',seed_len)
print('suffix_sha256',ha)
print('suffix_bytes',na)
PY
```

**Expected observable result:** Prefix hashes match, overwritten seed region differs, suffix hashes and lengths match.

**Evidence:** Region hashes and tested source/output digests.

**Pass/fail:** Any difference outside the tar-sized seed prefix fails.

#### E-05 — Overwrite prompt, no-input refusal, force, and backup cleanup

**Promise and citation:** `docs/docs/reference/cli.md` — existing finals and `--force`; `docs/docs/how-to/build-offline-media.md` — `Overwrite behavior`; `docs/docs/reference/automation.md` — no-input.

**Preconditions:** Real TTY for prompt portions.

**Commands/actions:**

```bash
cp "$WORK/matrix/raw-x86_64.img" "$WORK/overwrite.img"
sha256sum "$WORK/overwrite.img" >"$EVIDENCE/overwrite-before.sha256"
```

Run without `--force` in the terminal:

```bash
env -u CI "$IB" build -f "$WORK/matrix/raw-x86_64.yaml" -o "$WORK/overwrite.img" \
  --server "$MIRROR" --cache-dir "$CACHE" --color never --progress never
```

Confirm stderr says `overwrite existing output? [y/N] `. Answer `n`; expect exit 2 and unchanged hash. Repeat and answer `YES`; expect success.

Then test noninteractive and force:

```bash
cp "$WORK/matrix/raw-x86_64.img" "$WORK/no-input-overwrite.img"
set +e
CI=1 "$IB" build -f "$WORK/matrix/raw-x86_64.yaml" \
  -o "$WORK/no-input-overwrite.img" --server "$MIRROR" --cache-dir "$CACHE" \
  >"$EVIDENCE/no-input-overwrite.stdout" 2>"$EVIDENCE/no-input-overwrite.stderr"
rc=$?
set -e
test "$rc" -eq 2
grep -F 'refusing to overwrite' "$EVIDENCE/no-input-overwrite.stderr"

printf 'old-generation' >"$WORK/force.img"
"$IB" build --json --force -f "$WORK/matrix/raw-x86_64.yaml" \
  -o "$WORK/force.img" --server "$MIRROR" --cache-dir "$CACHE" \
  --color never --progress never >"$EVIDENCE/force.json"
test ! -e "$WORK/force.img.incusos-builder.bak"
test "$(sha256sum "$WORK/force.img" | awk '{print $1}')" = \
     "$(jq -r '.result.sha256' "$EVIDENCE/force.json")"
```

**Expected observable result:** Prompt is on stderr and accepts only case-insensitive `y`/`yes`; refusal is exit 2 and unchanged. CI gives no prompt. Force replaces through backup path and removes its backup on success.

**Evidence:** TTY transcript, before/after hashes, diagnostics, directory listings.

**Pass/fail:** Prompt/stream/code/content/backup mismatch fails.

#### E-06 — No-clobber race and offline both-or-neither

**Promise and citation:** `docs/docs/reference/cli.md` — output-appeared and both-or-neither paragraphs; `docs/docs/how-to/build-offline-media.md` — overwrite race wording.

**Commands:**

```bash
rm -f "$WORK/race.img" "$WORK/race.resources.img"
rm -f "$WORK"/.race.img-*.tmp "$WORK"/.race.resources.img-*.tmp
set +e
"$IB" build --json -f "$WORK/offline.yaml" -o "$WORK/race.img" \
  --server "$MIRROR" --cache-dir "$CACHE" --color never --progress never \
  >"$EVIDENCE/race.stdout" 2>"$EVIDENCE/race.stderr" &
pid=$!
for i in $(seq 1 200); do
  find "$WORK" -name '.race.img-*.tmp' -print -quit | grep -q . && break
  sleep 0.05
done
printf 'racing-writer' >"$WORK/race.img"
wait "$pid"
rc=$?
set -e
test "$rc" -eq 6
grep -F 'output appeared during the build; re-run with --force' "$EVIDENCE/race.stderr"
grep -Fx 'racing-writer' "$WORK/race.img"
test ! -e "$WORK/race.resources.img"
test -z "$(find "$WORK" \( -name '.race.img-*.tmp' -o -name '.race.resources.img-*.tmp' \) -print)"
```

**Expected observable result:** Resources may be attempted first internally, but rollback leaves no resources final; the racing image remains untouched; exit 6 has exact wording; temps are removed. Thus the builder publishes both intended artifacts or neither.

**Evidence:** Envelope/error, racing file, final/temp listing.

**Pass/fail:** A resources final or partial replacement fails.

#### E-07 — Handled rollback and interrupted-build temp recovery

**Promise and citation:** `docs/docs/how-to/recover-interrupted-build.md` — `What --force does`, steps 1–6.

**Handled rollback commands:** Use separate final directories so a permission change can fail the second backup after image backup begins.

```bash
mkdir -p "$WORK/rollback-image" "$WORK/rollback-resources"
printf 'old-image' >"$WORK/rollback-image/out.img"
printf 'old-resources' >"$WORK/rollback-resources/out.img"
sha256sum "$WORK/rollback-image/out.img" "$WORK/rollback-resources/out.img" \
  >"$EVIDENCE/rollback-before.sha256"
set +e
"$IB" build --json --force -f "$WORK/offline.yaml" \
  -o "$WORK/rollback-image/out.img" \
  --resources-output "$WORK/rollback-resources/out.img" \
  --server "$MIRROR" --cache-dir "$CACHE" --color never --progress never \
  >"$EVIDENCE/rollback.stdout" 2>"$EVIDENCE/rollback.stderr" &
pid=$!
for i in $(seq 1 400); do
  tmp="$(find "$WORK/rollback-resources" -name '.out.img-*.tmp' -size +0c -print -quit)"
  [ -n "$tmp" ] && break
  sleep 0.05
done
chmod 0555 "$WORK/rollback-resources"
wait "$pid"
rc=$?
chmod 0755 "$WORK/rollback-resources"
set -e
test "$rc" -eq 6
sha256sum --check "$EVIDENCE/rollback-before.sha256"
grep -E 'restored image|leftover' "$EVIDENCE/rollback.stderr"
rm -f "$WORK/rollback-resources"/.out.img-*.tmp
```

If the filesystem schedules the permission failure before the image backup, repeat and capture that handled failure separately; a pass requires one run whose diagnostic records `restored image` and whose old pair hashes remain unchanged.

**Interrupted-build commands:**

```bash
rm -f "$WORK/interrupted.img" "$WORK"/.interrupted.img-*.tmp
set +e
"$IB" build --force -f "$WORK/matrix/raw-x86_64.yaml" \
  -o "$WORK/interrupted.img" --server "$MIRROR" --cache-dir "$CACHE" \
  --color never --progress never \
  >"$EVIDENCE/interrupted.stdout" 2>"$EVIDENCE/interrupted.stderr" &
pid=$!
for i in $(seq 1 200); do
  find "$WORK" -name '.interrupted.img-*.tmp' -print -quit | grep -q . && break
  sleep 0.05
done
kill -KILL "$pid"
wait "$pid"
rc=$?
set -e
test "$rc" -ne 0
find "$WORK" -name '.interrupted.img-*.tmp' -print | tee "$EVIDENCE/interrupted-temps.txt"
test -s "$EVIDENCE/interrupted-temps.txt"
rm -f $(cat "$EVIDENCE/interrupted-temps.txt")
test -z "$(find "$WORK" -name '.interrupted.img-*.tmp' -print)"
```

**Expected observable result:** Handled publication failure restores previous finals and reports rollback. SIGKILL cannot clean up and leaves a documented temp; after confirming the process is dead, the guide's inventory/delete procedure removes it. Never promote a temp to a final.

**Evidence:** Old hashes, failure/rollback text, PID/exit, temp inventory and cleanup listing.

**Pass/fail:** Old-final loss on handled failure or undocumented residual state fails.

### Suite F — offline media and rescue

#### F-01 — Raw installer plus GPT/FAT32 `RESCUE_DATA`

**Promise and citation:** `docs/docs/how-to/build-offline-media.md` — steps 1–4; `docs/docs/reference/automation.md` — offline build envelope.

**Commands:**

```bash
"$IB" build --json -f "$WORK/offline.yaml" -o "$WORK/offline-raw.img" \
  --server "$MIRROR" --cache-dir "$CACHE" --color never --progress never \
  >"$EVIDENCE/offline-raw.json" 2>"$EVIDENCE/offline-raw.stderr"
test -f "$WORK/offline-raw.img"
test -f "$WORK/offline-raw.resources.img"
jq -e --arg o "$WORK/offline-raw.img" --arg r "$WORK/offline-raw.resources.img" '
  .result.output==$o and .result.resources_output==$r and .result.type=="raw" and
  (.result.sha256|test("^[0-9a-f]{64}$")) and
  (.result.resources_sha256|test("^[0-9a-f]{64}$"))
' "$EVIDENCE/offline-raw.json"
test "$(sha256sum "$WORK/offline-raw.img" | awk '{print $1}')" = \
  "$(jq -r '.result.sha256' "$EVIDENCE/offline-raw.json")"
test "$(sha256sum "$WORK/offline-raw.resources.img" | awk '{print $1}')" = \
  "$(jq -r '.result.resources_sha256' "$EVIDENCE/offline-raw.json")"

sgdisk -i 1 "$WORK/offline-raw.resources.img" | tee "$EVIDENCE/raw-gpt.txt"
grep -F 'Partition name: ' "$EVIDENCE/raw-gpt.txt" | grep -F 'RESCUE_DATA'
mlabel -i "$WORK/offline-raw.resources.img@@1048576" -s :: \
  | tee "$EVIDENCE/raw-fat-label.txt"
grep -F 'RESCUE_DATA' "$EVIDENCE/raw-fat-label.txt"
mdir -b -s -i "$WORK/offline-raw.resources.img@@1048576" ::/ \
  | tee "$EVIDENCE/raw-rescue-tree.txt"
for path in update/update.json update/update.sjson "update/$INCUS_APP_REL"; do
  grep -F "$path" "$EVIDENCE/raw-rescue-tree.txt"
done
! grep -F 'hotfix.sh.sig' "$EVIDENCE/raw-rescue-tree.txt"
```

Extract the three files with `mcopy -i "$WORK/offline-raw.resources.img@@1048576"` and compare `update.json`/`update.sjson` byte-for-byte with the mirror and the application hash with the index.

**Expected observable result:** GPT partition 1 starts at 1 MiB, is Microsoft Basic Data, partition and FAT label are `RESCUE_DATA`, FAT32 media is at least 256 MiB, and the tree contains exactly the two metadata files and selected per-arch application path.

**Evidence:** Envelope, hashes, GPT/FAT output, tree and extracted-file comparisons.

**Pass/fail:** Every structural, path, and byte/digest assertion must pass.

#### F-02 — ISO installer plus ISO9660 `RESCUE_DATA`

**Promise and citation:** `docs/docs/how-to/build-offline-media.md` — `Choose ISO or FAT32 rescue media`.

**Commands:**

```bash
cat >"$WORK/offline-iso.yaml" <<EOF
version: 1
image:
  type: iso
  architecture: x86_64
  release: "$INCUSOS_VERSION"
  offline: true
seeds:
  applications:
    applications:
      - name: incus
EOF
"$IB" build --json -f "$WORK/offline-iso.yaml" -o "$WORK/offline.iso" \
  --resources-output "$WORK/rescue-data.iso" \
  --server "$MIRROR" --cache-dir "$CACHE" --color never --progress never \
  >"$EVIDENCE/offline-iso.json"
xorriso -indev "$WORK/rescue-data.iso" -pvd_info \
  >"$EVIDENCE/iso-pvd.txt" 2>&1
grep -F 'Volume Id' "$EVIDENCE/iso-pvd.txt" | grep -F 'RESCUE_DATA'
xorriso -indev "$WORK/rescue-data.iso" -find / -type f -print \
  >"$EVIDENCE/iso-rescue-tree.txt" 2>&1
for path in /update/update.json /update/update.sjson "/update/$INCUS_APP_REL"; do
  grep -F "$path" "$EVIDENCE/iso-rescue-tree.txt"
done
! grep -F 'hotfix.sh.sig' "$EVIDENCE/iso-rescue-tree.txt"
test "$(sha256sum "$WORK/rescue-data.iso" | awk '{print $1}')" = \
  "$(jq -r '.result.resources_sha256' "$EVIDENCE/offline-iso.json")"
```

Use `xorriso -osirrox on -extract` to extract and compare the same three files as F-01.

**Expected observable result:** Explicit resources path is honored; rescue media is ISO9660/Rock Ridge labeled `RESCUE_DATA`; exact tree and hashes match.

**Evidence:** Envelope, PVD/tree, extracted comparisons.

**Pass/fail:** Every assertion must pass.

#### F-03 — Offline defaults and forced `check_frequency: never`

**Promise and citation:** `docs/docs/reference/configuration.md` — `Defaults`, `image.offline`; `docs/docs/how-to/build-offline-media.md` — default resource naming.

**Commands:** Inspect the installer seed tar from F-01:

```bash
mkdir -p "$WORK/offline-seed"
python3 - "$WORK/offline-raw.img" "$WORK/offline-seed" <<'PY'
import pathlib,sys,tarfile
with open(sys.argv[1],'rb') as f:
 f.seek(2148532224)
 with tarfile.open(fileobj=f,mode='r|') as tf:
  for m in tf:
   pathlib.Path(sys.argv[2],m.name).write_bytes(tf.extractfile(m).read())
PY
grep -F 'check_frequency: never' "$WORK/offline-seed/update.yaml"
grep -F 'version: "1"' "$WORK/offline-seed/update.yaml"
test -f "$WORK/offline-raw.resources.img"
```

Now add `seeds.update.check_frequency: 6h`, rebuild to a fresh path, extract, and require it is still `never` and does not contain `6h`.

**Expected observable result:** Missing update seed is created; existing frequency is overridden; raw default path is `<stem>.resources.img`; ISO default is `<stem>.resources.iso`.

**Evidence:** Extracted `update.yaml` from both builds and directory listings.

**Pass/fail:** Any network-check frequency other than `never` fails.

#### F-04 — Offline usage boundaries

**Promise and citation:** `docs/docs/how-to/build-offline-media.md` — prerequisites and steps 1–2; `docs/docs/reference/cli.md` — build usage.

**Commands:** Reuse B-05 to assert offline requires applications, offline rejects `-o -`, resources path rejects `-`, online rejects resources, and cleaned paths must differ. Additionally verify a pinned absent release is exit 5:

```bash
sed "s/release: \"$INCUSOS_VERSION\"/release: \"199901010000\"/" \
  "$WORK/offline.yaml" >"$WORK/offline-missing-release.yaml"
set +e
"$IB" build --json -f "$WORK/offline-missing-release.yaml" \
  -o "$WORK/offline-missing.img" --server "$MIRROR" --cache-dir "$CACHE" \
  >"$EVIDENCE/offline-missing.stdout" 2>"$EVIDENCE/offline-missing.stderr"
rc=$?
set -e
test "$rc" -eq 5
grep -F 'version not found: release "199901010000" not in channel "stable"; available:' \
  "$EVIDENCE/offline-missing.stderr"
```

**Expected observable result:** Validation/usage failures happen before downloads and have their documented codes; missing release is acquisition/version exit 5.

**Evidence:** Cross-referenced B-05 records and missing-release streams.

**Pass/fail:** Every boundary must pass.

### Suite G — documentation as written and repository documents

These cases are deliberate documentation replays. Earlier cases may supply the files and cache, but do not replace running the commands shown here in a fresh directory.

#### G-01 — First seeded ISO tutorial and README quick start

**Promise and citation:** `README.md` — `Quick start`; `docs/docs/tutorials/first-seeded-iso.md` — steps 1–7.

**Commands:** Put the built binary on `PATH`, enter an empty directory, and run the tutorial commands in order:

```bash
mkdir -p "$WORK/doc-tutorial" "$WORK/bin"
ln -sf "$IB" "$WORK/bin/incusos-builder"
export PATH="$WORK/bin:$PATH"
cd "$WORK/doc-tutorial"
incusos-builder --version
incusos-builder init --no-input -o config.yaml
cat >config.yaml <<'EOF'
version: 1
image:
  type: iso
  architecture: x86_64
  channel: stable
seeds:
  applications:
    applications:
      - name: incus
EOF
incusos-builder validate -f config.yaml --color never
incusos-builder build -f config.yaml -o seeded.iso --color never
test -f seeded.iso
test ! -e seeded.resources.iso
```

**Expected observable result:** Exact version and init/validate messages; build human summary fields from the tutorial; only `seeded.iso` is published; no claim of boot consumption is made.

**Evidence:** Full terminal transcript, config, summary, hash, artifact listing.

**Pass/fail:** Any command/document mismatch fails. Successful build does not pass I-01.

#### G-02 — SOPS how-to replay

**Promise and citation:** `docs/docs/how-to/sops-encryption.md` — all steps and verification.

**Commands:** In `$WORK/doc-tutorial`, set the throwaway key and run exactly:

```bash
export SOPS_AGE_KEY="$(grep '^AGE-SECRET-KEY-' "$WORK/sops/age.key")"
unset SOPS_AGE_KEY_FILE SOPS_AGE_KEY_CMD
sops --age "$AGE_RECIPIENT" -e config.yaml > config.enc.yaml
incusos-builder validate -f config.enc.yaml --color never
incusos-builder validate -f - --color never < config.enc.yaml
incusos-builder build -f config.enc.yaml -o seeded-encrypted.iso --color never
incusos-builder build -f - -o seeded-encrypted-stdin.iso --color never < config.enc.yaml
set +e
env SOPS_AGE_KEY= incusos-builder validate -f config.enc.yaml
rc=$?
set -e
test "$rc" -eq 4
```

**Expected observable result:** Both validates print exact success; both builds succeed; cleared key is exit 4 with `decryption failed`.

**Evidence:** Transcript, encrypted file, output hashes, diagnostics; omit private key.

**Pass/fail:** Every guide command must work as stated.

#### G-03 — Offline-media how-to replay

**Promise and citation:** `docs/docs/how-to/build-offline-media.md` — all steps.

**Commands:** Use the guide's raw YAML, then run its two explicit build forms with absolute paths and `--json`; repeat with `type: iso`; run `sha256sum` on both outputs; inspect raw/ISO structures as in F-01/F-02. Run the guide's missing-apps, `-o -`, online-resources, overwrite refusal, and `--force` examples.

```bash
cd "$WORK/doc-tutorial"
cp "$WORK/offline.yaml" offline.yaml
INCUSOS_BUILDER_SERVER="$MIRROR" INCUSOS_BUILDER_CACHE_DIR="$CACHE" \
  incusos-builder build --json -f offline.yaml -o "$WORK/doc-tutorial/seeded.img"
INCUSOS_BUILDER_SERVER="$MIRROR" INCUSOS_BUILDER_CACHE_DIR="$CACHE" \
  incusos-builder build --json --force -f offline.yaml \
  -o "$WORK/doc-tutorial/seeded.img" \
  --resources-output "$WORK/doc-tutorial/rescue-data.img"
sha256sum -- "$WORK/doc-tutorial/seeded.img" "$WORK/doc-tutorial/rescue-data.img"
```

**Expected observable result:** All stated paths, envelopes, formats, labels, overwrite messages, and hashes match the guide.

**Evidence:** Complete transcript and artifact inspection.

**Pass/fail:** Pass only after both ISO and raw variants and negative examples pass.

#### G-04 — CI how-to replay

**Promise and citation:** `docs/docs/how-to/run-in-ci.md` — steps 1–7 and verification.

**Commands:** In a non-TTY shell or CI job:

```bash
cd "$WORK/doc-tutorial"
cp config.yaml seed.yaml
incusos-builder validate --json -f seed.yaml --color never
INCUSOS_BUILDER_SERVER="$MIRROR" incusos-builder build --json \
  -f seed.yaml \
  -o "$WORK/doc-tutorial/ci-seeded.img" \
  --cache-dir "$WORK/doc-tutorial/ci-cache" \
  --color never \
  --progress never
incusos-builder validate --json -f - --color never < seed.yaml
INCUSOS_BUILDER_SERVER="$MIRROR" incusos-builder build --json \
  -f seed.yaml -o "$WORK/doc-tutorial/ci-seeded.img" --force \
  --cache-dir "$WORK/doc-tutorial/ci-cache" --color never --progress never
```

**Expected observable result:** One JSON line on stdout for each command; progress/diagnostics only on stderr; no prompt; published hash matches envelope; exit branching matches B-09.

**Evidence:** CI job log with stdout and stderr separately preserved.

**Pass/fail:** Any hang, extra stdout, or hash/code mismatch fails.

#### G-05 — Local-mirror how-to replay

**Promise and citation:** `docs/docs/how-to/use-local-mirror.md` — steps 1–5.

**Commands:** Run the guide's two positive commands against `$MIRROR`, its `versions --json`, the plain-HTTP example, missing-path example, unknown-channel example, and absent release build. Use the exact directory tree and validation records from mirror setup.

```bash
incusos-builder versions --server "$MIRROR" --cache-dir "$CACHE" --architecture x86_64
incusos-builder build --json -f "$WORK/online.yaml" \
  -o "$WORK/doc-tutorial/mirror-seeded.img" --server "$MIRROR" --cache-dir "$CACHE"
incusos-builder versions --json --server "$MIRROR" --cache-dir "$CACHE" \
  --architecture x86_64
```

**Expected observable result:** Guide output and all adversarial wording match D-01/D-04/D-05.

**Evidence:** Transcript, tree, envelope, negative diagnostics.

**Pass/fail:** Every guide command and stated outcome must match.

#### G-06 — Interrupted-build recovery how-to replay

**Promise and citation:** `docs/docs/how-to/recover-interrupted-build.md` — steps 1–7 and `Verification`.

**Preconditions:** Use E-07's killed process and work only after confirming its PID is gone. Run this on Linux so the guide's `sha256sum --` syntax is native.

**Commands:** Set the actual paths and run the guide's inventory loop verbatim:

```bash
IMAGE="$WORK/interrupted.img"
RESOURCES="$WORK/interrupted.resources.img"
for path in "$IMAGE" "$IMAGE.incusos-builder.bak" \
  "$RESOURCES" "$RESOURCES.incusos-builder.bak"; do
  if [ -e "$path" ]; then
    ls -l -- "$path"
    sha256sum -- "$path"
  fi
done
```

Exercise the documented restore condition with a test backup whose final is absent, record its hash, then run:

```bash
printf 'previous-generation' >"$WORK/recovery.img.incusos-builder.bak"
IMAGE="$WORK/recovery.img"
sha256sum -- "$IMAGE.incusos-builder.bak" >"$EVIDENCE/recovery-bak.sha256"
if [ -e "$IMAGE.incusos-builder.bak" ]; then
  if [ ! -e "$IMAGE" ]; then
    mv -- "$IMAGE.incusos-builder.bak" "$IMAGE"
  elif [ ! -s "$IMAGE" ]; then
    rm -- "$IMAGE"
    mv -- "$IMAGE.incusos-builder.bak" "$IMAGE"
  fi
fi
test -e "$IMAGE"
test ! -e "$IMAGE.incusos-builder.bak"
test "$(sha256sum "$IMAGE" | awk '{print $1}')" = \
     "$(awk '{print $1}' "$EVIDENCE/recovery-bak.sha256")"
```

Also create a non-empty final plus `.bak`, run the same conditional, and confirm it does not overwrite the non-empty final. Delete only inventoried temps after the restore decision, then re-run a fresh build.

**Expected observable result:** Recovery decisions follow file state, not assumed exit code; absent/zero-byte finals can be restored; non-empty finals are not overwritten; temps are never promoted; final hash matches recorded backup.

**Evidence:** Before/after inventories and hashes.

**Pass/fail:** Any unsafe overwrite or mismatch fails.

#### G-07 — Docs site, licenses, security policy, contributor workflow, and status accuracy

**Promise and citation:** `README.md` — Documentation/License/Status; `SECURITY.md`; `CONTRIBUTING.md`; `docs/mkdocs.yml`; `.github/workflows/docs-pages.yml`.

**Commands:**

```bash
cd "$REPO"
mise x -- moon run docs:build 2>&1 | tee "$EVIDENCE/docs-build.txt"
test -f docs/build/index.html
for page in \
  tutorials/first-seeded-iso \
  how-to/sops-encryption how-to/build-offline-media how-to/run-in-ci \
  how-to/use-local-mirror how-to/recover-interrupted-build \
  how-to/verify-boot-acceptance \
  reference/cli reference/automation reference/configuration reference/cache \
  explanation/trust-model explanation/seed-injection explanation/upstream-version-coupling; do
  test -f "docs/build/$page/index.html"
done

grep -F 'Apache License' LICENSE-APACHE
grep -F 'MIT License' LICENSE-MIT
grep -F 'Apache-2.0 OR MIT' README.md
grep -F 'only the latest published release' SECURITY.md
grep -F 'private vulnerability reporting' SECURITY.md
grep -F 'moon run root:check' CONTRIBUTING.md
```

Check Pages and the public content:

```bash
PAGES_RUN="$(gh run list --repo componere/incusos-builder --workflow 'GitHub Pages' \
  --branch master --limit 1 --json databaseId,conclusion --jq '.[0].databaseId')"
gh run view "$PAGES_RUN" --repo componere/incusos-builder --exit-status
curl --fail --location https://componere.github.io/incusos-builder/ \
  -o "$EVIDENCE/pages-index.html"
grep -F 'incusos-builder' "$EVIDENCE/pages-index.html"
```

In a browser, open the repository Security tab and the private-report URL from `SECURITY.md`; verify an eligible external reporter sees the private advisory form rather than a 404/disabled message.

Finally, repeat `mise install` and `mise x -- moon run root:check` from the fresh A-01 clone. Before tagging, README/SECURITY must accurately say no release exists. Before publishing the first release, update/review those statements so the published tag does not falsely claim that no release exists; after publication, verify the tag's rendered README and SECURITY page state the latest-release policy accurately.

**Expected observable result:** Strict docs build and Pages deployment succeed; every nav page exists; both complete license texts exist; private reporting works; a fresh contributor can execute the documented gate with the pinned lint; status text matches reality at each release phase.

**Evidence:** Build output, Pages run/HTML, browser screenshot, fresh-clone gate log, status review.

**Pass/fail:** Broken docs/links, disabled reporting, missing licenses, stale published status, or contributor setup failure blocks release.

### Suite H — release pipeline and supply chain

#### H-01 — Apply and verify repository settings

**Promise and citation:** `.github/repository-settings.toml` — supported settings and rulesets; `SECURITY.md` — private reporting.

**Preconditions:** Repository admin token available to the script; this configuration is known to be unapplied at the start.

**Commands:**

```bash
cd "$REPO"
mise x -- uv run .github/scripts/configure_github_repo.py plan \
  --repo componere/incusos-builder \
  --json-report "$EVIDENCE/repository-plan-before.json" \
  | tee "$EVIDENCE/repository-plan-before.txt"
```

Review every planned supported change. With maintainer approval, apply the committed desired state:

```bash
mise x -- uv run .github/scripts/configure_github_repo.py apply \
  --repo componere/incusos-builder \
  --json-report "$EVIDENCE/repository-apply.json" \
  | tee "$EVIDENCE/repository-apply.txt"
mise x -- uv run .github/scripts/configure_github_repo.py plan \
  --repo componere/incusos-builder \
  --json-report "$EVIDENCE/repository-plan-after.json" \
  | tee "$EVIDENCE/repository-plan-after.txt"
jq -e '.changes == []' "$EVIDENCE/repository-plan-after.json"
```

In GitHub UI, confirm unsupported/manual settings explicitly or record why they cannot be applied. Confirm immutable releases, private vulnerability reporting, Pages HTTPS/workflow build, default branch, squash-only merge, active branch/tag rulesets, and required check contexts.

**Expected observable result:** The initial plan exposes known drift; the final plan says `No supported changes are required.` Unsupported entries remain clearly listed as manual follow-ups.

**Evidence:** Before/apply/after JSON, UI screenshots, ruleset IDs.

**Pass/fail:** Unresolved supported drift in release/security settings blocks tagging. Unsupported settings are a recorded limitation unless another public promise depends on them.

#### H-02 — Release Please PR shape and tag boundary

**Promise and citation:** `CONTRIBUTING.md` — `Commits and releases`; `release-please-config.json`; `.github/workflows/release-please.yml`.

**Preconditions:** All intended commits are on `master`; Release App configuration verified; do not merge yet.

**Commands:**

```bash
gh workflow run release-please.yml --repo componere/incusos-builder --ref master
sleep 5
RP_RUN="$(gh run list --repo componere/incusos-builder --workflow 'Release Please' \
  --event workflow_dispatch --branch master --limit 1 --json databaseId --jq '.[0].databaseId')"
gh run watch "$RP_RUN" --repo componere/incusos-builder --compact --exit-status
RP_PR="$(gh pr list --repo componere/incusos-builder --state open \
  --search 'head:release-please--' --json number --jq '.[0].number')"
test -n "$RP_PR"
gh pr view "$RP_PR" --repo componere/incusos-builder \
  --json number,title,headRefName,baseRefName,isDraft,url,statusCheckRollup \
  >"$EVIDENCE/release-pr.json"
gh pr diff "$RP_PR" --repo componere/incusos-builder \
  >"$EVIDENCE/release-pr.diff"
grep -F 'CHANGELOG.md' "$EVIDENCE/release-pr.diff"
grep -F 'melange.yaml' "$EVIDENCE/release-pr.diff"
grep -F 'apko.yaml' "$EVIDENCE/release-pr.diff"
```

Inspect that the title is a Release Please release subject, base is `master`, changelog contains only visible configured sections (`Features`, `Bug Fixes`, `Performance` as applicable), `.release-please-manifest.json`, `melange.yaml`, and `apko.yaml` agree on the version, and the resulting tag will be `vX.Y.Z`.

**Expected observable result:** Workflow succeeds and one coherent release PR exists. It does not publish anything before merge.

**Evidence:** Run URL, PR JSON/diff, resolved proposed tag/version.

**Pass/fail:** Missing/incorrect changelog or version marker, multiple release PRs, or premature publication fails.

#### H-03 — Non-publishing release rehearsal

**Promise and citation:** `.github/workflows/release-dry-run.yml` — header and all jobs.

**Commands:**

```bash
gh workflow run release-dry-run.yml --repo componere/incusos-builder --ref master
sleep 5
DRY_RUN="$(gh run list --repo componere/incusos-builder --workflow 'Release Dry Run' \
  --event workflow_dispatch --branch master --limit 1 --json databaseId --jq '.[0].databaseId')"
gh run watch "$DRY_RUN" --repo componere/incusos-builder --compact --exit-status
gh run view "$DRY_RUN" --repo componere/incusos-builder --verbose \
  | tee "$EVIDENCE/release-dry-run.txt"
```

In the UI/logs, require success for Binary Release Dry Run, both Melange matrix arches, and Container Image Dry Run. Confirm staging reported exactly nine assets, both `linux/amd64` and `linux/arm64`, image user `65532`, correct synthetic version, and nonempty generated SBOMs. Confirm no GitHub release asset, GHCR dry-run tag, cosign signature, or attestation was published.

**Expected observable result:** All rehearsal jobs succeed using the real release commands minus publication.

**Evidence:** Run URL, job/log archive, absence checks in Releases/GHCR.

**Pass/fail:** Any failed/skipped required dispatch job or accidental publication blocks tagging.

#### H-04 — Tag-triggered draft release and exact native assets

**Promise and citation:** `.github/workflows/release.yml`; `.goreleaser.yaml`; `ghd.toml`; staging script expected asset count.

**Preconditions:** Every pre-tag gate including I-01 passed. A maintainer reviews and merges the Release Please PR; do not create a tag manually.

**Actions/commands:** Record the merge SHA and generated tag. Wait for Release workflow:

```bash
export TAG=vX.Y.Z
export VERSION="${TAG#v}"
RELEASE_RUN="$(gh run list --repo componere/incusos-builder --workflow Release \
  --branch "$TAG" --limit 1 --json databaseId --jq '.[0].databaseId')"
gh run watch "$RELEASE_RUN" --repo componere/incusos-builder --compact --exit-status
gh release view "$TAG" --repo componere/incusos-builder \
  --json isDraft,tagName,targetCommitish,assets,url >"$EVIDENCE/draft-release.json"
jq -e '.isDraft == true' "$EVIDENCE/draft-release.json"

mkdir -p "$WORK/release-assets"
gh release download "$TAG" --repo componere/incusos-builder \
  --dir "$WORK/release-assets"
find "$WORK/release-assets" -maxdepth 1 -type f -print | sort \
  >"$EVIDENCE/release-assets.txt"
test "$(wc -l < "$EVIDENCE/release-assets.txt" | tr -d ' ')" -eq 9
for platform in darwin_amd64 darwin_arm64 linux_amd64 linux_arm64; do
  test -x "$WORK/release-assets/incusos-builder_${VERSION}_${platform}"
  test -f "$WORK/release-assets/incusos-builder_${VERSION}_${platform}.sbom.json"
  jq -e '.packages | length > 0' \
    "$WORK/release-assets/incusos-builder_${VERSION}_${platform}.sbom.json"
done
(
  cd "$WORK/release-assets"
  sha256sum --check checksums.txt
)
file "$WORK/release-assets"/incusos-builder_*_*_* \
  | tee "$EVIDENCE/release-binary-types.txt"

HOST_BIN="$WORK/release-assets/incusos-builder_${VERSION}_darwin_arm64"
"$HOST_BIN" --version | tee "$EVIDENCE/release-host-version.txt"
grep -E "^incusos-builder ${VERSION} \([0-9a-f]{40}\) built .+" \
  "$EVIDENCE/release-host-version.txt"
grep -Fx 'incus-os API: v0.0.0-20260815030500-0f5b8057f2fc' \
  "$EVIDENCE/release-host-version.txt"
```

Execute Linux amd64 on the Linux host. Execute Darwin amd64 under Rosetta if available. Inspect Linux arm64 as ELF arm64 here; execute it on an arm64 Linux/Docker environment if available.

**Expected observable result:** Release workflow is entirely green; release remains draft; exactly four executable binaries, four nonempty SBOM JSON files, and `checksums.txt`; names exactly match `ghd.toml`; all checksums pass; stamped version/commit/date and pin are correct.

**Evidence:** Release/run URLs, asset list, checksums, file types, version outputs.

**Pass/fail:** Any missing/extra/misnamed asset, bad checksum/SBOM, wrong architecture, wrong metadata, or non-draft release fails.

#### H-05 — Public `ghd install` and `ghd download`

**Promise and citation:** `README.md` — `From a GitHub release`; `ghd.toml`.

**Preconditions:** H-04/H-06/H-07 passed against the draft; maintainer publishes the draft; test without a privileged GitHub token where possible.

**Commands:**

```bash
rm -rf "$WORK/ghd-store" "$WORK/ghd-bin" "$WORK/ghd-download"
mkdir -p "$WORK/ghd-download"
ghd install componere/incusos-builder/incusos-builder@"$VERSION" \
  --store-dir "$WORK/ghd-store" --bin-dir "$WORK/ghd-bin"
"$WORK/ghd-bin/incusos-builder" --version \
  | tee "$EVIDENCE/ghd-installed-version.txt"
ghd download componere/incusos-builder/incusos-builder@"$VERSION" \
  --output "$WORK/ghd-download"
find "$WORK/ghd-download" -maxdepth 2 -type f -print \
  | tee "$EVIDENCE/ghd-download-files.txt"
```

Repeat unpinned latest install in a clean store:

```bash
rm -rf "$WORK/ghd-latest-store" "$WORK/ghd-latest-bin"
ghd install componere/incusos-builder/incusos-builder \
  --store-dir "$WORK/ghd-latest-store" --bin-dir "$WORK/ghd-latest-bin"
"$WORK/ghd-latest-bin/incusos-builder" --version
```

**Expected observable result:** `ghd` selects the host asset, verifies configured provenance, installs an executable, and both pinned and latest print the released version/two-line pin. Download produces the correct host asset without installation.

**Evidence:** Complete `ghd` logs, selected asset, installed/downloaded hashes and version.

**Pass/fail:** This is mandatory after first publication. A failure makes the published release a no-go and must be treated as a release incident, not waived.

#### H-06 — GHCR multi-arch, version, and nonroot execution

**Promise and citation:** `README.md` — `Container image`; `apko.yaml` — accounts/archs; `.github/workflows/release.yml` — image verification.

**Commands:** Resolve the immutable digest:

```bash
export IMAGE=ghcr.io/componere/incusos-builder
export IMAGE_DIGEST="$(docker buildx imagetools inspect "$IMAGE:$TAG" \
  --format '{{json .}}' | jq -r '.manifest.digest // .name' | sed -n 's/.*\(sha256:[0-9a-f]\{64\}\).*/\1/p')"
test -n "$IMAGE_DIGEST"
docker buildx imagetools inspect "$IMAGE@$IMAGE_DIGEST" \
  >"$EVIDENCE/image-manifest.txt"
grep -F 'linux/amd64' "$EVIDENCE/image-manifest.txt"
grep -F 'linux/arm64' "$EVIDENCE/image-manifest.txt"
```

On macOS arm64:

```bash
docker pull --platform linux/arm64 "$IMAGE@$IMAGE_DIGEST"
docker run --rm --platform linux/arm64 "$IMAGE@$IMAGE_DIGEST" --version \
  | tee "$EVIDENCE/image-arm64-version.txt"
test "$(docker image inspect "$IMAGE@$IMAGE_DIGEST" --format '{{.Config.User}}')" = 65532
```

On Linux amd64:

```bash
docker pull --platform linux/amd64 "$IMAGE@$IMAGE_DIGEST"
docker run --rm --platform linux/amd64 "$IMAGE@$IMAGE_DIGEST" --version \
  | tee "$EVIDENCE/image-amd64-version.txt"
test "$(docker image inspect "$IMAGE@$IMAGE_DIGEST" --format '{{.Config.User}}')" = 65532
```

**Expected observable result:** Manifest has exactly the two required Linux platforms; both execute natively, print the released two-line version, and OCI config user is `65532`.

**Evidence:** Digest, manifest, pull/run logs, user inspections.

**Pass/fail:** Missing platform, emulation-only success on the designated native host, root user, or wrong version fails.

#### H-07 — SBOM, cosign signature, and GitHub attestations

**Promise and citation:** `.github/workflows/release.yml` — sign/SBOM/attestation steps; `.github/workflows/attest.yml`; `ghd.toml` — provenance signer.

**Commands:** Binary provenance:

```bash
ASSET="$WORK/release-assets/incusos-builder_${VERSION}_darwin_arm64"
gh attestation verify "$ASSET" \
  --repo componere/incusos-builder \
  --signer-workflow componere/incusos-builder/.github/workflows/attest.yml \
  --source-ref "refs/tags/$TAG" --deny-self-hosted-runners \
  --format json >"$EVIDENCE/binary-attestation.json"
jq -e 'length > 0 and all(.[].verificationResult.statement.predicateType;
  . == "https://slsa.dev/provenance/v1")' "$EVIDENCE/binary-attestation.json"
```

Image provenance and cosign:

```bash
gh attestation verify "oci://$IMAGE@$IMAGE_DIGEST" \
  --repo componere/incusos-builder \
  --signer-workflow componere/incusos-builder/.github/workflows/attest.yml \
  --source-ref "refs/tags/$TAG" --deny-self-hosted-runners \
  --format json >"$EVIDENCE/image-provenance.json"

cosign verify "$IMAGE@$IMAGE_DIGEST" \
  --certificate-identity-regexp '^https://github.com/componere/incusos-builder/.github/workflows/release.yml@.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  | tee "$EVIDENCE/cosign-verify.txt"
```

Image SBOM attestation (created in `release.yml`, not the isolated provenance workflow):

```bash
gh attestation verify "oci://$IMAGE@$IMAGE_DIGEST" \
  --repo componere/incusos-builder \
  --predicate-type https://spdx.dev/Document/v2.3 \
  --signer-workflow componere/incusos-builder/.github/workflows/release.yml \
  --source-ref "refs/tags/$TAG" --deny-self-hosted-runners \
  --format json >"$EVIDENCE/image-sbom-attestation.json"
jq -e 'length > 0 and
  any(.[].verificationResult.statement;
      .predicateType == "https://spdx.dev/Document/v2.3" and
      (.predicate.packages | length > 0))' "$EVIDENCE/image-sbom-attestation.json"

mkdir -p "$WORK/attestation-download" && cd "$WORK/attestation-download"
gh attestation download "oci://$IMAGE@$IMAGE_DIGEST" \
  --repo componere/incusos-builder
ls -l | tee "$EVIDENCE/downloaded-attestations.txt"
```

Use `cosign tree "$IMAGE@$IMAGE_DIGEST"` and record registry referrers. Confirm a signature, provenance attestation, and SBOM-related attestation/referrer exist.

**Expected observable result:** Every verification is cryptographically valid, subject digest equals the immutable image or local binary, source ref is the tag, provenance signer is `attest.yml`, SBOM signer is `release.yml`, and binary/image SBOMs contain packages.

**Evidence:** Verification JSON, cosign output, referrer tree, downloaded bundles, asset SBOMs.

**Pass/fail:** Missing or identity-mismatched signature/attestation/SBOM fails.

#### H-08 — Container vulnerability scan workflow

**Promise and citation:** `.github/workflows/security-scan.yml` — build and scan jobs.

**Commands:**

```bash
gh workflow run security-scan.yml --repo componere/incusos-builder --ref master
sleep 5
SCAN_RUN="$(gh run list --repo componere/incusos-builder --workflow 'Security Scan' \
  --event workflow_dispatch --branch master --limit 1 --json databaseId --jq '.[0].databaseId')"
gh run watch "$SCAN_RUN" --repo componere/incusos-builder --compact --exit-status
gh run view "$SCAN_RUN" --repo componere/incusos-builder --verbose \
  | tee "$EVIDENCE/security-scan.txt"
```

**Expected observable result:** Local melange/apko image builds; Trivy finds no unfixed HIGH/CRITICAL OS or library vulnerability; SARIF uploads even on scan failure.

**Evidence:** Run URL/log and Security tab result.

**Pass/fail:** A scan finding or build/upload failure blocks release pending triage.

### Suite I — real boot acceptance

#### I-01 — x86_64 install, seed wipe, `RESCUE_DATA`, and signed recovery effect

**Promise and citation:** `docs/docs/how-to/verify-boot-acceptance.md` — entire guide; `docs/notes/phase-5-boot-probe.md` — negative prior result.

**Preconditions:** The required x86_64 Linux Incus host, KVM, `default` pool, `incusbr0`, SPICE, sudo, VirtIO-SCSI, and 50 GiB target are available. The release candidate binary and its built raw artifacts are unchanged. Do not run this after tagging; it is a pre-tag gate.

**Commands/actions:** Execute the guide from step 1 through step 8 without substituting a QEMU-only proxy. The essential setup is:

```bash
cat >release-gate.yaml <<'EOF'
version: 1
image:
  type: raw
  architecture: x86_64
  offline: true
seeds:
  applications:
    applications:
      - name: incus
  install:
    target:
      min_size: 50GiB
EOF
incusos-builder build --json -f release-gate.yaml \
  -o /absolute/path/seeded-x86_64.raw \
  --resources-output /absolute/path/rescue-data.raw \
  | tee /absolute/path/release-gate-build.json
```

Set the guide variables and map loop devices:

```bash
set -u
POOL=default
NETWORK=incusbr0
PROFILE=phase5-release-gate
VM=phase5-release-gate
TARGET_VOL=phase5-install-target
INSTALLED_VOL=phase5-installed-copy
SOURCE_RAW=/absolute/path/seeded-x86_64.raw
SOURCE_WORK=/absolute/path/seeded-x86_64.gate.raw
RESCUE_RAW=/absolute/path/rescue-data.raw
EVIDENCE=/absolute/path/phase5-manual-evidence
mkdir -p "$EVIDENCE"
test "$(uname -m)" = x86_64
test -b /dev/kvm
incus version | tee "$EVIDENCE/incus-version.txt"
incus storage show "$POOL"
incus network show "$NETWORK"
cp --reflink=auto --sparse=always -- "$SOURCE_RAW" "$SOURCE_WORK"
SOURCE_BLOCK=$(sudo losetup --find --show --partscan "$SOURCE_WORK")
RESCUE_BLOCK=$(sudo losetup --find --show --read-only --partscan "$RESCUE_RAW")
printf 'SOURCE_BLOCK=%s\nRESCUE_BLOCK=%s\n' "$SOURCE_BLOCK" "$RESCUE_BLOCK" \
  | tee "$EVIDENCE/loop-devices.txt"
sha256sum -- "$SOURCE_RAW" "$SOURCE_WORK" "$RESCUE_RAW" \
  | tee "$EVIDENCE/artifact-sha256.txt"
```

Create the profile, target, VM, TPM, installer and target devices exactly as in the guide:

```bash
incus profile create "$PROFILE"
incus profile device add "$PROFILE" root disk pool="$POOL" path=/
incus profile device add "$PROFILE" eth0 nic network="$NETWORK"
incus storage volume create "$POOL" "$TARGET_VOL" --type=block size=50GiB
incus init --empty --vm "$VM" --profile "$PROFILE" \
  --config security.secureboot=false --config limits.cpu=2 \
  --config limits.memory=4GiB --device root,size=4GiB \
  --device root,boot.priority=10 --device root,io.bus=virtio-scsi
incus config device add "$VM" vtpm tpm
incus config device add "$VM" install-media disk \
  source="$SOURCE_BLOCK" io.bus=virtio-scsi readonly=false boot.priority=30
incus config device add "$VM" install-target disk \
  pool="$POOL" source="$TARGET_VOL" io.bus=virtio-scsi boot.priority=20
incus config show "$VM" --expanded | tee "$EVIDENCE/install-config.yaml"
```

Record the seed before boot, install through VGA/serial, and retain logs:

```bash
SEED_PART=$(lsblk -nrpo NAME,PARTLABEL "$SOURCE_BLOCK" | awk '$2 == "seed-data" {print $1; exit}')
test -n "$SEED_PART"
sudo dd if="$SEED_PART" bs=4M status=none | sha256sum \
  | tee "$EVIDENCE/seed-partition.before.sha256"
sudo dd if="$SEED_PART" bs=4M status=none | tar -tf - \
  >"$EVIDENCE/seed.before.list"
grep -E '(^|/)install\.(json|ya?ml)$' "$EVIDENCE/seed.before.list"
incus start "$VM"
incus console "$VM" --type=vga
```

In another terminal use `incus console "$VM"`, detach with Ctrl+A then Q, and archive `incus console "$VM" --show-log`. Wait for explicit installer completion; network traffic is not success.

Stop and prove seed wipe exactly as the guide requires:

```bash
incus stop "$VM"
incus config device remove "$VM" install-media
incus config device remove "$VM" install-target
sudo blockdev --flushbufs "$SOURCE_BLOCK"
sudo dd if="$SEED_PART" bs=4M status=none | sha256sum \
  | tee "$EVIDENCE/seed-partition.after.sha256"
sudo dd if="$SEED_PART" bs=4M status=none | tar -tf - \
  >"$EVIDENCE/seed.after.list" 2>"$EVIDENCE/seed.after.tar.stderr" || true
BEFORE=$(cut -d' ' -f1 "$EVIDENCE/seed-partition.before.sha256")
AFTER=$(cut -d' ' -f1 "$EVIDENCE/seed-partition.after.sha256")
test "$BEFORE" != "$AFTER"
! grep -Eq '(^|/)install\.(json|ya?ml)$' "$EVIDENCE/seed.after.list"
```

Copy the detached volume, attach the installed copy and rescue data, then boot:

```bash
incus storage volume copy "$POOL/$TARGET_VOL" "$POOL/$INSTALLED_VOL" --volume-only
incus config device add "$VM" installed-target disk \
  pool="$POOL" source="$INSTALLED_VOL" io.bus=virtio-scsi boot.priority=20
incus config device add "$VM" rescue-data disk \
  source="$RESCUE_BLOCK" io.bus=virtio-scsi readonly=true boot.priority=0
incus config show "$VM" --expanded | tee "$EVIDENCE/recovery-config.yaml"
lsblk -o NAME,FSTYPE,LABEL,PARTLABEL "$RESCUE_BLOCK" \
  | tee "$EVIDENCE/rescue-block-layout.txt"
incus start "$VM"
incus console "$VM" --type=vga
```

Capture serial/VGA evidence. Do not grep for an invented stable success string. A human reviewer must identify the post-boot OS/application version or effect that proves the signed recovery payload was accepted and applied.

After archiving every file listed by the guide, clean up:

```bash
incus stop "$VM"
incus config device remove "$VM" rescue-data
incus config device remove "$VM" installed-target
incus delete "$VM"
incus storage volume delete "$POOL" "$INSTALLED_VOL"
incus storage volume delete "$POOL" "$TARGET_VOL"
incus profile delete "$PROFILE"
sudo losetup --detach "$RESCUE_BLOCK"
sudo losetup --detach "$SOURCE_BLOCK"
```

**Expected observable result:** All four independent observations are present:

1. installer completion on the 50 GiB target;
2. changed seed-data hash and no readable `install.*` member after installation;
3. explicit evidence that installed IncusOS detected the attached FAT/ISO volume labeled `RESCUE_DATA`;
4. explicit expected post-boot version/effect proving the real signed `update.sjson` and recovery payload were accepted and applied.

Builder JSON hashes must match the unchanged raw files and evidence archive.

**Evidence:** Every file listed in step 8 of the guide, build JSON, screenshots/console records, reviewer name/date, release-candidate commit.

**Pass/fail:** Missing any one observation is a failure. Prior Phase 5.2 logs, network frames, source-overlay growth, a staged file tree, or a successful builder exit are not substitutes.

## 6. Known gaps and risk register

| Gap or boundary | Current evidence | Residual risk | Mitigation and gate decision |
|---|---|---|---|
| Installer seed consumption has never been observed. | `docs/notes/phase-5-boot-probe.md` and `phase-5-boot-evidence.json` classify the prior run negative. | Media can build and boot yet never apply the seed or install target. | I-01 is mandatory before every tag. On the available macOS-only environment this is `Blocked` and therefore **NO-GO**, not a known limitation that can ship. |
| `RESCUE_DATA` detection has never been observed. | Prior recovery phase was not reached. | Structurally correct rescue media may be ignored by the installed OS. | I-01 must show actual detection. No Linux/Incus host means **NO-GO**. |
| Signed recovery-metadata acceptance has never been observed. | Builder validates MIME/binding only; it does not authenticate PKCS#7. | The real OS may reject the staged signature, file binding, or media layout. | I-01 must show the expected recovery effect using unmodified live signed metadata. Structural D/F cases are not proxies. |
| Release workflow has never published. | Only configuration exists until H-03/H-04. | Permissions, draft timing, native arm runner, GHCR, OIDC, or registry referrers can fail. | H-03 is pre-tag; H-04/H-06/H-07 are draft gate checks. Any failure blocks public release. |
| Anonymous `ghd install`/`download` cannot be proven against a draft or nonexistent release. | README correctly states no release exists. | Asset mapping/provenance can still fail only after public visibility. | Treat tag/draft as provisional. H-05 after first publication is required for final GO. If it fails, declare a release incident; do not claim readiness. |
| GHCR has no draft state. | `release.yml` notes the image is pushed during draft preparation. | A broken candidate may be visible even though GitHub release remains draft. | Verify the digest, both platforms, signature, SBOM, and provenance immediately in H-06/H-07; never point users at the tag before the draft gate passes. |
| `.github/repository-settings.toml` starts unapplied. | Authoritative project context; H-01 initial plan shows drift. | Missing immutable releases, tag protection, required checks, or private reporting weakens release and security promises. | Apply supported settings and require zero supported drift before tagging. Record unsupported settings as manual follow-ups. |
| Aarch64 installer boot is not part of the published manual gate. | E-01/F structural cases only. | aarch64 output may pass byte/layout checks but fail on real firmware. | Record as `Known limitation`; do not imply an aarch64 boot was tested. Add native boot evidence in a future release process if that becomes a promise. |
| The builder intentionally does not authenticate the unsigned index or the PKCS#7 chain. | Trust model explicitly limits the claim to HTTPS, structure, and hash binding. | A compromised named HTTPS origin/local mirror can choose index digests; only booted IncusOS authenticates recovery. | Not a failed promise. D-05/D-06 prove the documented boundary; I-01 proves the boot-side decision for the candidate. |
| Repository settings marked unsupported have no automated REST application. | H-01 plan lists each reason. | Desired UI posture can drift. | Review each in the GitHub UI and record its state. Only settings tied to a public promise (for example private reporting) block release when absent. |
| Public docs status text changes meaning at the first release. | README/SECURITY currently correctly say no release. | The first tag could publish stale “no release” instructions. | G-07 performs a pre-tag and post-publication status review. Stale release-tag docs fail the gate. |

## 7. Execution order, effort, and result recording

### Recommended order

| Order | Work | Approximate elapsed/human effort | Parallelization |
|---|---|---|---|
| 1 | Environment, mirror, age key, A-01/A-02 | 1–2 hours plus downloads | Tool installation and Linux-host preparation can run in parallel. |
| 2 | A-03, B, C validation/SOPS | 3–5 hours | A-03 can run while manual B/C fixtures are prepared, but do not let its output replace functional cases. |
| 3 | D acquisition/cache/trust | 4–7 hours | Independent mirror mutations D-04/D-05/D-06 can run in parallel with separate caches. Avoid sharing a mirror copy being mutated. |
| 4 | E build/publication | 6–10 hours | Matrix builds can run in parallel only if disk and I/O budget allow; use separate outputs and one verified cache. TTY publication cases remain serial. |
| 5 | F offline media | 3–5 hours | ISO and raw builds can run in parallel after the application cache is warm. |
| 6 | G documentation replay and docs/repository review | 3–5 hours | Fresh-contributor gate and Pages/security browser checks can run in parallel with F. |
| 7 | H-01 through H-03 | 1–2 hours plus Actions queue time | Repository settings review and dry-run workflow can overlap after credentials are confirmed. |
| 8 | I-01 on Linux Incus | 4–8 hours | Can run while non-mutating pre-tag GitHub review finishes. It must finish before the Release Please PR is merged. |
| 9 | Merge release PR; H-04/H-06/H-07 draft checks | 2–4 hours plus Actions time | Native macOS arm64 and Linux amd64 checks run in parallel after the immutable digest exists. |
| 10 | Publish draft; H-05 and post-publication G-07 | 1–2 hours | Run anonymous ghd, Pages/status, and final support checks in parallel. |

Do not parallelize cases that mutate the same mirror, cache blob, final path, repository settings, release PR, or tag. Preserve each test's fresh cache where tamper detection depends on avoiding a prior valid hit.

### Evidence layout

Use one directory per case under `$EVIDENCE`, or prefix every file with the stable case ID. Archive at minimum:

- tested commit SHA, branch, host OS/architecture, tool versions, start/end time;
- exact command transcript with separate stdout/stderr and process exit status;
- input configs with secrets removed;
- JSON envelopes and independent file hashes;
- mirror version/index hash and mutation copies for adversarial cases;
- cache and final-directory listings before/after failure cases;
- release/Actions/PR URLs, tag, asset hashes, image digest, signature/SBOM/attestation results;
- the complete I-01 evidence set.

Do not archive the age private key, GitHub tokens, release-app key, or other credentials.

### Results table template

Copy one row per case. Do not collapse negative subcases into a single pass unless every named subcase ran.

| Case ID | Host | Commit/tag/image digest | Start/end UTC | Result (`Pass`/`Fail`/`Blocked`/`Known limitation`/`N/A`) | Actual exit/state | Evidence path or URL | Deviation/issue | Executor | Reviewer |
|---|---|---|---|---|---|---|---|---|---|
| A-01 | | | | | | | | | |
| A-02 | | | | | | | | | |
| A-03 | | | | | | | | | |
| B-01 … B-09 | | | | | | | | | |
| C-01 … C-05 | | | | | | | | | |
| D-01 … D-09 | | | | | | | | | |
| E-01 … E-07 | | | | | | | | | |
| F-01 … F-04 | | | | | | | | | |
| G-01 … G-07 | | | | | | | | | |
| H-01 … H-08 | | | | | | | | | |
| I-01 | | | | | | | | | |

### Final decision record

```text
Tested commit:
Release tag (when created):
GHCR digest:
Pre-tag gate: PASS / FAIL / BLOCKED
Draft-release gate: PASS / FAIL / BLOCKED / NOT STARTED
Published-release gate: PASS / FAIL / BLOCKED / NOT STARTED
Open failed or blocked case IDs:
Known limitations accepted (with source and owner):
Boot evidence archive:
Release evidence archive:
Decision: GO / NO-GO
Decision maker:
Decision UTC time:
```

A provisional pre-tag or draft pass is not the final `GO`. The final decision remains `NO-GO` or `NOT COMPLETE` until I-01 passes and the public `ghd` checks have run against the first published release.