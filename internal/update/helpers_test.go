package update

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiimages "github.com/lxc/incus-os/incus-osd/api/images"

	"github.com/componere/incusos-builder/internal/build"
)

const (
	testVersion  = "202608102114"
	testFileName = "aarch64/probe.bin"
	testOrigin   = "test.example"
)

// recordingReporter records Reporter calls for assertions.
type recordingReporter struct {
	mu       sync.Mutex
	steps    []string
	progress [][2]int64
	dones    []string
}

// Step records a step name.
func (r *recordingReporter) Step(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.steps = append(r.steps, name)
}

// Progress records a progress sample.
func (r *recordingReporter) Progress(done, total int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.progress = append(r.progress, [2]int64{done, total})
}

// Done records a done name.
func (r *recordingReporter) Done(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dones = append(r.dones, name)
}

// hasStep reports whether name was recorded.
func (r *recordingReporter) hasStep(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Contains(r.steps, name)
}

// lastProgressTotal returns the total of the last Progress call, or -1.
func (r *recordingReporter) lastProgressTotal() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.progress) == 0 {
		return -1
	}
	return r.progress[len(r.progress)-1][1]
}

// lastProgressPair returns the last Progress (done, total), or (-1, -1).
func (r *recordingReporter) lastProgressPair() (int64, int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.progress) == 0 {
		return -1, -1
	}
	p := r.progress[len(r.progress)-1]
	return p[0], p[1]
}

// testServer is an httptest TLS server that records hits and serves a fixture tree.
type testServer struct {
	hits   atomic.Int64
	mu     sync.Mutex
	files  map[string][]byte
	status map[string][]int
	seen   []string
	server *httptest.Server
}

// newTestServer starts a TLS server serving files keyed by URL path.
func newTestServer(t *testing.T, files map[string][]byte) *testServer {
	t.Helper()
	ts := &testServer{files: files, status: map[string][]int{}}
	ts.server = httptest.NewTLSServer(http.HandlerFunc(ts.serve))
	t.Cleanup(ts.server.Close)
	return ts
}

// serve records the request and writes the configured body or status sequence.
func (s *testServer) serve(w http.ResponseWriter, r *http.Request) {
	s.hits.Add(1)
	s.mu.Lock()
	s.seen = append(s.seen, r.URL.Path)
	queue := s.status[r.URL.Path]
	if len(queue) > 0 {
		code := queue[0]
		s.status[r.URL.Path] = queue[1:]
		s.mu.Unlock()
		w.WriteHeader(code)
		if code != http.StatusOK {
			return
		}
		s.mu.Lock()
	}
	body := s.files[r.URL.Path]
	s.mu.Unlock()
	if body == nil {
		http.NotFound(w, r)
		return
	}
	_, _ = w.Write(body)
}

// setStatusQueue plays status codes in order for path, then 200+body.
func (s *testServer) setStatusQueue(path string, codes ...int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status[path] = append([]int(nil), codes...)
}

// newHTTPS builds an HTTPSSource against ts with zero backoff.
func (s *testServer) newHTTPS(t *testing.T, reporter build.Reporter) *HTTPSSource {
	t.Helper()
	if reporter == nil {
		reporter = &recordingReporter{}
	}
	src, err := NewHTTPSSource(s.server.URL, t.TempDir(), reporter, s.server.Client())
	require.NoError(t, err)
	src.backoff = func(int) time.Duration { return 0 }
	return src
}

// testFile returns an UpdateFile for payload with a nested filename.
func testFile(payload []byte) apiimages.UpdateFile {
	sum := sha256.Sum256(payload)
	return apiimages.UpdateFile{
		Architecture: apiimages.UpdateFileArchitecture64BitARM,
		Component:    apiimages.UpdateFileComponentOS,
		Filename:     testFileName,
		Sha256:       hex.EncodeToString(sum[:]),
		Size:         int64(len(payload)),
		Type:         apiimages.UpdateFileTypeImageRaw,
	}
}

