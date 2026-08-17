package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/componere/incusos-builder/internal/errdefs"
)

// TestExitCode maps sentinels and wraps onto process exit codes.
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
		{name: "config wrapped 2 deep", err: wrap2(errdefs.ErrConfig, "field x", "decode"), want: exitConfig},
		{name: "decrypt", err: errdefs.ErrDecrypt, want: exitDecrypt},
		{name: "wrapped decrypt", err: fmt.Errorf("%w: sops", errdefs.ErrDecrypt), want: exitDecrypt},
		{name: "decrypt wrapped 2 deep", err: wrap2(errdefs.ErrDecrypt, "sops", "age key"), want: exitDecrypt},
		{name: "fetch", err: errdefs.ErrFetch, want: exitFetch},
		{name: "wrapped fetch", err: fmt.Errorf("%w: index", errdefs.ErrFetch), want: exitFetch},
		{name: "fetch wrapped 2 deep", err: wrap2(errdefs.ErrFetch, "index", "https"), want: exitFetch},
		{name: "canceled inside fetch", err: fmt.Errorf("%w: %w", errdefs.ErrFetch, context.Canceled), want: exitFetch},
		{
			name: "canceled inside fetch wrapped 2 deep",
			err:  fmt.Errorf("asset: %w", fmt.Errorf("%w: %w", errdefs.ErrFetch, context.Canceled)),
			want: exitFetch,
		},
		{name: "version not found", err: errdefs.ErrVersionNotFound, want: exitFetch},
		{name: "wrapped version not found", err: fmt.Errorf("%w: pin", errdefs.ErrVersionNotFound), want: exitFetch},
		{
			name: "version not found wrapped 2 deep",
			err:  wrap2(errdefs.ErrVersionNotFound, "pin", "channel"),
			want: exitFetch,
		},
		{name: "output", err: errdefs.ErrOutput, want: exitOutput},
		{name: "wrapped output", err: fmt.Errorf("%w: rename", errdefs.ErrOutput), want: exitOutput},
		{name: "output wrapped 2 deep", err: wrap2(errdefs.ErrOutput, "rename", "claim"), want: exitOutput},
		{name: "unknown", err: errors.New("boom"), want: exitInternal},
		{name: "plain wrapped", err: fmt.Errorf("inner: %w", errors.New("boom")), want: exitInternal},
		{name: "canceled alone", err: context.Canceled, want: exitInternal},
		{
			name: "cobra unknown command",
			err:  fmt.Errorf("unknown command %q for %q", "frobnicate", appName),
			want: exitUsage,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, exitCode(tc.err))
		})
	}
}

// TestExitCodeUnknownFlagThroughRoot maps an unknown root flag to exit 2.
func TestExitCodeUnknownFlagThroughRoot(t *testing.T) {
	t.Parallel()

	root := NewRootCommand(Options{
		Out: ioDiscard(),
		Err: ioDiscard(),
		In:  strings.NewReader(""),
	})
	root.SetArgs([]string{"--not-a-real-flag"})
	err := root.Execute()
	require.Error(t, err)
	require.True(t, IsUsage(err), "err=%v", err)
	require.Equal(t, exitUsage, exitCode(err))
}

// TestExitCodeCobraUnknownCommand maps Cobra's untyped unknown-command error to exit 2.
func TestExitCodeCobraUnknownCommand(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("unknown command %q for %q", "frobnicate", appName)
	require.False(t, IsUsage(err), "cobra's error is untyped")
	require.True(t, isUnknownCommandError(err), "err=%v", err)
	require.Equal(t, exitUsage, exitCode(err))
	require.Equal(t, exitInternal, exitCode(errors.New("boom")))
}

// TestWriteErrorJSON writes one --json failure document.
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

// TestWriteErrorJSONGoldens locks the --json failure document for each exit class.
func TestWriteErrorJSONGoldens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		want     string
		wantCode int
	}{
		{
			name:     "nil",
			err:      nil,
			want:     "{\"error\":{\"code\":0,\"message\":\"\"}}\n",
			wantCode: exitSuccess,
		},
		{
			name:     "usage",
			err:      usagef("bad path"),
			want:     "{\"error\":{\"code\":2,\"message\":\"usage error: bad path\"}}\n",
			wantCode: exitUsage,
		},
		{
			name:     "config",
			err:      fmt.Errorf("%w: field seeds.install", errdefs.ErrConfig),
			want:     "{\"error\":{\"code\":3,\"message\":\"invalid config: field seeds.install\"}}\n",
			wantCode: exitConfig,
		},
		{
			name:     "decrypt",
			err:      fmt.Errorf("%w: sops", errdefs.ErrDecrypt),
			want:     "{\"error\":{\"code\":4,\"message\":\"decryption failed: sops\"}}\n",
			wantCode: exitDecrypt,
		},
		{
			name:     "fetch",
			err:      fmt.Errorf("%w: index", errdefs.ErrFetch),
			want:     "{\"error\":{\"code\":5,\"message\":\"acquisition failed: index\"}}\n",
			wantCode: exitFetch,
		},
		{
			name:     "canceled inside fetch",
			err:      fmt.Errorf("%w: %w", errdefs.ErrFetch, context.Canceled),
			want:     "{\"error\":{\"code\":5,\"message\":\"acquisition failed: context canceled\"}}\n",
			wantCode: exitFetch,
		},
		{
			name:     "version not found",
			err:      fmt.Errorf("%w: pin", errdefs.ErrVersionNotFound),
			want:     "{\"error\":{\"code\":5,\"message\":\"version not found: pin\"}}\n",
			wantCode: exitFetch,
		},
		{
			name:     "output",
			err:      fmt.Errorf("%w: rename", errdefs.ErrOutput),
			want:     "{\"error\":{\"code\":6,\"message\":\"output write failed: rename\"}}\n",
			wantCode: exitOutput,
		},
		{
			name:     "plain",
			err:      errors.New("boom"),
			want:     "{\"error\":{\"code\":1,\"message\":\"boom\"}}\n",
			wantCode: exitInternal,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			require.NoError(t, writeErrorJSON(&buf, tc.err))
			require.Equal(t, tc.want, buf.String())

			var got errorEnvelope
			require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
			require.Equal(t, tc.wantCode, got.Error.Code)
			if tc.err == nil {
				require.Empty(t, got.Error.Message)
				return
			}
			require.Equal(t, tc.err.Error(), got.Error.Message)
		})
	}
}

// wrap2 wraps sentinel two levels deep for exitCode cases.
func wrap2(sentinel error, inner, outer string) error {
	return fmt.Errorf("%s: %w", outer, fmt.Errorf("%w: %s", sentinel, inner))
}
