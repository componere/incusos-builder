// Package testfixture writes a local-dir update-server mirror for e2e tests.
//
// [Generate] fills a caller-supplied directory with the layout
// [update.LocalSource] reads: index.json at the root, then
// <version>/<filename> assets plus update.json and update.sjson. The OS
// image is a streamed gzip of a sparse ~2.10 GiB GPT disk whose seed-data
// partition starts at byte 2_148_532_224 — the production probe offset in
// internal/build — and is 100 MiB long. Almost all of the decompressed
// stream is zeros, so the gzip is a few MiB and generation stays well
// under five seconds without holding the decompressed image in memory.
//
// Application assets are tiny arch-prefixed <name>.raw.gz files. The
// signed metadata is a structurally valid multipart/signed S/MIME
// document (self-signed throwaway certificate; the update adapter checks
// MIME structure and Filename+Sha256 binding, not recovery-grade
// signatures). Generation is deterministic given the same Go version.
package testfixture
