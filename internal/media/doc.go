// Package media implements [build.RescueWriter] with go-diskfs: iso9660
// (Rock Ridge, no Joliet) and GPT+FAT32 raw media labeled RESCUE_DATA.
//
// WriteRescue never chooses paths. It writes into the caller-owned tmpPath
// after validating the input. An empty UpdateSJSON is refused up front
// (recovery.go:178–182: media without update.sjson is a silent no-op on
// the booted system). Every RescueAsset.RelPath must start with "update/"
// and then pass [update.ValidateFilename] on the whole relative path —
// each '/' segment matches [A-Za-z0-9._-]+, with empty / "." / ".."
// rejected. That admits the nested layout upstream buildImage writes
// (update/<arch>/name.raw.gz) while still blocking traversal, absolute
// paths, and URL-delimiter bytes. RelPaths that collide with
// update/update.json, update/update.sjson, or each other are refused.
// All failures wrap [build.ErrOutput].
//
// tmpPath is unlinked and recreated (diskfs.Create is O_EXCL). Callers
// must reopen it by path after WriteRescue returns: a file descriptor
// from before the call refers to the unlinked empty placeholder, and the
// new file's mode is 0666 masked by umask rather than [os.CreateTemp]'s 0600.
//
// Sizing is computed from Asset.Size() plus metadata bytes plus directory
// slack, before the image file is created:
//
//   - ISO: content + 8 MiB slack, rounded up to 2048. After Finalize the
//     backing file is truncated to the PVD volume-space-size × 2048 so
//     the on-disk ISO is block-aligned (hdiutil rejects a non-multiple).
//   - Raw: GPT protective MBR, one partition named RESCUE_DATA starting
//     at 1 MiB. The partition is max(content+1 MiB slack, 256 MiB), then
//     grown by 1/50 (~2%) for the two FAT tables that live inside it
//     (go-diskfs fat32.go: ~1.54% at 512-byte clusters ≤260 MiB, ~0.195%
//     at 4 KiB), then aligned to 512 bytes. The 256 MiB floor is 65525 ×
//     4 KiB clusters plus FAT/reserved overhead, matching the 1.B spike.
//     This is a deliberate divergence from upstream, which sizes
//     content+11 MiB and lets mkfs.vfat emit FAT16 for small selections;
//     we only emit FAT32.
//
// Spike 1.B gotchas encoded in this package: ISO SectorSize(2048); PVD
// truncate; Rock Ridge only (go-diskfs Joliet is unusable); FAT32 cluster
// floor; gpt.Partition.Index must be 1; Mkdir is single-level on FAT so
// parents are created shallow-to-deep. Tests walk with unrooted ReadDir
// paths (OpenFile accepts rooted paths; ReadDir does not).
package media
