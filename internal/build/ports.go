package build

import (
	"context"
	"io"

	apiimages "github.com/lxc/incus-os/incus-osd/api/images"
)

// ImageSource acquires update metadata and verified assets.
type ImageSource interface {
	// Index returns the update-server index used to resolve a release.
	// Failures wrap [errdefs.ErrFetch].
	Index(ctx context.Context) (apiimages.Index, error)

	// Asset verifies and retains one asset, returning a reusable handle.
	// Verification of Size and Sha256 happens once per call. Download
	// progress is reported through the Reporter the adapter was constructed
	// with. Failures wrap [errdefs.ErrFetch].
	Asset(ctx context.Context, version string, file apiimages.UpdateFile) (VerifiedAsset, error)

	// ReleaseMetadata returns the release's update.json and update.sjson
	// bodies verbatim. They are not UpdateFile entries, so the adapter
	// validates them structurally: each read is size-capped; update.sjson
	// must be a multipart/signed S/MIME message whose clear-text payload
	// decodes as apiimages.Update with Version == version and whose Files
	// cover every selected Filename and Sha256; update.json must decode as
	// apiimages.Update with the same Version. This is structural consistency,
	// not signature authentication. Failures wrap [errdefs.ErrFetch].
	ReleaseMetadata(ctx context.Context, version string, selected []apiimages.UpdateFile) (ReleaseMetadata, error)
}

// VerifiedAsset is a handle to one verified asset retained by the source.
// Open may be called any number of times; each call returns a fresh reader
// over the same verified bytes (compressed, exactly as served). The caller
// that opened a reader closes it. Handles stay valid for the process
// lifetime: the cache has no eviction.
type VerifiedAsset interface {
	// Open returns a fresh reader over the verified compressed bytes.
	// The caller that opened the reader closes it.
	Open(ctx context.Context) (io.ReadCloser, error)
	// Size is the exact verified byte count, equal to UpdateFile.Size.
	Size() int64
}

// ReleaseMetadata carries the verbatim release documents for rescue media.
type ReleaseMetadata struct {
	// UpdateJSON is the parsed, version-checked update.json body, stored
	// verbatim.
	UpdateJSON []byte
	// UpdateSJSON is the structurally validated update.sjson body, stored
	// verbatim.
	UpdateSJSON []byte
}

// Reporter receives build-step and progress events. The update adapter and
// the domain both hold one.
type Reporter interface {
	// Step announces that a named build step has started.
	Step(name string)
	// Progress reports bytes completed of a known total.
	Progress(done, total int64)
	// Done announces that a named build step has finished.
	Done(name string)
}

// RescueWriter builds RESCUE_DATA media into a caller-owned tmpPath.
type RescueWriter interface {
	// WriteRescue writes rescue media of typ into tmpPath. The caller
	// creates and owns tmpPath and must reopen it by path after the call
	// returns, because WriteRescue replaces the inode. An empty
	// UpdateSJSON is refused: media without update.sjson leaves the
	// booted system silently unrecovered. Assets are streamed from their
	// VerifiedAsset handles. Failures wrap [ErrOutput].
	WriteRescue(ctx context.Context, typ ImageType, in RescueInput, tmpPath string) error
}

// RescueInput is everything staged under the media's update/ tree.
type RescueInput struct {
	// Assets are the application files staged under update/.
	Assets []RescueAsset
	// UpdateJSON is written verbatim to update/update.json.
	UpdateJSON []byte
	// UpdateSJSON is written verbatim to update/update.sjson. Empty is
	// refused by WriteRescue.
	UpdateSJSON []byte
}

// RescueAsset names one file inside the media's update/ tree and the
// verified handle its bytes stream from.
type RescueAsset struct {
	// RelPath is the path inside the media, such as
	// "update/aarch64/incus.raw.gz".
	RelPath string
	// Asset is opened once by the writer and streamed into the media.
	Asset VerifiedAsset
}
