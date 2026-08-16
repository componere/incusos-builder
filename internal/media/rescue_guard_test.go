package media

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/componere/incusos-builder/internal/build"
	"github.com/componere/incusos-builder/internal/errdefs"
	"github.com/componere/incusos-builder/internal/update"
)

const isoLeakChildEnv = "TEST_ISO_LEAK_CHILD"

func TestWriteRescueRelPathCollision(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		assets []string
	}{
		{name: "collides with update.sjson", assets: []string{"update/update.sjson"}},
		{name: "collides with update.json", assets: []string{"update/update.json"}},
		{name: "duplicate RelPath", assets: []string{"update/incus.raw.gz", "update/incus.raw.gz"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			listed := make([]build.RescueAsset, 0, len(tt.assets))
			handles := make([]*memAsset, 0, len(tt.assets))
			for _, rel := range tt.assets {
				a := &memAsset{body: []byte("payload")}
				handles = append(handles, a)
				listed = append(listed, build.RescueAsset{RelPath: rel, Asset: a})
			}
			tmpPath := exclusiveTemp(t)
			err := NewWriter().WriteRescue(context.Background(), build.ImageTypeRaw, build.RescueInput{
				Assets:      listed,
				UpdateJSON:  []byte("{}"),
				UpdateSJSON: []byte("signed"),
			}, tmpPath)
			require.ErrorIs(t, err, build.ErrOutput)
			require.NotErrorIs(t, err, errdefs.ErrFetch)
			require.NotErrorIs(t, err, update.ErrFetch)
			assert.Contains(t, err.Error(), "duplicates an already staged file")
			for _, a := range handles {
				assert.Equal(t, int32(0), a.opens.Load())
			}
			assertEmptyFile(t, tmpPath)
		})
	}
}

func TestWriteRescueFetchErrorSurfacesAsOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		asset *errAsset
	}{
		{
			name:  "Open wraps ErrFetch",
			asset: &errAsset{size: 16, openErr: fmt.Errorf("%w: open failed", update.ErrFetch)},
		},
		{
			name:  "Read wraps ErrFetch",
			asset: &errAsset{size: 16, readErr: fmt.Errorf("%w: read failed", update.ErrFetch)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpPath := exclusiveTemp(t)
			err := NewWriter().WriteRescue(context.Background(), build.ImageTypeISO, build.RescueInput{
				Assets:      []build.RescueAsset{{RelPath: "update/incus.raw.gz", Asset: tt.asset}},
				UpdateJSON:  []byte("{}"),
				UpdateSJSON: []byte("signed"),
			}, tmpPath)
			require.ErrorIs(t, err, build.ErrOutput)
			require.NotErrorIs(t, err, errdefs.ErrFetch)
			require.NotErrorIs(t, err, update.ErrFetch)
			assert.Equal(t, int32(1), tt.asset.opens.Load())
		})
	}
}

func TestISOWorkspaceNotLeakedOnStageFailure(t *testing.T) {
	if os.Getenv(isoLeakChildEnv) == "1" {
		runISOWorkspaceLeakChild(t)
		return
	}

	tmp := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestISOWorkspaceNotLeakedOnStageFailure$", "-test.count=1")
	cmd.Env = append(os.Environ(), isoLeakChildEnv+"=1", "TMPDIR="+tmp)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "child: %s", out)

	matches, err := filepath.Glob(filepath.Join(tmp, "diskfs_iso*"))
	require.NoError(t, err)
	assert.Empty(t, matches, "leaked ISO workspaces: %v\nchild:\n%s", matches, out)
}

func runISOWorkspaceLeakChild(t *testing.T) {
	t.Helper()

	asset := &errAsset{size: 64, openErr: fmt.Errorf("%w: open failed", update.ErrFetch)}
	tmpPath := exclusiveTemp(t)
	err := NewWriter().WriteRescue(context.Background(), build.ImageTypeISO, build.RescueInput{
		Assets:      []build.RescueAsset{{RelPath: "update/incus.raw.gz", Asset: asset}},
		UpdateJSON:  []byte("{}"),
		UpdateSJSON: []byte("signed"),
	}, tmpPath)
	require.Error(t, err)
	require.ErrorIs(t, err, build.ErrOutput)
}

// errAsset is a VerifiedAsset whose Open or Read fails.
type errAsset struct {
	size    int64
	openErr error
	readErr error
	opens   atomic.Int32
}

// Open returns a failing reader or a failing ReadCloser.
func (a *errAsset) Open(context.Context) (io.ReadCloser, error) {
	a.opens.Add(1)
	if a.openErr != nil {
		return nil, a.openErr
	}
	return io.NopCloser(&errReader{err: a.readErr}), nil
}

// Size is the advertised byte count.
func (a *errAsset) Size() int64 {
	return a.size
}

type errReader struct {
	err error
}

func (r *errReader) Read([]byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	return 0, io.EOF
}
