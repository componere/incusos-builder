---
title: How to build offline media
description: Publish a seeded installer and RESCUE_DATA rescue media from one offline seed config
---

# How to build offline media

Publish both artifacts of an offline build: the seeded installer and
the `RESCUE_DATA` rescue media.

## Prerequisites

- `incusos-builder` on `PATH`.
- A [seed config](../reference/configuration.md) with `image.offline:
  true` and at least one `seeds.applications.applications` entry.
- An update source: the default HTTPS server, another `https://` URL,
  or an [existing local mirror directory](./use-local-mirror.md).
- A writable directory for `-o` and the rescue-media file. Offline
  builds cannot use `-o -`.
- A content-addressed cache directory. The default is Go
  `os.UserCacheDir()` joined with `incusos-builder`: `$XDG_CACHE_HOME`
  or `$HOME/.cache` on Linux, `$HOME/Library/Caches` on macOS. Pass
  `--cache-dir` if that default is empty.

## 1. Write the seed config

`image.type` selects both the installer format and the rescue-media
format. `iso` writes iso9660 rescue media. `raw` writes GPT+FAT32
rescue media. The two formats cannot be mixed in one build.

```yaml
version: 1
image:
  type: raw
  architecture: x86_64
  channel: stable
  offline: true
seeds:
  applications:
    applications:
      - name: incus
```

Omit `image.channel` to use `stable`. Set `image.release` to pin an
exact update version.

A config with `image.offline: true` and no applications is rejected
before any download:

```bash
incusos-builder build -f missing-apps.yaml -o out.img
```

```text
invalid config: seeds.applications: required when image.offline is true
```

Exit status is `3`.

## 2. Build both artifacts

```bash
incusos-builder build --json \
  -f offline.yaml \
  -o /absolute/path/seeded.img
```

That command writes:

| Artifact | Path |
| --- | --- |
| Seeded installer | `/absolute/path/seeded.img` |
| Rescue media | `/absolute/path/seeded.resources.img` |

The default rescue-media path is `<stem>.resources.<ext>` beside `-o`.
`<stem>` is the `-o` basename with its last extension removed. `<ext>`
follows `image.type`, not the `-o` suffix: `iso` → `iso`, `raw` →
`img`.

Set the rescue-media path explicitly:

```bash
incusos-builder build --json \
  -f offline.yaml \
  -o /absolute/path/seeded.img \
  --resources-output /absolute/path/rescue-data.img
```

The two paths must be distinct after cleaning. `-` is rejected for
the rescue-media path.

Streaming the installer to stdout is a usage error:

```bash
incusos-builder build -f offline.yaml -o -
```

```text
usage error: offline builds cannot use -o -
```

Exit status is `2`. `--json` with `-o -` is also rejected, even for an
online config: `usage error: --json cannot be combined with -o -`.

`--resources-output` on a config with `image.offline: false` (or
omitted) is a usage error:

```text
usage error: --resources-output requires offline: true in the config
```

## 3. Record the published hashes

`--json` writes one `{"result":{...}}` envelope to stdout. Offline
success includes:

- `result.output` and `result.resources_output`: the published paths
- `result.type`, `result.architecture`, `result.version`,
  `result.channel`, `result.seed_bytes`
- `result.sha256`: lowercase hex digest of the stored installer
- `result.resources_sha256`: lowercase hex digest of the stored
  rescue media

Confirm the published hashes against the files. On Linux and macOS 26
or later, run:

```bash
sha256sum -- /absolute/path/seeded.img /absolute/path/seeded.resources.img
```

On older macOS, run:

```bash
shasum -a 256 -- /absolute/path/seeded.img /absolute/path/seeded.resources.img
```

`result.resources_sha256` authenticates the rescue-media bytes from
that invocation. It is not a reproducibility guarantee. Raw rescue
media is not byte-reproducible across builds because go-diskfs
generates a random GPT disk GUID and FAT32 volume serial. In five
builds from one config, the installer digest was identical and all
five rescue-media digests differed.

On macOS, mount raw rescue media read-only before inspecting it:

