// Package media implements [build.RescueWriter] with go-diskfs: iso9660
// (Rock Ridge, no Joliet) and GPT+FAT32 raw media labeled RESCUE_DATA.
//
// WriteRescue never chooses paths. It writes into the caller-owned tmpPath
// after validating the input. An empty UpdateSJSON is refused: media without
// update.sjson leaves the booted system silently unrecovered. Every
// RescueAsset.RelPath must start with "update/" and then pass
// [update.ValidateFilename] on the whole relative path — each '/' segment
// matches [A-Za-z0-9._-]+, with empty / "." / ".." rejected. That admits
// the nested layout update/<arch>/name.raw.gz while still blocking
// traversal, absolute paths, and URL-delimiter bytes. RelPaths that collide
// with update/update.json, update/update.sjson, or each other are refused.
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
//     grown by 1/50 (~2%) for the two FAT tables that live inside it,
//     then aligned to 512 bytes. The 256 MiB floor is 65525 × 4 KiB
//     clusters plus FAT/reserved overhead. Raw media is always FAT32.
package media
