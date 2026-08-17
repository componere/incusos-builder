package media

import (
	"context"

	diskfs "github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/disk"
	"github.com/diskfs/go-diskfs/filesystem"
	"github.com/diskfs/go-diskfs/partition/gpt"
)

// writeRaw creates a GPT disk with one Microsoft Basic Data partition named
// RESCUE_DATA starting at 1 MiB, formats it FAT32 with the same volume
// label, and stages files. gpt.Partition.Index must be 1.
func writeRaw(ctx context.Context, tmpPath string, files []treeFile, buf []byte) error {
	partSize, diskSize := rawSizes(files)
	d, err := diskfs.Create(tmpPath, diskSize, diskfs.SectorSize512)
	if err != nil {
		return outputWrap(err, "create raw %s", tmpPath)
	}
	defer func() { _ = d.Close() }()
	start, err := toUint64(gptHead / rawSector)
	if err != nil {
		return err
	}
	end, err := toUint64((gptHead+partSize)/rawSector - 1)
	if err != nil {
		return err
	}
	table := &gpt.Table{
		LogicalSectorSize:  int(rawSector),
		PhysicalSectorSize: int(rawSector),
		ProtectiveMBR:      true,
		Partitions: []*gpt.Partition{{
			Index: gptPartIndex,
			Start: start,
			End:   end,
			Type:  gpt.MicrosoftBasicData,
			Name:  volumeLabel,
		}},
	}
	if err = d.Partition(table); err != nil {
		return outputWrap(err, "write gpt")
	}
	fs, err := d.CreateFilesystem(disk.FilesystemSpec{
		Partition:   fatPartition,
		FSType:      filesystem.TypeFat32,
		VolumeLabel: volumeLabel,
	})
	if err != nil {
		return outputWrap(err, "create fat32 filesystem")
	}
	return stage(ctx, fs, files, buf)
}

// rawSizes returns the FAT partition length and the full disk image length
// for the staged tree. See [rawSizesFor].
func rawSizes(files []treeFile) (int64, int64) {
	return rawSizesFor(contentBytes(files))
}

// rawSizesFor returns the FAT partition length and the full disk image
// length from content bytes alone. The partition is
// max(content+fatSlack, fat32Floor), then grown by 1/fatTableOverheadDivisor
// (~2%) so the two FAT tables and reserved sectors that live inside it
// still leave room for the payload.
func rawSizesFor(content int64) (int64, int64) {
	partSize := max(content+fatSlack, fat32Floor)
	partSize = alignUp(partSize+partSize/fatTableOverheadDivisor, rawSector)
	return partSize, gptHead + partSize + gptTail
}
