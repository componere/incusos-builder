---
title: About seed injection
description: Why incusos-builder locates the seed-data partition with a GPT probe, then splices a tar instead of rebuilding the disk
---

# About seed injection

A seeded IncusOS image is the stock installer with a tar of seed-config
YAML written into the `seed-data` partition. The rest of the disk —
ESP, verity metadata, and the signed OS payload — is supposed to be
byte-identical to the image that was fetched.

That constraint is why incusos-builder splices. Remaking the GPT or
copying files into a mounted filesystem would be a second author of the
installer. The builder's job is narrower: find `seed-data`, prove the
partition still starts where this IncusOS layout puts it, overwrite only
the tar-sized prefix of that partition, and stream everything else
through unchanged.

How IncusOS later *consumes* that tar is a boot-boundary question. See
[About the trust model](trust-model.md). This page is about the write.

## Why the offset is probed, then still required to match

The upstream web customizer hardcodes the seed-data start as
`2148532224` and seeks there. On release `202608102114` that number is
correct: GPT `seed-data` begins at first LBA 4 196 352 with a 512-byte
sector, which is 2 148 532 224 bytes, and the partition is 100 MiB.

Hardcoding the number without reading the table is a footgun if the
layout moves. The production path therefore does both:

1. Open the verified gzip, gunzip only the first 1 MiB, and parse GPT.
2. Refuse the image unless the located `seed-data` start equals
   `2148532224`.

Drift is a fetch failure, not a config error. The image is not the
layout this splice was written for. Tests inject a compact fixture
offset so they do not have to ship a 2 GiB prefix; production never
relaxes the constant.

The hand parser looks for `"EFI PART"` at LBA 1 for logical sector sizes
512, 2048, and 4096. Raw images use 512. Upstream's img-to-iso conversion
loop-mounts with `--sector-size 2048`, so the GPT header lives at byte
2048; the *byte* start of `seed-data` is still 2 148 532 224 because the
ESP remains 2 GiB after a 1 MiB alignment. 4096 is the remaining common
logical size. go-diskfs would still need that same probe: its regular-file
reader defaults to 512, and 2048 is not a named sector size. Parse cost
is noise next to decompression.

Names are UTF-16LE. An ASCII `"seed-data"` in the name field does not
match. Implausible entry counts, overflowed LBA arithmetic, and a missing
`seed-data` name all fail closed.

## The splice invariant

After the probe, the renderer emits an uncompressed USTAR. Each non-nil
seed-config section is one YAML file from `yaml.Dump(..., yaml.WithV2Defaults())`,
header fields `Name`, mode `0600`, and `Size` only. The nine web-customizer
sections use writeSeed's names and order. `kernel.yaml` and
`security.yaml` follow; the web service cannot emit those stems, but
incus-osd reads them. Closing the tar writer supplies the end-of-archive
blocks that are part of the returned size.

The renderer-reported size must equal `len(tar)`. A disagreement is a
programming error in the collaborator. The tar must also fit in the
partition length; overflow is a config error.

Splice then reopens the same verified handle and gunzips again:

1. Copy `[0, offset)` to the output.
2. Write the tar bytes.
3. Discard `len(tar)` bytes from the source so the skipped region is
   exactly the overwritten prefix, not a guessed remainder.
4. Copy the rest of the stream.

Read-side shortfalls wrap as fetch errors (truncated image). Write-side
failures wrap as output errors. There is no bare `io.Copy` across those
ports. Equal total size alone would not catch a compensated skip; the
discard is what keeps the suffix aligned.

The seed-config tar occupies only the start of the partition. Whatever
stock bytes lived there are discarded. Nothing after `offset + len(tar)`
is rewritten. The 100 MiB length on current images is an observation,
not a second hardcoded constant.

## Observed layout and round-trip

On `202608102114` aarch64 `image-raw`, hand parse and go-diskfs agreed
on every populated partition. The table was:

| name | start byte | length |
|---|---|---|
| `esp` | 1 048 576 | 2 048 MiB |
| `seed-data` | **2 148 532 224** | 100 MiB |
| `…_verity_sig` | 2 253 389 824 | 16 KiB |
| `…_verity` | 2 253 406 208 | 100 MiB |
| `IncusOS_202608102114` | 2 358 263 808 | 1 024 MiB |

A 3 072-byte install+network tar and a 5 120-byte tar that also carried
`kernel.yaml` and `security.yaml` both spliced with output size equal to
the 3 432 026 112-byte input. Bytes at
`offset .. offset+len(tar)` were the input tar. Streamed SHA-256 of
`[0, offset)` and `[offset+len(tar), EOF)` matched source versus
spliced. Entries strict-decoded with `yaml.WithKnownFields()` into the
pinned upstream seed types.

The same byte offset accepted the same tar on the x86_64 raw image of
that release. ISO GPT was derived from `convert-img-to-iso.sh`, not
downloaded and probed in that spike.

Boot of that spliced aarch64 image reached systemd PID 1 and therefore
exercised the ESP (before the offset) and the dm-verity partitions
(after it). That is corroboration that the splice did not scramble those
regions. It is not a substitute for the region digests, and it is not
seed consumption: no guest wrote the install target, and later QEMU
runs — including Phase 5.2 on Linux — classified negative on that
oracle. The UKI command line has no `console=` and sets
`quiet loglevel=0 systemd.show_status=0`, so post-handoff serial
silence is expected.

## What injection is not

Splice does not mount the installer, does not rewrite the GPT, and does
not touch rescue media. Rescue media is a separate `RESCUE_DATA` image
built only for offline specs. Recovery looks up that volume by GPT
partlabel or filesystem label and reads `update/`, including a verbatim
`update.sjson`. Staging that tree is not injection into `seed-data`.

Injection also does not authenticate the stock image. Admission already
bound the gzip to its index digest; the probe only insists that *this*
verified image still has `seed-data` where the splice expects it. If
upstream moves the partition, the builder should fail, not guess.