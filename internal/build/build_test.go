package build_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/componere/incusos-builder/internal/build"
	"github.com/componere/incusos-builder/internal/errdefs"
	mediamocks "github.com/componere/incusos-builder/internal/media/mocks"
	updatemocks "github.com/componere/incusos-builder/internal/update/mocks"
	uxmocks "github.com/componere/incusos-builder/internal/ux/mocks"

	apiimages "github.com/lxc/incus-os/incus-osd/api/images"
	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
)

const (
	testVersion = "202608102114"
	testImage   = "aarch64/IncusOS_202608102114.img.gz"
	testApp     = "aarch64/incus.raw.gz"
	testTar     = "SEED-TAR-BYTES"
)

// TestBuildSplicesSeedTar checks prefix/tar/suffix equality (by digest) and
// that the image handle is opened exactly twice (probe, then splice).
func TestBuildSplicesSeedTar(t *testing.T) {
	t.Parallel()

	img, gz, index, imageFile := fixtureImage(t)
	tar := []byte(testTar)
	require.Less(t, int64(len(tar)), img.Length)

	src := updatemocks.NewMockImageSource(t)
	image := updatemocks.NewMockVerifiedAsset(t)
	rescue := mediamocks.NewMockRescueWriter(t)
	rep := uxmocks.NewMockReporter(t)
	expectReporter(rep, 5, 5)

	src.EXPECT().Index(mock.Anything).Return(index, nil).Once()
	src.EXPECT().Asset(mock.Anything, testVersion, imageFile).Return(image, nil).Once()
	expectOpen(image, gz, 2)

	var out bytes.Buffer
	result, err := build.RunBuild(
		context.Background(),
		onlineSpec(),
		src,
		rescue,
		rep,
		stubRender(tar),
		&out,
		"",
		img.Start,
	)
	require.NoError(t, err)
	assert.Equal(t, testVersion, result.Version)
	assert.Equal(t, int64(len(tar)), result.SeedBytes)
	assert.False(t, result.Offline)
	assert.Equal(t, int64(len(img.Bytes)), result.BytesWritten)

	want := spliced(img.Bytes, img.Start, tar)
	assert.Equal(t, sha256.Sum256(want), sha256.Sum256(out.Bytes()), "spliced image digest")
	assert.Equal(t, want, out.Bytes())
}

// TestBuildOfflineRescueInput asserts the exact RescueInput (including
// verbatim metadata), one Open per rescue handle, and two Opens on the image.
func TestBuildOfflineRescueInput(t *testing.T) {
	t.Parallel()

	img, gz, index, imageFile := fixtureImage(t)
	appFile := appUpdateFile()
	index.Updates[0].Files = append(index.Updates[0].Files, appFile)

	tar := []byte(testTar)
	jsonMeta := []byte(`{"version":"202608102114"}`)
	sjsonMeta := []byte("-----BEGIN PKCS7-----\nverbatim-sjson\n-----END PKCS7-----")
	resourcesTmp := filepath.Join(t.TempDir(), "rescue.img")

	src := updatemocks.NewMockImageSource(t)
	image := updatemocks.NewMockVerifiedAsset(t)
	app := updatemocks.NewMockVerifiedAsset(t)
	rescue := mediamocks.NewMockRescueWriter(t)
	rep := uxmocks.NewMockReporter(t)
	expectReporter(rep, 6, 6)

	src.EXPECT().Index(mock.Anything).Return(index, nil).Once()
	src.EXPECT().Asset(mock.Anything, testVersion, imageFile).Return(image, nil).Once()
	src.EXPECT().Asset(mock.Anything, testVersion, appFile).Return(app, nil).Once()
	src.EXPECT().ReleaseMetadata(mock.Anything, testVersion, []apiimages.UpdateFile{appFile}).
		Return(build.ReleaseMetadata{UpdateJSON: jsonMeta, UpdateSJSON: sjsonMeta}, nil).Once()
	expectOpen(image, gz, 2)
	expectOpen(app, []byte("app-bytes"), 1)

	var got build.RescueInput
	rescue.EXPECT().WriteRescue(mock.Anything, build.ImageTypeRaw, mock.Anything, resourcesTmp).
		Run(func(ctx context.Context, _ build.ImageType, in build.RescueInput, _ string) {
			got = in
			for _, asset := range in.Assets {
				rc, err := asset.Asset.Open(ctx)
				require.NoError(t, err)
				require.NoError(t, rc.Close())
			}
		}).Return(nil).Once()

	var out bytes.Buffer
	result, err := build.RunBuild(
		context.Background(),
		offlineSpec(),
		src,
		rescue,
		rep,
		stubRender(tar),
		&out,
		resourcesTmp,
		img.Start,
	)
	require.NoError(t, err)
	assert.True(t, result.Offline)
	assert.Equal(t, resourcesTmp, result.ResourcesTmp)
	want := spliced(img.Bytes, img.Start, tar)
	assert.Equal(t, sha256.Sum256(want), sha256.Sum256(out.Bytes()), "spliced image digest")
	assert.True(t, bytes.Equal(jsonMeta, got.UpdateJSON), "update.json stored verbatim")
	assert.True(t, bytes.Equal(sjsonMeta, got.UpdateSJSON), "update.sjson stored verbatim")
	require.Len(t, got.Assets, 1)
	assert.Equal(t, "update/aarch64/incus.raw.gz", got.Assets[0].RelPath)
	assert.Equal(t, app, got.Assets[0].Asset)
}