```bash
hdiutil attach -readonly /absolute/path/seeded.resources.img
```

Alternatively, inspect a copy. A read-write mount makes macOS create
`.fseventsd/fseventsd-uuid` in the FAT32 volume and changes the
rescue-media digest. Do not compare a post-mount digest to
`result.resources_sha256`.

Without `--json`, the same fields print as a human summary
(`output`, `resources_output`, `type`, `architecture`, `version`,
`channel`, `seed_bytes`, `sha256`, `resources_sha256`). `-q` suppresses
that summary.

## 4. Choose ISO or FAT32 rescue media

| `image.type` | Installer | Rescue media |
| --- | --- | --- |
| `iso` | Seeded ISO | iso9660, Rock Ridge, volume label `RESCUE_DATA` |
| `raw` | Seeded raw disk | GPT disk, one Microsoft Basic Data partition named `RESCUE_DATA` starting at 1 MiB, FAT32 volume label `RESCUE_DATA` |

Use `iso` when the rescue file must be an ISO image. Keep the same
seed config and set `image.type: iso`:

```bash
incusos-builder build --json \
  -f offline.yaml \
  -o /absolute/path/seeded.iso
```

That writes `/absolute/path/seeded.iso` and
`/absolute/path/seeded.resources.iso`. Use `raw` when the rescue file
must be a GPT+FAT32 disk image. The builder always emits FAT32 for
raw rescue media; the FAT partition is at least 256 MiB.

The rescue-media tree is:

```text
update/update.json
update/update.sjson
update/<filename>
```

`<filename>` keeps the update-server path, including a per-arch
prefix such as `update/x86_64/incus.raw.gz`. The builder does not
write `hotfix.sh.sig`.

## 5. Attach rescue media as data, not as firmware

The installer at `-o` is the bootable seeded image. The rescue-media
file is data-only `RESCUE_DATA` for an already-installed system. Do
not boot the host from the rescue-media file.

After install, attach the rescue-media file as a second disk or ISO
and boot the installed system. IncusOS looks for a `vfat` or
`iso9660` volume labeled `RESCUE_DATA`, then reads signed updates
under `update/`.

Do not treat network traffic, a changed file size, or a log substring
as recovery success. Official IncusOS docs do not publish a stable
recovery success string. For a release-gate procedure that records
install and `RESCUE_DATA` detection, see
[How to verify boot acceptance](./verify-boot-acceptance.md).

## Overwrite behavior

If either final path already exists and you did not pass `--force`,
an interactive session prompts on stderr:

```text
overwrite existing output? [y/N]
```

Answer `y` or `yes` to replace both artifacts. Any other answer, or
end of input, refuses the build:

```text
usage error: refusing to overwrite /absolute/path/seeded.img, /absolute/path/seeded.resources.img; re-run with --force
```

Exit status is `2`. This example assumes both final paths exist. The
refusal lists each existing final, with the image first. Existing
files are left unchanged.

`--no-input`, a non-TTY stdin or stdout, or a set `CI` environment
variable disables the prompt and uses the same refusal.

Replace both files without a prompt:

```bash
incusos-builder build --json --force \
  -f offline.yaml \
  -o /absolute/path/seeded.img \
  --resources-output /absolute/path/rescue-data.img
```

`--force` moves each existing final aside to
`<path>.incusos-builder.bak`, then publishes rescue media first and
the installer last. A handled failure restores the previous pair.
After a successful publish, best-effort cleanup normally deletes the
`.bak` files. A backup remains usable only after an interruption in
the narrow publish window or when that cleanup cannot remove it.
Follow
[How to recover an interrupted --force build](./recover-interrupted-build.md)
before renaming or deleting a leftover backup.

A file that appears at a final path after the pre-check, when
replacement was not authorized, is refused with exit status `6`:

```text
output write failed: output appeared during the build; re-run with --force
```

## Related

- [How to use a local mirror](./use-local-mirror.md)
- [How to verify boot acceptance](./verify-boot-acceptance.md)
- [Configuration reference](../reference/configuration.md)
- [CLI reference](../reference/cli.md)
