package build

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"unicode/utf16"

	"github.com/klauspost/pgzip"

	"github.com/componere/incusos-builder/internal/update"
)

const (
	// productionSeedStart is the upstream customizer splice offset
	// (image-customizer sendOSImage remainder). The acquired image's
	// seed-data partition must start here.
	productionSeedStart int64 = 2148532224

	gptSignature      = "EFI PART"
	gptHeaderLen      = 92
	gptPartLBAOff     = 72
	gptNPartsOff      = 80
	gptESizeOff       = 84
	gptMinEntrySize   = 128
	gptMaxParts       = 4096
	gptEntryTypeLen   = 16
	gptEntryFirstOff  = 32
	gptEntryLastOff   = 40
	gptEntryNameOff   = 56
	gptEntryNameBytes = 72
	gptNameUTF16Count = 36
	gptHeadLimit      = 1 << 20
	sector512         = 512
	sector2048        = 2048
	sector4096        = 4096
	seedPartName      = "seed-data"
)

// seedPartition is the seed-data partition located by the GPT probe.
type seedPartition struct {
	// StartByte is the first byte of the partition (first LBA × sector size).
	StartByte int64
	// Length is the partition length in bytes.
	Length int64
}

// probe opens handle (Open #1), gunzips the decompressed head, and locates
// the GPT seed-data partition. Drift from [productionSeedStart] or an
// unreadable table wraps [update.ErrFetch] (read-side, ARCHITECTURE §6).
func probe(ctx context.Context, handle VerifiedAsset) (seedPartition, error) {
	return probeAt(ctx, handle, productionSeedStart)
}

// probeAt is probe with an explicit expected start byte.
func probeAt(ctx context.Context, handle VerifiedAsset, expectedStart int64) (seedPartition, error) {
	rc, err := handle.Open(ctx)
	if err != nil {
		return seedPartition{}, fmt.Errorf("%w: %w", update.ErrFetch, err)
	}
	defer func() { _ = rc.Close() }()

	zr, err := pgzip.NewReader(rc)
	if err != nil {
		return seedPartition{}, fmt.Errorf("%w: gzip: %w", update.ErrFetch, err)
	}
	defer func() { _ = zr.Close() }()

	part, err := parseGPT(zr)
	if err != nil {
		return seedPartition{}, fmt.Errorf("%w: %w", update.ErrFetch, err)
	}

	if part.StartByte != expectedStart {
		return seedPartition{}, fmt.Errorf(
			"%w: seed-data starts at byte %d, expected %d",
			update.ErrFetch,
			part.StartByte,
			expectedStart,
		)
	}

	return part, nil
}

// parseGPT reads a decompressed image head from r (it does not consume the
// whole stream past [gptHeadLimit]) and returns the seed-data partition.
// Logical sector sizes 512, 2048, and 4096 are probed for "EFI PART" at LBA1.
func parseGPT(r io.Reader) (seedPartition, error) {
	data, err := io.ReadAll(io.LimitReader(r, gptHeadLimit))
	if err != nil {
		return seedPartition{}, fmt.Errorf("read GPT head: %w", err)
	}

	secsz, hdrOff, err := findGPTHeader(data)
	if err != nil {
		return seedPartition{}, err
	}

	hdr := data[hdrOff : hdrOff+gptHeaderLen]
	partLBA := binary.LittleEndian.Uint64(hdr[gptPartLBAOff : gptPartLBAOff+8])
	nparts := binary.LittleEndian.Uint32(hdr[gptNPartsOff : gptNPartsOff+4])
	esize := binary.LittleEndian.Uint32(hdr[gptESizeOff : gptESizeOff+4])

	if esize < gptMinEntrySize || nparts == 0 || nparts > gptMaxParts {
		return seedPartition{}, fmt.Errorf("implausible GPT: nparts=%d esize=%d", nparts, esize)
	}

	entriesOff := int(partLBA) * secsz
	entriesLen := int(nparts) * int(esize)

	if entriesOff < 0 || entriesOff+entriesLen > len(data) {
		return seedPartition{}, fmt.Errorf(
			"GPT entry array at LBA %d does not fit in the streamed head",
			partLBA,
		)
	}

	entries := data[entriesOff : entriesOff+entriesLen]
	for i := range int(nparts) {
		entry := entries[i*int(esize) : (i+1)*int(esize)]
		if partitionTypeZero(entry[:gptEntryTypeLen]) {
			continue
		}

		name := gptName(entry[gptEntryNameOff : gptEntryNameOff+gptEntryNameBytes])
		if name != seedPartName {
			continue
		}

		first := binary.LittleEndian.Uint64(entry[gptEntryFirstOff : gptEntryFirstOff+8])
		last := binary.LittleEndian.Uint64(entry[gptEntryLastOff : gptEntryLastOff+8])
		if last < first {
			return seedPartition{}, fmt.Errorf("seed-data last LBA %d < first LBA %d", last, first)
		}

		return seedPartition{
			StartByte: int64(first) * int64(secsz),
			Length:    int64(last-first+1) * int64(secsz),
		}, nil
	}

	return seedPartition{}, fmt.Errorf("no seed-data partition in GPT")
}

// findGPTHeader returns the logical sector size and header byte offset.
func findGPTHeader(data []byte) (int, int, error) {
	for _, sz := range []int{sector512, sector2048, sector4096} {
		off := sz
		if off+gptHeaderLen > len(data) {
			continue
		}

		if string(data[off:off+len(gptSignature)]) == gptSignature {
			return sz, off, nil
		}
	}

	return 0, 0, fmt.Errorf("no EFI PART signature at LBA1 (512, 2048, or 4096)")
}

// partitionTypeZero reports whether the 16-byte type GUID is unset.
func partitionTypeZero(typeGUID []byte) bool {
	for _, b := range typeGUID {
		if b != 0 {
			return false
		}
	}

	return true
}

// gptName decodes a GPT partition name (36 UTF-16LE code units).
func gptName(b []byte) string {
	units := make([]uint16, gptNameUTF16Count)
	for i := range gptNameUTF16Count {
		units[i] = binary.LittleEndian.Uint16(b[i*2 : i*2+2])
	}

	n := gptNameUTF16Count
	for i, c := range units {
		if c == 0 {
			n = i

			break
		}
	}

	return string(utf16.Decode(units[:n]))
}
