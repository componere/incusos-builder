---
title: How to recover an interrupted --force build
description: Inspect and restore .incusos-builder.bak files after an interrupted --force publish
---

# How to recover an interrupted --force build

Inspect the destination directory after a `build --force` that did
not finish, then restore or discard `<path>.incusos-builder.bak`
files only when the conditions below hold.

This procedure applies to file publication. `-o -` does not use the
publisher and creates no `.bak` files. `init` has no `--force`.

## Prerequisites

- The `incusos-builder` process that owned these paths is not still
  running. A live `build` may still rename or delete the same files.
- You know the intended `-o` path. For an offline seed config, also
  know `--resources-output`, or the default
  `<stem>.resources.<iso|img>` beside `-o` (`iso` when
  `image.type` is `iso`, `img` when it is `raw`).
- You can read and rename files in that directory.

Do not start another `--force` until you have inventoried any
existing `.bak` files. A new `--force` replaces a stale
same-named `.bak` with the current final.

## What `--force` does

`--force` (or a confirmed overwrite) selects the replace path. After
the new image and rescue media are fsynced and hashed in unique
temps, publish runs these steps:

1. Finish and hash the temps.
2. Rename the existing image final to
   `<image>.incusos-builder.bak`. A missing image is skipped.
3. Rename the existing rescue-media final to
   `<resources>.incusos-builder.bak`. A missing file is skipped.
   Online builds have no rescue-media artifact.
4. Claim the rescue-media path with `O_CREAT|O_EXCL`, then rename
   the complete temp over that claim.
5. Claim the image path the same way.
6. Best-effort delete the `.bak` files created this run.

Handled failures run cleanup before the process exits: drop any new
claim or published final, restore the rescue-media `.bak`, then
restore the image `.bak` last. Temps are removed. The error text
lists each rollback step (`removed new image`, `removed new
resources`, `restored resources`, `restored image`). Paths that
could not be removed or restored are listed after `leftover`.

A kill, crash, or power loss does not run that cleanup. The
directory can then hold any prefix of steps 2–6 plus
`.<base>-*.tmp` files in the same directory
(`os.CreateTemp` with pattern `.<base>-*.tmp`).

On a successful `--force`, leftover `.bak` files that step 6 could
not delete are harmless. The CLI success envelope does not list
them. Recovery is a rename or a delete after you accept the new
finals.

A file that appears at a final path during a non-`--force`
publish is an output error (`output appeared during the build;
re-run with --force`) and does not create a `.bak`.

## 1. Confirm the publisher is not running

If a `build` that targeted these paths is still running, wait for it
to exit. `Abort` after a handled failure already restores `.bak`
files and deletes temps.

## 2. Inventory finals, backups, and temps

In the destination directory, list the image path, the rescue-media
path if this was an offline build, every `*.incusos-builder.bak`,
and every `.<base>-*.tmp` next to those names.

```bash
IMAGE=/absolute/path/seeded.img
RESOURCES=/absolute/path/seeded.resources.img

for path in "$IMAGE" "$IMAGE.incusos-builder.bak" \
  "$RESOURCES" "$RESOURCES.incusos-builder.bak"; do
  if [ -e "$path" ]; then
    ls -l -- "$path"
    sha256sum -- "$path"
  fi
done
```

Temps use `os.CreateTemp` in the destination directory with pattern
`.<base>-*.tmp` (for `-o seeded.img`, names look like
`.seeded.img-1234567890.tmp`). List those names next to the finals.
Do not delete them in this step.

Omit the `RESOURCES` paths for an online build.

Treat a path that does not exist as absent. Do not create
placeholders. Do not rename or delete yet.

## 3. Classify the directory

Use the files that exist, not the original exit code. A crash leaves
no rollback notes.

A final that exists after an interrupt may be a complete artifact
or an empty `O_CREAT|O_EXCL` claim created just before the temp
rename. A claim is a 0-byte file. Check the size and the digest
from step 2 before you treat a final as published media.

| Image final | Image `.bak` | Rescue final | Rescue `.bak` | What you can conclude |
|-------------|--------------|--------------|---------------|------------------------|
| present | absent | present or n/a | absent | No leftover `--force` backup. Nothing to restore. |
| present | present | any | any | A previous image is in the `.bak`. Leave a non-empty final unless you have compared digests and want the previous generation. |
| absent | present | any | any | The previous image is in the `.bak`. The image final is not published. |
| any | any | absent | present | The previous rescue media is in the `.bak`. The rescue-media final is not published. |
| any | any | present | present | A previous rescue-media file is in the `.bak`. Confirm the final is not a 0-byte claim before you keep it. |
| absent | absent | — | — | No previous image was moved aside. Do not invent a restore. |

Temps can exist in any interrupted state. They are incomplete
publish inputs, not previous finals.

## 4. Restore a previous final only when the final is absent or a claim

Restore a `.bak` onto its final path only when all of these are
true:

- The `incusos-builder` process is not running.
- The `.bak` path exists.
- You want the previous generation back at the final name.
- The final path is absent, or it exists and is a 0-byte claim.

If the final is a 0-byte claim, remove that claim first. Do not
rename a `.bak` over a non-empty final.

```bash
if [ -e "$IMAGE.incusos-builder.bak" ]; then
  if [ ! -e "$IMAGE" ]; then
    mv -- "$IMAGE.incusos-builder.bak" "$IMAGE"
  elif [ ! -s "$IMAGE" ]; then
    rm -- "$IMAGE"
    mv -- "$IMAGE.incusos-builder.bak" "$IMAGE"
  fi
fi
```

For an offline build, apply the same test to the rescue-media pair
only when you also want that previous file back. A non-empty
rescue-media final may be the new artifact from step 4 even when
the image final is still missing. Leave that pair untouched unless
you have compared digests and chosen the previous generation.

Never rename a `.bak` over a non-empty final. That replaces the
new artifact with the previous one.

## 5. Remove temps after the restore decision

Delete `.<base>-*.tmp` files only when no `build` is running and you
have finished step 4. A temp is not a previous final and is not a
safe substitute for one. Re-run `build` for a new artifact.

Remove only the temp names you listed in step 2, for both the image
and the rescue-media basename when this was an offline build.

## 6. Discard leftover backups only after you accept the new finals

If both the final and its `.bak` exist, the final is non-empty, and
that digest is the artifact you want, delete or move the `.bak`:

```bash
if [ -s "$IMAGE" ] && [ -e "$IMAGE.incusos-builder.bak" ]; then
  rm -- "$IMAGE.incusos-builder.bak"
fi
```

If you might need the previous generation, move the `.bak` out of
the destination directory first. A later `--force` replaces a
same-named `.bak`.

## 7. Re-run the build

When the finals you want are either restored or absent, run `build`
again. Use a fresh `-o` path if you do not want to replace anything.
Use `--force` only when you intend to replace the current finals.

Non-interactive jobs must pass `--force` to replace existing files.
See [How to run incusos-builder in CI](./run-in-ci.md).

## Verification

After a restore, each recovered final exists, the matching
`.incusos-builder.bak` is gone, and `sha256sum` of the final matches
the digest you recorded from that `.bak` in step 2.

After you accept new finals, each intended output exists, leftover
`.bak` files you chose to delete are gone, and no `.<base>-*.tmp`
files remain for those names.

## Related

- [CLI reference](../reference/cli.md)
- [Automation reference](../reference/automation.md)
- [How to run incusos-builder in CI](./run-in-ci.md)
- [How to build offline media](./build-offline-media.md)
