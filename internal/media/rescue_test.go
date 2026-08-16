package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"os"
	"path"
	"strings"
	"sync/atomic"
	"testing"

	diskfs "github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/filesystem"
	"github.com/diskfs/go-diskfs/partition/gpt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/componere/incusos-builder/internal/build"
	"github.com/componere/incusos-builder/internal/update"
)

// memAsset is a VerifiedAsset over fixed bytes that counts Open calls.
type memAsset struct {
	body  []byte
	opens atomic.Int32
}

// Open returns a fresh reader over the same bytes and increments the counter.
func (a *memAsset) Open(context.Context) (io.ReadCloser, error) {
	a.opens.Add(1)
	return io.NopCloser(bytes.NewReader(a.body)), nil
}

// Size is the exact byte count of body.
func (a *memAsset) Size() int64 {
	return int64(len(a.body))
}

func TestWriterImplementsRescueWriter(t *testing.T) {
	t.Parallel()
	var _ build.RescueWriter = (*Writer)(nil)
	var _ build.RescueWriter = NewWriter()
}

func TestWriteRescueFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		typ  build.ImageType
	}{
		{name: "iso", typ: build.ImageTypeISO},
		{name: "raw", typ: build.ImageTypeRaw},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			nested := &memAsset{body: []byte("nested-arch-payload")}
			flat := &memAsset{body: []byte("flat-payload")}
			updateJSON := []byte(`{"version":"test","files":[]}`)
			updateSJSON := []byte("-----BEGIN PKCS7-----\nsigned-body\n-----END PKCS7-----\n")
			in := build.RescueInput{
				Assets: []build.RescueAsset{
					{RelPath: "update/aarch64/incus.raw.gz", Asset: nested},
					{RelPath: "update/incus.raw.gz", Asset: flat},
				},
				UpdateJSON:  updateJSON,
				UpdateSJSON: updateSJSON,
			}
			want := map[string][]byte{
				"update/update.json":          updateJSON,
				"update/update.sjson":         updateSJSON,
				"update/aarch64/incus.raw.gz": nested.body,
				"update/incus.raw.gz":         flat.body,
			}

			tmpPath := exclusiveTemp(t)
			err := NewWriter().WriteRescue(context.Background(), tt.typ, in, tmpPath)
			require.NoError(t, err)
			assert.Equal(t, int32(1), nested.opens.Load(), "nested asset opened once")
			assert.Equal(t, int32(1), flat.opens.Load(), "flat asset opened once")

			got := readBack(t, tmpPath, tt.typ == build.ImageTypeISO)
			require.Len(t, got, len(want), "walked file count")
			for name, body := range want {
				gotBody, ok := got[name]
				require.True(t, ok, "missing %s", name)
				assert.Equal(t, hex.EncodeToString(sha256Sum(body)), hex.EncodeToString(sha256Sum(gotBody)), name)
				assert.Equal(t, body, gotBody, name)
			}

			if tt.typ == build.ImageTypeISO {
				assertISOSizeMatchesPVD(t, tmpPath)
			}
		})
	}
}

func TestWriteRescueEmptySJSON(t *testing.T) {
	t.Parallel()

	asset := &memAsset{body: []byte("payload")}
	tmpPath := exclusiveTemp(t)
	err := NewWriter().WriteRescue(context.Background(), build.ImageTypeISO, build.RescueInput{
		Assets:     []build.RescueAsset{{RelPath: "update/incus.raw.gz", Asset: asset}},
		UpdateJSON: []byte("{}"),
	}, tmpPath)
	require.ErrorIs(t, err, build.ErrOutput)
	require.NotErrorIs(t, err, update.ErrFetch)
	assert.Equal(t, int32(0), asset.opens.Load())
	assertEmptyFile(t, tmpPath)
}

func TestWriteRescueBadRelPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		relPath string
	}{
		{name: "traversal", relPath: "update/../passwd"},
		{name: "absolute", relPath: "/update/incus.raw.gz"},
		{name: "percent", relPath: "update/a%2e%2e"},
		{name: "empty prefix only", relPath: "update/"},
		{name: "missing prefix", relPath: "incus.raw.gz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			asset := &memAsset{body: []byte("payload")}
			tmpPath := exclusiveTemp(t)
			err := NewWriter().WriteRescue(context.Background(), build.ImageTypeRaw, build.RescueInput{
				Assets:      []build.RescueAsset{{RelPath: tt.relPath, Asset: asset}},
				UpdateSJSON: []byte("signed"),
			}, tmpPath)
			require.ErrorIs(t, err, build.ErrOutput)
			require.NotErrorIs(t, err, update.ErrFetch)
			assert.Equal(t, int32(0), asset.opens.Load())
			assertEmptyFile(t, tmpPath)
		})
	}
}

func TestWriteRescueCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tmpPath := exclusiveTemp(t)
	err := NewWriter().WriteRescue(ctx, build.ImageTypeISO, build.RescueInput{
		UpdateSJSON: []byte("signed"),
	}, tmpPath)
	require.ErrorIs(t, err, build.ErrOutput)
	require.ErrorIs(t, err, context.Canceled)
	assertEmptyFile(t, tmpPath)
}

// exclusiveTemp creates the empty exclusive file the CLI passes as tmpPath.
func exclusiveTemp(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "rescue-*")
	require.NoError(t, err)
	name := f.Name()
	require.NoError(t, f.Close())
	return name
}

// assertEmptyFile reports that path still exists and has size 0: validation
// failed before replaceFile / diskfs.Create.
func assertEmptyFile(t *testing.T, path string) {
	t.Helper()
	fi, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, int64(0), fi.Size())
}

// sha256Sum returns the SHA-256 digest of b.
func sha256Sum(b []byte) []byte {
	sum := sha256.Sum256(b)
	return sum[:]
}

// readBack opens the image read-only, checks labels / fs type / GPT (raw),
// and returns the update/ tree by walking with unrooted ReadDir paths
// (gotcha 4). OpenFile uses rooted paths; the two APIs are asymmetric.
func readBack(t *testing.T, tmpPath string, isISO bool) map[string][]byte {
	t.Helper()
	d, err := diskfs.Open(tmpPath, diskfs.WithOpenMode(diskfs.ReadOnly))
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	var fsys filesystem.FileSystem
	if isISO {
		fsys, err = d.GetFilesystem(isoPartition)
		require.NoError(t, err)
		require.Equal(t, filesystem.TypeISO9660, fsys.Type())
	} else {
		table, err := d.GetPartitionTable()
		require.NoError(t, err)
		g, ok := table.(*gpt.Table)
		require.True(t, ok, "want GPT, got %T", table)
		require.Len(t, g.Partitions, 1)
		p := g.Partitions[0]
		assert.Equal(t, volumeLabel, p.Name)
		assert.Equal(t, gpt.MicrosoftBasicData, p.Type)
		assert.Equal(t, uint64(gptHead/rawSector), p.Start)
		fsys, err = d.GetFilesystem(fatPartition)
		require.NoError(t, err)
		require.Equal(t, filesystem.TypeFat32, fsys.Type())
	}
	assert.Equal(t, volumeLabel, strings.TrimRight(fsys.Label(), "\x00 "))

	got := make(map[string][]byte)
	var walk func(dir string)
	walk = func(dir string) {
		ents, err := fsys.ReadDir(dir)
		require.NoError(t, err, "ReadDir(%s)", dir)
		for _, e := range ents {
			name := e.Name()
			if name == "." || name == ".." {
				continue
			}
			child := path.Join(dir, name)
			if e.IsDir() {
				walk(child)
				continue
			}
			f, err := fsys.OpenFile("/"+child, os.O_RDONLY)
			require.NoError(t, err, "OpenFile(/%s)", child)
			body, err := io.ReadAll(f)
			_ = f.Close()
			require.NoError(t, err, "read %s", child)
			got[child] = body
		}
	}
	walk("update")
	return got
}

// assertISOSizeMatchesPVD checks the on-disk ISO is a 2048-multiple equal to
// the PVD volume-space-size and smaller than the pre-Finalize backing file.
func assertISOSizeMatchesPVD(t *testing.T, tmpPath string) {
	t.Helper()
	fi, err := os.Stat(tmpPath)
	require.NoError(t, err)
	require.Equal(t, int64(0), fi.Size()%isoBlock, "ISO size not 2048-aligned")

	fh, err := os.Open(tmpPath)
	require.NoError(t, err)
	defer fh.Close()
	pvd := make([]byte, isoBlock)
	_, err = fh.ReadAt(pvd, isoPVDSector*isoBlock)
	require.NoError(t, err)
	pvdSize := int64(binary.LittleEndian.Uint32(pvd[isoPVDSizeOff:isoPVDSizeOff+isoPVDSizeLen])) * isoBlock
	assert.Equal(t, pvdSize, fi.Size())
	assert.Positive(t, pvdSize)
	assert.Less(t, fi.Size(), isoSlack+isoBlock)
}
