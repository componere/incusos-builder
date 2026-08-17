package build

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"math/bits"
	"unicode/utf16"

	"github.com/klauspost/pgzip"

	"github.com/componere/incusos-builder/internal/errdefs"
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
// the GPT seed-data partition. Drift from expectedStart or an unreadable
// table wraps [errdefs.ErrFetch]. Production Build passes
// [productionSeedStart]; tests inject a compact fixture offset.
func probe(ctx context.Context, handle VerifiedAsset, expectedStart int64) (seedPartition, error) {
	rc, err := handle.Open(ctx)
	if err != nil {
		return seedPartition{}, fmt.Errorf("%w: %w", errdefs.ErrFetch, err)
	}
	defer func() { _ = rc.Close() }()

	zr, err := pgzip.NewReader(rc)
	if err != nil {
		return seedPartition{}, fmt.Errorf("%w: %w", errdefs.ErrFetch, err)
	}
	defer func() { _ = zr.Close() }()

	part, err := parseGPT(zr)
	if err != nil {
		return seedPartition{}, fmt.Errorf("%w: %w", errdefs.ErrFetch, err)
	}

	if part.StartByte != expectedStart {
		return seedPartition{}, fmt.Errorf(
			"%w: seed-data starts at byte %d, expected %d",
			errdefs.ErrFetch,
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

	entriesOff, err := gptIndex(partLBA, secsz)
	if err != nil {
		return seedPartition{}, err
	}

	entriesLen, err := gptEntriesLen(nparts, esize)
	if err != nil {
		return seedPartition{}, err
	}

	if entriesOff < 0 || entriesOff > len(data) || entriesLen > len(data)-entriesOff {
		return seedPartition{}, fmt.Errorf(
			"GPT entry array at LBA %d does not fit in the streamed head",
			partLBA,
		)
	}

	entries := data[entriesOff : entriesOff+entriesLen]
	for i := range int(nparts) {
		off := i * int(esize)
		entry := entries[off : off+int(esize)]
		if partitionTypeZero(entry[:gptEntryTypeLen]) {
			continue
		}

		name := gptName(entry[gptEntryNameOff : gptEntryNameOff+gptEntryNameBytes])
		if name != seedPartName {
			continue
		}

		first := binary.LittleEndian.Uint64(entry[gptEntryFirstOff : gptEntryFirstOff+8])
		last := binary.LittleEndian.Uint64(entry[gptEntryLastOff : gptEntryLastOff+8])
		start, length, err := partitionRange(first, last, secsz)
		if err != nil {
			return seedPartition{}, err
		}

		return seedPartition{StartByte: start, Length: length}, nil
	}

	return seedPartition{}, errors.New("no seed-data partition in GPT")
}

// gptBytes converts an untrusted LBA count to a byte length. Malicious GPT
// headers can set LBA fields to values whose product with the sector size
// overflows int64; that must fail closed, not wrap.
func gptBytes(lba uint64, secsz int) (int64, error) {
	if secsz <= 0 {
		return 0, fmt.Errorf("invalid sector size %d", secsz)
	}

	hi, lo := bits.Mul64(lba, uint64(secsz))
	if hi != 0 || lo > uint64(math.MaxInt64) {
		return 0, fmt.Errorf("GPT LBA %d overflows at sector size %d", lba, secsz)
	}

	return int64(lo), nil
}

// gptIndex is gptBytes for a slice index into the streamed GPT head.
func gptIndex(lba uint64, secsz int) (int, error) {
	off, err := gptBytes(lba, secsz)
	if err != nil {
		return 0, err
	}

	if off > int64(math.MaxInt) {
		return 0, fmt.Errorf("GPT LBA %d overflows addressable range at sector size %d", lba, secsz)
	}

	return int(off), nil
}

// gptEntriesLen is the GPT entry-array size. nparts and esize come from an
// untrusted header; the product must fit in int because it indexes a slice.
func gptEntriesLen(nparts, esize uint32) (int, error) {
	hi, lo := bits.Mul64(uint64(nparts), uint64(esize))
	if hi != 0 || lo > uint64(math.MaxInt) {
		return 0, fmt.Errorf("GPT entry array size overflows: nparts=%d esize=%d", nparts, esize)
	}

	return int(lo), nil
}

// partitionRange is the seed-data [first, last] LBA span in bytes.
func partitionRange(first, last uint64, secsz int) (int64, int64, error) {
	if last < first {
		return 0, 0, fmt.Errorf("seed-data last LBA %d < first LBA %d", last, first)
	}

	if last-first == math.MaxUint64 {
		return 0, 0, fmt.Errorf("seed-data span overflows: first LBA %d last LBA %d", first, last)
	}

	start, err := gptBytes(first, secsz)
	if err != nil {
		return 0, 0, err
	}

	length, err := gptBytes(last-first+1, secsz)
	if err != nil {
		return 0, 0, err
	}

	return start, length, nil
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

	return 0, 0, errors.New("no EFI PART signature at LBA1 (512, 2048, or 4096)")
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
