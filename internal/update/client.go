package update

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"time"

	apiimages "github.com/lxc/incus-os/incus-osd/api/images"

	"github.com/componere/incusos-builder/internal/build"
)

const (
	httpsScheme          = "https"
	indexName            = "index.json"
	stepIndex            = "index"
	stepDownload         = "download"
	retryAttempts        = 3
	retryBackoffShiftCap = 3
	retryBase            = 100 * time.Millisecond
	// indexCap is the ARCHITECTURE §6 LimitReader bound on index.json.
	indexCap int64 = 64 << 20
)

// HTTPSSource is an ImageSource that fetches from an HTTPS update server
// and retains verified assets in a content-addressed cache.
type HTTPSSource struct {
	// base is the https server URL, with no trailing slash required.
	base string
	// client performs HTTP requests. Tests inject httptest's TLS client.
	client *http.Client
	// reporter receives Step/Progress/Done around network I/O.
	reporter build.Reporter
	// cache retains verified assets.
	cache *assetCache
	// attempts is the retry budget (first try plus retries). Tests may lower it.
	attempts int
	// backoff returns the delay before retry number attempt (0-based).
	backoff func(attempt int) time.Duration
	// indexLimit caps index.json reads. Tests may lower it.
	indexLimit int64
	// metaLimit caps each ReleaseMetadata document. Tests may lower it.
	metaLimit int64
}

// NewHTTPSSource constructs an HTTPS ImageSource for serverURL.
// serverURL must use the https scheme (plain http is rejected at construction).
// cacheDir is the content-addressed cache root. reporter receives download
// progress; it must be non-nil. client may be nil, in which case a default
// [http.Client] with no overall timeout is used (cancellation is via ctx).
func NewHTTPSSource(serverURL, cacheDir string, reporter build.Reporter, client *http.Client) (*HTTPSSource, error) {
	if reporter == nil {
		return nil, fmt.Errorf("%w: reporter is required", ErrFetch)
	}
	base, err := parseHTTPSBase(serverURL)
	if err != nil {
		return nil, err
	}
	cache, err := newAssetCache(cacheDir, reporter)
	if err != nil {
		return nil, err
	}
	if client == nil {
		client = &http.Client{}
	}
	return &HTTPSSource{
		base:       base,
		client:     client,
		reporter:   reporter,
		cache:      cache,
		attempts:   retryAttempts,
		backoff:    jitteredBackoff,
		indexLimit: indexCap,
		metaLimit:  metadataCap,
	}, nil
}

// Index fetches <server>/index.json, capped at 64 MiB, and strict-decodes
// it as [apiimages.Index].
func (s *HTTPSSource) Index(ctx context.Context) (apiimages.Index, error) {
	s.reporter.Step(stepIndex)
	defer s.reporter.Done(stepIndex)

	rawURL, err := url.JoinPath(s.base, indexName)
	if err != nil {
		return apiimages.Index{}, fmt.Errorf("%w: index URL: %w", ErrFetch, err)
	}
	body, err := s.getCapped(ctx, rawURL, s.indexLimit)
	if err != nil {
		return apiimages.Index{}, err
	}
	s.reporter.Progress(int64(len(body)), int64(len(body)))

	var index apiimages.Index
	if err := strictDecode(body, &index, indexName); err != nil {
		return apiimages.Index{}, err
	}
	return index, nil
}

// Asset validates version/filename/sha256/size, then reuses or downloads
// the file into the content-addressed cache and returns a VerifiedAsset.
func (s *HTTPSSource) Asset(
	ctx context.Context,
	version string,
	file apiimages.UpdateFile,
) (build.VerifiedAsset, error) {
	if err := validateAsset(version, file); err != nil {
		return nil, err
	}
	return s.cache.get(ctx, file, func(ctx context.Context) (io.ReadCloser, error) {
		return s.openAsset(ctx, version, file)
	})
}

