---
name: melange
description: >
  Package the GoReleaser-built Linux binary into a signed Wolfi apk with melange.
  Use when editing melange.yaml's install-only pipeline, the staged `application`
  or version-vars contract, ephemeral apk signing, the melange-to-apko handoff,
  `mise run image-local`, or the pinned `go-oci-build.yml` release integration.
---

# Melange

melange has one job in this repository: install the already-built Linux binary into a signed
Wolfi apk described by `melange.yaml`. It compiles no Go code. apko then installs that apk in
the runtime image. There is no Dockerfile.

## Verified against

- melange `0.59.1`, pinned in `mise.toml` as `aqua:chainguard-dev/melange` and locked per
  platform in `mise.lock`. Run it through mise or an activated mise shell.
- The current `melange.yaml`, `apko.yaml`, `.goreleaser.yaml`, `mise.toml`, and
  `.github/workflows/release.yml`.
- `meigma/release` commit `0dee66ff6c4cc7e28d7bb65e97a37d701e0eff4a` (v0.1.17), especially
  `.github/workflows/go-oci-build.yml` and the `release-cli image build` implementation.
- Sibling skills: `mise` provisions the CLI; `apko` consumes the signed apk and assembles the
  OCI layout.

## Use this skill when

- Editing `melange.yaml` or its install-only pipeline.
- Changing the `application` staging path, the apk version vars file, or the installed binary
  path.
- Changing ephemeral apk signing or the repository/key handoff to apko.
- Debugging `mise run image-local` or the pinned reusable OCI build.
- Bumping the pinned melange CLI.

## melange's lane (non-negotiable)

1. Package the staged file named `application`; do not compile source. The release input is
   the already-built, already-attested GoReleaser Linux binary. Reintroducing a `go/build`
   pipeline would create a second, unattested binary whose bytes do not match the canonical
   release asset.
2. GoReleaser owns binary metadata. `.goreleaser.yaml` stamps `main.version`, `main.commit`,
   and `main.date` with linker flags before melange runs. melange's vars file carries only the
   apk version.
3. Keep `package.version: ${{vars.version}}`. `release-cli image build` writes the release
   version to its vars file; the local task writes `version: "0.0.0"`.
4. Keep the pipeline install-only. It must install `application` as
   `/usr/bin/incusos-builder`, mode `0755`, owner `0`, group `0`.
5. Never commit an apk signing key. The local task and release CLI create ephemeral keys.
   apko receives the matching public key at invocation time.
6. No Dockerfile, `RUN`, `apt`, or Go toolchain belongs here. The only build-environment
   package is `busybox`, which supplies `install`.
7. The supported local invocation passes `--runner docker`; Docker must be running.

## `melange.yaml` anatomy

- `package`: `name: incusos-builder`, `version: ${{vars.version}}`, `epoch: 0`, the package
  description, `target-architecture: [x86_64, aarch64]`, and the SPDX copyright license.
- `vars.version`: the fallback `0.0.0`, overridden by the build's vars file.
- `environment.contents`: the Wolfi repository and signing key plus `busybox`. There is no Go
  compiler in the melange environment.
- `pipeline`: one `install` command that copies the staged `application` file to
  `${{targets.destdir}}/usr/bin/incusos-builder` with the required mode and ownership.

## The `application` staging contract

The canonical binaries are produced by GoReleaser, whose Linux builds use `CGO_ENABLED=0`,
`-trimpath`, and linker flags for version, full commit, and commit date. The reusable OCI
workflow verifies the canonical-binary artifact handoff before downloading it.

`release-cli image build` verifies each expected digest and the static ELF contract, then
writes the binaries as `application` under separate `x86_64` and `aarch64` source trees with
mode `0755`. It writes a vars file containing the release version, generates an ephemeral
melange signing key, and runs melange once for each source tree. The private key stays in the
scratch workspace; the public key is copied into the OCI build output for apko.

The filename is part of the interface between GoReleaser, release CLI, and `melange.yaml`.
Changing it in one place alone makes the build fail rather than selecting another binary.

## Local build

`mise run image-local` is the supported local path. Its staging and melange portion is:

```bash
arch="$(go env GOARCH)"
case "$arch" in
  amd64) apkarch=x86_64 ;;
  arm64) apkarch=aarch64 ;;
  *) echo "unsupported host arch $arch" >&2; exit 1 ;;
esac
work=".image-local"
rm -rf "$work" packages image.tar
mkdir -p "$work"
CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath \
  -ldflags "-s -w -buildid= -X main.version=0.0.0 -X main.commit=$(git rev-parse --short HEAD 2>/dev/null || echo none) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o "$work/application" ./cmd/incusos-builder
printf 'version: "0.0.0"\n' > "$work/vars.yaml"
melange keygen melange.rsa
melange build melange.yaml --arch "$apkarch" --signing-key melange.rsa --runner docker \
  --source-dir "$work" --vars-file "$work/vars.yaml"
```

This is the local equivalent of the canonical-binary staging sequence, not a separate
melange compilation path. The apk repository is written under `./packages`; the apko step
then appends that repository and `./melange.rsa.pub` on its command line.

## Release build and verification

`.github/workflows/release.yml` is tag-triggered and delegates the OCI build to
`meigma/release/.github/workflows/go-oci-build.yml` at the pinned commit. This repository has
no local melange build job.

Inside the reusable workflow, `release-cli image build --json` performs the canonical binary
staging, melange config compile-check, ephemeral key generation, per-architecture melange
builds, and apko composition. It also supplies melange's release namespace, build date,
source repository, tagged commit, and package provenance flags. Those values describe the
apk build; they do not restamp the staged application.

The workflow then runs `release-cli image verify --json`. Verification reads the OCI layout,
the installed application bytes, and the architecture SBOMs before the authoritative OCI
artifact is uploaded. Signing, attestation, and publication are owned by the pinned reusable
publisher workflows, not by steps in this repository.

## Signing and the apko handoff

melange signs each apk with the ephemeral private key created for that image build. apko
needs two invocation-time inputs to install it:

- the generated apk repository via `--repository-append`; and
- the matching public key via `--keyring-append`.

Neither input is hardcoded in `apko.yaml`. Locally they are `./packages` and
`./melange.rsa.pub`; release CLI supplies paths inside its isolated output and scratch roots.

## Gotchas

### Invalid `package:` metadata fields

`vendor`, `homepage`, and `maintainer` are not valid fields under `package:` in a melange
configuration. melange fails to decode `melange.yaml` when any of them is present. Keep that
publisher metadata in `.goreleaser.yaml`; do not copy its nFPM fields into `melange.yaml`.

### A `go/build` pipeline breaks artifact identity

The input `application` is already the canonical GoReleaser output. Compiling again inside
melange produces different, unattested bytes. Keep the single `install` step even when
changing Go flags or release metadata; make those changes in `.goreleaser.yaml` and preserve
the staging contract.

### Architecture names differ by layer

Go and OCI use `amd64` and `arm64`; melange package/source directories use `x86_64` and
`aarch64`. The local task maps them explicitly. Do not pass the Go architecture to the
melange `--arch` flag.

### The source directory must contain `application`

The local task passes `.image-local`; release CLI passes one per-architecture source tree.
Pointing `--source-dir` at the repository root no longer works because `melange.yaml` does not
compile source and expects `application` at the source-tree root.

## Command reference

See [references/melange-commands.md](references/melange-commands.md) for the pinned command and
flag reference.