// TestBuildOversizedTar maps a seed tar larger than the partition to
// [errdefs.ErrConfig].
func TestBuildOversizedTar(t *testing.T) {
	t.Parallel()

	img, gz, index, imageFile := fixtureImage(t)
	tar := bytes.Repeat([]byte("X"), int(img.Length)+1)

	src := updatemocks.NewMockImageSource(t)
	image := updatemocks.NewMockVerifiedAsset(t)
	rescue := mediamocks.NewMockRescueWriter(t)
	rep := uxmocks.NewMockReporter(t)
	expectReporter(rep, 4, 3)

	src.EXPECT().Index(mock.Anything).Return(index, nil).Once()
	src.EXPECT().Asset(mock.Anything, testVersion, imageFile).Return(image, nil).Once()
	expectOpen(image, gz, 1)

	var out bytes.Buffer
	_, err := build.RunBuild(
		context.Background(),
		onlineSpec(),
		src,
		rescue,
		rep,
		stubRender(tar),
		&out,
		"",
		img.Start,
	)
	require.Error(t, err)
	require.ErrorIs(t, err, errdefs.ErrConfig)
	assert.Empty(t, out.Bytes())
}

// TestBuildWriteFailure wraps a stream write error as [build.ErrOutput].
func TestBuildWriteFailure(t *testing.T) {
	t.Parallel()

	img, gz, index, imageFile := fixtureImage(t)
	tar := []byte(testTar)

	src := updatemocks.NewMockImageSource(t)
	image := updatemocks.NewMockVerifiedAsset(t)
	rescue := mediamocks.NewMockRescueWriter(t)
	rep := uxmocks.NewMockReporter(t)
	expectReporter(rep, 5, 4)

	src.EXPECT().Index(mock.Anything).Return(index, nil).Once()
	src.EXPECT().Asset(mock.Anything, testVersion, imageFile).Return(image, nil).Once()
	expectOpen(image, gz, 2)

	_, err := build.RunBuild(
		context.Background(),
		onlineSpec(),
		src,
		rescue,
		rep,
		stubRender(tar),
		errWriter{err: errors.New("disk full")},
		"",
		img.Start,
	)
	require.Error(t, err)
	require.ErrorIs(t, err, build.ErrOutput)
	assert.NotErrorIs(t, err, errdefs.ErrFetch)
}

// TestBuildReadFailure wraps a truncated splice stream as [errdefs.ErrFetch].
func TestBuildReadFailure(t *testing.T) {
	t.Parallel()

	img, gz, index, imageFile := fixtureImage(t)
	tar := []byte(testTar)
	require.Greater(t, len(gz), 32)

	src := updatemocks.NewMockImageSource(t)
	image := updatemocks.NewMockVerifiedAsset(t)
	rescue := mediamocks.NewMockRescueWriter(t)
	rep := uxmocks.NewMockReporter(t)
	expectReporter(rep, 5, 4)

	src.EXPECT().Index(mock.Anything).Return(index, nil).Once()
	src.EXPECT().Asset(mock.Anything, testVersion, imageFile).Return(image, nil).Once()
	image.EXPECT().Open(mock.Anything).RunAndReturn(func(context.Context) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(gz)), nil
	}).Once()
	image.EXPECT().Open(mock.Anything).RunAndReturn(func(context.Context) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(gz[:32])), nil
	}).Once()

	_, err := build.RunBuild(
		context.Background(),
		onlineSpec(),
		src,
		rescue,
		rep,
		stubRender(tar),
		io.Discard,
		"",
		img.Start,
	)
	require.Error(t, err)
	require.ErrorIs(t, err, errdefs.ErrFetch)
	assert.NotErrorIs(t, err, build.ErrOutput)
}

