package media

import (
	"context"
	"encoding/binary"
	"os"

	diskfs "github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/disk"
	"github.com/diskfs/go-diskfs/filesystem"
	"github.com/diskfs/go-diskfs/filesystem/iso9660"
)

// writeISO creates a 2048-byte-block ISO at tmpPath, stages files, finalizes
// Rock Ridge only (gotcha 3: go-diskfs Joliet presents unusable names), and
// truncates the backing file to the PVD size (gotcha 2).
func writeISO(ctx context.Context, tmpPath string, files []treeFile, buf []byte) error {
	size := isoBackingSize(files)
	d, err := diskfs.Create(tmpPath, size, diskfs.SectorSize(isoBlock))
	if err != nil {
		return outputWrap(err, "create iso %s", tmpPath)
	}
	closed := false
	defer func() {
		if !closed {
			_ = d.Close()
		}
	}()
	fs, err := d.CreateFilesystem(disk.FilesystemSpec{
		Partition:   isoPartition,
		FSType:      filesystem.TypeISO9660,
		VolumeLabel: volumeLabel,
	})
	if err != nil {
		return outputWrap(err, "create iso9660 filesystem")
	}
	if err := stage(ctx, fs, files, buf); err != nil {
		return err
	}
	iso, ok := fs.(*iso9660.FileSystem)
	if !ok {
		return outputf("iso filesystem type %T", fs)
	}
	// Joliet is deliberately off: go-diskfs v1.9.4 writes a Joliet tree
	// whose UCS-2 names decode as the ISO-9660 8.3 names. Linux recovery
	// prefers Rock Ridge, which go-diskfs writes correctly (spike 1.B,
	// independently confirmed by libarchive). Upstream mkisofs -joliet-long
	// is Windows cosmetics only.
	if err := iso.Finalize(iso9660.FinalizeOptions{
		RockRidge:        true,
		VolumeIdentifier: volumeLabel,
	}); err != nil {
		return outputWrap(err, "finalize iso9660")
	}
	if err := iso.Close(); err != nil {
		return outputWrap(err, "close iso filesystem")
	}
	if err := d.Close(); err != nil {
		return outputWrap(err, "close iso %s", tmpPath)
	}
	closed = true
	return truncateToPVD(tmpPath)
}

// isoBackingSize is content plus directory slack, aligned to 2048. A
// non-multiple makes hdiutil report "image not recognized" (gotcha 2).
func isoBackingSize(files []treeFile) int64 {
	return alignUp(contentBytes(files)+isoSlack, isoBlock)
}

// truncateToPVD shrinks tmpPath to volume-space-size (uint32 LE at PVD byte
// 80) × 2048. Everything past that is leftover workspace zeros.
func truncateToPVD(tmpPath string) error {
	fh, err := os.OpenFile(tmpPath, os.O_RDWR, 0)
	if err != nil {
		return outputWrap(err, "reopen iso %s", tmpPath)
	}
	pvd := make([]byte, isoBlock)
	_, err = fh.ReadAt(pvd, isoPVDSector*isoBlock)
	if err != nil {
		_ = fh.Close()
		return outputWrap(err, "read PVD")
	}
	isoBytes := int64(binary.LittleEndian.Uint32(pvd[isoPVDSizeOff:isoPVDSizeOff+isoPVDSizeLen])) * isoBlock
	if isoBytes <= 0 || isoBytes%isoBlock != 0 {
		_ = fh.Close()
		return outputf("invalid PVD volume-space-size %d", isoBytes)
	}
	if err := fh.Truncate(isoBytes); err != nil {
		_ = fh.Close()
		return outputWrap(err, "truncate iso to PVD size %d", isoBytes)
	}
	if err := fh.Close(); err != nil {
		return outputWrap(err, "close truncated iso")
	}
	return nil
}
