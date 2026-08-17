package ux

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"charm.land/log/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/componere/incusos-builder/internal/build"
)

const ansiCSI = "\x1b["

var (
	_ build.Reporter = (*fancy)(nil)
	_ build.Reporter = (*plain)(nil)
)

// TestPlainReporterShape goldens the ASCII event lines.
func TestPlainReporterShape(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	rep := New(ColorModeNever, ProgressModeAlways, &buf)
	_, isPlain := rep.(*plain)
	require.True(t, isPlain, "never must select the plain renderer")

	rep.Step("resolve")
	rep.Progress(0, 100)
	rep.Progress(100, 100)
	rep.Done("resolve")
	rep.Step("acquire")
	rep.Done("acquire")

	got := buf.String()
	want := "" +
		"==> resolve\n" +
		"progress 0% (0/100)\n" +
		"progress 100% (100/100)\n" +
		"done resolve\n" +
		"==> acquire\n" +
		"done acquire\n"
	assert.Equal(t, want, got)
	assert.NotContains(t, got, ansiCSI)
	assert.NotContains(t, got, "\r")
}

// TestPlainProgressRateLimit keeps piped percentage lines sparse.
func TestPlainProgressRateLimit(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	rep := New(ColorModeNever, ProgressModeAlways, &buf)
	for i := int64(1); i < 50; i++ {
		rep.Progress(i, 100)
	}
	rep.Progress(100, 100)

	lines := nonEmptyLines(buf.String())
	require.GreaterOrEqual(t, len(lines), 2)
	require.Less(t, len(lines), 10, "rapid ticks must not emit a line per call")
	assert.Equal(t, "progress 1% (1/100)", lines[0])
	assert.Equal(t, "progress 100% (100/100)", lines[len(lines)-1])
}

// TestModeMatrixAutoResolvesOff covers NO_COLOR, TERM=dumb, and a non-TTY
// buffer all selecting the plain renderer under auto.
func TestModeMatrixAutoResolvesOff(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{name: "non-tty buffer", env: map[string]string{"NO_COLOR": "", "TERM": "xterm-256color"}},
		{name: "NO_COLOR set", env: map[string]string{"NO_COLOR": "1", "TERM": "xterm-256color"}},
		{name: "TERM=dumb", env: map[string]string{"NO_COLOR": "", "TERM": "dumb"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for key, val := range tc.env {
				t.Setenv(key, val)
			}
			var buf bytes.Buffer
			rep := New(ColorModeAuto, ProgressModeAuto, &buf)
			_, isPlain := rep.(*plain)
			require.True(t, isPlain)

			rep.Step("resolve")
			rep.Progress(50, 100)
			rep.Done("resolve")

			got := buf.String()
			assert.Equal(t, "==> resolve\ndone resolve\n", got)
			assert.NotContains(t, got, ansiCSI)
			assert.NotContains(t, got, "\r")
			assert.False(t, colorAutoEnabled(&buf))

			var extra bytes.Buffer
			writeSummaryAndVersions(ColorModeAuto, &extra)
			assertSummaryAndVersionsShape(t, extra.String())
			assert.NotContains(t, extra.String(), ansiCSI)
		})
	}
}

// TestColorAlwaysForcesANSI writes CSI sequences even to a capture buffer.
func TestColorAlwaysForcesANSI(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "dumb")

	var buf bytes.Buffer
	rep := New(ColorModeAlways, ProgressModeAlways, &buf)
	_, isFancy := rep.(*fancy)
	require.True(t, isFancy, "always must select the fancy renderer")

	rep.Step("resolve")
	rep.Progress(50, 100)
	rep.Done("resolve")

	got := buf.String()
	assert.Contains(t, got, ansiCSI)
	assert.Contains(t, got, "\r")
	assert.Contains(t, got, "resolve")

	var extra bytes.Buffer
	writeSummaryAndVersions(ColorModeAlways, &extra)
	assertSummaryAndVersionsShape(t, extra.String())
	assert.Contains(t, extra.String(), ansiCSI)
}

// TestColorNeverStrips has no CSI even when progress is forced on.
func TestColorNeverStrips(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	rep := New(ColorModeNever, ProgressModeAlways, &buf)
	rep.Step("seed")
	rep.Progress(25, 100)
	rep.Done("seed")
	got := buf.String()
	assert.NotContains(t, got, ansiCSI)
	assert.Contains(t, got, "==> seed\n")
	assert.Contains(t, got, "progress 25% (25/100)\n")
	assert.Contains(t, got, "done seed\n")

	var extra bytes.Buffer
	writeSummaryAndVersions(ColorModeNever, &extra)
	assertSummaryAndVersionsShape(t, extra.String())
	assert.NotContains(t, extra.String(), ansiCSI)
}

