// Package errdefs is the leaf home of the E1 sentinel errors shared across
// the domain core and the adapters. It exists so that packages on both
// sides of a dependency edge (internal/build ↔ internal/config,
// internal/build ↔ internal/update) can wrap the same sentinels without
// import cycles. The owning packages re-export their sentinels under the
// ARCHITECTURE §6 names (config.ErrConfig, update.ErrFetch, …); [errors.Is]
// matches through either name because the values are identical.
//
// internal/cli maps sentinels to process exit codes and nothing else does.
package errdefs
