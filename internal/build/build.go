package build

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"

	"github.com/klauspost/pgzip"

	"github.com/componere/incusos-builder/internal/errdefs"
)

const (
	copyBufSize      = 4 * 1024 * 1024
	stepResolve      = "resolve"
	stepAcquire      = "acquire"
	stepProbe        = "probe"
	stepSeed         = "seed"
	stepSplice       = "splice"
	stepRescue       = "rescue"
	rescueTreePrefix = "update/"
)

// SeedRenderFunc renders a seed tar from Seeds. Production wiring in main
// assigns seed.Render; tests inject a stub. Declared here so Build takes an
// explicit collaborator instead of importing internal/seed (Seeds lives in
// this package, so seed must import build — a cycle if Build imported seed).
type SeedRenderFunc func(Seeds) ([]byte, int64, error)

// Build resolves a release, probes the image GPT, splices the seed tar, and
// optionally builds offline rescue media. src, rescue, and rep are ports;
// render is wired to seed.Render in main; out is the caller-owned image
// stream; resourcesTmp is the caller-owned rescue temp path. There is no
// filesystem or network I/O in this function beyond the injected ports,
// renderer, and streams (A1).
func Build(
	ctx context.Context,
	spec Spec,
	src ImageSource,
	rescue RescueWriter,
	rep Reporter,
	render SeedRenderFunc,
	out io.Writer,
	resourcesTmp string,
) (Result, error) {
	return runBuild(ctx, spec, src, rescue, rep, render, out, resourcesTmp, productionSeedStart)
}

// runBuild is Build with an explicit seed-data start, so tests can inject a
// compact fixture offset (ARCHITECTURE §9) without mutating package state.
func runBuild(
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
	if render == nil {
		return Result{}, errors.New("internal: seed renderer is not wired")
	}

	rep.Step(stepResolve)

	index, err := src.Index(ctx)
	if err != nil {
		return Result{}, err
	}

	plan, err := Resolve(spec, index)
	if err != nil {
		return Result{}, err
	}

	rep.Done(stepResolve)
	rep.Step(stepAcquire)

	image, err := src.Asset(ctx, plan.Version, plan.Image)
	if err != nil {
		return Result{}, err
	}

	rep.Done(stepAcquire)
	rep.Step(stepProbe)

	part, err := probe(ctx, image, expectedStart)
	if err != nil {
		return Result{}, err
	}

	rep.Done(stepProbe)
	rep.Step(stepSeed)

	tarBytes, seedSize, err := render(spec.Seeds)
	if err != nil {
		return Result{}, err
	}

	err = checkRenderedSeedSize(seedSize, tarBytes)
	if err != nil {
		return Result{}, err
	}

	if seedSize > part.Length {
		return Result{}, fmt.Errorf(
			"%w: seed tar is %d bytes, seed-data partition holds %d",
			errdefs.ErrConfig,
			seedSize,
			part.Length,
		)
	}

	rep.Done(stepSeed)
	rep.Step(stepSplice)

	written, err := splice(ctx, image, out, part.StartByte, tarBytes)
	if err != nil {
		return Result{}, err
	}

	rep.Done(stepSplice)

	result := Result{
		Version:      plan.Version,
		Channel:      spec.Channel,
		Type:         spec.Type,
		Architecture: spec.Architecture,
		BytesWritten: written,
		SeedBytes:    seedSize,
		Offline:      spec.Offline,
	}

	if !spec.Offline {
		return result, nil
	}

	rep.Step(stepRescue)

	if err := writeRescue(ctx, spec, src, rescue, plan, resourcesTmp); err != nil {
		return Result{}, err
	}

	rep.Done(stepRescue)
	result.ResourcesTmp = resourcesTmp

	return result, nil
}

// splice opens handle (Open #2), gunzips, copies [0, offset), writes tar,
// discards len(tar) from the source, then copies the remainder. Read-side
// errors wrap [errdefs.ErrFetch]; write-side errors wrap [ErrOutput]. There
// is no bare [io.Copy] across ports (ARCHITECTURE §6, P2).
func splice(
	ctx context.Context,
	handle VerifiedAsset,
	out io.Writer,
	offset int64,
	tarBytes []byte,
) (int64, error) {
	rc, err := handle.Open(ctx)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", errdefs.ErrFetch, err)
	}
	defer func() { _ = rc.Close() }()

	zr, err := pgzip.NewReader(rc)
	if err != nil {
		return 0, fmt.Errorf("%w: gzip: %w", errdefs.ErrFetch, err)
	}
	defer func() { _ = zr.Close() }()

	buf := make([]byte, copyBufSize)
	written := int64(0)

	n, err := copyN(ctx, out, zr, offset, buf)
	written += n
	if err != nil {
		return written, err
	}

	n, err = writeAll(out, tarBytes)
	written += n
	if err != nil {
		return written, fmt.Errorf("%w: %w", ErrOutput, err)
	}

	err = discardN(ctx, zr, int64(len(tarBytes)), buf)
	if err != nil {
		return written, err
	}

	n, err = copyAll(ctx, out, zr, buf)
	written += n
	if err != nil {
		return written, err
	}

	return written, nil
}

