package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/componere/incusos-builder/internal/testfixture"
)

// TestVersionsTableRendersLocalMirror lists the Generate fixture as a table.
func TestVersionsTableRendersLocalMirror(t *testing.T) {
	mirror := generateVersionsMirror(t)

	stdout, err := executeVersions(t,
		"--server", mirror,
		"--cache-dir", t.TempDir(),
		"--architecture", testfixture.ArchX8664,
		"--color", "never",
	)
	require.NoError(t, err)
	assert.Contains(t, stdout, "Version  Channel  Architecture  Type\n")
	assert.Contains(t, stdout, testfixture.Version+"  "+testfixture.ChannelStable+"  "+testfixture.ArchX8664+"  raw\n")
}

// TestVersionsJSONGolden locks the success envelope field names.
func TestVersionsJSONGolden(t *testing.T) {
	mirror := generateVersionsMirror(t)

	stdout, err := executeVersions(t,
		"--server", mirror,
		"--cache-dir", t.TempDir(),
		"--architecture", testfixture.ArchX8664,
		"--json",
	)
	require.NoError(t, err)

	var env struct {
		Result versionsResult `json:"result"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &env))
	require.Len(t, env.Result.Versions, 1)
	got := env.Result.Versions[0]
	assert.Equal(t, testfixture.Version, got.Version)
	assert.Equal(t, []string{testfixture.ChannelStable}, got.Channels)
	assert.Equal(t, []string{testfixture.ArchX8664}, got.Architectures)
	assert.True(t, got.PublishedAt.Equal(time.Date(2026, time.August, 16, 0, 0, 0, 0, time.UTC)))

	want := `{"result":{"versions":[{"version":"` + testfixture.Version +
		`","channels":["` + testfixture.ChannelStable +
		`"],"published_at":"2026-08-16T00:00:00Z","architectures":["` +
		testfixture.ArchX8664 + `"]}]}}`
	assert.JSONEq(t, want, stdout)
}

// TestVersionsUnknownChannelIsEmpty matches Resolve: no match is not an error.
func TestVersionsUnknownChannelIsEmpty(t *testing.T) {
	mirror := generateVersionsMirror(t)

	stdout, err := executeVersions(t,
		"--server", mirror,
		"--cache-dir", t.TempDir(),
		"--architecture", testfixture.ArchX8664,
		"--channel", "nightly",
		"--json",
	)
	require.NoError(t, err)
	assert.JSONEq(t, `{"result":{"versions":[]}}`, stdout)
}

// TestVersionsBadServerIsUsageError rejects a non-URL, non-directory --server.
func TestVersionsBadServerIsUsageError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		server string
	}{
		{name: "garbage", server: "not-a-server"},
		{name: "plain http", server: "http://example.invalid/os"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := executeVersions(t, "--server", tc.server, "--cache-dir", t.TempDir())
			require.Error(t, err)
			require.True(t, IsUsage(err), "err=%v", err)
			assert.Equal(t, exitUsage, exitCode(err))
		})
	}
}

// generateVersionsMirror writes a local update-server tree via testfixture.Generate.
func generateVersionsMirror(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "mirror")
	_, err := testfixture.Generate(dir)
	require.NoError(t, err)
	return dir
}

// executeVersions wires versions onto NewRootCommand and runs args.
func executeVersions(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	opts := Options{
		In:        bytes.NewReader(nil),
		Out:       &stdout,
		Err:       &stderr,
		Viper:     viper.New(),
		StdinTTY:  func() bool { return false },
		StdoutTTY: func() bool { return false },
		StderrTTY: func() bool { return false },
	}
	root := NewRootCommand(opts)
	root.AddCommand(newVersionsCommand(opts))
	root.SetArgs(append([]string{"versions"}, args...))
	err := root.Execute()
	return stdout.String(), err
}
