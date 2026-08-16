package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/diskfs/go-diskfs/filesystem"

	"github.com/componere/incusos-builder/internal/build"
	"github.com/componere/incusos-builder/internal/update"
)

const (
	// volumeLabel is the ISO/FAT volume label and GPT partlabel recovery looks up.
	volumeLabel = "RESCUE_DATA"
	// treePrefix is the required RelPath prefix. Upstream buildImage joins
	// each UpdateFile.Filename onto update/, including an <arch>/ segment.
	treePrefix = "update/"
	// updateJSONPath is the unsigned twin recovery does not consult.
	updateJSONPath = "update/update.json"
	// updateSJSONPath is the signed document recovery requires.
	updateSJSONPath = "update/update.sjson"
	// copyBufSize is the reused staging buffer (P2).
	copyBufSize = 1 << 20
	// isoBlock is the ISO9660 logical block size (gotcha 1).
	isoBlock int64 = 2048
	// isoSlack covers the 32 KiB system area, volume descriptors, path
	// tables, directory records, and Rock Ridge entries. Computed from
	// file sizes alone would undersize the workspace-backed ISO.
	isoSlack int64 = 8 << 20
	// rawSector is the GPT/FAT logical sector size.
	rawSector int64 = 512
	// gptHead is the 1 MiB gap before the FAT partition (sector 2048).
	gptHead int64 = 1 << 20
	// gptTail leaves room for the secondary GPT.
	gptTail int64 = 1 << 20
	// fatSlack is extra space inside the partition for directory
	// clusters and rounding. Distinct from gptHead. FAT tables scale
	// with the partition and are reserved separately in rawSizesFor.
	fatSlack int64 = 1 << 20
	// fatTableOverheadDivisor grows the partition by 1/50 (~2%) to cover
	// the two FAT tables that live inside it. See rawSizesFor.
	fatTableOverheadDivisor int64 = 50
	// fat32Floor is the partition size that yields ≥65525 clusters even
	// at 4 KiB/cluster (65525×4096 plus FAT/reserved overhead ≈ 256 MiB).
	// go-diskfs uses 512-byte clusters below 260 MiB, so this is
	// conservative; the 1.B spike proved a 256 MiB partition. Upstream
	// lets mkfs.vfat pick FAT16 for small media; we do not.
	fat32Floor int64 = 256 << 20
	// isoPVDSector is the Primary Volume Descriptor's LBA.
	isoPVDSector int64 = 16
	// isoPVDSizeOff is the byte offset of volume-space-size (uint32 LE).
	isoPVDSizeOff = 80
	// isoPVDSizeLen is the width of volume-space-size.
	isoPVDSizeLen = 4
	// isoPartition is CreateFilesystem/GetFilesystem index 0 (whole image).
	isoPartition = 0
	// fatPartition is the GPT partition number passed to CreateFilesystem.
	fatPartition = 1
	// gptPartIndex is 1-based; 0 makes go-diskfs refuse to write the table
	// (gotcha 7).
	gptPartIndex = 1
)

// Writer implements [build.RescueWriter] using go-diskfs. The zero value is
// usable; [NewWriter] is the documented constructor.
type Writer struct{}

// NewWriter returns a [Writer] ready to build RESCUE_DATA media into a
// caller-owned tmpPath.
func NewWriter() *Writer {
	return &Writer{}
}

// WriteRescue builds iso9660 or GPT+FAT32 rescue media at tmpPath.
// Validation (empty sjson, RelPath allowlist, non-empty tmpPath, known
// type) runs before the image file is created or replaced.
func (w *Writer) WriteRescue(ctx context.Context, typ build.ImageType, in build.RescueInput, tmpPath string) error {
	if err := ctx.Err(); err != nil {
		return outputf("%w", err)
	}
	switch typ {
	case build.ImageTypeISO, build.ImageTypeRaw:
	default:
		return outputf("unknown image type %q", typ)
	}
	if tmpPath == "" {
		return outputf("empty tmpPath")
	}
	files, err := prepareTree(in)
	if err != nil {
		return err
	}
	if err := replaceFile(tmpPath); err != nil {
		return err
	}
	buf := make([]byte, copyBufSize)
	switch typ {
	case build.ImageTypeISO:
		return writeISO(ctx, tmpPath, files, buf)
	case build.ImageTypeRaw:
		return writeRaw(ctx, tmpPath, files, buf)
	default:
		return outputf("unknown image type %q", typ)
	}
}

