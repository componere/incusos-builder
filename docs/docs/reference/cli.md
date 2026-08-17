---
title: CLI reference
description: incusos-builder commands, flags, operands, and defaults
---

# CLI reference

`incusos-builder` builds seeded IncusOS installation media from a YAML
[seed config](configuration.md). Its product commands are `build`,
`validate`, `versions`, and `init`; none accepts positional operands.
Cobra also lists its built-in `completion` and `help` commands in
`--help`. Those framework utilities are not product commands. An extra
operand or an unrecognized command is a usage error (exit `2`).

Process exit codes, JSON envelopes, environment variables, stdout and
stderr, `-`, TTY detection, and `--no-input` are defined in
[Automation](automation.md). The download cache is defined in
[Cache](cache.md).

## incusos-builder

```text
incusos-builder [flags]
incusos-builder [command]
```

Build seeded IncusOS installation media from a YAML config.

A root invocation with no command returns success and writes nothing.
`--help` prints usage. `-v` and `--version` write two lines to stdout:

```text
incusos-builder <version> (<commit>) built <date>
incus-os API: <incus-osd module version>
```

Unset linker metadata is `dev`, `none`, and `unknown`. A missing
`github.com/lxc/incus-os/incus-osd` module version is `unknown`.

### Persistent flags

These flags are recognized by every command, although some affect only
the commands identified below. Six follow
`flag` > `INCUSOS_BUILDER_*` > default. `--verbose` and `--quiet` are
flags only.

| Flag | Type | Default | Environment | Description |
|------|------|---------|-------------|-------------|
| `--color` | string | `auto` | `INCUSOS_BUILDER_COLOR` | Color output: `auto`, `always`, or `never` |
| `--progress` | string | `auto` | `INCUSOS_BUILDER_PROGRESS` | Percentage or bar updates: `auto`, `always`, or `never` |
| `--no-input` | bool | `false` | `INCUSOS_BUILDER_NO_INPUT` | Disable all prompts |
| `--verbose` | bool | `false` | — | Add build-plan debug records after successful build work |
| `-q`, `--quiet` | bool | `false` | — | Suppress stdout human success |
| `--server` | string | `https://images.linuxcontainers.org/os` | `INCUSOS_BUILDER_SERVER` | Update source for `build` and `versions`: HTTPS URL or local mirror directory |
| `--cache-dir` | string | `<user-cache>/incusos-builder` | `INCUSOS_BUILDER_CACHE_DIR` | Content-addressed download cache directory |
| `--json` | bool | `false` | `INCUSOS_BUILDER_JSON` | Write one JSON envelope to stdout |

`--verbose` and `-q` together are a usage error. Invalid `--color` or
`--progress` values are usage errors.

`--progress` controls percentage or bar updates only. `--progress never`
does not suppress stderr step headers (`==> <step>`) or successful
completion lines (`done <step>`). A failed step has no `done <step>`
line. `-q` suppresses stdout human success, but it does not change
stderr step headers or percentage updates.

`--verbose` has observable output only after `build` work succeeds. It
adds two debug records: one for the resolved version and selected asset,
and one for the image and rescue-media output paths. It adds no output
to `validate`, `versions`, or `init`.

`--server` affects and is validated by `build` and `versions` only. For
those commands it must be an `https://` URL or an existing directory. A
plain `http://` URL is a usage error. Any other value is a usage error:
the quoted server is neither an https URL nor an existing directory.
`validate` and `init` ignore this flag.

`--cache-dir` is required at acquisition time. An empty resolved value
fails as an acquisition error. See [Cache](cache.md).

## build

```text
incusos-builder build [flags]
```

