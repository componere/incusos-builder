package build

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/componere/incusos-builder/internal/update"
)

func TestCopyHelpersEOFWithData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		helper      string
		data        string
		n           int64
		bufSize     int
		wantErr     error
		errContains string
	}{
		{
			name:    "copyN single read last bytes with EOF",
			helper:  "copyN",
			data:    "hello",
			n:       5,
			bufSize: 16,
		},
		{
			name:    "copyN multi-read last bytes with EOF",
			helper:  "copyN",
			data:    "hello",
			n:       5,
			bufSize: 3,
		},
		{
			name:        "copyN short source with EOF is truncated",
			helper:      "copyN",
			data:        "hi",
			n:           5,
			bufSize:     16,
			wantErr:     update.ErrFetch,
			errContains: "image truncated after 2 of 5 bytes",
		},
		{
			name:    "discardN single read last bytes with EOF",
			helper:  "discardN",
			data:    "hello",
			n:       5,
			bufSize: 16,
		},
		{
			name:    "discardN multi-read last bytes with EOF",
			helper:  "discardN",
			data:    "hello",
			n:       5,
			bufSize: 3,
		},
		{
			name:        "discardN short source with EOF is truncated",
			helper:      "discardN",
			data:        "hi",
			n:           5,
			bufSize:     16,
			wantErr:     update.ErrFetch,
			errContains: "image truncated while skipping seed region",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			src := &eofWithDataReader{rest: []byte(tt.data)}
			buf := make([]byte, tt.bufSize)
			var err error
			var written int64
			var dst bytes.Buffer

			switch tt.helper {
			case "copyN":
				written, err = copyN(context.Background(), &dst, src, tt.n, buf)
			case "discardN":
				err = discardN(context.Background(), src, tt.n, buf)
			default:
				t.Fatalf("unknown helper %q", tt.helper)
			}

			if tt.wantErr != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tt.wantErr)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}

				return
			}

			require.NoError(t, err)
			if tt.helper == "copyN" {
				assert.Equal(t, tt.n, written)
				assert.Equal(t, tt.data, dst.String())
			}
		})
	}
}

func TestCopyHelpersCanceledContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		helper string
	}{
		{name: "copyN wraps cancel as ErrFetch", helper: "copyN"},
		{name: "copyAll wraps cancel as ErrFetch", helper: "copyAll"},
		{name: "discardN wraps cancel as ErrFetch", helper: "discardN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			src := bytes.NewReader([]byte("abcdefgh"))
			buf := make([]byte, 8)
			var err error

			switch tt.helper {
			case "copyN":
				_, err = copyN(ctx, io.Discard, src, 8, buf)
			case "copyAll":
				_, err = copyAll(ctx, io.Discard, src, buf)
			case "discardN":
				err = discardN(ctx, src, 8, buf)
			default:
				t.Fatalf("unknown helper %q", tt.helper)
			}

			require.Error(t, err)
			require.ErrorIs(t, err, update.ErrFetch)
			require.ErrorIs(t, err, context.Canceled)
			assert.NotErrorIs(t, err, ErrOutput)
		})
	}
}

// eofWithDataReader returns remaining bytes together with [io.EOF] on the
// call that exhausts data, which the [io.Reader] contract permits.
type eofWithDataReader struct {
	rest []byte
}

func (r *eofWithDataReader) Read(p []byte) (int, error) {
	if len(r.rest) == 0 {
		return 0, io.EOF
	}

	n := copy(p, r.rest)
	r.rest = r.rest[n:]
	if len(r.rest) == 0 {
		return n, io.EOF
	}

	return n, nil
}