// TestBuildShiftedPartitionWrapsErrFetch asserts the production Build path
// probes GPT on handle Open #1 and maps a seed-data offset that is not
// productionSeedStart to [errdefs.ErrFetch] before splice.
func TestBuildShiftedPartitionWrapsErrFetch(t *testing.T) {
	t.Parallel()

	_, gz, index, imageFile := fixtureImage(t)

	src := updatemocks.NewMockImageSource(t)
	image := updatemocks.NewMockVerifiedAsset(t)
	rescue := mediamocks.NewMockRescueWriter(t)
	rep := uxmocks.NewMockReporter(t)
	expectReporter(rep, 3, 2)

	src.EXPECT().Index(mock.Anything).Return(index, nil).Once()
	src.EXPECT().Asset(mock.Anything, testVersion, imageFile).Return(image, nil).Once()
	expectOpen(image, gz, 1)

	var out bytes.Buffer
	_, err := build.Build(
		context.Background(),
		onlineSpec(),
		src,
		rescue,
		rep,
		stubRender([]byte(testTar)),
		&out,
		"",
	)
	require.Error(t, err)
	require.ErrorIs(t, err, errdefs.ErrFetch)
	assert.Contains(t, err.Error(), "seed-data starts at byte")
	assert.Empty(t, out.Bytes())
}

// TestBuildRendererSizeMismatch rejects a renderer whose reported size
// disagrees with the returned tar, without writing output.
func TestBuildRendererSizeMismatch(t *testing.T) {
	t.Parallel()

	img, gz, index, imageFile := fixtureImage(t)
	tar := []byte(testTar)

	src := updatemocks.NewMockImageSource(t)
	image := updatemocks.NewMockVerifiedAsset(t)
	rescue := mediamocks.NewMockRescueWriter(t)
	rep := uxmocks.NewMockReporter(t)
	expectReporter(rep, 4, 3)

	src.EXPECT().Index(mock.Anything).Return(index, nil).Once()
	src.EXPECT().Asset(mock.Anything, testVersion, imageFile).Return(image, nil).Once()
	expectOpen(image, gz, 1)

	var out bytes.Buffer
	_, err := build.RunBuild(
		context.Background(),
		onlineSpec(),
		src,
		rescue,
		rep,
		func(build.Seeds) ([]byte, int64, error) {
			return tar, int64(len(tar)) + 1, nil
		},
		&out,
		"",
		img.Start,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "seed renderer reported")
	require.NotErrorIs(t, err, errdefs.ErrFetch)
	require.NotErrorIs(t, err, build.ErrOutput)
	assert.Empty(t, out.Bytes())
}

// TestBuildCancelMidSplice maps a cancelled splice to [errdefs.ErrFetch]
// so Ctrl-C during copy is process exit code 5.
func TestBuildCancelMidSplice(t *testing.T) {
	t.Parallel()

	img, gz, index, imageFile := fixtureImage(t)
	tar := []byte(testTar)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	src := updatemocks.NewMockImageSource(t)
	image := updatemocks.NewMockVerifiedAsset(t)
	rescue := mediamocks.NewMockRescueWriter(t)
	rep := uxmocks.NewMockReporter(t)
	expectReporter(rep, 5, 4)

	src.EXPECT().Index(mock.Anything).Return(index, nil).Once()
	src.EXPECT().Asset(mock.Anything, testVersion, imageFile).Return(image, nil).Once()
	image.EXPECT().Open(mock.Anything).RunAndReturn(func(context.Context) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(gz)), nil
	}).Once()
	image.EXPECT().Open(mock.Anything).RunAndReturn(func(context.Context) (io.ReadCloser, error) {
		cancel()

		return io.NopCloser(bytes.NewReader(gz)), nil
	}).Once()

	_, err := build.RunBuild(
		ctx,
		onlineSpec(),
		src,
		rescue,
		rep,
		stubRender(tar),
		io.Discard,
		"",
		img.Start,
	)
	require.Error(t, err)
	require.ErrorIs(t, err, errdefs.ErrFetch)
	require.ErrorIs(t, err, context.Canceled)
	assert.NotErrorIs(t, err, build.ErrOutput)
}

