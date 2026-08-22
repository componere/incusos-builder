# Melange Command Map

Curated operator reference for `melange v0.59.1`, pinned in `mise.toml`. Prefer local
`melange --help` / `melange <cmd> --help` output and the official docs
(https://github.com/chainguard-dev/melange) if anything here drifts. Repo-specific notes
below describe the current install-only package build.

## Global flags

Apply across all `melange` commands:

- `--log-level string`: log level — `debug`, `info`, `warn`, `error` (default `INFO`).
- `-h`, `--help`: help for the command.

## `melange build`

Purpose: build a package (the signed apk) from a YAML configuration file. This is the repo's
core operation.

Usage:

```bash
melange build [config.yaml] [flags]
```

Flags that matter here:

- `--arch strings`: architectures to build for. The repo passes one melange architecture per
  invocation: `x86_64` or `aarch64`. The local task maps from Go's `amd64` or `arm64`.
- `--runner string`: runner used to execute build steps — `bubblewrap`, `docker`, or `qemu`.
  The repo passes `docker` (required on macOS; needs Docker running).
- `--signing-key string`: key used to sign the produced apk.
- `--source-dir string`: directory mounted as the package source. The local task passes
  `.image-local`, whose root contains the staged `application`; release CLI passes a separate
  source tree for each architecture. The repository source is not compiled by melange.
- `--vars-file string`: file of preloaded build vars; overrides `vars:` in the config. The
  repo supplies only `version`.
- `-k`, `--keyring-append strings`: extra keys to include in the build environment keyring (for
  pulling from signed repositories; the apko keyring handoff is a separate concern).
- `--out-dir string`: directory packages are written to (default `./packages/`). The apk lands
  in `<out-dir>/<wolfi-arch>/`, where the Wolfi arch is `x86_64`/`aarch64`, not `amd64`/`arm64`.
- `-r`, `--repository-append strings`: extra repositories for the build environment.
- `--package-append strings`: extra packages to install into the build environment.
- `--namespace string`: namespace for package URLs in the generated apk SBOM (default `unknown`).
  release CLI supplies the release repository namespace.
- `--generate-index`: whether to generate `APKINDEX.tar.gz` (default `true`).
- `--cache-dir string`: cached inputs directory (default `./melange-cache/`).
- `--debug`: enable debug logging of build pipelines.
- `--debug-runner`: keep the builder container after success/failure for inspection.
- `-i`, `--interactive`: attach a tty to the builder pod on failure.
- `--build-date string`: package build timestamp. The reusable OCI workflow supplies the
  tagged commit's committer date.
- `--git-repo-url string` and `--git-commit string`: source identity recorded for the package.
  release CLI supplies both from the tagged GitHub build.
- `--generate-provenance`: emit package provenance. release CLI enables it for the reusable
  OCI build; the local `image-local` task does not.

Notes:

- The local task's invocation is:

  ```bash
  melange build melange.yaml --arch "$apkarch" --signing-key melange.rsa --runner docker \
    --source-dir "$work" --vars-file "$work/vars.yaml"
  ```
- The matching public key is handed to apko with `--keyring-append`; the generated package
  repository is handed over separately with `--repository-append`.

## `melange compile`

Purpose: decode and compile the YAML configuration into melange's build plan. This is a
configuration preflight, not compilation of the staged application.

The reusable `release-cli image build` path runs this preflight with the first package
architecture and the generated version vars file before it creates a key or builds either
apk. A decode error stops the release before package construction.

## `melange keygen`

Purpose: generate an RSA keypair for package signing.

Usage:

```bash
melange keygen [key.rsa] [flags]
```

Flags:

- `--key-size int`: size of the RSA key in bits (default `4096`).

Notes:

- Writes `<name>.rsa` (private) and `<name>.rsa.pub` (public). The local task creates
  `melange.rsa` and uses `melange.rsa.pub` for apko. release CLI creates its key inside the
  isolated scratch workspace and copies only the public key into the OCI build output.

## `melange sign`

Purpose: sign an existing `.apk` on disk in place with the provided key.

Usage:

```bash
melange sign [--signing-key=key.rsa] package.apk
melange sign [--signing-key=key.rsa] *.apk
```

Flags:

- `-k`, `--signing-key string`: signing key (default `local-melange.rsa`).

Notes:

- The repo signs during `build` via `--signing-key`, so this standalone command is rarely
  needed. Use it only to re-sign a prebuilt apk.

## `melange sign-index`

Purpose: sign an APK repository index (`APKINDEX.tar.gz`).

Usage:

```bash
melange sign-index [--signing-key=key.rsa] <APKINDEX.tar.gz>
melange sign-index [--signing-key=key.rsa] <APKINDEX.tar.gz> --force
```

Flags:

- `-f`, `--force`: overwrite the existing index with a newly signed one.
- `--signing-key string`: signing key (default `melange.rsa`).

Note: the default key name here (`melange.rsa`) differs from `sign`'s default
(`local-melange.rsa`). Not part of the repo's build flow.

## `melange package-version`

Purpose: print the target package id for a config, i.e.
`{{ .Package.Name }}-{{ .Package.Version }}-r{{ .Package.Epoch }}`.

Usage:

```bash
melange package-version [config.yaml]
```

Notes:

- Read-only; useful for scripting the expected apk filename. It is sugar over `melange query`.

## `melange bump`

Purpose: update a melange YAML to a new package version.

Notes:

- Do NOT use in this repo. `package.version` is `${{vars.version}}`; release CLI writes the
  release version to its vars file and the local task writes `0.0.0`. Change the supplying
  vars file rather than mutating `melange.yaml`.

## Other subcommands (present, unused here)

`index`, `initramfs`, `license-check`, `lint` (experimental), `query`, `scan`, `source`,
`test`, `update-cache`, `version`, and `completion` are not part of the repo's build path.
Consult `melange <cmd> --help` before relying on any of them.
