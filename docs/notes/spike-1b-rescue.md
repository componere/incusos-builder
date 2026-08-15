# Spike 1.B — Pure-Go rescue media (go-diskfs)

Scratch code: `spikes/rescue/` (module `spike-rescue`, throwaway). Artifacts
(gitignored): `spikes/rescue/out/rescue.iso`, `spikes/rescue/out/rescue.img`.
Library: `github.com/diskfs/go-diskfs v1.9.4`.

Revised after review (`FindingsReview`): the first pass contained a
ReadDir-usage error and a wrong Joliet root cause; both corrected below with
re-run evidence.

Recovery boot validation is **pending the boot wave**; this spike covers
build + read-back + independent-reader evidence only.

## What upstream actually expects (grounded in reference/incus-os)

- `incus-osd/internal/recovery/recovery.go`: checks
  `/dev/disk/by-partlabel/RESCUE_DATA`, falls back to
  `/dev/disk/by-label/RESCUE_DATA`; accepts **vfat or iso9660**; reads the
  `update/` tree (`filepath.Join(updateDir, file.Filename)` — filenames may
  contain an `<arch>/` directory segment) and an optional **`hotfix.sh.sig`**
  (S/MIME-signed document at the partition root; a plain `hotfix.sh` is never
  read — `recovery.go:93-104`, upstream `doc/reference/recovery.md`).
- Upstream ISO: `mkisofs -V RESCUE_DATA -joliet-long -rock` (customizer
  `main.go:959`).
- Upstream raw: `sgdisk -n 1 -c 1:RESCUE_DATA` + `mkfs.vfat -S 512
  --offset=2048` + `mcopy -i img@@1048576` (image-publisher README) — GPT
  partlabel `RESCUE_DATA`, FAT at 1 MiB. Upstream sizes media as content
  + 1 MiB + 10 MiB padding and lets `mkfs.vfat` pick the FAT width (small
  selections get FAT16); recovery mounts `vfat` either way.

## DECISION

**go-diskfs suffices for both formats — provisional on the boot wave for the
recovery-path itself, with independent-reader confirmation in hand:**

1. **ISO: Rock Ridge only, no Joliet.** With `Joliet: true`, a
   Joliet-preferring reader (macOS) showed 8.3-style names carrying the
   ISO-9660 `;` version separator rendered as mojibake — i.e. the Joliet tree
   go-diskfs writes is not presenting usable long names. Root cause in
   go-diskfs is **undetermined** (its `ucs2StringToBytes` is explicitly
   big-endian, so a simple byte-swap explanation is wrong; the observed bytes
   decode as primary-tree 8.3 ASCII names read as UCS-2). Not worth chasing:
   Linux recovery prefers Rock Ridge, which go-diskfs writes correctly —
   **independently confirmed** by libarchive (`bsdtar -tf rescue.iso` lists
   full case-sensitive long names, including the nested tree; output below).
   Upstream's `-joliet-long` serves Windows cosmetics only. Phase 3b writes
   RR-only.
2. **FAT32 raw exactly as planned works.** GPT (`ProtectiveMBR: true`) + one
   partition `Name: "RESCUE_DATA"` (Type MicrosoftBasicData) at sector 2048
   (1 MiB), FAT32 volume label `RESCUE_DATA`. Only friction:
   `gpt.Partition.Index` must be set (1-based).

The pre-approved fallback (mkisofs passthrough) is **not needed on current
evidence**; final confirmation rides on the boot wave's recovery check
(ARCHITECTURE §10.1 makes the boot spike the acceptance criterion).

## Working call sequences (proven)

- ISO: `diskfs.Create(path, size, diskfs.SectorSize(2048))` →
  `CreateFilesystem({Partition: 0, TypeISO9660, VolumeLabel})` → mkdir
  shallow-to-deep, write tree → `iso.Finalize({RockRidge: true,
  VolumeIdentifier: "RESCUE_DATA"})` → truncate file to PVD
  volume-space-size × 2048.
- Raw: `diskfs.Create(path, size, SectorSize512)` → `d.Partition(&gpt.Table{
  ProtectiveMBR: true, Partitions: [{Index: 1, Start: 2048, End: …,
  Type: MicrosoftBasicData, Name: "RESCUE_DATA"}]})` →
  `CreateFilesystem({Partition: 1, TypeFat32, VolumeLabel: "RESCUE_DATA"})` →
  mkdir + write tree.

**Gaps this spike does NOT close** (Phase 3b must handle):

- Write shape: files were written with single in-memory `Write`s of ≤5 MiB;
  the production `RescueWriter` streams from `VerifiedAsset.Open()` handles.
  Chunked `io.Copy` of a ~273 MB (`aarch64/incus.raw.gz`) or ~567 MB
  (`x86_64/migration-manager.raw.gz`) asset through go-diskfs fat32/iso9660
  is unmeasured.
