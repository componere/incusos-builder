package update

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWarnIfLowSpaceWalksMissingCacheDir(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	missing := filepath.Join(parent, "fresh-cache")
	rep := &recordingReporter{}
	cache, err := newAssetCache(missing, rep)
	require.NoError(t, err)
	_, err = os.Stat(cache.dir)
	require.ErrorIs(t, err, os.ErrNotExist)

	cache.freeBytes = func(dir string) (uint64, error) {
		if dir == cache.dir {
			return 0, os.ErrNotExist
		}
		return 1, nil
	}
	cache.warnIfLowSpace(4096)
	assert.True(t, rep.hasStep(stepSpaceWarn))
	assert.True(t, rep.hasDone(stepSpaceWarn))
	_, err = os.Stat(cache.dir)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestWarnIfLowSpaceRealStatfsMissingDir(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	missing := filepath.Join(parent, "fresh-cache")
	rep := &recordingReporter{}
	cache, err := newAssetCache(missing, rep)
	require.NoError(t, err)
	_, err = os.Stat(cache.dir)
	require.ErrorIs(t, err, os.ErrNotExist)

	cache.warnIfLowSpace(1 << 62)
	assert.True(t, rep.hasStep(stepSpaceWarn))
	assert.True(t, rep.hasDone(stepSpaceWarn))
	_, err = os.Stat(cache.dir)
	assert.ErrorIs(t, err, os.ErrNotExist)
}
