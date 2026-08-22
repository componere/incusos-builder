---
title: Automation reference
description: CLI automation contracts and the release workflow, artifacts, publication controls, and signer identities
---

# Automation reference

Machine contracts for using and releasing `incusos-builder`. Command and flag
names are in [CLI](cli.md). Cache paths are in [Cache](cache.md).

## Exit codes

The process maps the returned error to an exit code after the command
finishes. Help text is not reprinted on failure. The error is written
to stderr unless that write itself fails (exit `1`).

| Code | Condition |
|------|-----------|
| `0` | Success |
| `1` | Unexpected error, including a canceled context that is not an acquisition failure, or a failure writing the error line to stderr |
| `2` | Usage: unknown flags or commands, extra operands, missing required flags, invalid `--color` / `--progress`, `--verbose` with `-q`, `--json` with `-o -`, offline `-o -`, bad `--server`, overwrite refused, `init` cancel, and other usage errors |
| `3` | Invalid seed config, including an oversized seed tar at splice time |
| `4` | SOPS decryption failure after a top-level `sops` key |
| `5` | Acquisition or version resolution, including GPT-probe drift, empty cache directory, a canceled fetch, and a canceled read or splice of an already-verified image |
| `6` | Failure writing a built artifact: image stream, rescue media, or publication |

Wrapped errors keep the same code.

## JSON envelopes

`--json` / `INCUSOS_BUILDER_JSON` writes exactly one JSON document to
stdout, terminated by a newline. `-q` does not change the document.
Human summaries and tables are omitted.

`--json` cannot be combined with `-o -` on `build` or `init` (exit `2`,
error envelope). After a `-o -` stream has started, a mid-stream
failure does not append a second document to the artifact.

### Error

Every `--json` failure path, including flag parse when `--json` is on
the command line:

```json
{"error":{"code":2,"message":"usage error: bad path"}}
```

| Field | Type | Meaning |
|-------|------|---------|
| `error.code` | integer | Same value as the process exit code |
| `error.message` | string | The error text; empty only for a nil error |

Goldens:

```json
{"error":{"code":0,"message":""}}
{"error":{"code":2,"message":"usage error: bad path"}}
{"error":{"code":3,"message":"invalid config: field seeds.install"}}
{"error":{"code":4,"message":"decryption failed: sops"}}
{"error":{"code":5,"message":"acquisition failed: index"}}
{"error":{"code":5,"message":"acquisition failed: context canceled"}}
{"error":{"code":5,"message":"version not found: pin"}}
{"error":{"code":6,"message":"output write failed: rename"}}
{"error":{"code":1,"message":"boom"}}
```

### build

```json
{
  "result": {
    "output": "out.img",
    "resources_output": "out.resources.img",
    "type": "raw",
    "architecture": "x86_64",
    "version": "202608102114",
    "channel": "stable",
    "seed_bytes": 123,
    "sha256": "…64 lowercase hex…",
    "resources_sha256": "…64 lowercase hex…"
  }
}
```

| Field | Type | Meaning |
|-------|------|---------|
| `result.output` | string | `-o` path, or `-` when the image was streamed |
| `result.resources_output` | string | Rescue-media path, explicit or derived; omitted when online |
| `result.type` | string | `iso` or `raw` |
| `result.architecture` | string | `x86_64` or `aarch64` |
| `result.version` | string | Resolved update version |
| `result.channel` | string | Channel the version was selected from |
| `result.seed_bytes` | integer | Spliced seed-tar size |
| `result.sha256` | string | Lowercase hex SHA-256 of the stored image bytes |
| `result.resources_sha256` | string | Lowercase hex SHA-256 of the rescue-media bytes produced by this invocation; omitted when online |

`sha256` matches a second hash of the published file. For a `.gz`
output it is the compressed stored bytes.

`resources_sha256` authenticates the rescue-media bytes from that
invocation. Raw rescue media is not byte-reproducible across builds:
go-diskfs generates a GPT disk GUID and FAT volume serial for each build,
so identical inputs can produce a different `resources_sha256`.