Build seeded IncusOS installation media from a YAML seed config. The
command takes no operands. `-f -` reads the seed config from stdin;
`-o -` writes the image to stdout.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-f`, `--config` | string | `""` (required) | Path to config YAML (`-` reads stdin) |
| `-o`, `--output` | string | `""` (required) | Image output path (`-` writes stdout) |
| `--resources-output` | string | `""` | rescue-media output path (offline builds) |
| `--force` | bool | `false` | Replace existing output files |

Missing `-f` or `-o` is a usage error. `--json` with `-o -` is a usage
error. An offline seed config with `-o -` is a usage error
(`offline builds cannot use -o -`). `--resources-output` on an online
seed config is a usage error. Image and rescue-media paths must be
distinct. `--resources-output` cannot be `-`.

In `build --help`, both `-f` and `-o` use `string` as their metavar.

When `image.offline` is true and `--resources-output` is empty, rescue
media is written beside the image as
`<stem>.resources.<iso|img>`: `iso` for `image.type: iso`, `img` for
`image.type: raw`.

An output path that ends in `.gz` is stored with pgzip after splice.
`result.sha256` is the digest of those stored bytes, including the gzip
footer.

Reading, probing, or splicing an already-verified image is classified as
acquisition. Cancellation during that work exits `5` with
`acquisition failed: context canceled`, including cancellation during
the `splice` step after acquisition has completed.

Existing finals without `--force`:

- Interactive input allowed: stderr prompt
  `overwrite existing output? [y/N] `. `y` or `yes` (any case) selects
  the `--force` publication path. Any other answer, including EOF, is a
  usage error (`refusing to overwrite …; re-run with --force`).
- `--no-input` or auto `--no-input`: no prompt; same usage error.

`--force` (or a confirmed replace) moves each existing final to
`<path>.incusos-builder.bak`, publishes both artifacts or neither, and
removes the backups on success. A leftover `.incusos-builder.bak` is
harmless; recovery is a rename. A file that appears at a final path
during a no-clobber publish is an output error
(`output appeared during the build; re-run with --force`).

Human success (stdout, unless `-q` or `-o -`) is a summary of
`output`, optional `resources_output`, `type`, `architecture`,
`version`, `channel`, `seed_bytes`, `sha256`, and optional
`resources_sha256`. `--json` writes the [build envelope](automation.md#build).
`-o -` writes only image bytes on stdout.

After build work succeeds, `--verbose` writes the two build-plan debug
records described under [persistent flags](#persistent-flags).

## validate

```text
incusos-builder validate [flags]
```

Validate a seed config without fetching images. The command takes no
operands. It performs no network or update-source validation. In
particular, it ignores `--server`, including values that `build` and
`versions` reject.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-f`, `--config` | string | `""` (required) | Path to config file, or `-` for stdin |

Missing `-f` is a usage error.

Human success (stdout, unless `-q`) is the line `configuration valid`.
`--json` writes the [validate envelope](automation.md#validate).

## versions

```text
incusos-builder versions [flags]
```

List IncusOS releases from `--server` that belong to `--channel` and
publish an `iso` or `raw` image for `--architecture`. The command
takes no operands. An unknown channel is an empty list, not an error.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--channel` | string | `stable` | Release channel to list |
| `--architecture` | string | host mapping | Architecture to list |

Host mapping: `amd64` → `x86_64`, `arm64` → `aarch64`, anything else →
`x86_64`. An empty `--architecture` keeps every architecture. An empty
`--channel` is treated as `stable`.

Human success (stdout, unless `-q`) is a table with columns
`Version`, `Channel`, `Architecture`, and `Type`. `--json` writes the
[versions envelope](automation.md#versions).

## init

```text
incusos-builder init [flags]
```

Write a starter seed config. Interactive mode collects image settings;
`--no-input` writes a commented example generated from the schema. The
command takes no operands.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-o`, `--output` | string | `config.yaml` | Output path (`-` writes stdout) |

`--json` with `-o -` is a usage error. An existing output path is a
usage error (`refusing to overwrite existing file …`). `init` has no
`--force`.

`--no-input` (including auto `--no-input`) writes `version: 1` and
`image.type: iso`, `architecture: x86_64`, `channel: stable`,
`offline: false`, plus commented `seeds` keys: `applications`,
`incus`, `install`, `migration-manager`, `network`,
`operations-center`, `provider`, `services`, `update`, `kernel`,
`security`.

When prompts are allowed, the form asks for image type (`iso` / `raw`),
architecture (`x86_64` / `aarch64`), channel (default `stable`), and
offline. An offline answer also asks for an Application name (default
`incus`) and writes it under `seeds.applications.applications`. Prompt
descriptions and prompts write to stderr. A non-empty `ACCESSIBLE`
environment variable selects the line-oriented prompt path. User abort
is a usage error (`init cancelled`).

Human success after a file write (stdout, unless `-q`) is
`wrote <path>`. `-o -` writes only the YAML on stdout. `--json`
writes the [init envelope](automation.md#init).
