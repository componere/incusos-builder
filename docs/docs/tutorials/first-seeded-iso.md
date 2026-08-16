---
title: Build your first seeded ISO
description: Write a seed config, validate it, and build a seeded IncusOS installer ISO
---

# Build your first seeded ISO

In this tutorial we will write a seed config, validate it, and build a
seeded IncusOS installer ISO with `incusos-builder`. By the end you will
have an `x86_64` ISO whose `seed-data` partition carries an `incus`
application seed.

This tutorial stops when the ISO is written. Booting the image and
proving IncusOS consumed the seed is a separate manual check.

## Prerequisites

- A clone of this repository and [mise](https://mise.jdx.dev/) installed. The
  project is not released, so we will build the CLI from source.
- Network access to the default update server,
  `https://images.linuxcontainers.org/os`.
- Enough free disk for the downloaded image, the content-addressed
  cache, and the ISO.

From the repository root, install the pinned toolchain, build the binary, and
create a scratch directory for the generated config and ISO:

```bash
mise install
mise x -- moon run root:build
IOB="$PWD/bin/incusos-builder"
WORK=$(mktemp -d)
cd "$WORK"
```

The commands below use the binary at `$IOB`. They write generated files under
`$WORK`, not in the source checkout.

## What we are building

A seed config that selects:

- `image.type: iso`
- `image.architecture: x86_64`
- `image.channel: stable`
- one application seed, `incus`

`incusos-builder build` fetches that IncusOS release, splices the seed
tar into the image's `seed-data` partition, and writes `seeded.iso`.

## 1. Confirm the CLI

```bash
"$IOB" --version
```

You should see two lines:

```text
incusos-builder <version> (<commit>) built <date>
incus-os API: <version pinned in go.mod>
```

The second line reports the incus-os API version pinned in `go.mod`.

From this checkout the first line is `incusos-builder dev (none) built unknown`.

## 2. Write a starter config

```bash
"$IOB" init --no-input -o config.yaml
```

You should see:

```text
wrote config.yaml
```

`--no-input` writes a deterministic example. The uncommented keys are
already a valid seed config: `iso`, `x86_64`, `stable`, online. The
`seeds` keys are comments only, so this file does not yet carry an
application seed.

`init` refuses an existing path (exit `2`) and has no `--force`. If
`config.yaml` is already there, choose another `-o` path.

## 3. Add the application seed

Replace `config.yaml` with this seed config:

```yaml
version: 1
image:
  type: iso
  architecture: x86_64
  channel: stable
seeds:
  applications:
    applications:
      - name: incus
```

That is the committed `internal/config/testdata/valid.yaml` fixture.
Omitted `image.offline` stays false, so this build does not write
rescue media.

## 4. Validate the seed config

```bash
"$IOB" validate -f config.yaml --color never
```

You should see:

```text
configuration valid
```

`validate` reads the file and checks the schema. It does not fetch an
image.

If a required field is wrong, the process exits `3` and names the field
path on stderr. For example, `image.type: disk` prints
`must be iso or raw`.

## 5. Build the ISO

```bash
"$IOB" build -f config.yaml -o seeded.iso --color never
```

`-f` and `-o` are required. The first run downloads the `stable`
`x86_64` ISO from `https://images.linuxcontainers.org/os` into the
cache directory (`--cache-dir`; when omitted, the user cache directory
plus `incusos-builder`). Later runs reuse the cache.

When the build finishes, stdout starts with `summary` and includes
these fields:

```text
summary
output  seeded.iso
type  iso
architecture  x86_64
version  <resolved IncusOS version>
channel  stable
seed_bytes  <tar size in bytes>
sha256  <64 lowercase hex characters>
```

`seed_bytes` is the size of the seed tar spliced into the `seed-data`
partition. It is greater than zero because we included
`seeds.applications`. `sha256` is the digest of the stored `seeded.iso`
bytes.

## 6. Confirm the artifact

```bash
test -f seeded.iso
```

The published output is only `seeded.iso`. An online ISO build does not
write a second resources file and does not build rescue media.

At an interactive terminal, an existing `seeded.iso` produces this prompt on
stderr:

```text
overwrite existing output? [y/N] 
```

Enter `y` or `yes` to replace the file. Any other answer refuses the overwrite
with exit `2`. With `--no-input`, in CI, or when either stdin or stdout is not a
TTY, `build` does not prompt and prints:

```text
usage error: refusing to overwrite seeded.iso; re-run with --force
```

The refusal exits `2` and leaves the file unchanged. To replace it without a
prompt, re-run with `--force`; otherwise, choose a new `-o` path.

## 7. Stop at the boot-acceptance boundary

`seeded.iso` is a built installer image. It is not proof that IncusOS
booted, applied the `incus` seed, or wiped the installer seed after
install.

Do not treat a changed file size, cache growth, or network traffic as
seed consumption. That check is the manual procedure in
[How to verify boot acceptance](../how-to/verify-boot-acceptance.md).
That checklist uses raw offline media on an `x86_64` Linux Incus host;
it is not the next command in this tutorial.

## What we learned

- `incusos-builder init --no-input` writes a valid starter seed config.
- A seeded ISO needs `version: 1`, `image.type`, and
  `image.architecture`. Channel defaults to `stable`.
- `validate -f` checks the seed config without fetching images.
- `build -f … -o …` writes the ISO and a summary with `seed_bytes` and
  `sha256`.
- A successful build is not boot acceptance.

## Next steps

- [How to encrypt a seed config with age and SOPS](../how-to/sops-encryption.md)
- [How to verify boot acceptance](../how-to/verify-boot-acceptance.md)
