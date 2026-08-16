package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/pgzip"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/componere/incusos-builder/internal/build"
	"github.com/componere/incusos-builder/internal/ux"
)

const (
	onlineConfigYAML  = "version: 1\nimage:\n  type: iso\n  architecture: x86_64\n"
	offlineConfigYAML = "" +
		"version: 1\n" +
		"image:\n" +
		"  type: iso\n" +
		"  architecture: x86_64\n" +
		"  offline: true\n" +
		"seeds:\n" +
		"  applications:\n" +
		"    applications:\n" +
		"      - name: incus\n"
)

// TestBuildUsageMatrix covers required-flag and stream-usage errors.
func TestBuildUsageMatrix(t *testing.T) {
	t.Setenv(envCI, "")

	tests := []struct {
		name    string
		args    []string
		stdin   string
		want    string
		jsonOut bool
	}{
		{
			name: "missing config",
			args: []string{"build", "-o", "out.iso"},
			want: "usage error: -f/--config is required",
		},
		{
			name: "missing output",
			args: []string{"build", "-f", "-"},
			want: "usage error: -o/--output is required",
		},
		{
			name:    "json with stdout dash",
			args:    []string{"build", "-f", "-", "-o", "-", "--json"},
			stdin:   onlineConfigYAML,
			want:    "usage error: --json cannot be combined with -o -",
			jsonOut: true,
		},
		{
			name:  "offline with stdout dash",
			args:  []string{"build", "-f", "-", "-o", "-"},
			stdin: offlineConfigYAML,
			want:  "usage error: offline builds cannot use -o -",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			opts := Options{
				In:        strings.NewReader(tc.stdin),
				Out:       &stdout,
				Err:       io.Discard,
				Viper:     viper.New(),
				StdinTTY:  func() bool { return true },
				StdoutTTY: func() bool { return true },
				StderrTTY: func() bool { return true },
			}
			root := NewRootCommand(opts)
			root.SetArgs(tc.args)

			err := root.Execute()
			require.Error(t, err)
			require.True(t, IsUsage(err), "got %v", err)
			require.Equal(t, tc.want, err.Error())
			require.Equal(t, exitUsage, exitCode(err))
			if tc.jsonOut {
				assertErrorEnvelope(t, stdout.Bytes(), exitUsage, tc.want)
			} else {
				require.Empty(t, stdout.String())
			}
		})
	}
}

// TestSelectImageSourceByServerShape maps --server onto HTTPS, local, or usage.
func TestSelectImageSourceByServerShape(t *testing.T) {
	t.Parallel()

	reporter := ux.New(ux.ColorModeNever, ux.ProgressModeNever, io.Discard)
	cache := t.TempDir()

	httpsSrc, err := selectImageSource("https://images.example/os", cache, reporter)
	require.NoError(t, err)
	require.NotNil(t, httpsSrc)

	localDir := t.TempDir()
	localSrc, err := selectImageSource(localDir, cache, reporter)
	require.NoError(t, err)
	require.NotNil(t, localSrc)

	_, err = selectImageSource("http://insecure.example/os", cache, reporter)
	require.Error(t, err)
	require.True(t, IsUsage(err), "plain http is a usage error, got %v", err)

	_, err = selectImageSource("/no/such/mirror", cache, reporter)
	require.Error(t, err)
	require.True(t, IsUsage(err), "got %v", err)

	filePath := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(filePath, []byte("x"), 0o600))
	_, err = selectImageSource(filePath, cache, reporter)
	require.Error(t, err)
	require.True(t, IsUsage(err), "got %v", err)
}

// TestBuildConfirmSeamHonorsNoInput refuses overwrite without --force.
//
// The injected Confirm is the command that actually runs. A positive
// control with input allowed proves the seam is reached; --no-input
// must not call it.
func TestBuildConfirmSeamHonorsNoInput(t *testing.T) {
	t.Setenv(envCI, "")

	tests := []struct {
		name        string
		args        []string
		wantBlocked bool
	}{
		{
			name:        "no-input refuses without calling confirm",
			args:        []string{"--no-input"},
			wantBlocked: false,
		},
		{
			name:        "input allowed reaches confirm seam",
			args:        nil,
			wantBlocked: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cfg := filepath.Join(dir, "config.yaml")
			out := filepath.Join(dir, "out.iso")
			require.NoError(t, os.WriteFile(cfg, []byte(onlineConfigYAML), 0o600))
			require.NoError(t, os.WriteFile(out, []byte("existing"), 0o600))

			blocked := false
			opts := Options{
				In:        strings.NewReader(""),
				Out:       io.Discard,
				Err:       io.Discard,
				Viper:     viper.New(),
				StdinTTY:  func() bool { return true },
				StdoutTTY: func() bool { return true },
				StderrTTY: func() bool { return true },
			}
			root := newRootWithBuild(t, opts, buildDeps{
				Confirm: func() (bool, error) {
					blocked = true
					return false, nil
				},
			})
			root.SetArgs(append([]string{"build", "-f", cfg, "-o", out}, tc.args...))

			err := root.Execute()
			require.Error(t, err)
			require.True(t, IsUsage(err), "got %v", err)
			require.Contains(t, err.Error(), "refusing to overwrite")
			require.Equal(t, exitUsage, exitCode(err))
			require.Equal(t, tc.wantBlocked, blocked, "confirm seam reached=%v, want %v", blocked, tc.wantBlocked)
			require.Equal(t, "existing", readFile(t, out))
		})
	}
}