// testIndexJSON marshals a one-update index containing files.
func testIndexJSON(t *testing.T, files ...apiimages.UpdateFile) []byte {
	t.Helper()
	idx := apiimages.Index{
		Format: "1.0",
		Updates: []apiimages.UpdateFull{{
			Update: apiimages.Update{
				Format:      "1.0",
				Version:     testVersion,
				Origin:      testOrigin,
				Severity:    apiimages.UpdateSeverityNone,
				PublishedAt: time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
				Channels:    []string{"stable"},
				Files:       files,
			},
		}},
	}
	body, err := json.Marshal(idx)
	require.NoError(t, err)
	return body
}

// testUpdateJSON marshals an Update document for version with files.
func testUpdateJSON(t *testing.T, version string, files ...apiimages.UpdateFile) []byte {
	t.Helper()
	doc := apiimages.Update{
		Format:      "1.0",
		Version:     version,
		Origin:      testOrigin,
		Severity:    apiimages.UpdateSeverityNone,
		PublishedAt: time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
		Channels:    []string{"stable"},
		Files:       files,
	}
	body, err := json.Marshal(doc)
	require.NoError(t, err)
	return body
}

// testSJSON builds a multipart/signed document around payload JSON.
func testSJSON(payload []byte) []byte {
	var b strings.Builder
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString(
		"Content-Type: multipart/signed; protocol=\"application/x-pkcs7-signature\"; micalg=sha-256; boundary=AAA\r\n",
	)
	b.WriteString("\r\n")
	b.WriteString("--AAA\r\n")
	b.WriteString("Content-Type: text/plain\r\n")
	b.WriteString("\r\n")
	b.Write(payload)
	b.WriteString("\r\n")
	b.WriteString("--AAA\r\n")
	b.WriteString("Content-Type: application/x-pkcs7-signature; name=\"smime.p7s\"\r\n")
	b.WriteString("Content-Transfer-Encoding: base64\r\n")
	b.WriteString("\r\n")
	b.WriteString("AAAA\r\n")
	b.WriteString("--AAA--\r\n")
	return []byte(b.String())
}

// writeLocalMirror writes index/assets/metadata into dir.
func writeLocalMirror(t *testing.T, dir string, files map[string][]byte) {
	t.Helper()
	for rel, body := range files {
		path := filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(rel, "/")))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, body, 0o644))
	}
}

// readAll opens handle and returns its bytes.
func readAll(t *testing.T, handle build.VerifiedAsset) []byte {
	t.Helper()
	rc, err := handle.Open(t.Context())
	require.NoError(t, err)
	defer func() { require.NoError(t, rc.Close()) }()
	body, err := io.ReadAll(rc)
	require.NoError(t, err)
	return body
}

// hexOf returns the sha256 hex of p.
func hexOf(p []byte) string {
	sum := sha256.Sum256(p)
	return hex.EncodeToString(sum[:])
}

// testFileNamed returns an UpdateFile for payload under filename.
func testFileNamed(payload []byte, filename string) apiimages.UpdateFile {
	file := testFile(payload)
	file.Filename = filename
	return file
}

// assetPath is the server path for version/filename.
func assetPath(filename string) string {
	return "/" + testVersion + "/" + filename
}

// assertNoCacheBlobs reports that cache holds no digest file and no fetch temps.
func assertNoCacheBlobs(t *testing.T, cache *assetCache) {
	t.Helper()
	entries, err := os.ReadDir(cache.dir)
	require.NoError(t, err)
	for _, e := range entries {
		name := e.Name()
		require.False(t, strings.HasPrefix(name, ".fetch"), "leftover temp %s", name)
		if name != digestDirName {
			continue
		}
		digests, err := os.ReadDir(filepath.Join(cache.dir, digestDirName))
		require.NoError(t, err)
		assert.Empty(t, digests)
	}
}
