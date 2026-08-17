// Package testfixture writes a local-dir update-server mirror for e2e tests.
//
// [Generate] fills a caller-supplied directory with the layout
// [update.LocalSource] reads: index.json at the root, then
// <version>/<filename> assets plus update.json and update.sjson. The OS
// image is a streamed gzip of a sparse ~2.10 GiB GPT disk whose seed-data
// partition starts at byte 2_148_532_224 and is 100 MiB long. Almost all
// of the decompressed stream is zeros. The decompressed image is never
// held in memory.
//
// Application assets are arch-prefixed <name>.raw.gz files. The signed
// metadata is a multipart/signed S/MIME document with a self-signed
// throwaway certificate. The update adapter checks MIME structure and the
// three-way Filename/Sha256/Size binding, not the PKCS#7 signature.
// Generation is deterministic given the same Go version.
package testfixture
