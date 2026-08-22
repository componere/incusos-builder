---
name: apko
description: >
  Assemble the multi-architecture, nonroot OCI image from the melange-produced apk
  with apko. Use when changing apko.yaml, runtime packages, the uid/gid 65532
  account, the entrypoint, static OCI annotations, `mise run image-local`, or the
  `release-cli image build` and `image verify` contract in the pinned
  `go-oci-build.yml` reusable workflow.
---

# apko

apko turns the melange-built apk plus a small Wolfi runtime set into the OCI image. There is
no Dockerfile, shell, package-manager layer, or base-image `FROM`. `apko.yaml` declares the
runtime filesystem and image configuration; release CLI supplies build-specific repository,
keyring, and annotation values at invocation time.

## Verified against

- apko `1.2.37`, pinned in `mise.toml` as `aqua:chainguard-dev/apko` and locked per platform
  in `mise.lock`. Run it through mise or an activated mise shell.
- The current `apko.yaml`, `melange.yaml`, `mise.toml`, `.goreleaser.yaml`, and
  `.github/workflows/release.yml`.
- `meigma/release` commit `0dee66ff6c4cc7e28d7bb65e97a37d701e0eff4a` (v0.1.17), especially
  `.github/workflows/go-oci-build.yml` and the `release-cli image build` and `image verify`
  implementations.
- Sibling skills: `melange` builds the signed apk; `mise` provisions both CLIs.

## Use this skill when

- Adding or removing a Wolfi runtime package.
- Editing the numeric nonroot account, entrypoint, architecture set, or static OCI annotations
  in `apko.yaml`.
- Changing the invocation-time apk repository, signing key, version, or revision handoff.
- Debugging `mise run image-local` or the pinned reusable OCI build and verifier.
- Bumping the pinned apko CLI.

## apko's lane (do not cross it)

1. Define the runtime image in `apko.yaml`. Never add a Dockerfile, `RUN`, `apt`, `apk add`, a
   shell, or a base-image `FROM`.
2. Add runtime dependencies only as Wolfi package names under `contents.packages`. Keep the
   set minimal.
3. apko installs the signed apk; it does not compile the application or build the apk. See the
   `melange` skill for the install-only packaging contract.
4. Keep the runtime nonroot. The config `User` must resolve to the exact string `65532`.
5. Keep the entrypoint as the exact one-element array `[/usr/bin/incusos-builder]` in the
   generated image config.
6. Do not put the generated apk repository or ephemeral melange public key in `apko.yaml`.
   The local task and release CLI append both on the apko command line.
7. Do not hardcode image version or revision annotations. release CLI injects them from the
   release tag and tagged commit, and the verifier compares them with that release.
8. This repository does not publish, sign, or attest an image itself. The tag workflow calls
   the pinned reusable build and publisher workflows; keep those responsibilities out of
   local apko commands.

## `apko.yaml` anatomy

Read `apko.yaml` before changing it. Its load-bearing parts are:

- `contents.repositories`: only the Wolfi OS repository. The generated apk repository is
  supplied with `--repository-append`.
- `contents.keyring`: only the Wolfi signing key. The ephemeral melange public key is supplied
  with `--keyring-append`.
- `contents.packages`: `wolfi-baselayout`, `ca-certificates-bundle`, `tzdata`, and the plain
  `incusos-builder` package name.
- `entrypoint.command`: `/usr/bin/incusos-builder`, the path installed by `melange.yaml`.
- `accounts`: group and user `nonroot` at gid/uid `65532`, with `run-as: 65532`.
- `environment`: the runtime `PATH` and CA certificate path.
- `archs`: `amd64` and `arm64` for the OCI index.
- `annotations`: the four static values: title, description, source, and SPDX licenses.
  Version and revision are deliberately absent because release CLI injects them.

## Invocation-time inputs

The image build has four values that do not belong in `apko.yaml`:

- the generated apk repository, appended with `--repository-append`;
- the matching ephemeral public key, appended with `--keyring-append`;
- `org.opencontainers.image.version`, injected from the release tag without its `v` prefix;
  and
- `org.opencontainers.image.revision`, injected from the tagged commit.

`release-cli image build` passes the first two to both its apko lock and build operations and
passes the annotations to the build operation. The generated lock is part of that isolated
OCI build output; there is no committed `apko.lock.json` and the local task does not create
one.

