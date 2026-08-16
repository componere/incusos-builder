package build

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/componere/incusos-builder/internal/errdefs"
)

func TestParseGPTSectorSizes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		secsz  int
		first  uint64
		last   uint64
		wantAt int64
		wantN  int64
	}{
		{
			name:   "512",
			secsz:  sector512,
			first:  8,
			last:   15,
			wantAt: 8 * sector512,
			wantN:  8 * sector512,
		},
		{
			name:   "2048",
			secsz:  sector2048,
			first:  4,
			last:   5,
			wantAt: 4 * sector2048,
			wantN:  2 * sector2048,
		},
		{
			name:   "4096",
			secsz:  sector4096,
			first:  3,
			last:   4,
			wantAt: 3 * sector4096,
			wantN:  2 * sector4096,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			img := makeGPTImage(t, tt.secsz, tt.first, tt.last)
			part, err := parseGPT(bytes.NewReader(img.Bytes))
			require.NoError(t, err)
			assert.Equal(t, tt.wantAt, part.StartByte)
			assert.Equal(t, tt.wantN, part.Length)
		})
	}
}

func TestProbeShiftedPartitionWrapsErrFetch(t *testing.T) {
	t.Parallel()

	img := makeGPTImage(t, sector512, 8, 15)
	handle := staticAsset{gz: gzipBytes(t, img.Bytes)}

	_, err := probe(context.Background(), handle, img.Start+512)
	require.Error(t, err)
	require.ErrorIs(t, err, errdefs.ErrFetch)
	assert.Contains(t, err.Error(), "seed-data starts at byte")
}

func TestParseGPTMissingSignature(t *testing.T) {
	t.Parallel()

	_, err := parseGPT(bytes.NewReader(bytes.Repeat([]byte{0x00}, 8192)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no EFI PART signature")
}

func TestParseGPTNoSeedData(t *testing.T) {
	t.Parallel()

	img := makeGPTImage(t, sector512, 8, 15)
	copy(img.Bytes[2*sector512+gptEntryNameOff:], make([]byte, gptEntryNameBytes))

	_, err := parseGPT(bytes.NewReader(img.Bytes))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no seed-data partition")
}

func TestParseGPTRejectsASCIIName(t *testing.T) {
	t.Parallel()

	img := makeGPTImage(t, sector512, 8, 15)
	name := img.Bytes[2*sector512+gptEntryNameOff : 2*sector512+gptEntryNameOff+gptEntryNameBytes]
	clear(name)
	copy(name, []byte(seedPartName))

	_, err := parseGPT(bytes.NewReader(img.Bytes))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no seed-data partition")
}

func TestProbeUnreadableGzipWrapsErrFetch(t *testing.T) {
	t.Parallel()

	handle := staticAsset{gz: []byte("not-gzip")}
	_, err := probe(context.Background(), handle, 0)
	require.Error(t, err)
	require.ErrorIs(t, err, errdefs.ErrFetch)
}

func TestParseGPTRejectsOverflowingEntryLBA(t *testing.T) {
	t.Parallel()

	img := makeGPTImage(t, sector512, 8, 15)
	binary.LittleEndian.PutUint64(img.Bytes[sector512+gptPartLBAOff:], math.MaxUint64)

	_, err := parseGPT(bytes.NewReader(img.Bytes))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "overflows")
}

func TestParseGPTRejectsOverflowingPartitionRange(t *testing.T) {
	t.Parallel()

	img := makeGPTImage(t, sector512, 8, 15)
	entry := img.Bytes[2*sector512 : 2*sector512+gptMinEntrySize]
	binary.LittleEndian.PutUint64(entry[gptEntryFirstOff:], math.MaxUint64)
	binary.LittleEndian.PutUint64(entry[gptEntryLastOff:], math.MaxUint64)

	_, err := parseGPT(bytes.NewReader(img.Bytes))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "overflows")
}

// staticAsset is a VerifiedAsset over fixed gzip bytes. Tests use it only as
// a fixture handle; production mocks are mockery-generated.
type staticAsset struct {
	gz []byte
}

func (s staticAsset) Open(context.Context) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.gz)), nil
}

func (s staticAsset) Size() int64 {
	return int64(len(s.gz))
}