// TestProgressResolvedOffEmitsNoANSI checks Progress is a no-op when off.
func TestProgressResolvedOffEmitsNoANSI(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	rep := New(ColorModeAlways, ProgressModeNever, &buf)
	rep.Step("splice")
	before := buf.String()
	rep.Progress(1, 100)
	rep.Progress(50, 100)
	rep.Progress(100, 100)
	after := buf.String()
	assert.Equal(t, before, after, "progress-off must not write on Progress")
	assert.NotContains(t, after, "\r")
	assert.Contains(t, after, ansiCSI, "step headers still style when color is on")
}

// TestNewLoggerColorMode follows the same always/never/auto matrix.
func TestNewLoggerColorMode(t *testing.T) {
	t.Run("never strips", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewLogger(ColorModeNever, &buf)
		logger.Info("hello")
		got := buf.String()
		assert.Contains(t, got, "hello")
		assert.NotContains(t, got, ansiCSI)
	})
	t.Run("always forces ANSI", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")
		t.Setenv("TERM", "dumb")
		var buf bytes.Buffer
		logger := NewLogger(ColorModeAlways, &buf)
		logger.Info("hello")
		got := buf.String()
		assert.Contains(t, got, "hello")
		assert.Contains(t, got, ansiCSI)
	})
	t.Run("auto on buffer is unstyled", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		t.Setenv("TERM", "xterm-256color")
		var buf bytes.Buffer
		logger := NewLogger(ColorModeAuto, &buf)
		logger.SetLevel(log.InfoLevel)
		logger.Info("hello")
		assert.Contains(t, buf.String(), "hello")
		assert.NotContains(t, buf.String(), ansiCSI)
	})
}

// TestSummaryAndVersionsTableColorMode follows the same always/never/auto matrix.
func TestSummaryAndVersionsTableColorMode(t *testing.T) {
	t.Run("never strips", func(t *testing.T) {
		var buf bytes.Buffer
		writeSummaryAndVersions(ColorModeNever, &buf)
		assertSummaryAndVersionsShape(t, buf.String())
		assert.NotContains(t, buf.String(), ansiCSI)
	})
	t.Run("always forces ANSI", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")
		t.Setenv("TERM", "dumb")
		var buf bytes.Buffer
		writeSummaryAndVersions(ColorModeAlways, &buf)
		assertSummaryAndVersionsShape(t, buf.String())
		assert.Contains(t, buf.String(), ansiCSI)
	})
	t.Run("auto on buffer is unstyled", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		t.Setenv("TERM", "xterm-256color")
		var buf bytes.Buffer
		writeSummaryAndVersions(ColorModeAuto, &buf)
		assertSummaryAndVersionsShape(t, buf.String())
		assert.NotContains(t, buf.String(), ansiCSI)
	})
}

// TestSummaryAndVersionsTable goldens the unstyled writer output.
func TestSummaryAndVersionsTable(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	writeSummaryAndVersions(ColorModeNever, &buf)
	want := "" +
		"summary\n" +
		"version  202608102114\n" +
		"type  iso\n" +
		"Version  Channel  Architecture  Type\n" +
		"202608102114  stable  x86_64  iso\n"
	assert.Equal(t, want, buf.String())
	assert.NotContains(t, buf.String(), ansiCSI)
}

// TestNilWriterDoesNotPanic treats a nil writer as discard.
func TestNilWriterDoesNotPanic(t *testing.T) {
	t.Parallel()

	rep := New(ColorModeAlways, ProgressModeAlways, nil)
	require.NotNil(t, rep)
	rep.Step("resolve")
	rep.Progress(1, 1)
	rep.Done("resolve")
	Summary(ColorModeAlways, nil, []SummaryRow{{Label: "k", Value: "v"}})
	VersionsTable(ColorModeNever, nil, []VersionRow{{
		Version: "1", Channel: "stable", Architecture: "x86_64", Type: "iso",
	}})
	require.NotNil(t, NewLogger(ColorModeNever, nil))
	require.NotNil(t, New(ColorModeNever, ProgressModeNever, io.Discard))
}

// writeSummaryAndVersions writes a [Summary] and a [VersionsTable] to w.
func writeSummaryAndVersions(color ColorMode, w io.Writer) {
	Summary(color, w, []SummaryRow{
		{Label: "version", Value: "202608102114"},
		{Label: "type", Value: "iso"},
	})
	VersionsTable(color, w, []VersionRow{{
		Version:      "202608102114",
		Channel:      "stable",
		Architecture: "x86_64",
		Type:         "iso",
	}})
}

// assertSummaryAndVersionsShape checks labels both surfaces must emit.
func assertSummaryAndVersionsShape(t *testing.T, got string) {
	t.Helper()
	assert.Contains(t, got, "summary")
	assert.Contains(t, got, "version")
	assert.Contains(t, got, "202608102114")
	assert.Contains(t, got, "iso")
	assert.Contains(t, got, "Version")
	assert.Contains(t, got, "stable")
	assert.Contains(t, got, "x86_64")
}

func nonEmptyLines(s string) []string {
	raw := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
