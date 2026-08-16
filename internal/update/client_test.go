package update

import (
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiimages "github.com/lxc/incus-os/incus-osd/api/images"

	"github.com/componere/incusos-builder/internal/build"
)

func TestHTTPSSourceImplementsImageSource(t *testing.T) {
	t.Parallel()
	var _ build.ImageSource = (*HTTPSSource)(nil)
	var _ build.ImageSource = (*LocalSource)(nil)
	var _ build.VerifiedAsset = (*cachedAsset)(nil)
}

func TestNewHTTPSSourceRejectsHTTP(t *testing.T) {
	t.Parallel()
	_, err := NewHTTPSSource("http://example.com/os", t.TempDir(), &recordingReporter{}, nil)
	require.ErrorIs(t, err, ErrFetch)
	assert.Contains(t, err.Error(), "https")
}

func TestIndexStrictDecodeAndJoinPath(t *testing.T) {
	t.Parallel()
	payload := []byte("hello")
	file := testFile(payload)
	ts := newTestServer(t, map[string][]byte{
		"/index.json": testIndexJSON(t, file),
	})
	src := ts.newHTTPS(t, nil)

	idx, err := src.Index(t.Context())
	require.NoError(t, err)
	require.Len(t, idx.Updates, 1)
	assert.Equal(t, testVersion, idx.Updates[0].Version)
	assert.Equal(t, int64(1), ts.hits.Load())
	assert.Equal(t, []string{"/index.json"}, ts.seen)
}

func TestIndexCapEnforced(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t, map[string][]byte{
		"/index.json": []byte(`{"format":"1.0","updates":[]}` + "trailing-extra"),
	})
	src := ts.newHTTPS(t, nil)
	src.indexLimit = 8
	_, err := src.Index(t.Context())
	require.ErrorIs(t, err, ErrFetch)
	assert.Contains(t, err.Error(), "cap")
}

func TestRetryOn5xxThenSuccess(t *testing.T) {
	t.Parallel()
	body := testIndexJSON(t)
	ts := newTestServer(t, map[string][]byte{"/index.json": body})
	ts.setStatusQueue("/index.json", http.StatusInternalServerError, http.StatusBadGateway)
	src := ts.newHTTPS(t, nil)

	idx, err := src.Index(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "1.0", idx.Format)
	assert.Equal(t, int64(3), ts.hits.Load())
}

func Test4xxNotRetried(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t, map[string][]byte{})
	ts.setStatusQueue("/index.json", http.StatusNotFound)
	src := ts.newHTTPS(t, nil)
	_, err := src.Index(t.Context())
	require.ErrorIs(t, err, ErrFetch)
	assert.Equal(t, int64(1), ts.hits.Load())
}

func TestAllowlistRejectedBeforeAnyRequest(t *testing.T) {
	payload := []byte("hello")
	good := testFile(payload)
	ts := newTestServer(t, map[string][]byte{
		assetPath(good.Filename): payload,
	})
	src := ts.newHTTPS(t, nil)

	tests := []struct {
		name     string
		version  string
		filename string
	}{
		{name: "question version", version: "v?1"},
		{name: "hash version", version: "v#1"},
		{name: "percent version", version: "v%61"},
		{name: "dotdot version", version: ".."},
		{name: "question filename", version: testVersion, filename: "a?b"},
		{name: "hash filename", version: testVersion, filename: "a#b"},
		{name: "percent filename", version: testVersion, filename: "a%2e"},
		{name: "dotdot filename", version: testVersion, filename: "a/../b"},
		{name: "absolute filename", version: testVersion, filename: "/etc/passwd"},
		{name: "backslash filename", version: testVersion, filename: `a\b`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := good
			if tt.filename != "" {
				file.Filename = tt.filename
			}
			before := ts.hits.Load()
			_, err := src.Asset(t.Context(), tt.version, file)
			require.ErrorIs(t, err, ErrFetch)
			assert.Equal(t, before, ts.hits.Load())
		})
	}
}

func TestAssetDownloadProgressAndHandle(t *testing.T) {
	t.Parallel()
	payload := []byte("payload!")
	file := testFile(payload)
	rep := &recordingReporter{}
	ts := newTestServer(t, map[string][]byte{
		assetPath(file.Filename): payload,
	})
	src := ts.newHTTPS(t, rep)

	handle, err := src.Asset(t.Context(), testVersion, file)
	require.NoError(t, err)
	assert.Equal(t, int64(len(payload)), handle.Size())
	first := readAll(t, handle)
	second := readAll(t, handle)
	assert.Equal(t, payload, first)
	assert.Equal(t, first, second)
	assert.Equal(t, int64(len(payload)), rep.lastProgressTotal())
	assert.True(t, rep.hasStep(stepDownload))
}

