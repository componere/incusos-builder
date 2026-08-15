# Spike 1.B — Pure-Go rescue media (go-diskfs)

Scratch code: `spikes/rescue/` (module `spike-rescue`, throwaway). Artifacts
(gitignored): `spikes/rescue/out/rescue.iso`, `spikes/rescue/out/rescue.img`.
Library: `github.com/diskfs/go-diskfs v1.9.4`.

Recovery boot validation is **pending the boot wave** (spike 1.E context); this
spike covers build + read-back + host-mount evidence only.

## What upstream actually expects (grounded in reference/incus-os)

- `incus-osd/internal/recovery/recovery.go`: checks
  `/dev/disk/by-partlabel/RESCUE_DATA`, falls back to
  `/dev/disk/by-label/RESCUE_DATA`; accepts **vfat or iso9660**; reads
  `update/` tree (and optional `hotfix.sh`).
- Upstream ISO: `mkisofs -V RESCUE_DATA -joliet-long -rock` (customizer
  `main.go:959`).
- Upstream raw: `sgdisk -n 1 -c 1:RESCUE_DATA` + `mkfs.vfat -S 512
  --offset=2048` + `mcopy -i img@@1048576` (image-publisher README) — i.e. GPT
  partlabel `RESCUE_DATA`, FAT at 1 MiB.

## DECISION

**go-diskfs suffices for both formats — with two shape adjustments:**

1. **ISO: Rock Ridge only, no Joliet.** go-diskfs v1.9.4 writes Joliet UCS-2
   names **byte-swapped** (little-endian; ECMA-119/Joliet requires big-endian).
   Evidence: macOS (Joliet-preferring) lists `䑅䉕䜮剁圻` for `debug.raw.gz`
   — decoding each UTF-16 code unit's bytes as big-endian ASCII yields
   `DEBUG.RAW;`. Linux recovery prefers Rock Ridge (correctly written), so
   Joliet is cosmetic for Windows only; upstream's `-joliet-long` is not
   load-bearing for recovery. Phase 3b writes RR-only.
2. **FAT32 raw exactly as planned works.** GPT (`ProtectiveMBR: true`) + one
   partition `Name: "RESCUE_DATA"` (Type MicrosoftBasicData) starting at
   sector 2048 (1 MiB), FAT32 with volume label `RESCUE_DATA`. No API friction
   beyond `gpt.Partition.Index` being required (1-based).

The pre-approved fallback (mkisofs passthrough) is **not needed**.

## Working call sequences (proven)

- ISO: `diskfs.Create(path, size, diskfs.SectorSize(2048))` →
  `CreateFilesystem({Partition: 0, TypeISO9660, VolumeLabel})` → write tree →
  `iso.Finalize({RockRidge: true, VolumeIdentifier: "RESCUE_DATA"})` →
  truncate file to PVD volume-space-size × 2048.
- Raw: `diskfs.Create(path, size, SectorSize512)` → `d.Partition(&gpt.Table{
  ProtectiveMBR: true, Partitions: [{Index: 1, Start: 2048, End: …,
  Type: MicrosoftBasicData, Name: "RESCUE_DATA"}]})` →
  `CreateFilesystem({Partition: 1, TypeFat32, VolumeLabel: "RESCUE_DATA"})` →
  write tree.

## go-diskfs gotchas Phase 3b must encode

| # | Gotcha | Detail |
|---|--------|--------|
| 1 | ISO block size | `diskfs.Create` for iso9660 must use `SectorSize(2048)` or `CreateFilesystem` errors |
| 2 | Backing size alignment | The created file keeps its full workspace size; a non-2048-multiple size makes hdiutil reject the ISO ("image not recognized"). Fix: align, then truncate to PVD size (uint32 LE at PVD byte 80 × 2048) after `Finalize` |
| 3 | Joliet writer broken | Byte-swapped UCS-2 (see DECISION); do not enable |
| 4 | `ReadDir` broken on read-back | go-diskfs cannot `ReadDir` its own iso9660 **or** fat32 output (`invalid argument`), while `OpenFile` by exact path works and returns byte-perfect content. Phase 5 read-back must verify by known paths (we always know the manifest), not directory walks |
| 5 | ISO label padding | `fs.Label()` returns the raw 32-byte NUL/space-padded PVD volume identifier; trim before compare |
| 6 | FAT32 minimum | FAT32 needs ≥65525 clusters; spike used a 256 MiB partition. Real media sized by content will normally exceed this; enforce a floor in 3b |
| 7 | `gpt.Partition.Index` | Must be set (1-based) or partition table write fails |

## Read-back verification (go-diskfs, by known path)

Both images reopened read-only; GPT partlabel/FS labels checked; every staged
file hash-verified:

```
fs type=1 label="RESCUE_DATA" (raw "RESCUE_DATA\x00…")   # iso9660
  update/update.sjson 14268 bytes sha256 match
  update/update.json 11859 bytes sha256 match
  update/IncusOS_test.efi.gz 3145728 bytes sha256 match
  update/IncusOS_test.usr-x86-64.raw.gz 5242880 bytes sha256 match
  update/debug.raw.gz 2097152 bytes sha256 match
verify-iso OK in 10ms

gpt: partlabel="RESCUE_DATA" type=EBD0A0A2-B9E5-4433-87C0-68B6B72699C7 start=1048576 bytes end=269484032 bytes
fs type=0 label="RESCUE_DATA" (raw "RESCUE_DATA")        # fat32
  (same five files, all sha256 match)
verify-raw OK in 24ms
```

Staged `update.sjson`/`update.json` use the real live byte sizes from spike
1.C (14,268 / 11,859).

## Host mount evidence (macOS, supporting only)

- `rescue.img` (GPT+FAT32): `hdiutil attach -imagekey
  diskimage-class=CRawDiskImage` mounts; **full filenames correct**; hashes
  match.
- `rescue.iso` (RR-only): mounts; macOS shows 8.3 fallback names
  (`UPDATE.SJS` etc.) because macOS does not implement Rock Ridge — cosmetic;
  Linux reads RR. Boot wave confirms the Linux view.
- Joliet build (before removal): mounted but mojibake names — the bug above.

## Timings (feeds 1.E)

- build-iso ~30 ms, build-raw ~90 ms, verify ~10–40 ms (10.5 MiB / 258 MiB
  media). Media construction is negligible next to image download/splice.
