---
title: How to run incusos-builder in CI
description: Run validate and build non-interactively with --json, --no-input, and script-safe I/O
---

# How to run incusos-builder in CI

Run `incusos-builder` in a job that has no TTY and must not hang
on prompts. Parse one JSON envelope from stdout and branch on the
process exit code.

Flag names and defaults are in the [CLI reference](../reference/cli.md).
Exit codes, envelopes, environment variables, and stream rules are in
the [automation reference](../reference/automation.md).

## Prerequisites

- `incusos-builder` on `PATH`.
- A [seed config](../reference/configuration.md) file, or YAML on
  stdin for `-f -`.
- A writable directory for `-o`. Do not combine `--json` with `-o -`.
- A content-addressed cache directory. The default is
  `$XDG_CACHE_HOME/incusos-builder` (or the platform user cache
  directory plus `incusos-builder`). If that default is empty,
  acquisition fails with exit `5`. Pass `--cache-dir` or
  `INCUSOS_BUILDER_CACHE_DIR`.
- For an encrypted seed config, `SOPS_AGE_KEY` in the job
  environment. Absence of every SOPS key source is exit `4`. See
  [How to encrypt a seed config with age and SOPS](./sops-encryption.md).

## 1. Disable prompts

`--no-input` is on when any of these is true:

- `--no-input` or `INCUSOS_BUILDER_NO_INPUT=true`
- `CI` is non-empty
- stdin is not a TTY
- stdout is not a TTY

If `CI` is non-empty or either stream is not a TTY, prompts stay
off even if you pass `--no-input=false`. That flag only beats
`INCUSOS_BUILDER_NO_INPUT` when both streams are TTYs and `CI` is
unset.

With `--no-input` on, `build` does not prompt. An existing `-o` (or
rescue-media) file is a usage error unless you pass `--force`:

```text
refusing to overwrite out.img; re-run with --force
```

There is no `--yes`. `--force` is the non-interactive overwrite path.
`--force` renames each existing final to
`<path>.incusos-builder.bak` before publish. See
[How to recover an interrupted --force build](./recover-interrupted-build.md)
before you reuse a destination that already has a `.bak` you need.

`init` has no `--force`. An existing `-o` path is always a usage
error.

## 2. Reserve stdout for one JSON document

Pass `--json` (or `INCUSOS_BUILDER_JSON=true`). The process writes
exactly one JSON document to stdout, terminated by a newline. `-q`
does not change that document. Human summaries and tables are
omitted.

`--json` cannot be combined with `-o -` on `build` or `init` (exit
`2`). Offline seed configs cannot use `-o -` either.

Write the image to a file:

```bash
incusos-builder validate --json -f seed.yaml --color never
incusos-builder build --json \
  -f seed.yaml \
  -o "$PWD/seeded.img" \
  --cache-dir "$PWD/cache" \
  --color never \
  --progress never
```

`validate` performs no network I/O. `build` then fetches and
publishes.

Success for `validate`:

```json
{"result":{"valid":true,"type":"iso","architecture":"x86_64","offline":false}}
```

Success for an online `build` includes `result.output`,
`result.sha256`, `result.version`, and `result.channel`. An offline
build also includes `result.resources_output` and
`result.resources_sha256`. Compare `result.sha256` to a second hash
of the published file.

On failure, stdout is one error envelope and stderr reprints the
error text:

```json
{"error":{"code":3,"message":"invalid config: field seeds.install"}}
```

`error.code` is the same integer as the process exit code. Parse
stdout; do not scrape stderr for machine data.

## 3. Keep diagnostics on stderr

stdout holds only the envelope (or, without `--json`, the human
success line or table). stderr holds the `Execute` error reprint,
progress, and the Charm log.

In CI, pass `--color never`. `--progress auto` already becomes
`never` unless both stdout and stderr are TTYs, and `--json` forces
AUTO progress to `never`. `--progress never` makes that explicit.
`--progress always` still writes a progress line on stderr.

`-q` suppresses human success writers. It does not suppress `--json`
or error reprints. `--verbose` and `-q` together are a usage error.

## 4. Pin Viper-backed settings you care about

Precedence is flag, then `INCUSOS_BUILDER_*`, then the built-in
default. Only flags the process actually parsed are bound, so an
unparsed Cobra default cannot mask the environment.

| Setting | Flag | Environment |
|---------|------|-------------|
| JSON | `--json` | `INCUSOS_BUILDER_JSON` |
| No input | `--no-input` | `INCUSOS_BUILDER_NO_INPUT` |
| Server | `--server` | `INCUSOS_BUILDER_SERVER` |
| Cache | `--cache-dir` | `INCUSOS_BUILDER_CACHE_DIR` |
| Color | `--color` | `INCUSOS_BUILDER_COLOR` |
| Progress | `--progress` | `INCUSOS_BUILDER_PROGRESS` |

`--verbose` and `-q` have no environment mapping. `--json=false` and
`--no-input=false` beat a true environment value; `CI` and non-TTY
auto-on still apply to `--no-input` as in step 1.

`--server` must be an `https://` URL or an existing directory. A
plain `http://` URL is exit `2`. For a job-local tree, see
[How to use a local mirror](./use-local-mirror.md).

## 5. Feed the seed config from a file or stdin

`-f` is required on `validate` and `build`. `-f -` reads the seed
config from stdin (plain YAML or a SOPS document with a top-level
`sops` key). Decrypted bytes never touch the filesystem.

```bash
incusos-builder validate --json -f - --color never < seed.yaml
```

Do not use stdin for anything else in the same invocation. The
overwrite prompt is disabled under `--no-input`, so stdin is not
shared with a confirm read.

## 6. Branch on the exit code

`cli.Execute` maps the returned error after the command finishes:

| Code | Meaning in a job |
|------|------------------|
| `0` | Success |
| `1` | Unexpected error, or a failure writing the error line to stderr |
| `2` | Usage: missing flags, `--json` with `-o -`, overwrite refused, bad `--server` |
| `3` | Invalid seed config |
| `4` | SOPS decryption failed after a top-level `sops` key |
| `5` | Acquisition or version resolution, including an empty cache directory |
| `6` | Failure writing the image, rescue media, or publication |

Wrapped sentinels keep the same code. A typical job fails the step
on any non-zero status and records `error.message` from the envelope
when `--json` is set.

## 7. Overwrite only when the job owns the path

If a previous step already wrote `-o` (or the default
`<stem>.resources.<iso|img>` rescue-media file), either publish to a
fresh path or pass `--force`.

```bash
incusos-builder build --json \
  -f seed.yaml \
  -o "$PWD/seeded.img" \
  --force \
  --cache-dir "$PWD/cache" \
  --color never \
  --progress never
```

`--force` is not required when the destinations do not exist.

If the job is killed during `--force`, inspect
`<path>.incusos-builder.bak` before you delete anything. Follow
[How to recover an interrupted --force build](./recover-interrupted-build.md).

## Verification

A successful `--json` `validate` prints one document whose
`result.valid` is `true` and exits `0`. A successful `--json` `build`
prints one document that has `result.sha256` and exits `0`. The
published file's SHA-256 matches `result.sha256`. stdout contains no
`summary` line.

## Related

- [Automation reference](../reference/automation.md)
- [CLI reference](../reference/cli.md)
- [Cache reference](../reference/cache.md)
- [How to recover an interrupted --force build](./recover-interrupted-build.md)
- [How to build offline media](./build-offline-media.md)
- [How to encrypt a seed config with age and SOPS](./sops-encryption.md)