// treeFile is one path to stage: a metadata document or a streamed asset.
type treeFile struct {
	// rel is the /-separated path under the media root (update/…).
	rel string
	// size is the exact byte count that must be written.
	size int64
	// asset is opened exactly once when non-nil.
	asset build.VerifiedAsset
	// bytes holds metadata documents written verbatim.
	bytes []byte
}

// prepareTree validates RescueInput and returns the files to stage, including
// the two metadata documents. It does not open handles or touch tmpPath.
func prepareTree(in build.RescueInput) ([]treeFile, error) {
	if len(in.UpdateSJSON) == 0 {
		return nil, outputf("empty UpdateSJSON; recovery would no-op (recovery.go:178-182)")
	}
	files := []treeFile{
		{rel: updateJSONPath, size: int64(len(in.UpdateJSON)), bytes: in.UpdateJSON},
		{rel: updateSJSONPath, size: int64(len(in.UpdateSJSON)), bytes: in.UpdateSJSON},
	}
	seen := map[string]struct{}{updateJSONPath: {}, updateSJSONPath: {}}
	for i, a := range in.Assets {
		if err := validateRelPath(a.RelPath); err != nil {
			return nil, err
		}
		if _, dup := seen[a.RelPath]; dup {
			return nil, outputf("asset %d: RelPath %q duplicates an already staged file", i, a.RelPath)
		}
		seen[a.RelPath] = struct{}{}
		if a.Asset == nil {
			return nil, outputf("asset %d (%s): nil handle", i, a.RelPath)
		}
		size := a.Asset.Size()
		if size < 0 {
			return nil, outputf("asset %s: negative size %d", a.RelPath, size)
		}
		files = append(files, treeFile{rel: a.RelPath, size: size, asset: a.Asset})
	}
	return files, nil
}

// validateRelPath requires RelPath to start with update/ (a file under that
// tree, including nested update/<arch>/…) and to pass [update.ValidateFilename]
// on every segment. Failures wrap [build.ErrOutput] without leaking
// [update.ErrFetch] into the exit-code mapping.
func validateRelPath(rel string) error {
	if !strings.HasPrefix(rel, treePrefix) || rel == treePrefix {
		return outputf("RelPath %q must start with %q and name a file under it", rel, treePrefix)
	}
	if err := update.ValidateFilename(rel); err != nil {
		return outputf("RelPath %q rejected: %s", rel, err.Error())
	}
	return nil
}

// replaceFile unlinks tmpPath so diskfs.Create can O_EXCL-create a new inode
// at the same path. The CLI's exclusive temp is an empty placeholder;
// go-diskfs refuses an already-existing path (gotcha encoded from spike
// 1.B's [os.Remove]). Callers must reopen tmpPath by path after WriteRescue
// returns: a file descriptor opened before the call refers to the unlinked
// empty placeholder (mode 0600 from [os.CreateTemp]). The replacement is
// created 0666 masked by umask.
func replaceFile(tmpPath string) error {
	err := os.Remove(tmpPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return outputWrap(err, "remove %s", tmpPath)
	}
	return nil
}

// stage writes files into fs: parents shallow-to-deep (gotcha 8: FAT Mkdir
// is single-level; ISO MkdirAll would tolerate a deep mkdir but we keep one
// sequence), then each file. Asset handles are opened exactly once.
func stage(ctx context.Context, fs filesystem.FileSystem, files []treeFile, buf []byte) error {
	if err := mkdirParents(fs, files); err != nil {
		return err
	}
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return outputf("%w", err)
		}
		if err := stageFile(ctx, fs, file, buf); err != nil {
			return err
		}
	}
	return nil
}

