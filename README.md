# incusos-builder

incusos-builder builds seeded IncusOS installation media from a YAML config. It is a local CLI alternative to the [IncusOS web customizer](https://incusos-customizer.linuxcontainers.org/ui/).

## Install

### Homebrew cask (macOS)

```sh
brew install --cask componere/tap/incusos-builder
incusos-builder --version
```

### Container image

The OCI image supports Linux containers on `amd64` and `arm64`. Set `DIGEST`
to the published manifest digest, then pull and run that exact image:

```sh
DIGEST='sha256:<published-manifest-digest>'
IMAGE="ghcr.io/componere/incusos-builder@${DIGEST}"
docker pull "$IMAGE"
docker run --rm "$IMAGE" --version
```

Pin the digest when the image must be repeatable.

For Scoop, native Linux repositories, direct archive downloads, and `ghd`, see
[How to install incusos-builder](docs/docs/how-to/install.md).

## Quick start

`init` and `build` write into the working directory, so run them in a scratch directory rather than a source checkout:

```sh
WORK=$(mktemp -d)
cd "$WORK"
incusos-builder init --no-input
incusos-builder validate -f config.yaml
incusos-builder build -f config.yaml -o incusos.iso
```

From a source build, set `IOB="$PWD/bin/incusos-builder"` before `cd`, then run `"$IOB"` in place of `incusos-builder`.

`init --no-input` writes a starter config for an online `iso` build targeting `x86_64` on channel `stable`. Interactive `init` prompts for those image settings instead.

Every `seeds:` section in the starter config is commented out. Building it unedited succeeds and reports `seed_bytes 1024`, an empty seed tar, so the installer starts with no seed applied. Uncomment and fill the sections you need before `build`.

`build` splices the seed config into a seed-data partition on the installer image. It fetches images from `https://images.linuxcontainers.org/os` unless `--server` points at another HTTPS update server or a local mirror directory.

An offline config also produces rescue media, at a path derived from `-o`. Use `--resources-output` to override that path. Passing `--resources-output` for an online config is a usage error and exits `2`.

## Commands

| Command | Purpose |
|---|---|
| `init` | Write a starter `config.yaml` |
| `validate` | Check a config without fetching images |
| `build` | Build installation media from a config |
| `versions` | List available IncusOS releases from the update server |

`--json` writes a single JSON envelope to stdout instead of human-readable output. See [automation](docs/docs/reference/automation.md) for the envelope shape and the exit-code table.

## Documentation

The user manual is the MkDocs tree under [`docs/docs/`](docs/docs/index.md). Start with that index for the tutorial, how-to guides, reference, and explanations.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Security

See [SECURITY.md](SECURITY.md).

## License

Licensed under [Apache-2.0](LICENSE-APACHE) or [MIT](LICENSE-MIT), at your option (`Apache-2.0 OR MIT`).
