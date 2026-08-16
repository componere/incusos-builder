package build_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/componere/incusos-builder/internal/build"
	mediamocks "github.com/componere/incusos-builder/internal/media/mocks"
	"github.com/componere/incusos-builder/internal/update"
	updatemocks "github.com/componere/incusos-builder/internal/update/mocks"
	uxmocks "github.com/componere/incusos-builder/internal/ux/mocks"

	apiimages "github.com/lxc/incus-os/incus-osd/api/images"
	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
)

const (
	testVersion = "202608102114"
	testImage   = "aarch64/IncusOS_202608102114.img.gz"
	testApp     = "aarch64/incus.raw.gz"
)

// TestBuildSplicesSeedTar checks prefix/tar/suffix equality and that the
// image handle is opened exactly twice (probe, then splice).
func TestBuildSplicesSeedTar(t *testing.T) {
	t.Parallel()

	img, gz, index, imageFile := fixtureImage(t)
	tar := []byte("SEED-TAR-BYTES")
	require.Less(t, int64(len(tar)), img.Length)

	src := updatemocks.NewMockImageSource(t)
	image := updatemocks.NewMockVerifiedAsset(t)
	rescue := mediamocks.NewMockRescueWriter(t)
	rep := uxmocks.NewMockReporter(t)
	expectReporter(rep)

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
		&out,
		"",
		stubRender(tar),
		img.Start,
	)
	require.NoError(t, err)
	assert.Equal(t, testVersion, result.Version)
	assert.Equal(t, int64(len(tar)), result.SeedBytes)
	assert.False(t, result.Offline)
	assert.Equal(t, spliced(img.Bytes, img.Start, tar), out.Bytes())
	assert.Equal(t, int64(len(img.Bytes)), result.BytesWritten)
}

// TestBuildOfflineRescueInput asserts the exact RescueInput (including
// verbatim metadata), one Open per rescue handle, and two Opens on the image.
func TestBuildOfflineRescueInput(t *testing.T) {
	t.Parallel()

	img, gz, index, imageFile := fixtureImage(t)
	appFile := appUpdateFile()
	index.Updates[0].Files = append(index.Updates[0].Files, appFile)

	tar := []byte("SEED-TAR-BYTES")
	jsonMeta := []byte(`{"version":"202608102114"}`)
	sjsonMeta := []byte("-----BEGIN PKCS7-----\nverbatim-sjson\n-----END PKCS7-----")
	resourcesTmp := t.TempDir() + "/rescue.img"

	src := updatemocks.NewMockImageSource(t)
	image := updatemocks.NewMockVerifiedAsset(t)
	app := updatemocks.NewMockVerifiedAsset(t)
	rescue := mediamocks.NewMockRescueWriter(t)
	rep := uxmocks.NewMockReporter(t)
	expectReporter(rep)

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
				_, _ = io.Copy(io.Discard, rc)
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
		&out,
		resourcesTmp,
		stubRender(tar),
		img.Start,
	)
	require.NoError(t, err)
	assert.True(t, result.Offline)
	assert.Equal(t, resourcesTmp, result.ResourcesTmp)
	assert.Equal(t, spliced(img.Bytes, img.Start, tar), out.Bytes())
	assert.Equal(t, jsonMeta, got.UpdateJSON)
	assert.Equal(t, sjsonMeta, got.UpdateSJSON)
	require.Len(t, got.Assets, 1)
	assert.Equal(t, "update/aarch64/incus.raw.gz", got.Assets[0].RelPath)
	assert.Equal(t, app, got.Assets[0].Asset)
}

// TestBuildOversizedTar maps a seed tar larger than the partition to the
// exit-3-family sentinel [build.ErrSeedTooLarge].
func TestBuildOversizedTar(t *testing.T) {
	t.Parallel()

	img, gz, index, imageFile := fixtureImage(t)
	tar := bytes.Repeat([]byte("X"), int(img.Length)+1)

	src := updatemocks.NewMockImageSource(t)
	image := updatemocks.NewMockVerifiedAsset(t)
	rescue := mediamocks.NewMockRescueWriter(t)
	rep := uxmocks.NewMockReporter(t)
	expectReporter(rep)

	src.EXPECT().Index(mock.Anything).Return(index, nil).Once()
	src.EXPECT().Asset(mock.Anything, testVersion, imageFile).Return(image, nil).Once()
	expectOpen(image, gz, 1)

	_, err := build.RunBuild(
		context.Background(),
		onlineSpec(),
		src,
		rescue,
		rep,
		io.Discard,
		"",
		stubRender(tar),
		img.Start,
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, build.ErrSeedTooLarge)
}

// TestBuildWriteFailure wraps a stream write error as [build.ErrOutput].
func TestBuildWriteFailure(t *testing.T) {
	t.Parallel()

	img, gz, index, imageFile := fixtureImage(t)
	tar := []byte("SEED-TAR-BYTES")

	src := updatemocks.NewMockImageSource(t)
	image := updatemocks.NewMockVerifiedAsset(t)
	rescue := mediamocks.NewMockRescueWriter(t)
	rep := uxmocks.NewMockReporter(t)
	expectReporter(rep)

	src.EXPECT().Index(mock.Anything).Return(index, nil).Once()
	src.EXPECT().Asset(mock.Anything, testVersion, imageFile).Return(image, nil).Once()
	expectOpen(image, gz, 2)

	_, err := build.RunBuild(
		context.Background(),
		onlineSpec(),
		src,
		rescue,
		rep,
		errWriter{err: errors.New("disk full")},
		"",
		stubRender(tar),
		img.Start,
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, build.ErrOutput)
	assert.NotErrorIs(t, err, update.ErrFetch)
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
		io.Discard,
		"",
		nil,
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

// expectReporter allows Step and Done calls from Build's phase reporting.
func expectReporter(rep *uxmocks.MockReporter) {
	rep.EXPECT().Step(mock.Anything)
	rep.EXPECT().Done(mock.Anything)
}

// errWriter fails every Write.
type errWriter struct {
	err error
}

// Write always returns the injected error.
func (w errWriter) Write([]byte) (int, error) {
	return 0, w.err
}