// writeRescue acquires application handles and release metadata, then asks
// the media adapter to write RESCUE_DATA into resourcesTmp.
func writeRescue(
	ctx context.Context,
	spec Spec,
	src ImageSource,
	rescue RescueWriter,
	plan Plan,
	resourcesTmp string,
) error {
	assets := make([]RescueAsset, 0, len(plan.Apps))

	for _, file := range plan.Apps {
		handle, err := src.Asset(ctx, plan.Version, file)
		if err != nil {
			return err
		}

		assets = append(assets, RescueAsset{
			RelPath: rescueRelPath(file.Filename),
			Asset:   handle,
		})
	}

	meta, err := src.ReleaseMetadata(ctx, plan.Version, plan.Apps)
	if err != nil {
		return err
	}

	return rescue.WriteRescue(ctx, spec.Type, RescueInput{
		Assets:      assets,
		UpdateJSON:  meta.UpdateJSON,
		UpdateSJSON: meta.UpdateSJSON,
	}, resourcesTmp)
}

// rescueRelPath stages an update file under the media's update/ tree,
// preserving the per-arch prefix (update/aarch64/incus.raw.gz) the way
// upstream buildImage joins the filename onto update/.
func rescueRelPath(filename string) string {
	return rescueTreePrefix + path.Clean("/" + filename)[1:]
}

// checkRenderedSeedSize rejects a renderer whose reported size disagrees
// with the returned tar bytes. That is a programming error in the injected
// collaborator, not a fetch or output failure.
func checkRenderedSeedSize(seedSize int64, tarBytes []byte) error {
	if seedSize == int64(len(tarBytes)) {
		return nil
	}

	return fmt.Errorf(
		"internal: seed renderer reported %d bytes but returned %d",
		seedSize,
		len(tarBytes),
	)
}

// copyN copies exactly n bytes from src to dst using buf. A short source
// is a read-side error (the acquired image is truncated). A final short
// read that delivers the last byte together with [io.EOF] is success.
// Context cancellation wraps [errdefs.ErrFetch].
func copyN(ctx context.Context, dst io.Writer, src io.Reader, n int64, buf []byte) (int64, error) {
	written := int64(0)

	for written < n {
		if err := ctx.Err(); err != nil {
			return written, fmt.Errorf("%w: %w", errdefs.ErrFetch, err)
		}

		chunk := int64(len(buf))
		remain := n - written
		if chunk > remain {
			chunk = remain
		}

		nr, rerr := src.Read(buf[:chunk])
		if nr > 0 {
			nw, werr := dst.Write(buf[:nr])
			written += int64(nw)
			if werr != nil {
				return written, fmt.Errorf("%w: %w", ErrOutput, werr)
			}

			if nw != nr {
				return written, fmt.Errorf("%w: short write", ErrOutput)
			}
		}

		if rerr != nil {
			return written, copyNReadErr(rerr, written, n)
		}
	}

	return written, nil
}

// copyNReadErr maps a source Read error. A final short read that delivers
// the last byte together with [io.EOF] is success.
func copyNReadErr(rerr error, written, n int64) error {
	if errors.Is(rerr, io.EOF) {
		if written >= n {
			return nil
		}

		return fmt.Errorf("%w: image truncated after %d of %d bytes", errdefs.ErrFetch, written, n)
	}

	return fmt.Errorf("%w: %w", errdefs.ErrFetch, rerr)
}

// copyAll copies src to dst until EOF using buf. Context cancellation
// wraps [errdefs.ErrFetch].
func copyAll(ctx context.Context, dst io.Writer, src io.Reader, buf []byte) (int64, error) {
	written := int64(0)

	for {
		if err := ctx.Err(); err != nil {
			return written, fmt.Errorf("%w: %w", errdefs.ErrFetch, err)
		}

		nr, rerr := src.Read(buf)
		if nr > 0 {
			nw, werr := dst.Write(buf[:nr])
			written += int64(nw)
			if werr != nil {
				return written, fmt.Errorf("%w: %w", ErrOutput, werr)
			}

			if nw != nr {
				return written, fmt.Errorf("%w: short write", ErrOutput)
			}
		}

		if rerr != nil {
			if errors.Is(rerr, io.EOF) {
				return written, nil
			}

			return written, fmt.Errorf("%w: %w", errdefs.ErrFetch, rerr)
		}
	}
}

// discardN reads and drops n bytes from src. A final short read that
// delivers the last byte together with [io.EOF] is success. Context
// cancellation wraps [errdefs.ErrFetch].
func discardN(ctx context.Context, src io.Reader, n int64, buf []byte) error {
	dropped := int64(0)

	for dropped < n {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("%w: %w", errdefs.ErrFetch, err)
		}

		chunk := int64(len(buf))
		remain := n - dropped
		if chunk > remain {
			chunk = remain
		}

		nr, rerr := src.Read(buf[:chunk])
		dropped += int64(nr)

		if rerr != nil {
			if errors.Is(rerr, io.EOF) {
				if dropped >= n {
					return nil
				}

				return fmt.Errorf("%w: image truncated while skipping seed region", errdefs.ErrFetch)
			}

			return fmt.Errorf("%w: %w", errdefs.ErrFetch, rerr)
		}
	}

	return nil
}

// writeAll writes p to dst in full.
func writeAll(dst io.Writer, p []byte) (int64, error) {
	written := 0

	for written < len(p) {
		n, err := dst.Write(p[written:])
		written += n
		if err != nil {
			return int64(written), err
		}

		if n == 0 {
			return int64(written), errors.New("short write")
		}
	}

	return int64(written), nil
}
