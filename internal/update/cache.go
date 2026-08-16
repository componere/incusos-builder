package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	apiimages "github.com/lxc/incus-os/incus-osd/api/images"
	"golang.org/x/sys/unix"

	"github.com/componere/incusos-builder/internal/build"
)

const (
	digestDirName   = "sha256"
	tmpFetchPrefix  = ".fetch-*"
	cacheFileMode   = 0o444
	cacheDirMode    = 0o755
	copyBufSize     = 1 << 20
	stepSpaceWarn   = "warning: cache free space below asset size"
	admissionFailed = "asset failed size/digest admission; untrusted metadata; possible tampering"
)

// assetCache is a content-addressed store of verified blobs at
// <dir>/sha256/<digest>. Handles always open these immutable entries, never
// the download source, so VerifiedAsset remains honest if a mirror file
// later changes.
type assetCache struct {
	// dir is the cache root.
	dir string
	// reporter receives progress and the free-space warning.
	reporter build.Reporter
	// freeBytes is a best-effort statfs; tests inject a stub.
	freeBytes func(dir string) (uint64, error)
}

// newAssetCache returns a cache rooted at dir. dir is created on first admission.
func newAssetCache(dir string, reporter build.Reporter) (*assetCache, error) {
	if dir == "" {
		return nil, fmt.Errorf("%w: cache directory is required", ErrFetch)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("%w: cache directory: %w", ErrFetch, err)
	}
	return &assetCache{dir: abs, reporter: reporter, freeBytes: statfsFree}, nil
}

// cachedAsset is a VerifiedAsset over one immutable cache entry.
type cachedAsset struct {
	// path is the verified <cache>/sha256/<digest> file.
	path string
	// size is the verified byte count.
	size int64
}

// Open returns a fresh reader over the verified bytes.
func (a *cachedAsset) Open(ctx context.Context) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFetch, err)
	}
	f, err := os.Open(a.path)
	if err != nil {
		return nil, fmt.Errorf("%w: open cached asset: %w", ErrFetch, err)
	}
	return f, nil
}

// Size returns the exact verified byte count.
func (a *cachedAsset) Size() int64 {
	return a.size
}

// get reuses a verified cache entry or admits bytes from fetch. fetch is
// only called on a miss or after a corrupt cached file is rejected.
func (c *assetCache) get(
	ctx context.Context,
	file apiimages.UpdateFile,
	fetch func(context.Context) (io.ReadCloser, error),
) (*cachedAsset, error) {
	if handle, ok, err := c.reuse(file); err != nil {
		return nil, err
	} else if ok {
		return handle, nil
	}
	c.warnIfLowSpace(file.Size)
	rc, err := fetch(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	return c.admit(ctx, file, rc)
}

// reuse re-hashes an existing cache entry and size-checks it. A corrupt
// entry is not issued as a handle (ok=false) so the caller re-fetches.
func (c *assetCache) reuse(file apiimages.UpdateFile) (*cachedAsset, bool, error) {
	path := c.entryPath(file.Sha256)
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("%w: stat cache entry: %w", ErrFetch, err)
	}
	if info.Size() != file.Size {
		return nil, false, nil
	}
	sum, n, err := hashFile(path)
	if err != nil {
		return nil, false, err
	}
	if n != file.Size || hex.EncodeToString(sum) != file.Sha256 {
		return nil, false, nil
	}
	return &cachedAsset{path: path, size: file.Size}, true, nil
}

// admit copies src into a temp file in the cache root, requiring exactly
// file.Size bytes then EOF and a matching digest, then renames into place.
func (c *assetCache) admit(ctx context.Context, file apiimages.UpdateFile, src io.Reader) (*cachedAsset, error) {
	if err := os.MkdirAll(c.digestDir(), cacheDirMode); err != nil {
		return nil, fmt.Errorf("%w: create cache: %w", ErrFetch, err)
	}
	tmp, err := os.CreateTemp(c.dir, tmpFetchPrefix)
	if err != nil {
		return nil, fmt.Errorf("%w: create cache temp: %w", ErrFetch, err)
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		_ = tmp.Close()
		if !committed {
			_ = os.Remove(tmpName)
		}
	}()

	sum, n, err := copyHashed(ctx, tmp, io.LimitReader(src, file.Size+1), file.Size, c.reporter)
	if err != nil {
		return nil, err
	}
	if n != file.Size || hex.EncodeToString(sum) != file.Sha256 {
		return nil, fmt.Errorf("%w: %q: %s", ErrFetch, file.Filename, admissionFailed)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("%w: close cache temp: %w", ErrFetch, err)
	}
	if err := os.Chmod(tmpName, cacheFileMode); err != nil {
		return nil, fmt.Errorf("%w: chmod cache temp: %w", ErrFetch, err)
	}
	dest := c.entryPath(file.Sha256)
	if err := os.Rename(tmpName, dest); err != nil {
		return nil, fmt.Errorf("%w: admit cache entry: %w", ErrFetch, err)
	}
	committed = true
	return &cachedAsset{path: dest, size: file.Size}, nil
}

// copyHashed writes src to dst while hashing and reporting Progress(done, total).
func copyHashed(
	ctx context.Context,
	dst io.Writer,
	src io.Reader,
	total int64,
	reporter build.Reporter,
) ([]byte, int64, error) {
	h := sha256.New()
	w := io.MultiWriter(dst, h)
	buf := make([]byte, copyBufSize)
	var done int64
	for {
		if err := ctx.Err(); err != nil {
			return nil, done, fmt.Errorf("%w: %w", ErrFetch, err)
		}
		nr, rerr := src.Read(buf)
		if nr > 0 {
			nw, werr := w.Write(buf[:nr])
			done += int64(nw)
			if werr != nil {
				return nil, done, fmt.Errorf("%w: write cache temp: %w", ErrFetch, werr)
			}
			if nw != nr {
				return nil, done, fmt.Errorf("%w: short write to cache temp", ErrFetch)
			}
			if reporter != nil {
				reporter.Progress(done, total)
			}
		}
		if rerr != nil {
			if errors.Is(rerr, io.EOF) {
				return h.Sum(nil), done, nil
			}
			return nil, done, fmt.Errorf("%w: read asset: %w", ErrFetch, rerr)
		}
	}
}

// hashFile SHA-256s path and returns the digest and byte count.
func hashFile(path string) ([]byte, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: open cache entry: %w", ErrFetch, err)
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return nil, n, fmt.Errorf("%w: re-hash cache entry: %w", ErrFetch, err)
	}
	return h.Sum(nil), n, nil
}

// warnIfLowSpace emits a Reporter warning when free space is below size.
// Failures and missing space data are ignored; this is never an error.
func (c *assetCache) warnIfLowSpace(size int64) {
	if c.reporter == nil || c.freeBytes == nil || size <= 0 {
		return
	}
	free, err := c.freeBytes(c.dir)
	if err != nil {
		return
	}
	if free < uint64(size) {
		c.reporter.Step(stepSpaceWarn)
		c.reporter.Done(stepSpaceWarn)
	}
}

// entryPath returns <dir>/sha256/<digest>.
func (c *assetCache) entryPath(digest string) string {
	return filepath.Join(c.digestDir(), digest)
}

// digestDir returns <dir>/sha256.
func (c *assetCache) digestDir() string {
	return filepath.Join(c.dir, digestDirName)
}

// statfsFree returns available bytes on the filesystem containing dir.
func statfsFree(dir string) (uint64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil //nolint:unconvert // Bavail width is OS-specific
}
