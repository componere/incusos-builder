package build

import "errors"

// ErrOutput is returned when writing a built artifact fails (the image
// stream or, via the media adapter, rescue media). Callers map it to
// process exit code 6.
var ErrOutput = errors.New("output write failed")

// ErrVersionNotFound is returned when Resolve cannot select a release
// (unknown pin, empty channel, missing image, or a requested application
// the update does not carry). Callers map it to process exit code 5. The
// error text lists nearby versions or the applications the update does
// carry, matching the plan's exit-5 wording.
var ErrVersionNotFound = errors.New("version not found")

// ErrSeedTooLarge is returned when the rendered seed tar does not fit in
// the acquired image's seed-data partition. Callers map it to process
// exit code 3.
//
// Deviation from ARCHITECTURE §6: that document attributes an oversized
// tar to config.ErrConfig. internal/config already imports this package
// for Spec, so importing config here would be an import cycle. This
// sentinel is the non-cyclic stand-in and is an exit-3-family error
// alongside config.ErrConfig.
var ErrSeedTooLarge = errors.New("seed tar exceeds partition")
