package build

import (
	"bytes"
	"encoding/binary"
	"testing"
	"unicode/utf16"

	"github.com/klauspost/pgzip"
	"github.com/stretchr/testify/require"
)

const (
	fixtureEntryLBA   = 2
	fixtureNParts     = 4
	fixtureEntrySize  = 128
	fixtureTailBytes  = 256
	fixturePrefixByte = 0xA1
	fixtureSeedByte   = 0xB2
	fixtureSuffixByte = 0xC3
)

// gptImage is a tiny decompressed disk image with a GPT and one seed-data
// partition, plus identifiable prefix/seed/suffix fills for splice tests.
type gptImage struct {
	// Bytes is the full decompressed image.
	Bytes []byte
	// Start is the seed-data start byte.
	Start int64
	// Length is the seed-data length in bytes.
	Length int64
	// Sector is the logical sector size.
	Sector int
}

// makeGPTImage builds a GPT whose seed-data partition starts at firstLBA.
func makeGPTImage(t *testing.T, secsz int, firstLBA, lastLBA uint64) gptImage {
	t.Helper()

	if lastLBA < firstLBA {
		t.Fatalf("last LBA %d < first LBA %d", lastLBA, firstLBA)
	}

	entriesOff := fixtureEntryLBA * secsz
	seedStart := int(firstLBA) * secsz
	seedLen := int(lastLBA-firstLBA+1) * secsz
	total := seedStart + seedLen + fixtureTailBytes
	minGPT := entriesOff + fixtureNParts*fixtureEntrySize
	if total < minGPT+fixtureTailBytes {
		total = minGPT + fixtureTailBytes
	}

	img := make([]byte, total)
	for i := range seedStart {
		img[i] = fixturePrefixByte
	}
	for i := seedStart; i < seedStart+seedLen; i++ {
		img[i] = fixtureSeedByte
	}
	for i := seedStart + seedLen; i < total; i++ {
		img[i] = fixtureSuffixByte
	}

	hdrOff := secsz
	copy(img[hdrOff:], []byte(gptSignature))
	binary.LittleEndian.PutUint64(img[hdrOff+gptPartLBAOff:], fixtureEntryLBA)
	binary.LittleEndian.PutUint32(img[hdrOff+gptNPartsOff:], fixtureNParts)
	binary.LittleEndian.PutUint32(img[hdrOff+gptESizeOff:], fixtureEntrySize)

	entry := img[entriesOff : entriesOff+fixtureEntrySize]
	entry[0] = 1
	binary.LittleEndian.PutUint64(entry[gptEntryFirstOff:], firstLBA)
	binary.LittleEndian.PutUint64(entry[gptEntryLastOff:], lastLBA)
	writeGPTName(entry[gptEntryNameOff:gptEntryNameOff+gptEntryNameBytes], seedPartName)

	return gptImage{
		Bytes:  img,
		Start:  int64(seedStart),
		Length: int64(seedLen),
		Sector: secsz,
	}
}

// writeGPTName encodes name as a GPT UTF-16LE partition name, NUL-padded.
func writeGPTName(dst []byte, name string) {
	clear(dst)
	units := utf16.Encode([]rune(name))
	for i := range gptNameUTF16Count {
		off := i * 2
		if i < len(units) {
			binary.LittleEndian.PutUint16(dst[off:], units[i])
		}
	}
}

// gzipBytes compresses p with pgzip.
func gzipBytes(t *testing.T, p []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	w := pgzip.NewWriter(&buf)
	_, err := w.Write(p)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	return buf.Bytes()
}
