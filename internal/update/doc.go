// Package update is the ImageSource adapter: an HTTPS update-server
// client and a local-directory source.
//
// The adapter implementation arrives in Phase 3a. This package currently
// exports only the [ErrFetch] sentinel. Every acquisition failure — index
// fetch, metadata validation, download, cache read, checksum/size
// mismatch, handle open, release-metadata fetch/structural validation —
// wraps ErrFetch. Callers map it to process exit code 5 (ARCHITECTURE §6).
//
// Phase 3a must not import internal/build from this package: the domain
// already imports update for ErrFetch, and the adapter will need the
// domain's ports. Put the client in a subpackage (for example
// internal/update/client) so this package stays a sentinel leaf.
package update
