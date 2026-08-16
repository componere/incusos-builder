package build

import (
	"context"
	"io"
	"testing"
)

// RunBuild is Build with an explicit seed-data start so tests can inject a
// compact fixture offset (ARCHITECTURE §9) without mutating package state.
func RunBuild(
	ctx context.Context,
	spec Spec,
	src ImageSource,
	rescue RescueWriter,
	rep Reporter,
	render SeedRenderFunc,
	out io.Writer,
	resourcesTmp string,
	expectedStart int64,
) (Result, error) {
	return runBuild(ctx, spec, src, rescue, rep, render, out, resourcesTmp, expectedStart)
}

// GPTImage is a synthetic decompressed disk image with a GPT and one
// seed-data partition, used by probe and splice tests.
type GPTImage = gptImage

// MakeGPTImage builds a GPT whose seed-data partition starts at firstLBA.
func MakeGPTImage(t *testing.T, secsz int, firstLBA, lastLBA uint64) GPTImage {
	return makeGPTImage(t, secsz, firstLBA, lastLBA)
}

// GzipBytes compresses p with pgzip.
func GzipBytes(t *testing.T, p []byte) []byte {
	return gzipBytes(t, p)
}
