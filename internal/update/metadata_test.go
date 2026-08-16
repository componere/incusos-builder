package update

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiimages "github.com/lxc/incus-os/incus-osd/api/images"
)

func TestReleaseMetadataFetchesBothAndReturnsVerbatim(t *testing.T) {
	t.Parallel()
	payload := []byte("app-bytes")
	file := testFile(payload)
	updateJSON := testUpdateJSON(t, testVersion, file)
	sjson := testSJSON(updateJSON)
	ts := newTestServer(t, map[string][]byte{
		"/" + testVersion + "/" + updateJSONName:  updateJSON,
		"/" + testVersion + "/" + updateSJSONName: sjson,
	})
	rep := &recordingReporter{}
	src := ts.newHTTPS(t, rep)

	meta, err := src.ReleaseMetadata(t.Context(), testVersion, []apiimages.UpdateFile{file})
	require.NoError(t, err)
	require.True(t, bytes.Equal(updateJSON, meta.UpdateJSON), "verbatim update.json")
	require.True(t, bytes.Equal(sjson, meta.UpdateSJSON), "verbatim update.sjson")
	assert.False(t, bytes.Equal(meta.UpdateJSON, meta.UpdateSJSON))
	require.Equal(t, int64(2), ts.hits.Load())
	done, total := rep.lastProgressPair()
	assert.Equal(t, int64(len(sjson)), done)
	assert.Equal(t, int64(len(sjson)), total)
	assert.ElementsMatch(t, []string{
		"/" + testVersion + "/" + updateJSONName,
		"/" + testVersion + "/" + updateSJSONName,
	}, ts.seen)
	assert.True(t, rep.hasStep(stepMetadata))
	assert.True(t, rep.hasDone(stepMetadata))
}

func TestReleaseMetadataCapEnforced(t *testing.T) {
	t.Parallel()
	file := testFile([]byte("x"))
	updateJSON := testUpdateJSON(t, testVersion, file)
	ts := newTestServer(t, map[string][]byte{
		"/" + testVersion + "/" + updateJSONName:  bytes.Repeat([]byte("a"), 32),
		"/" + testVersion + "/" + updateSJSONName: testSJSON(updateJSON),
	})
	src := ts.newHTTPS(t, nil)
	src.metaLimit = 8
	_, err := src.ReleaseMetadata(t.Context(), testVersion, []apiimages.UpdateFile{file})
	require.ErrorIs(t, err, ErrFetch)
	assert.Contains(t, err.Error(), "cap")
}

func TestReleaseMetadataStructuralFailures(t *testing.T) {
	t.Parallel()
	file := testFile([]byte("app"))
	goodJSON := testUpdateJSON(t, testVersion, file)
	goodSJSON := testSJSON(goodJSON)

	t.Run("wrong json version", func(t *testing.T) {
		t.Parallel()
		wrong := testUpdateJSON(t, "199901010000", file)
		ts := newTestServer(t, map[string][]byte{
			"/" + testVersion + "/" + updateJSONName:  wrong,
			"/" + testVersion + "/" + updateSJSONName: goodSJSON,
		})
		_, err := ts.newHTTPS(t, nil).ReleaseMetadata(t.Context(), testVersion, []apiimages.UpdateFile{file})
		require.ErrorIs(t, err, ErrFetch)
		assert.Contains(t, err.Error(), updateJSONName)
	})

	t.Run("wrong sjson version", func(t *testing.T) {
		t.Parallel()
		wrongPayload := testUpdateJSON(t, "199901010000", file)
		ts := newTestServer(t, map[string][]byte{
			"/" + testVersion + "/" + updateJSONName:  goodJSON,
			"/" + testVersion + "/" + updateSJSONName: testSJSON(wrongPayload),
		})
		_, err := ts.newHTTPS(t, nil).ReleaseMetadata(t.Context(), testVersion, []apiimages.UpdateFile{file})
		require.ErrorIs(t, err, ErrFetch)
		assert.Contains(t, err.Error(), updateSJSONName)
	})

	t.Run("missing selected file", func(t *testing.T) {
		t.Parallel()
		empty := testUpdateJSON(t, testVersion)
		ts := newTestServer(t, map[string][]byte{
			"/" + testVersion + "/" + updateJSONName:  goodJSON,
			"/" + testVersion + "/" + updateSJSONName: testSJSON(empty),
		})
		_, err := ts.newHTTPS(t, nil).ReleaseMetadata(t.Context(), testVersion, []apiimages.UpdateFile{file})
		require.ErrorIs(t, err, ErrFetch)
		assert.Contains(t, err.Error(), "missing selected file")
	})

	t.Run("hash mismatch in payload", func(t *testing.T) {
		t.Parallel()
		other := file
		other.Sha256 = hexOf([]byte("other"))
		payload := testUpdateJSON(t, testVersion, other)
		ts := newTestServer(t, map[string][]byte{
			"/" + testVersion + "/" + updateJSONName:  goodJSON,
			"/" + testVersion + "/" + updateSJSONName: testSJSON(payload),
		})
		_, err := ts.newHTTPS(t, nil).ReleaseMetadata(t.Context(), testVersion, []apiimages.UpdateFile{file})
		require.ErrorIs(t, err, ErrFetch)
		assert.Contains(t, err.Error(), "missing selected file")
	})

	t.Run("malformed mime", func(t *testing.T) {
		t.Parallel()
		ts := newTestServer(t, map[string][]byte{
			"/" + testVersion + "/" + updateJSONName:  goodJSON,
			"/" + testVersion + "/" + updateSJSONName: []byte("not-a-mime-document"),
		})
		_, err := ts.newHTTPS(t, nil).ReleaseMetadata(t.Context(), testVersion, []apiimages.UpdateFile{file})
		require.ErrorIs(t, err, ErrFetch)
	})
}

func TestReleaseMetadataMissingSJSON(t *testing.T) {
	t.Parallel()
	file := testFile([]byte("app"))
	updateJSON := testUpdateJSON(t, testVersion, file)
	ts := newTestServer(t, map[string][]byte{
		"/" + testVersion + "/" + updateJSONName: updateJSON,
	})
	_, err := ts.newHTTPS(t, nil).ReleaseMetadata(t.Context(), testVersion, []apiimages.UpdateFile{file})
	require.ErrorIs(t, err, ErrFetch)
	assert.Contains(t, err.Error(), "404")
}

func TestReleaseMetadataZeroByteSJSON(t *testing.T) {
	t.Parallel()
	file := testFile([]byte("app"))
	updateJSON := testUpdateJSON(t, testVersion, file)
	ts := newTestServer(t, map[string][]byte{
		"/" + testVersion + "/" + updateJSONName:  updateJSON,
		"/" + testVersion + "/" + updateSJSONName: []byte{},
	})
	_, err := ts.newHTTPS(t, nil).ReleaseMetadata(t.Context(), testVersion, []apiimages.UpdateFile{file})
	require.ErrorIs(t, err, ErrFetch)
}

func TestReleaseMetadataFailureOmitsDone(t *testing.T) {
	t.Parallel()
	file := testFile([]byte("app"))
	rep := &recordingReporter{}
	ts := newTestServer(t, map[string][]byte{})
	src := ts.newHTTPS(t, rep)
	_, err := src.ReleaseMetadata(t.Context(), testVersion, []apiimages.UpdateFile{file})
	require.ErrorIs(t, err, ErrFetch)
	assert.True(t, rep.hasStep(stepMetadata))
	assert.False(t, rep.hasDone(stepMetadata))
}