// TestBuildNilRender rejects a missing renderer without touching ports.
func TestBuildNilRender(t *testing.T) {
	t.Parallel()

	_, err := build.Build(
		context.Background(),
		onlineSpec(),
		updatemocks.NewMockImageSource(t),
		mediamocks.NewMockRescueWriter(t),
		uxmocks.NewMockReporter(t),
		nil,
		io.Discard,
		"",
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "seed renderer is not wired")
}

// fixtureImage returns a gzipped GPT fixture and an index that resolves to it.
func fixtureImage(t *testing.T) (build.GPTImage, []byte, apiimages.Index, apiimages.UpdateFile) {
	t.Helper()

	img := build.MakeGPTImage(t, 512, 8, 15)
	gz := build.GzipBytes(t, img.Bytes)
	imageFile := apiimages.UpdateFile{
		Architecture: apiimages.UpdateFileArchitecture(build.ArchAarch64),
		Filename:     testImage,
		Type:         apiimages.UpdateFileTypeImageRaw,
		Size:         int64(len(gz)),
		Sha256:       "00",
	}
	index := apiimages.Index{Updates: []apiimages.UpdateFull{{
		Update: apiimages.Update{
			Version:  testVersion,
			Channels: []string{"stable"},
			Files:    []apiimages.UpdateFile{imageFile},
		},
	}}}

	return img, gz, index, imageFile
}

// appUpdateFile is the aarch64 incus application asset used by offline tests.
func appUpdateFile() apiimages.UpdateFile {
	return apiimages.UpdateFile{
		Architecture: apiimages.UpdateFileArchitecture(build.ArchAarch64),
		Filename:     testApp,
		Type:         apiimages.UpdateFileTypeApplication,
		Size:         9,
		Sha256:       "aa",
	}
}

// onlineSpec is a raw aarch64 spec without rescue media.
func onlineSpec() build.Spec {
	return build.Spec{
		Type:         build.ImageTypeRaw,
		Architecture: build.ArchAarch64,
		Channel:      "stable",
	}
}

// offlineSpec is onlineSpec plus an incus application and the offline flag.
func offlineSpec() build.Spec {
	spec := onlineSpec()
	spec.Offline = true
	spec.Seeds = build.Seeds{Applications: &apiseed.Applications{
		Applications: []apiseed.Application{{Name: "incus"}},
	}}

	return spec
}

// stubRender returns a SeedRenderFunc that yields a fixed tar.
func stubRender(tar []byte) build.SeedRenderFunc {
	return func(build.Seeds) ([]byte, int64, error) {
		return tar, int64(len(tar)), nil
	}
}

// spliced is the expected decompressed image after writing tar at start.
func spliced(img []byte, start int64, tar []byte) []byte {
	out := make([]byte, 0, len(img))
	out = append(out, img[:start]...)
	out = append(out, tar...)
	out = append(out, img[start+int64(len(tar)):]...)

	return out
}

// expectOpen stubs handle.Open to yield a fresh reader over gz, times times.
func expectOpen(handle *updatemocks.MockVerifiedAsset, gz []byte, times int) {
	handle.EXPECT().Open(mock.Anything).RunAndReturn(func(context.Context) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(gz)), nil
	}).Times(times)
}

// expectReporter allows steps Step calls and dones Done calls.
func expectReporter(rep *uxmocks.MockReporter, steps, dones int) {
	rep.EXPECT().Step(mock.Anything).Times(steps)
	rep.EXPECT().Done(mock.Anything).Times(dones)
}

// errWriter fails every Write.
type errWriter struct {
	err error
}

// Write always returns the injected error.
func (w errWriter) Write([]byte) (int, error) {
	return 0, w.err
}
