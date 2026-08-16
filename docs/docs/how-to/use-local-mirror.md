---
title: How to use a local mirror
description: Point --server at an existing directory with the HTTPS update-server layout
---

# How to use a local mirror

Point `incusos-builder` at a local directory that uses the same layout
as the HTTPS update server.

## Prerequisites

- `incusos-builder` on `PATH`.
- An existing directory. `--server` selects the local adapter only
  when the path is already a directory. Create and populate the tree
  before the command runs.
- A content-addressed cache directory. Local assets are admitted into
  the same cache as HTTPS downloads. The default is
  `$XDG_CACHE_HOME/incusos-builder` (or the platform user cache
  directory plus `incusos-builder`). Pass `--cache-dir` if that
  default is empty.

## 1. Lay out the directory

The directory must match the HTTPS tree:

```text
<dir>/index.json
<dir>/<version>/<filename>
<dir>/<version>/update.json
<dir>/<version>/update.sjson
```

`<filename>` is the update-server path, which may include an
architecture segment. A complete `x86_64` raw tree looks like:

```text
mirror/index.json
mirror/202608160000/x86_64/IncusOS_202608160000.img.gz
mirror/202608160000/x86_64/incus.raw.gz
mirror/202608160000/update.json
mirror/202608160000/update.sjson
```

`index.json` is the update-server index. Each
`<dir>/<version>/<filename>` is the file named in that index.
`update.json` and `update.sjson` live directly under `<version>/`,
not under the architecture prefix.

Every version and path segment must match `[A-Za-z0-9._-]+`. Empty
segments, `.`, `..`, a leading `/`, and `\` are rejected.

## 2. Select the directory with `--server`

Pass the directory path. Relative paths are resolved to an absolute
directory. The value is not a `file://` URL.

```bash
incusos-builder versions \
  --server /absolute/path/mirror \
  --cache-dir /absolute/path/cache \
  --architecture x86_64
```

```bash
incusos-builder build --json \
  -f config.yaml \
  -o /absolute/path/seeded.img \
  --server /absolute/path/mirror \
  --cache-dir /absolute/path/cache
```

`--server` applies to `build` and `versions`. The equivalent
environment variable is `INCUSOS_BUILDER_SERVER`. The default when
neither is set is `https://images.linuxcontainers.org/os`.

## 3. Follow the HTTPS-versus-directory rule

`--server` is classified in this order:

1. A value whose lower-cased form starts with `http://` but not
   `https://` is a usage error.
2. A value whose lower-cased form starts with `https://` uses the
   HTTPS adapter.
3. An existing directory uses the local adapter.
4. Anything else is a usage error, including a missing path and a
   regular file.

Plain HTTP:

```bash
incusos-builder build -f config.yaml -o out.img \
  --server http://example.invalid/os
```

```text
usage error: --server "http://example.invalid/os": plain http is not supported; use https or a local mirror directory
```

Exit status is `2`.

A path that is not an existing directory:

```bash
incusos-builder build -f config.yaml -o out.img \
  --server /definitely-not-a-mirror
```

```text
usage error: --server "/definitely-not-a-mirror" is neither an https URL nor an existing directory
```

Exit status is `2`.

## 4. Confirm the required files

Online builds need `index.json` and the selected installer file under
`<version>/`. Offline builds also need every selected application
file plus both metadata documents:

| File | Used for |
| --- | --- |
| `<dir>/index.json` | Release list. Capped at 64 MiB. |
| `<dir>/<version>/<image filename>` | Seeded installer. |
| `<dir>/<version>/<application filename>` | Rescue-media payload when `image.offline` is `true`. |
| `<dir>/<version>/update.json` | Copied onto rescue media as `update/update.json`. |
| `<dir>/<version>/update.sjson` | Copied onto rescue media as `update/update.sjson`. |

`update.sjson` must be a `multipart/signed` S/MIME message whose
clear-text payload is an update document for that version. Its
`Files` list must include every selected application
`Filename`+`Sha256` pair.

A missing `index.json` fails acquisition (exit status `5`) with
`open index.json`. A missing asset fails with `open <filename>`.

List what the mirror actually publishes before building:

```bash
incusos-builder versions --json \
  --server /absolute/path/mirror \
  --cache-dir /absolute/path/cache \
  --architecture x86_64
```

An unknown `--channel` prints an empty `result.versions` list and
exits `0`. A build that pins a version the index does not carry in
that channel fails with exit status `5`:

```text
version not found: release "199901010000" not in channel "stable"; available: …
```

## 5. Expect digest checks on every asset

Each file listed in the index must have:

- `sha256`: exactly 64 lowercase hex characters
- `size`: greater than `0` and at most 8 GiB
- `filename`: the allowlisted relative path above

The local adapter copies `<dir>/<version>/<filename>` into
`<cache-dir>/sha256/<digest>` and issues a handle only after the
copied bytes match that size and digest. Later writes to the mirror
file do not change an already-issued handle.

A short file, a trailing extra byte, or a digest that does not match
the copied bytes fails with exit status `5`:

```text
acquisition failed: "<filename>": asset failed size/digest admission; untrusted metadata; possible tampering
```

The same wording is used when `index.json` names a digest that the
file on disk does not hash to. Rejected version, filename, or digest
strings also include `untrusted metadata; possible tampering`.

`update.json` whose `Version` does not equal the requested version,
or `update.sjson` that omits a selected `Filename`+`Sha256` pair,
fails the same way (exit status `5`). Trailing data after the first
JSON value in `index.json` is rejected.

An empty `--cache-dir` fails with `cache directory is required`.

## Related

- [How to build offline media](./build-offline-media.md)
- [CLI reference](../reference/cli.md)
- [Configuration reference](../reference/configuration.md)
