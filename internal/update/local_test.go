package update

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiimages "github.com/lxc/incus-os/incus-osd/api/images"
)

func TestLocalSourceParity(t *testing.T) {
	t.Parallel()
	payload := []byte("local-asset")
	file := testFile(payload)
	updateJSON := testUpdateJSON(t, testVersion, file)
	sjson := testSJSON(updateJSON)
	dir := t.TempDir()
	writeLocalMirror(t, dir, map[string][]byte{
		"index.json":                        testIndexJSON(t, file),
		testVersion + "/" + file.Filename:   payload,
		testVersion + "/" + updateJSONName:  updateJSON,
		testVersion + "/" + updateSJSONName: sjson,
	})
	src, err := NewLocalSource(dir, t.TempDir(), &recordingReporter{})
	require.NoError(t, err)

	idx, err := src.Index(t.Context())
	require.NoError(t, err)
	require.Len(t, idx.Updates, 1)

	handle, err := src.Asset(t.Context(), testVersion, file)
	require.NoError(t, err)
	assert.Equal(t, int64(len(payload)), handle.Size())
	first := readAll(t, handle)
	second := readAll(t, handle)
	assert.Equal(t, payload, first)
	assert.Equal(t, first, second)

	meta, err := src.ReleaseMetadata(t.Context(), testVersion, []apiimages.UpdateFile{file})
	require.NoError(t, err)
	require.True(t, bytes.Equal(updateJSON, meta.UpdateJSON), "verbatim update.json")
	require.True(t, bytes.Equal(sjson, meta.UpdateSJSON), "verbatim update.sjson")
}

func TestLocalAllowlistBeforeFilesystem(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src, err := NewLocalSource(dir, t.TempDir(), &recordingReporter{})
	require.NoError(t, err)

	file := apiimages.UpdateFile{
		Filename: "/etc/passwd",
		Sha256:   hexOf([]byte("x")),
		Size:     1,
	}
	_, err = src.Asset(t.Context(), testVersion, file)
	require.ErrorIs(t, err, ErrFetch)

	_, err = src.Asset(t.Context(), "..", testFile([]byte("x")))
	require.ErrorIs(t, err, ErrFetch)

	_, err = src.ReleaseMetadata(t.Context(), "v?bad", nil)
	require.ErrorIs(t, err, ErrFetch)
}

func TestLocalIndexCapEnforced(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, indexName), bytesRepeat('x', 32), 0o644))
	src, err := NewLocalSource(dir, t.TempDir(), &recordingReporter{})
	require.NoError(t, err)
	src.indexLimit = 8
	_, err = src.Index(t.Context())
	require.ErrorIs(t, err, ErrFetch)
	assert.Contains(t, err.Error(), "cap")
}

func TestLocalAdmissionFailuresIdentical(t *testing.T) {
	t.Parallel()
	payload := []byte("abcd")
	file := testFile(payload)

	t.Run("short", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeLocalMirror(t, dir, map[string][]byte{testVersion + "/" + file.Filename: payload[:2]})
		src, err := NewLocalSource(dir, t.TempDir(), &recordingReporter{})
		require.NoError(t, err)
		_, err = src.Asset(t.Context(), testVersion, file)
		require.ErrorIs(t, err, ErrFetch)
		assert.Contains(t, err.Error(), "admission")
	})
	t.Run("trailing", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		trailing := append(append([]byte{}, payload...), 'Z')
		writeLocalMirror(t, dir, map[string][]byte{testVersion + "/" + file.Filename: trailing})
		src, err := NewLocalSource(dir, t.TempDir(), &recordingReporter{})
		require.NoError(t, err)
		_, err = src.Asset(t.Context(), testVersion, file)
		require.ErrorIs(t, err, ErrFetch)
		assert.Contains(t, err.Error(), "admission")
	})
	t.Run("mismatch", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeLocalMirror(t, dir, map[string][]byte{testVersion + "/" + file.Filename: payload})
		src, err := NewLocalSource(dir, t.TempDir(), &recordingReporter{})
		require.NoError(t, err)
		wrong := file
		wrong.Sha256 = hexOf([]byte("wxyz"))
		_, err = src.Asset(t.Context(), testVersion, wrong)
		require.ErrorIs(t, err, ErrFetch)
		assert.Contains(t, err.Error(), "admission")
	})
}

func TestLocalCorruptCacheDoesNotSilentReuse(t *testing.T) {
	t.Parallel()
	payload := []byte("mirror-bytes")
	file := testFile(payload)
	dir := t.TempDir()
	writeLocalMirror(t, dir, map[string][]byte{testVersion + "/" + file.Filename: payload})
	cacheDir := t.TempDir()
	src, err := NewLocalSource(dir, cacheDir, &recordingReporter{})
	require.NoError(t, err)

	handle, err := src.Asset(t.Context(), testVersion, file)
	require.NoError(t, err)
	require.Equal(t, payload, readAll(t, handle))

	entry := src.cache.entryPath(file.Sha256)
	require.NoError(t, os.Chmod(entry, 0o644))
	require.NoError(t, os.WriteFile(entry, []byte("CORRUPT!!!!!!"), 0o644))
	handle, err = src.Asset(t.Context(), testVersion, file)
	require.NoError(t, err)
	assert.Equal(t, payload, readAll(t, handle))
}

func TestLocalReleaseMetadataFailures(t *testing.T) {
	t.Parallel()
	file := testFile([]byte("app"))
	goodJSON := testUpdateJSON(t, testVersion, file)

	t.Run("malformed mime", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeLocalMirror(t, dir, map[string][]byte{
			testVersion + "/" + updateJSONName:  goodJSON,
			testVersion + "/" + updateSJSONName: []byte("not-mime"),
		})
		src, err := NewLocalSource(dir, t.TempDir(), &recordingReporter{})
		require.NoError(t, err)
		_, err = src.ReleaseMetadata(t.Context(), testVersion, []apiimages.UpdateFile{file})
		require.ErrorIs(t, err, ErrFetch)
	})

	t.Run("cap", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeLocalMirror(t, dir, map[string][]byte{
			testVersion + "/" + updateJSONName:  bytesRepeat('a', 32),
			testVersion + "/" + updateSJSONName: testSJSON(goodJSON),
		})
		src, err := NewLocalSource(dir, t.TempDir(), &recordingReporter{})
		require.NoError(t, err)
		src.metaLimit = 8
		_, err = src.ReleaseMetadata(t.Context(), testVersion, []apiimages.UpdateFile{file})
		require.ErrorIs(t, err, ErrFetch)
		assert.Contains(t, err.Error(), "cap")
	})
}

func bytesRepeat(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}
