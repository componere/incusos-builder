// Package errdefs holds the sentinel errors shared across the domain core
// and the adapters. Packages on both sides of a dependency edge
// (internal/build ↔ internal/config, internal/build ↔ internal/update)
// wrap the same sentinels without import cycles. Owning packages re-export
// the same values under package-local names (config.ErrConfig,
// update.ErrFetch, and others); [errors.Is] matches through either because
// the values are identical.
//
// internal/cli maps sentinels to process exit codes; no other package does.
package errdefs
