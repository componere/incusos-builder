package update

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	apiimages "github.com/lxc/incus-os/incus-osd/api/images"

	"github.com/componere/incusos-builder/internal/build"
)

// LocalSource is an ImageSource rooted at a local mirror directory with
// the same layout as the HTTPS server: <dir>/index.json and
// <dir>/<version>/<filename>. Assets are verified (size+hash) on admission
// into the same content-addressed cache used by [HTTPSSource], then served
// from that immutable entry. Verifying in place would let a later write to
// the mirror change bytes behind an already-issued handle; the cache keeps
// the VerifiedAsset immutability contract honest.
type LocalSource struct {
	// dir is the mirror root.
	dir string
	// reporter receives Step/Progress/Done around reads.
	reporter build.Reporter
	// cache retains verified assets.
	cache *assetCache
	// indexLimit caps index.json reads. Tests may lower it.
	indexLimit int64
	// metaLimit caps each ReleaseMetadata document. Tests may lower it.
	metaLimit int64
}

// NewLocalSource constructs a directory ImageSource rooted at dir.
// cacheDir is the content-addressed cache root (shared layout with HTTPS).
// reporter must be non-nil.
func NewLocalSource(dir, cacheDir string, reporter build.Reporter) (*LocalSource, error) {
	if reporter == nil {
		return nil, fmt.Errorf("%w: reporter is required", ErrFetch)
	}
	if dir == "" {
		return nil, fmt.Errorf("%w: local source directory is required", ErrFetch)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("%w: local source directory: %w", ErrFetch, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("%w: local source directory: %w", ErrFetch, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%w: local source %q is not a directory", ErrFetch, abs)
	}
	cache, err := newAssetCache(cacheDir, reporter)
	if err != nil {
		return nil, err
	}
	return &LocalSource{
		dir:        abs,
		reporter:   reporter,
		cache:      cache,
		indexLimit: indexCap,
		metaLimit:  metadataCap,
	}, nil
}

// Index reads <dir>/index.json, capped at 64 MiB, and decodes it
// as [apiimages.Index].
func (s *LocalSource) Index(ctx context.Context) (apiimages.Index, error) {
	if err := ctx.Err(); err != nil {
		return apiimages.Index{}, fmt.Errorf("%w: %w", ErrFetch, err)
	}
	s.reporter.Step(stepIndex)
	body, err := s.readCapped("", indexName, s.indexLimit)
	if err != nil {
		return apiimages.Index{}, err
	}
	s.reporter.Progress(int64(len(body)), int64(len(body)))
	var index apiimages.Index
	if err := decodeJSON(body, &index, indexName); err != nil {
		return apiimages.Index{}, err
	}
	s.reporter.Done(stepIndex)
	return index, nil
}

// Asset validates version/filename/sha256/size, then admits
// <dir>/<version>/<filename> into the cache and returns a VerifiedAsset.
func (s *LocalSource) Asset(
	ctx context.Context,
	version string,
	file apiimages.UpdateFile,
) (build.VerifiedAsset, error) {
	if err := validateAsset(version, file); err != nil {
		return nil, err
	}
	return s.cache.get(ctx, file, func(ctx context.Context) (io.ReadCloser, error) {
		return s.openFile(ctx, version, file.Filename)
	})
}

// openFile opens <dir>/<version>/<filename> after allowlist gating.
func (s *LocalSource) openFile(ctx context.Context, version, filename string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFetch, err)
	}
	path, err := s.joinUnderRoot(version, filepath.FromSlash(filename))
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, wrapOpenError(filename, err)
	}
	return f, nil
}

// readCapped reads a file under the mirror, optionally namespaced by version.
func (s *LocalSource) readCapped(version, name string, limit int64) ([]byte, error) {
	var path string
	var err error
	if version == "" {
		path, err = s.joinUnderRoot(name)
	} else {
		path, err = s.joinUnderRoot(version, name)
	}
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, wrapOpenError(name, err)
	}
	defer func() { _ = f.Close() }()
	return readCapped(f, limit, name)
}

// joinUnderRoot joins elem onto s.dir after validating each path element
// and refuses any result that escapes the mirror root.
func (s *LocalSource) joinUnderRoot(elem ...string) (string, error) {
	for _, e := range elem {
		if err := ValidateFilename(filepath.ToSlash(e)); err != nil {
			return "", err
		}
	}
	parts := append([]string{s.dir}, elem...)
	joined := filepath.Join(parts...)
	abs, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("%w: resolve path: %w", ErrFetch, err)
	}
	rel, err := filepath.Rel(s.dir, abs)
	if err != nil {
		return "", fmt.Errorf("%w: resolve path: %w", ErrFetch, err)
	}
	if rel == ".." || stringsHasDotDot(rel) {
		return "", fmt.Errorf("%w: path %q escapes source directory; %s", ErrFetch, abs, tamperSuffix)
	}
	return abs, nil
}

// stringsHasDotDot reports whether rel starts with ".." as a path element.
func stringsHasDotDot(rel string) bool {
	return len(rel) >= 2 && rel[0] == '.' && rel[1] == '.' && (len(rel) == 2 || rel[2] == os.PathSeparator)
}

// wrapOpenError wraps an [os.Open] failure with the public mirror-relative
// name. *[os.PathError] is unwrapped so the message carries only .Err and not
// the resolved absolute path.
func wrapOpenError(name string, err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		err = pe.Err
	}
	return fmt.Errorf("%w: open %s: %w", ErrFetch, name, err)
}
