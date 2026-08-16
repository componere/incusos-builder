package testfixture

import (
	"encoding/binary"
	"hash/crc32"
	"unicode/utf16"
)

const (
	sectorSize        = 512
	gptPrimaryHdrLBA  = 1
	gptPrimaryPartLBA = 2
	gptSignature      = "EFI PART"
	gptRevision       = 0x00010000
	gptHeaderSize     = 92
	gptHeaderCRCOff   = 16
	gptCurrentLBAOff  = 24
	gptBackupLBAOff   = 32
	gptFirstUsableOff = 40
	gptLastUsableOff  = 48
	gptDiskGUIDOff    = 56
	gptPartLBAOff     = 72
	gptNPartsOff      = 80
	gptESizeOff       = 84
	gptArrayCRCOff    = 88
	gptNParts         = 128
	gptEntrySize      = 128
	gptEntryArrayLBAs = 32
	gptFirstUsableLBA = 34
	gptEntryTypeLen   = 16
	gptEntryGUIDOff   = 16
	gptEntryFirstOff  = 32
	gptEntryLastOff   = 40
	gptEntryNameOff   = 56
	gptEntryNameBytes = 72
	gptNameUTF16Count = 36
	mbrPartOff        = 446
	mbrTypeOff        = 4
	mbrStartLBAOff    = 8
	mbrSizeLBAOff     = 12
	mbrEntryLen       = 16
	mbrSignatureOff   = 510
	protectiveTypeEE  = 0xee
	seedPartName      = "seed-data"
)

// diskLayout is the LBA map of the synthetic GPT image.
type diskLayout struct {
	// TotalBytes is the decompressed image length.
	TotalBytes int64
	// SeedFirstLBA is the seed-data first LBA.
	SeedFirstLBA uint64
	// SeedLastLBA is the seed-data last LBA (inclusive).
	SeedLastLBA uint64
	// BackupEntriesLBA is the backup partition-array LBA.
	BackupEntriesLBA uint64
	// LastLBA is the backup-header LBA (last sector).
	LastLBA uint64
}

// fixtureLayout returns the GPT LBA map for [SeedStart] / [SeedLength].
func fixtureLayout() diskLayout {
	seedFirst := uint64(SeedStart / sectorSize)
	seedSectors := uint64(SeedLength / sectorSize)
	seedLast := seedFirst + seedSectors - 1
	backupEntries := seedLast + 1
	last := backupEntries + gptEntryArrayLBAs

	return diskLayout{
		TotalBytes:       int64(last+1) * sectorSize,
		SeedFirstLBA:     seedFirst,
		SeedLastLBA:      seedLast,
		BackupEntriesLBA: backupEntries,
		LastLBA:          last,
	}
}

// gptPrimary is the protective MBR, primary header, and 128-entry array
// (LBA 0 through 33, [gptFirstUsableLBA] sectors).
func gptPrimary(layout diskLayout) []byte {
	buf := make([]byte, gptFirstUsableLBA*sectorSize)
	writeProtectiveMBR(buf[:sectorSize], layout.LastLBA)
	entries := buf[gptPrimaryPartLBA*sectorSize:]
	writeSeedEntry(entries[:gptEntrySize], layout)
	arrayCRC := crc32.ChecksumIEEE(entries)
	writeGPTHeader(buf[sectorSize:gptPrimaryPartLBA*sectorSize], gptHeaderParams{
		currentLBA: gptPrimaryHdrLBA,
		backupLBA:  layout.LastLBA,
		partLBA:    gptPrimaryPartLBA,
		arrayCRC:   arrayCRC,
		lastUsable: layout.BackupEntriesLBA - 1,
	})

	return buf
}

// gptBackupEntries is the 32-sector backup partition array.
func gptBackupEntries(layout diskLayout) []byte {
	buf := make([]byte, gptEntryArrayLBAs*sectorSize)
	writeSeedEntry(buf[:gptEntrySize], layout)

	return buf
}

// gptBackupHeader is the last-sector backup GPT header.
func gptBackupHeader(layout diskLayout) []byte {
	buf := make([]byte, sectorSize)
	entries := gptBackupEntries(layout)
	writeGPTHeader(buf, gptHeaderParams{
		currentLBA: layout.LastLBA,
		backupLBA:  gptPrimaryHdrLBA,
		partLBA:    layout.BackupEntriesLBA,
		arrayCRC:   crc32.ChecksumIEEE(entries),
		lastUsable: layout.BackupEntriesLBA - 1,
	})

	return buf
}