### validate

```json
{"result":{"valid":true,"type":"iso","architecture":"x86_64","offline":false}}
```

| Field | Type | Meaning |
|-------|------|---------|
| `result.valid` | bool | Always `true` on the success path |
| `result.type` | string | `image.type` |
| `result.architecture` | string | `image.architecture` |
| `result.offline` | bool | `image.offline` |

### versions

```json
{
  "result": {
    "versions": [
      {
        "version": "202608102114",
        "channels": ["stable"],
        "published_at": "2026-08-16T00:00:00Z",
        "architectures": ["x86_64"]
      }
    ]
  }
}
```

| Field | Type | Meaning |
|-------|------|---------|
| `result.versions` | array | Filtered releases; `[]` when nothing matches |
| `versions[].version` | string | Update version name |
| `versions[].channels` | array of string | Channel membership; never JSON `null` |
| `versions[].published_at` | RFC 3339 time | Upstream publication timestamp |
| `versions[].architectures` | array of string | Architectures that have an `iso` or `raw` image after the `--architecture` filter |

Per-image type is not in the JSON body; it appears only in the human
table.

### init

```json
{"result":{"output":"config.yaml"}}
```

| Field | Type | Meaning |
|-------|------|---------|
| `result.output` | string | The `-o` path that was written |

## Environment variables and precedence

Settings that accept both a flag and an environment variable, highest first:

1. Command-line flag, and only if that flag was actually passed.
   Flag defaults that were not passed cannot mask environment variables.
2. `INCUSOS_BUILDER_*` (prefix `INCUSOS_BUILDER`; `-` and `.` become `_`).
3. Built-in default.

| Setting | Flag | Environment | Default |
|---------|------|-------------|---------|
| Server | `--server` | `INCUSOS_BUILDER_SERVER` | `https://images.linuxcontainers.org/os` |
| Cache directory | `--cache-dir` | `INCUSOS_BUILDER_CACHE_DIR` | `<user-cache>/incusos-builder`, or empty |
| JSON | `--json` | `INCUSOS_BUILDER_JSON` | `false` |
| Color | `--color` | `INCUSOS_BUILDER_COLOR` | `auto` |
| Progress | `--progress` | `INCUSOS_BUILDER_PROGRESS` | `auto` |
| No input | `--no-input` | `INCUSOS_BUILDER_NO_INPUT` | `false` |

`--json=false` and `--no-input=false` beat a true environment value.

An exported-but-empty `INCUSOS_BUILDER_*` variable for any setting in the
table is an explicit empty value. It has the same effect as passing an empty
value for the corresponding flag and does not fall through to the built-in
default. For example, an empty `INCUSOS_BUILDER_CACHE_DIR` has the same effect
as `--cache-dir ""`.

If `INCUSOS_BUILDER_SERVER` is exported with an empty value, the command
reports `usage error: --server "" is neither an https URL nor an existing
directory` and exits with status `2` instead of using the default server.
Leave the variable unset to use the default.

Not in that table:

| Variable | Effect |
|----------|--------|
| `CI` | Any non-empty value auto-enables no-input. `--no-input=false` does not clear this |
| `NO_COLOR` | Any non-empty value disables color when `--color` is `auto` |
| `TERM` | `dumb` disables color when `--color` is `auto` |
| `ACCESSIBLE` | Any non-empty value selects line-oriented `init` prompts |
| `SOPS_AGE_KEY` | Age key material for encrypted seed configs; absence of every SOPS key source is exit `4` |

`--verbose` and `-q` have no environment mapping.

## stdout and stderr

| Stream | Content |
|--------|---------|
| stdout | Human success (summary, `configuration valid`, versions table, `wrote <path>`), `--json` envelopes, `-o -` image bytes, `init -o -` YAML, `--help`, `--version` |
| stderr | Error line after failure (including `--json`), overwrite prompt, `init` form, step headers, percentage or bar updates, and two build-plan debug records when `--verbose` accompanies successful build work |

