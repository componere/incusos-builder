# incusos-builder

incusos-builder builds seeded IncusOS installation media from a YAML config. It is a local CLI alternative to the IncusOS web customizer at https://incusos-customizer.linuxcontainers.org/ui/.

## Status

incusos-builder is under initial development. This repository has no published GitHub release and no `v*` tag, so the ghd and GHCR install paths below have nothing to fetch yet. Build from source until a release exists.

After the first release, only the latest published release is supported. See [SECURITY.md](SECURITY.md).

## Install

### From source

Install [mise](https://mise.jdx.dev/), then:

```sh
git clone https://github.com/componere/incusos-builder.git
cd incusos-builder
mise install
go run ./cmd/incusos-builder --version
```

`mise install` provisions the pinned toolchain from `mise.toml` and `mise.lock`, including Go 1.26.4. `moon run root:build` writes `bin/incusos-builder`.

### From a GitHub release

After a release is published, ghd can install the verified binary:

```sh
ghd install componere/incusos-builder/incusos-builder \
  --store-dir "$HOME/.local/share/ghd/store" \
  --bin-dir "$HOME/.local/bin"
```

Pin a version with `componere/incusos-builder/incusos-builder@x.y.z`. To download one asset without installing it:

```sh
ghd download componere/incusos-builder/incusos-builder@x.y.z --output "$PWD/out"
```

These commands fail until a GitHub release exists with the assets declared in `ghd.toml`.

### Container image

After the release workflow has published an image for a tag:

```sh
docker pull ghcr.io/componere/incusos-builder:vX.Y.Z
docker run --rm ghcr.io/componere/incusos-builder:vX.Y.Z --version
```

Replace `vX.Y.Z` with a published tag. No image has been published yet.

## Quick start

From a source checkout, after `mise install`:

```sh
go run ./cmd/incusos-builder init --no-input
go run ./cmd/incusos-builder validate -f config.yaml
go run ./cmd/incusos-builder build -f config.yaml -o incusos.iso
```

`init --no-input` writes a starter seed config at `config.yaml` (`iso`, `x86_64`, channel `stable`, online). Interactive `init` collects those image settings instead. `build` splices the seed config into a seed-data partition on the installer image. Offline seed configs also produce rescue media; pass `--resources-output` for that path.

`build` fetches IncusOS images from `https://images.linuxcontainers.org/os` unless `--server` points at another HTTPS update server or a local mirror directory.

## Documentation

The user manual is the MkDocs tree under [`docs/docs/`](docs/docs/index.md). Start with that index for the tutorial, how-to guides, reference, and explanations.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Security

See [SECURITY.md](SECURITY.md).

## License

Licensed under [Apache-2.0](LICENSE-APACHE) or [MIT](LICENSE-MIT), at your option (`Apache-2.0 OR MIT`).
