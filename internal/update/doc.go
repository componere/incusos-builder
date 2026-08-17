// Package update is the ImageSource adapter: an HTTPS update-server
// client and a local-directory source, both exposing verified
// content-addressed assets through the build.ImageSource port.
//
// Every acquisition failure — index fetch, metadata validation, download,
// cache read, checksum/size mismatch, handle open, release-metadata
// fetch/structural validation — wraps [ErrFetch]. Callers map it to
// process exit code 5.
//
// The sentinel itself lives in internal/errdefs so that internal/build
// can wrap the same value for GPT-probe drift without importing this
// package; [ErrFetch] is the package-local name for [errdefs.ErrFetch].
package update
