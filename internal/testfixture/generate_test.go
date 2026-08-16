package testfixture_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	apiimages "github.com/lxc/incus-os/incus-osd/api/images"

	"github.com/componere/incusos-builder/internal/build"
	mediamocks "github.com/componere/incusos-builder/internal/media/mocks"
	"github.com/componere/incusos-builder/internal/seed"
	"github.com/componere/incusos-builder/internal/testfixture"
	"github.com/componere/incusos-builder/internal/update"
	uxmocks "github.com/componere/incusos-builder/internal/ux/mocks"
)

const (
	generateBudget    = 5 * time.Second
	maxGzipImageBytes = 8 << 20
)

// TestGenerateSatisfiesUpdateAdapter round-trips the fixture through the
// real local-dir adapter and production [build.Build] probe path.
func TestGenerateSatisfiesUpdateAdapter(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	started := time.Now()
	mirror, err := testfixture.Generate(dir)
	require.NoError(t, err)
	elapsed := time.Since(started)
	t.Logf(
		"Generate wall=%s gzip_image=%d raw_image=%d seed_start=%d seed_len=%d",
		elapsed,
		mirror.GzipImageSize,
		mirror.RawImageSize,
		mirror.SeedStart,
		mirror.SeedLength,
	)
	require.Less(t, elapsed, generateBudget, "fixture generation wall time")
	assert.Equal(t, testfixture.Version, mirror.Version)
	assert.Equal(t, testfixture.SeedStart, mirror.SeedStart)
	assert.Equal(t, testfixture.SeedLength, mirror.SeedLength)
	assert.Greater(t, mirror.RawImageSize, testfixture.SeedStart+testfixture.SeedLength)
	assert.Positive(t, mirror.GzipImageSize)
	assert.Less(t, mirror.GzipImageSize, int64(maxGzipImageBytes))
	require.Len(t, mirror.Apps, 2)

	rep := nopReporter(t)
	src, err := update.NewLocalSource(dir, t.TempDir(), rep)
	require.NoError(t, err)

	index, err := src.Index(t.Context())
	require.NoError(t, err)
	require.Equal(t, "1.0", index.Format)
	require.Len(t, index.Updates, 1)
	updateFull := index.Updates[0]
	assert.Equal(t, testfixture.Version, updateFull.Version)
	assert.Equal(t, []string{testfixture.ChannelStable}, updateFull.Channels)
	assert.Equal(t, "/"+testfixture.Version, updateFull.URL)
	require.Len(t, updateFull.Files, 3)
	assert.Equal(t, mirror.Image.Filename, updateFull.Files[0].Filename)
	assert.Equal(t, apiimages.UpdateFileTypeImageRaw, updateFull.Files[0].Type)
	assert.Equal(t, apiimages.UpdateFileArchitecture64BitX86, updateFull.Files[0].Architecture)

	handle, err := src.Asset(t.Context(), mirror.Version, mirror.Image)
	require.NoError(t, err)
	assert.Equal(t, mirror.Image.Size, handle.Size())
	assert.Equal(t, mirror.Image.Sha256, sha256File(t, filepath.Join(
		dir, mirror.Version, filepath.FromSlash(mirror.Image.Filename),
	)))

	selected := make([]apiimages.UpdateFile, 0, 1+len(mirror.Apps))
	selected = append(selected, mirror.Image)
	selected = append(selected, mirror.Apps...)
	meta, err := src.ReleaseMetadata(t.Context(), mirror.Version, selected)
	require.NoError(t, err)
	assert.NotEmpty(t, meta.UpdateJSON)
	assert.NotEmpty(t, meta.UpdateSJSON)
	assert.Contains(t, string(meta.UpdateSJSON), "multipart/signed")
	assert.Contains(t, string(meta.UpdateSJSON), string(meta.UpdateJSON))

	rescue := mediamocks.NewMockRescueWriter(t)
	counter := &countDiscard{}
	result, err := build.Build(
		t.Context(),
		build.Spec{
			Type:         build.ImageTypeRaw,
			Architecture: build.ArchX8664,
			Channel:      build.DefaultChannel,
			Release:      build.Release(mirror.Version),
		},
		src,
		rescue,
		rep,
		seed.Render,
		counter,
		"",
	)
	require.NoError(t, err, "production Build probe+splice against fixture")
	assert.Equal(t, mirror.Version, result.Version)
	assert.Equal(t, mirror.RawImageSize, result.BytesWritten)
	assert.Equal(t, mirror.RawImageSize, counter.n)
	assert.False(t, result.Offline)
}

// TestGenerateDeterministic checks two Generate calls write identical trees.
func TestGenerateDeterministic(t *testing.T) {
	t.Parallel()

	first := generateTree(t)
	second := generateTree(t)
	require.Equal(t, first, second)
}

// TestGenerateRequiresDirectory rejects an empty dir argument.
func TestGenerateRequiresDirectory(t *testing.T) {
	t.Parallel()

	_, err := testfixture.Generate("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "directory is required")
}

// generateTree maps relative paths under a generated fixture to sha256 hex.
func generateTree(t *testing.T) map[string]string {
	t.Helper()

	dir := t.TempDir()
	_, err := testfixture.Generate(dir)
	require.NoError(t, err)

	out := map[string]string{}
	err = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = sha256File(t, path)

		return nil
	})
	require.NoError(t, err)

	return out
}

// sha256File returns the hex digest of path.
func sha256File(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	sum := sha256.Sum256(data)

	return hex.EncodeToString(sum[:])
}

// nopReporter is a mockery Reporter that accepts any Step/Progress/Done.
func nopReporter(t *testing.T) *uxmocks.MockReporter {
	t.Helper()

	rep := uxmocks.NewMockReporter(t)
	rep.EXPECT().Step(mock.Anything).Maybe()
	rep.EXPECT().Progress(mock.Anything, mock.Anything).Maybe()
	rep.EXPECT().Done(mock.Anything).Maybe()

	return rep
}

// countDiscard is [io.Discard] that counts bytes.
type countDiscard struct {
	// n is the running write total.
	n int64
}

// Write discards p and adds its length to the counter.
func (c *countDiscard) Write(p []byte) (int, error) {
	c.n += int64(len(p))

	return len(p), nil
}
