package build

import "github.com/componere/incusos-builder/internal/errdefs"

// ErrOutput is returned when writing a built artifact fails (the image
// stream or, via the media adapter, rescue media). Callers map it to
// process exit code 6. It is the §6 name for [errdefs.ErrOutput].
var ErrOutput = errdefs.ErrOutput

// ErrVersionNotFound is returned when Resolve cannot select a release
// (unknown pin, empty channel, missing image, or a requested application
// the update does not carry). Callers map it to process exit code 5. The
// error text lists nearby versions or the applications the update does
// carry. It is the §6 name for [errdefs.ErrVersionNotFound].
var ErrVersionNotFound = errdefs.ErrVersionNotFound
