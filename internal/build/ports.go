package build

import (
	"context"
	"io"

	apiimages "github.com/lxc/incus-os/incus-osd/api/images"
)

// ImageSource acquires update metadata and verified assets.
type ImageSource interface {
	Index(ctx context.Context) (apiimages.Index, error)

	// Asset downloads (or reuses from cache), verifies, and locally
	// retains one asset, returning a reusable handle. Verification —
	// exact Size byte count and Sha256 — happens here, exactly once per
	// call; opening the returned handle is cheap. Download progress is
	// reported through the Reporter the adapter was constructed with —
	// progress originates where the network I/O happens, not in the domain.
	Asset(ctx context.Context, version string, file apiimages.UpdateFile) (VerifiedAsset, error)

	// ReleaseMetadata fetches the release's update.json and update.sjson —
	// upstream appends these literal names to every rescue asset list
	// (main.go:645) and serves them from the same per-version location as
	// every other asset. They are not UpdateFile entries and carry no index
	// digest, so the adapter validates them structurally instead:
	//  (a) each read is size-capped (8 MiB default until measured, §10);
	//  (b) update.sjson — the ONLY document recovery trusts
	//      (recovery.go:184–263) — must be a multipart/signed S/MIME
	//      message whose clear-text payload decodes as apiimages.Update
	//      with Version == version, and whose Files list contains every
	//      entry of selected with an equal Filename and Sha256. This is
	//      structural consistency validation, not signature authentication
	//      (the production CA is not in this checkout); the booted system
	//      remains the trust boundary. It catches the failure that matters:
	//      a stale, truncated, or HTML-error sjson that would otherwise
	//      yield a build that only fails at boot.
	//  (c) update.json, the unsigned twin, must decode as apiimages.Update
	//      with the same Version.
	// Bytes are returned VERBATIM — recovery verifies the signature over
	// the exact bytes; we never re-serialize.
	ReleaseMetadata(ctx context.Context, version string, selected []apiimages.UpdateFile) (ReleaseMetadata, error)
}

// VerifiedAsset is a handle to one verified asset retained by the source.
// Open may be called any number of times over the handle's lifetime; each
// call returns a fresh reader over the same verified bytes (compressed,
// exactly as served), and the caller that opened a reader closes it. The
// build opens the OS image twice — a short read for the GPT probe, then
// the full splice — and the rescue writer opens each application once.
// Handles stay valid until the process exits: the v1 cache has no
// eviction, so nothing invalidates a handle mid-build.
type VerifiedAsset interface {
	Open(ctx context.Context) (io.ReadCloser, error)
	Size() int64 // exact verified byte count == UpdateFile.Size
}

// ReleaseMetadata carries the verbatim release documents for rescue media.
type ReleaseMetadata struct {
	UpdateJSON  []byte // parsed + version-checked, stored verbatim
	UpdateSJSON []byte // payload structurally validated, stored verbatim
}

// Reporter receives phase and progress events. The update adapter and the
// domain both hold one (same interface, injected at wiring time in main).
type Reporter interface {
	Step(name string)
	Progress(done, total int64)
	Done(name string)
}

// RescueWriter builds RESCUE_DATA media into tmpPath — an exclusive temporary
// file created and owned by the caller (the CLI's output publisher), which
// fsyncs, hashes, and publishes it afterwards. WriteRescue replaces the
// tmpPath inode (unlink + O_EXCL create); callers must reopen by path after
// the call returns — a file descriptor from before the call refers to the
// unlinked placeholder, and the new file's mode is 0666 masked by umask.
// The adapter never chooses paths and never learns cache layout: it stages
// every asset by streaming from its VerifiedAsset handle. It refuses an
// input with empty UpdateSJSON: media without update.sjson is silently
// non-functional on the booted system (recovery.go:178–182).
type RescueWriter interface {
	WriteRescue(ctx context.Context, typ ImageType, in RescueInput, tmpPath string) error
}

// RescueInput is everything staged under the media's update/ tree. The two
// metadata documents are explicit typed fields, not generic assets — their
// presence is a compile-visible requirement, not a list convention.
type RescueInput struct {
	Assets      []RescueAsset // update/<name>.raw.gz entries
	UpdateJSON  []byte        // → update/update.json, verbatim
	UpdateSJSON []byte        // → update/update.sjson, verbatim
}

// RescueAsset names one file inside the media's update/ tree and the
// verified handle its bytes stream from.
type RescueAsset struct {
	RelPath string        // e.g. "update/incus.raw.gz" — validated, see below
	Asset   VerifiedAsset // opened once by the writer, streamed into the media
}