// gptHeaderParams is the LBA-dependent half of a GPT header.
type gptHeaderParams struct {
	// currentLBA is this header's LBA.
	currentLBA uint64
	// backupLBA is the other header's LBA.
	backupLBA uint64
	// partLBA is the partition-array LBA for this copy.
	partLBA uint64
	// arrayCRC is CRC32 of the 128-entry array.
	arrayCRC uint32
	// lastUsable is the last usable LBA.
	lastUsable uint64
}

// writeGPTHeader fills a 512-byte sector with a CRC'd GPT header.
func writeGPTHeader(sector []byte, p gptHeaderParams) {
	copy(sector[0:], []byte(gptSignature))
	binary.LittleEndian.PutUint32(sector[8:], gptRevision)
	binary.LittleEndian.PutUint32(sector[12:], gptHeaderSize)
	binary.LittleEndian.PutUint64(sector[gptCurrentLBAOff:], p.currentLBA)
	binary.LittleEndian.PutUint64(sector[gptBackupLBAOff:], p.backupLBA)
	binary.LittleEndian.PutUint64(sector[gptFirstUsableOff:], gptFirstUsableLBA)
	binary.LittleEndian.PutUint64(sector[gptLastUsableOff:], p.lastUsable)
	copy(sector[gptDiskGUIDOff:], fixtureDiskGUID())
	binary.LittleEndian.PutUint64(sector[gptPartLBAOff:], p.partLBA)
	binary.LittleEndian.PutUint32(sector[gptNPartsOff:], gptNParts)
	binary.LittleEndian.PutUint32(sector[gptESizeOff:], gptEntrySize)
	binary.LittleEndian.PutUint32(sector[gptArrayCRCOff:], p.arrayCRC)
	crc := crc32.ChecksumIEEE(sector[:gptHeaderSize])
	binary.LittleEndian.PutUint32(sector[gptHeaderCRCOff:], crc)
}

// writeProtectiveMBR writes a UEFI protective MBR covering [1, lastLBA].
func writeProtectiveMBR(mbr []byte, lastLBA uint64) {
	entry := mbr[mbrPartOff : mbrPartOff+mbrEntryLen]
	entry[mbrTypeOff] = protectiveTypeEE
	binary.LittleEndian.PutUint32(entry[mbrStartLBAOff:], 1)
	binary.LittleEndian.PutUint32(entry[mbrSizeLBAOff:], mbrSectorCount(lastLBA))
	mbr[mbrSignatureOff] = 0x55
	mbr[mbrSignatureOff+1] = 0xaa
}

const uint32Max uint32 = 0xffffffff

// mbrSectorCount is the protective-MBR size field (uint32, saturating).
func mbrSectorCount(lastLBA uint64) uint32 {
	if lastLBA >= uint64(uint32Max) {
		return uint32Max
	}

	return uint32(lastLBA)
}

// writeSeedEntry fills one GPT entry for the seed-data partition.
func writeSeedEntry(entry []byte, layout diskLayout) {
	copy(entry[:gptEntryTypeLen], fixtureTypeGUID())
	copy(entry[gptEntryGUIDOff:gptEntryGUIDOff+gptEntryTypeLen], fixturePartGUID())
	binary.LittleEndian.PutUint64(entry[gptEntryFirstOff:], layout.SeedFirstLBA)
	binary.LittleEndian.PutUint64(entry[gptEntryLastOff:], layout.SeedLastLBA)
	writeGPTName(entry[gptEntryNameOff:gptEntryNameOff+gptEntryNameBytes], seedPartName)
}

// writeGPTName encodes name as a GPT UTF-16LE partition name, NUL-padded.
func writeGPTName(dst []byte, name string) {
	clear(dst)
	units := utf16.Encode([]rune(name))
	for i := range gptNameUTF16Count {
		if i >= len(units) {
			break
		}
		binary.LittleEndian.PutUint16(dst[i*2:], units[i])
	}
}

// fixtureDiskGUID is a fixed mixed-endian disk GUID.
func fixtureDiskGUID() []byte {
	return []byte{
		0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
		0x10, 0x32, 0x54, 0x76, 0x98, 0xba, 0xdc, 0xfe,
	}
}

// fixtureTypeGUID is the Linux filesystem type GUID in GPT mixed-endian.
func fixtureTypeGUID() []byte {
	return []byte{
		0xaf, 0x3d, 0xc6, 0x0f, 0x83, 0x84, 0x72, 0x47,
		0x8e, 0x79, 0x3d, 0x69, 0xd8, 0x47, 0x7d, 0xe4,
	}
}

// fixturePartGUID is a fixed unique partition GUID.
func fixturePartGUID() []byte {
	return []byte{
		0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10,
		0xef, 0xcd, 0xab, 0x89, 0x67, 0x45, 0x23, 0x01,
	}
}