func TestAssetReuseCorruptEntryRedownloads(t *testing.T) {
	t.Parallel()
	payload := []byte("cached-bytes")
	file := testFile(payload)
	ts := newTestServer(t, map[string][]byte{
		assetPath(file.Filename): payload,
	})
	src := ts.newHTTPS(t, nil)

	handle, err := src.Asset(t.Context(), testVersion, file)
	require.NoError(t, err)
	require.Equal(t, payload, readAll(t, handle))
	require.Equal(t, int64(1), ts.hits.Load())

	entry := src.cache.entryPath(file.Sha256)
	require.NoError(t, os.Chmod(entry, 0o644))
	require.NoError(t, os.WriteFile(entry, []byte("CORRUPT!!!!!!"), 0o644))

	handle, err = src.Asset(t.Context(), testVersion, file)
	require.NoError(t, err)
	assert.Equal(t, payload, readAll(t, handle))
	assert.Equal(t, int64(2), ts.hits.Load())
}

func TestAdmissionFailuresIdentical(t *testing.T) {
	t.Parallel()
	payload := []byte("abcd")
	file := testFile(payload)

	t.Run("short", func(t *testing.T) {
		t.Parallel()
		ts := newTestServer(t, map[string][]byte{assetPath(file.Filename): payload[:2]})
		src := ts.newHTTPS(t, nil)
		_, err := src.Asset(t.Context(), testVersion, file)
		require.ErrorIs(t, err, ErrFetch)
		assert.Contains(t, err.Error(), "admission")
	})
	t.Run("trailing", func(t *testing.T) {
		t.Parallel()
		body := append(append([]byte{}, payload...), 'Z')
		ts := newTestServer(t, map[string][]byte{assetPath(file.Filename): body})
		src := ts.newHTTPS(t, nil)
		_, err := src.Asset(t.Context(), testVersion, file)
		require.ErrorIs(t, err, ErrFetch)
		assert.Contains(t, err.Error(), "admission")
	})
	t.Run("mismatch", func(t *testing.T) {
		t.Parallel()
		wrong := file
		wrong.Sha256 = hexOf([]byte("wxyz"))
		ts := newTestServer(t, map[string][]byte{assetPath(file.Filename): payload})
		src := ts.newHTTPS(t, nil)
		_, err := src.Asset(t.Context(), testVersion, wrong)
		require.ErrorIs(t, err, ErrFetch)
		assert.Contains(t, err.Error(), "admission")
	})
}

func TestFreeSpaceWarningIsNotError(t *testing.T) {
	t.Parallel()
	payload := []byte("xyz")
	file := testFile(payload)
	rep := &recordingReporter{}
	ts := newTestServer(t, map[string][]byte{assetPath(file.Filename): payload})
	src := ts.newHTTPS(t, rep)
	src.cache.freeBytes = func(string) (uint64, error) { return 1, nil }

	handle, err := src.Asset(t.Context(), testVersion, file)
	require.NoError(t, err)
	assert.Equal(t, payload, readAll(t, handle))
	assert.True(t, rep.hasStep(stepSpaceWarn))
}

func TestReleaseMetadataAllowlistBeforeRequest(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t, map[string][]byte{})
	src := ts.newHTTPS(t, nil)
	_, err := src.ReleaseMetadata(t.Context(), "v?bad", nil)
	require.ErrorIs(t, err, ErrFetch)
	assert.Equal(t, int64(0), ts.hits.Load())
}

func TestAssetReuseHitsCache(t *testing.T) {
	t.Parallel()
	payload := []byte("once-only")
	file := testFile(payload)
	ts := newTestServer(t, map[string][]byte{assetPath(file.Filename): payload})
	src := ts.newHTTPS(t, nil)

	_, err := src.Asset(t.Context(), testVersion, file)
	require.NoError(t, err)
	_, err = src.Asset(t.Context(), testVersion, file)
	require.NoError(t, err)
	assert.Equal(t, int64(1), ts.hits.Load())
}

func TestSHA256RejectedBeforeRequest(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t, map[string][]byte{})
	src := ts.newHTTPS(t, nil)
	file := apiimages.UpdateFile{Filename: "ok.bin", Sha256: "not-a-digest", Size: 1}
	_, err := src.Asset(t.Context(), testVersion, file)
	require.ErrorIs(t, err, ErrFetch)
	assert.Equal(t, int64(0), ts.hits.Load())
}