// newRootWithBuild is NewRootCommand with the production build command
// replaced by one that carries injected deps.
func newRootWithBuild(t *testing.T, opts Options, deps buildDeps) *cobra.Command {
	t.Helper()
	root := NewRootCommand(opts)
	for _, cmd := range root.Commands() {
		if cmd.Name() == "build" {
			root.RemoveCommand(cmd)
			break
		}
	}
	root.AddCommand(newBuildCommandWith(opts, deps))
	return root
}

// TestWrapStoredWriterGzipDigestHashesCompressedBytes covers the .gz
// recompression path independently of a full build.
func TestWrapStoredWriterGzipDigestHashesCompressedBytes(t *testing.T) {
	t.Parallel()

	payload := []byte("tiny spliced stream")
	var stored bytes.Buffer
	hasher := &hashingWriter{w: &stored, h: sha256.New()}
	dest, closer := wrapStoredWriter("out.iso.gz", hasher)
	_, err := dest.Write(payload)
	require.NoError(t, err)
	require.NoError(t, closer())

	wantCompressed := gzipBytesFixed(t, payload)
	require.Equal(t, wantCompressed, stored.Bytes())
	require.Equal(t, sha256Hex(wantCompressed), hex.EncodeToString(hasher.h.Sum(nil)))
	require.NotEqual(t, sha256Hex(payload), hex.EncodeToString(hasher.h.Sum(nil)))

	var plain bytes.Buffer
	plainHasher := &hashingWriter{w: &plain, h: sha256.New()}
	plainDest, plainCloser := wrapStoredWriter("out.iso", plainHasher)
	_, err = plainDest.Write(payload)
	require.NoError(t, err)
	require.NoError(t, plainCloser())
	require.Equal(t, payload, plain.Bytes())
	require.Equal(t, sha256Hex(payload), hex.EncodeToString(plainHasher.h.Sum(nil)))
}

// TestBuildJSONEnvelopeShape writes one success document and no extra stdout.
func TestBuildJSONEnvelopeShape(t *testing.T) {
	t.Parallel()

	payload := buildResult{
		Output:          "out.iso",
		ResourcesOutput: "out.resources.iso",
		Type:            "iso",
		Architecture:    "x86_64",
		Version:         "202508141200",
		Channel:         "stable",
		SeedBytes:       3072,
		SHA256:          strings.Repeat("ab", 32),
		ResourcesSHA256: strings.Repeat("cd", 32),
	}
	var stdout bytes.Buffer
	require.NoError(t, writeBuildJSON(&stdout, payload))

	var got buildEnvelope
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &got))
	require.Equal(t, payload, got.Result)
}

// TestLoadBuildSpecOfflineFixture loads an offline config for the dash-usage path.
func TestLoadBuildSpecOfflineFixture(t *testing.T) {
	t.Parallel()

	spec, err := loadBuildSpec(buildDeps{}, stdoutSentinel, strings.NewReader(offlineConfigYAML))
	require.NoError(t, err)
	require.True(t, spec.Offline)
	require.Equal(t, build.ImageTypeISO, spec.Type)
	err = checkBuildSpecUsage(stdoutSentinel, spec)
	require.Error(t, err)
	require.True(t, IsUsage(err))
}

// TestReportBuildErrorWritesOneEnvelope keeps a single JSON error document.
func TestReportBuildErrorWritesOneEnvelope(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	opts := Options{Out: &stdout}
	err := reportBuildError(opts, true, usagef("offline builds cannot use -o -"))
	require.True(t, IsUsage(err))
	assertErrorEnvelope(t, stdout.Bytes(), exitUsage, err.Error())
}

func assertErrorEnvelope(t *testing.T, raw []byte, code int, message string) {
	t.Helper()
	var got errorEnvelope
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, code, got.Error.Code)
	require.Equal(t, message, got.Error.Message)
}

func gzipBytesFixed(t *testing.T, p []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := pgzip.NewWriter(&buf)
	w.ModTime = time.Time{}
	w.OS = 255
	_, err := w.Write(p)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return buf.Bytes()
}
