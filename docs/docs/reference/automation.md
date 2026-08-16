---
title: Automation reference
description: Exit codes, JSON envelopes, environment variables, and non-interactive I/O for incusos-builder
---

# Automation reference

Machine contract for `incusos-builder`. Command and flag names are in
[CLI](cli.md). Cache paths are in [Cache](cache.md).

## Exit codes

The process maps the returned error to an exit code after the command
finishes. Help text is not reprinted on failure. The error is written
to stderr unless that write itself fails (exit `1`).

| Code | Condition |
|------|-----------|
| `0` | Success |
| `1` | Unexpected error, including a canceled context that is not an acquisition failure, or a failure writing the error line to stderr |
| `2` | Usage: unknown flags, missing required flags, invalid `--color` / `--progress`, `--verbose` with `-q`, `--json` with `-o -`, offline `-o -`, bad `--server`, overwrite refused, `init` cancel, and other usage errors |
| `3` | Invalid seed config, including an oversized seed tar at splice time |
| `4` | SOPS decryption failure after a top-level `sops` key |
| `5` | Acquisition or version resolution, including GPT-probe drift, empty cache directory, and a canceled fetch |
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
| `result.resources_output` | string | `--resources-output` path; omitted when online |
| `result.type` | string | `iso` or `raw` |
| `result.architecture` | string | `x86_64` or `aarch64` |
| `result.version` | string | Resolved update version |
| `result.channel` | string | Channel the version was selected from |
| `result.seed_bytes` | integer | Spliced seed-tar size |
| `result.sha256` | string | Lowercase hex SHA-256 of the stored image bytes |
| `result.resources_sha256` | string | Lowercase hex SHA-256 of the rescue media; omitted when online |

`sha256` matches a second hash of the published file. For a `.gz`
output it is the compressed stored bytes.

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
| stderr | Error reprint after every failure (including `--json`), overwrite prompt, `init` form, progress, log (`--verbose` debug, default warn and above, `-q` error only) |

`-q` suppresses the human success writers. It does not suppress
`--json`, `-o -` artifact bytes, or `init -o -` YAML.

`--progress auto` is pre-resolved to `never` unless both stdout and
stderr are TTYs. `--json` or `-o -` further forces AUTO progress to
`never`. Explicit `--progress always` / `never` is unchanged.

`--color auto` is not pre-resolved by the CLI. The reporter then
disables color if `NO_COLOR` is set, `TERM` is `dumb`, or the writer
is not a TTY. `always` and `never` override that.

## `-`

`-` is the reserved stream sentinel (`-` or a path that cleans to `-`).

| Use | Command | Meaning |
|-----|---------|---------|
| `-f -` | `build`, `validate` | Read the seed config from stdin |
| `-o -` | `build` | Write image bytes to stdout; no summary; no rescue media |
| `-o -` | `init` | Write the starter YAML to stdout; no `wrote` line |

`--resources-output -` is a usage error. `-` is not a valid image
publication path; streaming is handled by `-o -` on `build`.

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
