---
title: Cache reference
description: Location, layout, digest keys, and admission behavior of the incusos-builder download cache
---

# Cache reference

`incusos-builder` stores verified update-server blobs in a
content-addressed directory. HTTPS and local-mirror sources share this
layout. Handles always open the cache entry, never the download source
or mirror file, so a later change to the source cannot change bytes
behind an already issued asset.

Command flags are in [CLI](cli.md). Acquisition failures are exit `5`
([Automation](automation.md)).

## Location

`--cache-dir` / `INCUSOS_BUILDER_CACHE_DIR` selects the cache root.

Default: Go `os.UserCacheDir()` joined with `incusos-builder`. On
Linux, that is `$XDG_CACHE_HOME` when set, otherwise `$HOME/.cache`.
On macOS, that is `$HOME/Library/Caches`; macOS does not consult
`$XDG_CACHE_HOME`. If that directory cannot be determined, the
default is empty.

An empty resolved cache directory is an acquisition error
(`cache directory is required`). The path is made absolute at
construction. The root and `sha256/` subdirectory are created at the
start of the first admission attempt (`0o755`), even if that
admission later fails.

There is no cache subcommand and no TTL, prune, or size-limit cleanup.

## Layout

```text
<cache-dir>/
  .fetch-<random>          # admission temp; removed unless rename commits
  sha256/
    <64 lowercase hex>     # immutable verified blob, mode 0o444
```

The digest path is `<cache-dir>/sha256/<digest>`. `<digest>` is
the update file's SHA-256 after validation: exactly 64 lowercase
hexadecimal characters.

Admission writes a `.fetch-*` temp in the cache root. A failed
admission closes and removes the temp. The root and `sha256/`
directory remain. No digest file and no leftover `.fetch*` temp
remain after a failed admit.

## Digest keys

The cache key is the metadata SHA-256, not the filename or version.
Two files with different digests occupy two entries; the same digest
is one entry.

Before any URL or filesystem use, the source validates:

| Field | Rule |
|-------|------|
| version | Non-empty `[A-Za-z0-9._-]+`, not `.` or `..` |
| filename | Relative path; each `/` segment matches the version rule |
| sha256 | 64 lowercase hex |
| size | `0 < Size ≤ 8 GiB` |

A rejected field is an acquisition error (`untrusted metadata; possible tampering`)
and does not touch the cache. Size `0`, negative, or greater than 8 GiB
is rejected before the request.

## Hit, miss, and success

On each asset request:

1. If `<cache-dir>/sha256/<digest>` exists, its size equals the
   declared size, and a fresh SHA-256 of the file equals the declared
   digest, the entry is reused. No fetch runs.
2. Otherwise the source fetches (HTTPS GET of
   `<server>/<version>/<filename>`, or a local open of
   `<mirror>/<version>/<filename>`) and admits.

Admission copies into a `.fetch-*` temp while hashing, requires exactly
the declared size then EOF, requires the digest to match, sets mode
`0o444`, and renames onto `sha256/<digest>`. Success is that rename.
Later reads open that path. The declared size is the verified size.

A size or digest mismatch is
`asset failed size/digest admission; untrusted metadata; possible tampering`.
The temp is deleted. No digest entry is left.

If free space on the cache filesystem is below the declared size, the
progress reporter emits `warning: cache free space below asset size`
and continues. A space-query failure is ignored. Low space is never an
error.

## Override

`--cache-dir` / `INCUSOS_BUILDER_CACHE_DIR` is the implemented
override. There is no per-asset pin, no disable-cache flag, and no
in-place verify-from-mirror mode.

A local `--server` directory still admits into this cache. Verifying
the mirror file in place would let a later write change bytes behind a
handle.

## Implemented cleanup

The cache does not delete successful entries.

Implemented removal:

- Uncommitted `.fetch-*` temps after a failed or aborted admit.
- A cached file that fails reuse (wrong size or digest) is not issued.
  The next fetch admits again and `rename`s over the same digest path.
  The corrupt file is not deleted first; the successful rename replaces
  it.

HTTPS download retries (3 attempts, short backoff, context-cancellable)
apply to the fetch+admit pair. A checksum or size mismatch gets exactly
one clean re-download, then fails with the admission wording. Those
retries do not prune the cache.

Publication leftovers (`*.incusos-builder.bak` beside outputs) are not
cache entries. See [CLI `build`](cli.md#build).
