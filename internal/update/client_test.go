package update

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

func TestIndexDecodeAndJoinPath(t *testing.T) {
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

func TestIndexIgnoresUnknownFields(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t, map[string][]byte{
		"/index.json": []byte(`{"format":"1.0","updates":[],"future_field":true}`),
	})
	idx, err := ts.newHTTPS(t, nil).Index(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "1.0", idx.Format)
}

func TestIndexRejectsTrailingData(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t, map[string][]byte{
		"/index.json": []byte(`{"format":"1.0","updates":[]}{"more":true}`),
	})
	_, err := ts.newHTTPS(t, nil).Index(t.Context())
	require.ErrorIs(t, err, ErrFetch)
	assert.Contains(t, err.Error(), "trailing")
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

func TestDoGETRejectsHTTPSToHTTPRedirect(t *testing.T) {
	t.Parallel()
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"format":"1.0","updates":[]}`))
	}))
	t.Cleanup(httpSrv.Close)

	tlsSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, httpSrv.URL+"/index.json", http.StatusFound)
	}))
	t.Cleanup(tlsSrv.Close)

	src, err := NewHTTPSSource(tlsSrv.URL, t.TempDir(), &recordingReporter{}, tlsSrv.Client())
	require.NoError(t, err)
	_, err = src.Index(t.Context())
	require.ErrorIs(t, err, ErrFetch)
	assert.Contains(t, err.Error(), "non-https")
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
		assert.Equal(t, int64(2), ts.hits.Load())
	})
	t.Run("trailing", func(t *testing.T) {
		t.Parallel()
		body := append(append([]byte{}, payload...), 'Z')
		ts := newTestServer(t, map[string][]byte{assetPath(file.Filename): body})
		src := ts.newHTTPS(t, nil)
		_, err := src.Asset(t.Context(), testVersion, file)
		require.ErrorIs(t, err, ErrFetch)
		assert.Contains(t, err.Error(), "admission")
		assert.Equal(t, int64(2), ts.hits.Load())
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
		assert.Equal(t, int64(2), ts.hits.Load())
	})
}

func TestAssetFlakyBodyThenSuccess(t *testing.T) {
	t.Parallel()
	const dropSize = 32 << 10
	payload := bytes.Repeat([]byte("n"), dropSize)
	file := testFile(payload)
	var hits atomic.Int64
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := hits.Add(1)
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		if n == 1 {
			_, _ = w.Write(payload[:len(payload)/2])
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			hj, ok := w.(http.Hijacker)
			if !ok {
				return
			}
			conn, _, err := hj.Hijack()
			if err == nil {
				_ = conn.Close()
			}
			return
		}
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)
	src, err := NewHTTPSSource(srv.URL, t.TempDir(), &recordingReporter{}, srv.Client())
	require.NoError(t, err)
	src.backoff = func(int) time.Duration { return 0 }

	handle, err := src.Asset(t.Context(), testVersion, file)
	require.NoError(t, err)
	assert.Equal(t, payload, readAll(t, handle))
	assert.Equal(t, int64(2), hits.Load())
}

func TestAssetChecksumMismatchFailsAfterRedownload(t *testing.T) {
	t.Parallel()
	payload := []byte("abcd")
	file := testFile(payload)
	file.Sha256 = hexOf([]byte("wxyz"))
	ts := newTestServer(t, map[string][]byte{assetPath(file.Filename): payload})
	src := ts.newHTTPS(t, nil)
	_, err := src.Asset(t.Context(), testVersion, file)
	require.ErrorIs(t, err, ErrFetch)
	assert.Contains(t, err.Error(), "admission")
	assert.Equal(t, int64(2), ts.hits.Load())
	assertNoCacheBlobs(t, src.cache)
}

func TestConcurrentAdmissionDifferentDigests(t *testing.T) {
	t.Parallel()
	payloadA := []byte("content-alpha")
	payloadB := []byte("content-bravo!")
	fileA := testFileNamed(payloadA, "aarch64/a.bin")
	fileB := testFileNamed(payloadB, "x86_64/b.bin")
	ts := newTestServer(t, map[string][]byte{
		assetPath(fileA.Filename): payloadA,
		assetPath(fileB.Filename): payloadB,
	})
	src := ts.newHTTPS(t, nil)

	var wg sync.WaitGroup
	wg.Add(2)
	var gotA, gotB build.VerifiedAsset
	var errA, errB error
	go func() {
		defer wg.Done()
		gotA, errA = src.Asset(t.Context(), testVersion, fileA)
	}()
	go func() {
		defer wg.Done()
		gotB, errB = src.Asset(t.Context(), testVersion, fileB)
	}()
	wg.Wait()
	require.NoError(t, errA)
	require.NoError(t, errB)
	assert.Equal(t, payloadA, readAll(t, gotA))
	assert.Equal(t, payloadB, readAll(t, gotB))
	assert.NotEqual(t, fileA.Sha256, fileB.Sha256)
	_, err := os.Stat(src.cache.entryPath(fileA.Sha256))
	require.NoError(t, err)
	_, err = os.Stat(src.cache.entryPath(fileB.Sha256))
	require.NoError(t, err)
}

func TestFailedAdmissionLeavesNoCacheEntry(t *testing.T) {
	t.Parallel()
	payload := []byte("abcd")
	file := testFile(payload)
	ts := newTestServer(t, map[string][]byte{assetPath(file.Filename): payload[:2]})
	src := ts.newHTTPS(t, nil)
	_, err := src.Asset(t.Context(), testVersion, file)
	require.ErrorIs(t, err, ErrFetch)
	assertNoCacheBlobs(t, src.cache)
	_, err = os.Stat(src.cache.entryPath(file.Sha256))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestSizeRejectedBeforeRequest(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t, map[string][]byte{})
	src := ts.newHTTPS(t, nil)
	digest := hexOf([]byte("x"))
	for _, size := range []int64{0, -1, maxAssetSize + 1} {
		file := apiimages.UpdateFile{Filename: "ok.bin", Sha256: digest, Size: size}
		before := ts.hits.Load()
		_, err := src.Asset(t.Context(), testVersion, file)
		require.ErrorIs(t, err, ErrFetch)
		assert.Contains(t, err.Error(), "size")
		assert.Equal(t, before, ts.hits.Load())
	}
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