// openAsset GETs the asset URL and reports download progress via the cache copy.
func (s *HTTPSSource) openAsset(ctx context.Context, version string, file apiimages.UpdateFile) (io.ReadCloser, error) {
	rawURL, err := url.JoinPath(s.base, version, file.Filename)
	if err != nil {
		return nil, fmt.Errorf("%w: asset URL: %w", ErrFetch, err)
	}
	s.reporter.Step(stepDownload)
	resp, err := s.doGET(ctx, rawURL) //nolint:bodyclose // transferred to doneCloser, closed by cache.get
	if err != nil {
		s.reporter.Done(stepDownload)
		return nil, err
	}
	return &doneCloser{
		inner: resp.Body,
		done:  func() { s.reporter.Done(stepDownload) },
	}, nil
}

// parseHTTPSBase rejects non-https server URLs.
func parseHTTPSBase(serverURL string) (string, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return "", fmt.Errorf("%w: server URL: %w", ErrFetch, err)
	}
	if !strings.EqualFold(u.Scheme, httpsScheme) || u.Host == "" {
		return "", fmt.Errorf("%w: server URL must be https", ErrFetch)
	}
	u.Scheme = httpsScheme
	return strings.TrimRight(u.String(), "/"), nil
}

// getCapped GETs rawURL, retries 5xx, and reads at most limit bytes.
func (s *HTTPSSource) getCapped(ctx context.Context, rawURL string, limit int64) ([]byte, error) {
	resp, err := s.doGET(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return readCapped(resp.Body, limit, rawURL)
}

// doGET performs GET with retry-on-5xx (and transport errors) and no retry on 4xx.
func (s *HTTPSSource) doGET(ctx context.Context, rawURL string) (*http.Response, error) {
	attempts := max(s.attempts, 1)
	var lastErr error
	for attempt := range attempts {
		if attempt > 0 {
			if err := sleep(ctx, s.backoff(attempt-1)); err != nil {
				return nil, err
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrFetch, err)
		}
		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("%w: %w", ErrFetch, err)
			continue
		}
		if resp.StatusCode >= http.StatusInternalServerError {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("%w: %s", ErrFetch, resp.Status)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("%w: %s", ErrFetch, resp.Status)
		}
		return resp, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("%w: GET %s failed", ErrFetch, rawURL)
	}
	return nil, lastErr
}

// readCapped reads r up to limit bytes and rejects a body larger than limit.
func readCapped(r io.Reader, limit int64, name string) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, fmt.Errorf("%w: read %s: %w", ErrFetch, name, err)
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("%w: %s exceeds %d-byte cap; %s", ErrFetch, name, limit, tamperSuffix)
	}
	return body, nil
}

// strictDecode JSON-decodes data into v with unknown fields rejected.
func strictDecode(data []byte, v any, name string) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%w: decode %s: %w", ErrFetch, name, err)
	}
	var extra json.RawMessage
	err := dec.Decode(&extra)
	if err == nil {
		return fmt.Errorf("%w: trailing data in %s; %s", ErrFetch, name, tamperSuffix)
	}
	if err != io.EOF {
		return fmt.Errorf("%w: decode %s: %w", ErrFetch, name, err)
	}
	return nil
}

// jitteredBackoff returns a capped, jittered delay for retry attempt (0-based).
func jitteredBackoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	shift := min(attempt, retryBackoffShiftCap)
	base := retryBase * time.Duration(1<<shift)
	return base + time.Duration(rand.Int64N(int64(base)+1)) //nolint:gosec // jitter only, not a secret
}

// sleep waits d or until ctx is cancelled.
func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("%w: %w", ErrFetch, err)
		}
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("%w: %w", ErrFetch, ctx.Err())
	case <-timer.C:
		return nil
	}
}

// doneCloser closes the inner reader then calls done.
type doneCloser struct {
	inner io.ReadCloser
	done  func()
}

// Read implements [io.Reader].
func (c *doneCloser) Read(p []byte) (int, error) {
	return c.inner.Read(p)
}

// Close closes the inner reader and runs done once.
func (c *doneCloser) Close() error {
	err := c.inner.Close()
	if c.done != nil {
		c.done()
		c.done = nil
	}
	return err
}