`-q` suppresses the human success writers. It does not suppress
`--json`, `-o -` artifact bytes, `init -o -` YAML, or stderr output,
including step headers and enabled percentage updates.

`--progress` controls percentage or bar updates, not step headers.
`--progress auto` is pre-resolved to `never` unless both stdout and
stderr are TTYs. `--json` or `-o -` further forces AUTO progress to
`never`. Explicit `--progress always` / `never` is unchanged. Staged
work still writes `==> <step>` headers when progress is `never`, and
writes `done <step>` only after that step succeeds.

`--verbose` adds only two build-plan debug records after `build` work
succeeds. It has no observable effect on `validate`, `versions`, or
`init`.

`--color auto` is not pre-resolved by the CLI. The reporter then
disables color if `NO_COLOR` is set, `TERM` is `dumb`, or the writer
is not a TTY. `always` and `never` override that.

## `-`

`-` is the reserved stream sentinel. The same rule applies to a path
that cleans to `-`, such as `./-`.

| Use | Command | Meaning |
|-----|---------|---------|
| `-f -` | `build`, `validate` | Read the seed config from stdin |
| `-o -` | `build` | Write image bytes to stdout; no summary; no rescue media |
| `-o -` | `init` | Write the starter YAML to stdout; no `wrote` line |

`--resources-output -` (or another path that cleans to `-`) is a usage
error. The sentinel is not a valid image publication path; streaming is
handled by `-o -` on `build`.

## TTY, prompts, and `--no-input`

Resolved no-input is the `--no-input` flag or environment value, or
auto-on, whichever is true.

Auto-on is true when `CI` is non-empty, or stdin is not a TTY, or
stdout is not a TTY.

`--no-input=false` on the command line overrides
`INCUSOS_BUILDER_NO_INPUT=true`. Auto-on still applies, so a non-TTY
or set `CI` still disables prompts. `--no-input=false` on both TTYs
with `CI` unset leaves prompts enabled.

When no-input is true:

- `init` writes the deterministic example. It does not prompt.
- `build` does not prompt to overwrite. Existing finals are refused
  (exit `2`) unless `--force`.

When no-input is false:

- `init` prompts on stderr. `ACCESSIBLE` non-empty or `TERM=dumb`
  selects line-oriented prompts.
- `build` prompts on stderr before replacing existing finals.

There is no `--yes`. `--force` is the non-interactive overwrite path.

## Release automation

`.github/workflows/release.yml` is the tag-triggered caller. Every reusable
workflow reference and every checksum-signer reference in that caller is pinned
to `meigma/release` commit
`0dee66ff6c4cc7e28d7bb65e97a37d701e0eff4a` (tag `v0.1.17`). The reusable
workflows, their sibling setup action, and the matching `release-cli` are one
release unit. Mixed revisions are not supported.

Release Please creates a stable `vMAJOR.MINOR.PATCH` tag and the matching draft
release after its reviewed release pull request is merged. The tag-triggered
caller then invokes the pinned release unit in this order:

1. `go-pre-publish.yml` builds, verifies, signs, and stages release assets.
2. `go-oci-build.yml` builds the OCI layout from the verified Linux binaries.
3. `publish-oci-image.yml` verifies, signs, attests, and optionally publishes
   the image.
4. `publish-github-release.yml` verifies and fills the existing draft, attests
   its payloads, and optionally makes the draft public.
5. The Homebrew, Scoop, and native package-repository publishers run after the
   GitHub Release publisher when their publication inputs are enabled.

The manual [boot-acceptance gate](../how-to/verify-boot-acceptance.md) remains a
required pre-tag check. The reusable release unit does not perform or replace
that gate.

### Rehearsal and publication controls

A release rehearsal uses a real candidate tag. It is not a separate workflow.
For the rehearsal commit, all remote publication inputs are `false`:

```yaml
publish-image: false
publish-release: false
publish-homebrew: false
publish-scoop: false
publish-package-repository: false
```