// mkdirParents creates every ancestor directory of files, shallow first.
func mkdirParents(fs filesystem.FileSystem, files []treeFile) error {
	seen := make(map[string]struct{})
	for _, file := range files {
		for dir := path.Dir(file.rel); dir != "." && dir != "/"; dir = path.Dir(dir) {
			seen[dir] = struct{}{}
		}
	}
	dirs := make([]string, 0, len(seen))
	for dir := range seen {
		dirs = append(dirs, dir)
	}
	sort.Slice(dirs, func(i, j int) bool {
		di, dj := strings.Count(dirs[i], "/"), strings.Count(dirs[j], "/")
		if di != dj {
			return di < dj
		}
		return dirs[i] < dirs[j]
	})
	for _, dir := range dirs {
		if err := fs.Mkdir("/" + dir); err != nil {
			return outputWrap(err, "mkdir /%s", dir)
		}
	}
	return nil
}

// stageFile streams one treeFile into fs. Metadata is written from the
// verbatim slice; assets are opened once and copied through buf.
func stageFile(ctx context.Context, fs filesystem.FileSystem, file treeFile, buf []byte) error {
	if file.asset == nil {
		return writeFile(ctx, fs, file.rel, bytes.NewReader(file.bytes), file.size, buf)
	}
	rc, err := file.asset.Open(ctx)
	if err != nil {
		return outputWrap(err, "open asset %s", file.rel)
	}
	writeErr := writeFile(ctx, fs, file.rel, rc, file.size, buf)
	closeErr := rc.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return outputWrap(closeErr, "close asset %s", file.rel)
	}
	return nil
}

// writeFile creates rel on fs and copies exactly want bytes from src.
func writeFile(ctx context.Context, fs filesystem.FileSystem, rel string, src io.Reader, want int64, buf []byte) error {
	f, err := fs.OpenFile("/"+rel, os.O_CREATE|os.O_RDWR)
	if err != nil {
		return outputWrap(err, "create %s", rel)
	}
	n, copyErr := streamCopy(ctx, f, src, buf)
	closeErr := f.Close()
	if copyErr != nil {
		return outputWrap(copyErr, "write %s", rel)
	}
	if closeErr != nil {
		return outputWrap(closeErr, "close %s", rel)
	}
	if n != want {
		return outputf("%s: wrote %d bytes, want %d", rel, n, want)
	}
	return nil
}

// streamCopy copies src to dst using buf until EOF. A final short read that
// delivers the last byte together with [io.EOF] is success. There is no bare
// [io.Copy] (P2).
func streamCopy(ctx context.Context, dst io.Writer, src io.Reader, buf []byte) (int64, error) {
	written := int64(0)
	for {
		if err := ctx.Err(); err != nil {
			return written, err
		}
		nr, rerr := src.Read(buf)
		if nr > 0 {
			nw, werr := dst.Write(buf[:nr])
			written += int64(nw)
			if werr != nil {
				return written, werr
			}
			if nw != nr {
				return written, io.ErrShortWrite
			}
		}
		if rerr != nil {
			if errors.Is(rerr, io.EOF) {
				return written, nil
			}
			return written, rerr
		}
	}
}

// contentBytes is the sum of staged file sizes (assets via Size(), metadata
// from the verbatim slices).
func contentBytes(files []treeFile) int64 {
	var total int64
	for _, file := range files {
		total += file.size
	}
	return total
}

// alignUp rounds n up to a multiple of block. block must be a power of two.
func alignUp(n, block int64) int64 {
	return (n + block - 1) &^ (block - 1)
}

// toUint64 converts a non-negative byte or sector count for GPT fields.
func toUint64(n int64) (uint64, error) {
	if n < 0 {
		return 0, outputf("negative size %d", n)
	}
	return uint64(n), nil
}

// outputf wraps a formatted error in [build.ErrOutput].
func outputf(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{build.ErrOutput}, args...)...)
}

// outputWrap wraps err in [build.ErrOutput] using err.Error() so inner
// sentinels such as [update.ErrFetch] do not leak into exit-code mapping.
func outputWrap(err error, format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	if err == nil {
		return fmt.Errorf("%w: %s", build.ErrOutput, msg)
	}
	return fmt.Errorf("%w: %s: %s", build.ErrOutput, msg, err.Error())
}
