// Package build is the domain core of incusos-builder: it resolves a release
// from update-server metadata, probes the acquired image's GPT, renders and
// splices the seed tar, and orchestrates offline rescue-media construction —
// all through ports, with no direct network or filesystem side effects (A1).
//
// The ports (ImageSource, VerifiedAsset, Reporter, RescueWriter) are defined
// here and implemented by the adapters in internal/update, internal/media,
// and internal/ux. Build takes an explicit SeedRenderFunc (typically
// seed.Render) rather than importing internal/seed, which would cycle
// because Seeds is defined here.
package build