- Media sizing policy: upstream = content + 11 MiB with mkfs-chosen FAT
  width; we must pick FAT32 sizing (floor below) or match upstream's FAT16
  behavior for small selections — a deliberate, documented divergence either
  way.

## go-diskfs gotchas Phase 3b must encode

| # | Gotcha | Detail |
|---|--------|--------|
| 1 | ISO block size | `diskfs.Create` for iso9660 must use `SectorSize(2048)` or `CreateFilesystem` errors |
| 2 | Backing size alignment | The created file keeps its full workspace size; a non-2048-multiple size makes hdiutil reject the ISO ("image not recognized"). Fix: align, then truncate to PVD size (uint32 LE at PVD byte 80 × 2048) after `Finalize` |
| 3 | Joliet tree unusable | See DECISION #1; write RR-only |
| 4 | `ReadDir` takes unrooted `io/fs`-style paths | `ReadDir("/update")` fails `fs.ValidPath` with `invalid argument`; `ReadDir("update")` works. `OpenFile` does NOT validate and accepts rooted paths — asymmetric API, easy to misread as a read-side bug (this spike's first pass did). Directory walks ARE available for read-back verification |
| 5 | ISO label padding | `fs.Label()` returns the raw 32-byte NUL/space-padded PVD volume identifier; trim before compare |
| 6 | FAT32 minimum | FAT32 needs ≥65525 clusters. This is load-bearing, not a corner case: live single-app offline selections can be tiny (`aarch64/debug.raw.gz` 5,529,210 B; `aarch64/openfga.raw.gz` 22,884,340 B — spike 1.C index). Spike used a 256 MiB partition; 3b must enforce a floor ≈ 256 MiB (65525 × 4 KiB clusters + FAT overhead) or emit FAT16 for small media like upstream |
| 7 | `gpt.Partition.Index` | Must be set (1-based) or partition table write fails |
| 8 | Nested dirs | `Mkdir` is single-level; create `update/<arch>/` parents shallow-to-deep. Nested tree proven on both filesystems (below) |

## Read-back verification (go-diskfs, recursive walk + hashes)

Both images reopened read-only; GPT partlabel / FS labels checked; recursive
`ReadDir` walk enumerates the tree incl. `update/aarch64/`; every staged file
hash-verified. Staged `update.sjson`/`update.json` use the real live byte
sizes from spike 1.C (14,268 / 11,859).

```
fs type=1 label="RESCUE_DATA" (raw "RESCUE_DATA\x00…")   # iso9660
  ReadDir(update): 6 entries: IncusOS_test.efi.gz IncusOS_test.usr-x86-64.raw.gz aarch64 debug.raw.gz update.json update.sjson
  ReadDir(update/aarch64): 2 entries: debug.raw.gz incus.raw.gz
  update/aarch64/incus.raw.gz 4194304 bytes sha256 match
  update/aarch64/debug.raw.gz 1048576 bytes sha256 match
  update/update.sjson 14268 bytes sha256 match
  update/update.json 11859 bytes sha256 match
  update/IncusOS_test.efi.gz 3145728 bytes sha256 match
  update/IncusOS_test.usr-x86-64.raw.gz 5242880 bytes sha256 match
  update/debug.raw.gz 2097152 bytes sha256 match
verify-iso OK in 17ms

gpt: partlabel="RESCUE_DATA" type=EBD0A0A2-B9E5-4433-87C0-68B6B72699C7 start=1048576 bytes end=269484032 bytes
fs type=0 label="RESCUE_DATA" (raw "RESCUE_DATA")        # fat32
  (same tree walked, all 7 files sha256 match)
verify-raw OK in 39ms
```

## Independent Rock Ridge confirmation (libarchive)

`bsdtar -tf out/rescue.iso` — a non-go-diskfs RR reader — lists exact
case-sensitive long names including the nested arch dir:

```
update/debug.raw.gz
update/IncusOS_test.efi.gz
update/IncusOS_test.usr-x86-64.raw.gz
update/update.json
update/update.sjson
update/aarch64/debug.raw.gz
update/aarch64/incus.raw.gz
```

## Host mount evidence (macOS, supporting only)

- `rescue.img` (GPT+FAT32): `hdiutil attach -imagekey
  diskimage-class=CRawDiskImage` mounts; full filenames correct; hashes match.
- `rescue.iso` (RR-only): mounts; macOS shows 8.3 fallback names because
  macOS does not implement Rock Ridge — cosmetic; libarchive/Linux read RR.

## Timings (feeds 1.E)

build-iso ~55 ms, build-raw ~85 ms, verify ~20–40 ms on a 15.7 MiB payload /
258 MiB raw image. **Extrapolation caveat:** streaming hundreds of MB through
go-diskfs is unmeasured (see gaps above); construction cost still expected to
be dominated by asset download, not filesystem writing.
