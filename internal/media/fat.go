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
// label, and stages files. gpt.Partition.Index is 1 (gotcha 7).
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

// rawSizes returns the FAT partition length and the full disk image length.
// The partition is max(content+fatSlack, fat32Floor), aligned to 512 bytes.
func rawSizes(files []treeFile) (int64, int64) {
	partSize := max(contentBytes(files)+fatSlack, fat32Floor)
	partSize = alignUp(partSize, rawSector)
	return partSize, gptHead + partSize + gptTail
}