Hardcoding version or revision in `apko.yaml` conflicts with the injected release values and
fails verification. Release Please does not stamp either image configuration file.

## Local build

`mise run image-local` builds the host architecture and loads it into Docker as
`incusos-builder:dev`. Its apko portion is:

```bash
apko build apko.yaml incusos-builder:dev image.tar --arch "$arch" \
  --repository-append ./packages --keyring-append ./melange.rsa.pub
docker load < image.tar
docker tag "incusos-builder:dev-$arch" incusos-builder:dev
```

The preceding melange step creates `./packages` and `./melange.rsa.pub`. Both append flags are
required: the repository makes the generated apk discoverable and the key makes its signature
trusted. A single-architecture apko build uses the architecture-suffixed Docker tag, so the
final retag is required before running `incusos-builder:dev`.

## Release build

`.github/workflows/release.yml` is tag-triggered and contains only calls to pinned
`meigma/release` reusable workflows. There are no repository-local apko publish, registry
login, digest-scraping, signing, or attestation steps.

Inside the pinned `go-oci-build.yml`, `release-cli image build --json`:

1. consumes the verified GoReleaser Linux binaries and stages one `application` source tree
   per architecture;
2. runs melange to create signed `x86_64` and `aarch64` apk repositories;
3. appends the generated repository and public key to apko;
4. injects release version and tagged-commit revision annotations;
5. creates a per-build apko lock; and
6. builds the multi-architecture OCI layout and architecture SBOMs.

The workflow calls the melange and apko binaries pinned by this repository's `mise.toml`. It
then runs `release-cli image verify --json` before uploading the authoritative OCI artifact.
The reusable publisher workflow owns subsequent publication, signing, and attestation.

## Image verifier requirements

Treat the verifier as part of the image schema, not as an optional smoke test.

The OCI index must carry exactly these six checked annotation keys:

- `org.opencontainers.image.description`;
- `org.opencontainers.image.licenses`;
- `org.opencontainers.image.revision`;
- `org.opencontainers.image.source`;
- `org.opencontainers.image.title`; and
- `org.opencontainers.image.version`.

Description, licenses, and title must be nonempty. Revision, source, and version must match
the tagged release being built. Each platform manifest and each config's labels must repeat
the same six values as the index.

For both `linux/amd64` and `linux/arm64`, the config must also have:

- `User` exactly `65532`; and
- `Entrypoint` exactly `["/usr/bin/incusos-builder"]`.

The verifier also reads the installed binary from each layer, compares its digest with the
canonical staged `application`, and checks the per-architecture SPDX document for the release
package version. A superficially valid image with different application bytes is rejected.

## Multi-architecture model

The reusable workflow runs on `ubuntu-24.04` and installs QEMU for arm64. release CLI builds
one melange apk for each package architecture, then apko composes the two-platform OCI layout.
Do not replace this with repository-local native-runner matrices or separate apko publish
steps.

The local task intentionally builds only the host architecture. It is a runtime smoke path,
not a substitute for the reusable workflow's two-platform build and byte-for-byte verifier.

## Gotchas

- **Repository and key are invocation-time inputs.** Omitting either append flag prevents apko
  from finding or trusting the generated package. Do not solve that by committing generated
  paths or keys to `apko.yaml`.
- **Version and revision are injected.** Adding either annotation to `apko.yaml` breaks the
  release build rather than overriding the release identity.
- **`User` is a string-level contract.** The verifier requires config `User` to be exactly
  `65532`, not `nonroot`, `65532:65532`, or another equivalent runtime spelling.
- **Entrypoint is an array-level contract.** It must contain only
  `/usr/bin/incusos-builder`; a shell wrapper or extra argument fails verification.
- **The local tag is architecture-suffixed.** Keep the `docker tag` step in
  `[tasks.image-local]`.
- **apko is pinned by mise.** Bump `mise.toml` and regenerate `mise.lock` together; do not use
  a system installation for release work.

See [references/apko-commands.md](references/apko-commands.md) for the general apko command and
flag reference. For the authoritative release invocation, follow the pinned reusable workflow
and release CLI rather than assembling an ad hoc `apko publish` command.