The run still builds and validates the release bundle and OCI layout. It fills
and verifies the existing draft without making it public. Homebrew, Scoop, and
package-repository requests remain disabled. The operator inspects the draft,
the `release-assets` Actions artifact, the `oci-image` Actions artifact, SBOMs,
signatures, generated package-manager controls, and job logs. Publication is
enabled only in a later reviewed commit.

### Published artifacts and destinations

For version `<version>`, the GitHub Release contains:

| Kind | Names and platforms |
|------|---------------------|
| Archives | `incusos-builder_<version>_<os>_<arch>.tar.gz` for Darwin and Linux; `incusos-builder_<version>_windows_<arch>.zip` for Windows; `amd64` and `arm64` |
| Native packages | DEB, RPM, and APK for Linux `amd64` and `arm64` |
| SBOMs | One SPDX SBOM for each archive and each native package |
| Integrity controls | `checksums.txt` and its keyless Cosign bundle, `checksums.txt.sigstore.json` |

The Darwin archives are signed with
`Developer ID Application: Joshua Gilman (7MN6B2QY4W)` and notarized by Apple.
RPM and APK packages carry producer signatures. The signed checksum manifest
and GitHub artifact attestations cover all six native packages.

The OCI publisher publishes `ghcr.io/componere/incusos-builder` for
`linux/amd64` and `linux/arm64`. Consumers can select the immutable digest or
the exact `<version>` tag. Eligible releases also update the `MAJOR.MINOR`,
`MAJOR`, and `latest` tags. The index and its platform manifests have
recursive Cosign signatures. The image has one provenance attestation and two
platform SPDX SBOM attestations.

The Homebrew publisher opens a pull request for cask `incusos-builder` in
`componere/homebrew-tap`. The Scoop publisher opens a pull request for root
manifest `incusos-builder.json` in `componere/scoop-bucket`.

The native package publisher admits the release to the `stable` channel at
`https://pkgs.componere.dev` for `amd64` and `arm64`:

| Format | Repository | Repository signing key |
|--------|------------|------------------------|
| APT | `https://pkgs.componere.dev/apt`, roots under `dists/stable/` | `https://pkgs.componere.dev/keys/apt-repository-001.asc` |
| RPM | `https://pkgs.componere.dev/rpm/stable/<architecture>`; `x86_64` or `aarch64` | `https://pkgs.componere.dev/keys/rpm-repository-001.asc` |
| APK | `https://pkgs.componere.dev/apk/stable/main/<architecture>`; `x86_64` or `aarch64` | `https://pkgs.componere.dev/keys/apk-index-001.rsa.pub` |

RPM and APK are checked against their producer-native signatures before
repository publication. The APK producer key is named
`incusos-builder-apk-001.rsa.pub`. The central repository signs its APT, RPM,
and APK index metadata with the keys listed above.

### Release verification identities

GitHub Release asset attestations are signed by
`meigma/release/.github/workflows/publish-github-release.yml`. OCI signatures
and attestations are signed by
`meigma/release/.github/workflows/publish-oci-image.yml`. These identities
belong to `meigma/release`; no release signer workflow in this repository is
part of the verification identity.

Consumer verification binds an artifact to all of these values:

| Constraint | Value |
|------------|-------|
| Producer repository | `componere/incusos-builder` |
| Source ref | `refs/tags/<tag>` |
| Signer digest | `0dee66ff6c4cc7e28d7bb65e97a37d701e0eff4a` |
| Release-asset signer workflow | `meigma/release/.github/workflows/publish-github-release.yml` |
| OCI signer workflow | `meigma/release/.github/workflows/publish-oci-image.yml` |

`gh attestation verify` therefore uses
`--repo componere/incusos-builder`, the applicable shared signer workflow,
the full signer digest, and the source ref. OCI verification also reads the
registry bundle and operates on a digest-pinned image reference. Checksum
verification separately validates `checksums.txt` against
`checksums.txt.sigstore.json` and the keyless
`meigma/release/.github/workflows/go-pre-publish.yml` identity at the same
release-unit commit.
