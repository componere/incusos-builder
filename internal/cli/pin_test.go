package cli

import (
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/require"
)

const wantPin = "v0.0.0-20260815030500-0f5b8057f2fc"

// TestIncusOSPinFromCoversLookupCases injects fake build info so the
// helper can be exercised without a real module graph.
func TestIncusOSPinFromCoversLookupCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		info *debug.BuildInfo
		want string
	}{
		{name: "nil info", info: nil, want: unknownPin},
		{name: "empty deps", info: &debug.BuildInfo{}, want: unknownPin},
		{
			name: "other modules only",
			info: &debug.BuildInfo{
				Deps: []*debug.Module{{Path: "example.com/other", Version: "v1.2.3"}},
			},
			want: unknownPin,
		},
		{
			name: "matching empty version",
			info: &debug.BuildInfo{
				Deps: []*debug.Module{{Path: incusOSModule, Version: "  "}},
			},
			want: unknownPin,
		},
		{
			name: "matching recorded version",
			info: &debug.BuildInfo{
				Deps: []*debug.Module{{Path: incusOSModule, Version: wantPin}},
			},
			want: wantPin,
		},
		{
			name: "nil dep entries are skipped",
			info: &debug.BuildInfo{
				Deps: []*debug.Module{nil, {Path: incusOSModule, Version: wantPin}},
			},
			want: wantPin,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, IncusOSPinFrom(tc.info))
		})
	}
}

// TestIncusOSPinOnTestBinaryDocumentsMissingDeps records that `go test`
// binaries omit the module dep table, so [IncusOSPin] renders "unknown".
func TestIncusOSPinOnTestBinaryDocumentsMissingDeps(t *testing.T) {
	t.Parallel()
	require.Equal(t, unknownPin, IncusOSPin())
}
