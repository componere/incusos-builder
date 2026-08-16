package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/componere/incusos-builder/internal/errdefs"
)

func TestExitCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "nil", err: nil, want: exitSuccess},
		{name: "usage", err: ErrUsage, want: exitUsage},
		{name: "wrapped usage", err: usagef("bad path"), want: exitUsage},
		{name: "config", err: errdefs.ErrConfig, want: exitConfig},
		{name: "wrapped config", err: fmt.Errorf("%w: field x", errdefs.ErrConfig), want: exitConfig},
		{name: "decrypt", err: errdefs.ErrDecrypt, want: exitDecrypt},
		{name: "wrapped decrypt", err: fmt.Errorf("%w: sops", errdefs.ErrDecrypt), want: exitDecrypt},
		{name: "fetch", err: errdefs.ErrFetch, want: exitFetch},
		{name: "wrapped fetch", err: fmt.Errorf("%w: index", errdefs.ErrFetch), want: exitFetch},
		{name: "version not found", err: errdefs.ErrVersionNotFound, want: exitFetch},
		{name: "wrapped version not found", err: fmt.Errorf("%w: pin", errdefs.ErrVersionNotFound), want: exitFetch},
		{name: "output", err: errdefs.ErrOutput, want: exitOutput},
		{name: "wrapped output", err: fmt.Errorf("%w: rename", errdefs.ErrOutput), want: exitOutput},
		{name: "unknown", err: errors.New("boom"), want: exitInternal},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, exitCode(tc.err))
		})
	}
}

func TestWriteErrorJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := fmt.Errorf("%w: field seeds.install", errdefs.ErrConfig)
	require.NoError(t, writeErrorJSON(&buf, err))

	var got errorEnvelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	require.Equal(t, exitConfig, got.Error.Code)
	require.Equal(t, err.Error(), got.Error.Message)
}
