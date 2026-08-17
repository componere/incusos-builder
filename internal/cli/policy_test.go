package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/componere/incusos-builder/internal/ux"
)

// TestNoInputAutoOn covers the --no-input auto-on matrix.
func TestNoInputAutoOn(t *testing.T) {
	tests := []struct {
		name      string
		ci        string
		stdinTTY  bool
		stdoutTTY bool
		args      []string
		want      bool
	}{
		{name: "non-tty stdin", stdinTTY: false, stdoutTTY: true, want: true},
		{name: "non-tty stdout", stdinTTY: true, stdoutTTY: false, want: true},
		{name: "CI set", ci: "1", stdinTTY: true, stdoutTTY: true, want: true},
		{name: "both ttys no CI", ci: "", stdinTTY: true, stdoutTTY: true, want: false},
		{name: "flag forces on", ci: "", stdinTTY: true, stdoutTTY: true, args: []string{"--no-input"}, want: true},
		{
			name:      "flag overrides tty",
			ci:        "",
			stdinTTY:  true,
			stdoutTTY: true,
			args:      []string{"--no-input=true"},
			want:      true,
		},
		{name: "flag overrides CI", ci: "1", stdinTTY: true, stdoutTTY: true, args: []string{"--no-input"}, want: true},
		{
			name:      "flag overrides non-tty stdin",
			stdinTTY:  false,
			stdoutTTY: true,
			args:      []string{"--no-input"},
			want:      true,
		},
		{
			name:      "flag overrides non-tty stdout",
			stdinTTY:  true,
			stdoutTTY: false,
			args:      []string{"--no-input"},
			want:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(envCI, tc.ci)
			pol := mustResolvePolicy(t, Options{
				StdinTTY:  func() bool { return tc.stdinTTY },
				StdoutTTY: func() bool { return tc.stdoutTTY },
				StderrTTY: func() bool { return true },
			}, tc.args)
			require.Equal(t, tc.want, pol.NoInput)
		})
	}
}

// TestProgressAutoRequiresBothTTYs pre-resolves AUTO when either stream is not a TTY.
func TestProgressAutoRequiresBothTTYs(t *testing.T) {
	t.Setenv(envCI, "")

	tests := []struct {
		name      string
		stdoutTTY bool
		stderrTTY bool
		args      []string
		want      ux.ProgressMode
	}{
		{name: "both ttys stay auto", stdoutTTY: true, stderrTTY: true, want: ux.ProgressModeAuto},
		{name: "stdout pipe becomes never", stdoutTTY: false, stderrTTY: true, want: ux.ProgressModeNever},
		{name: "stderr pipe becomes never", stdoutTTY: true, stderrTTY: false, want: ux.ProgressModeNever},
		{name: "both pipes become never", stdoutTTY: false, stderrTTY: false, want: ux.ProgressModeNever},
		{
			name:      "always overrides",
			stdoutTTY: false,
			stderrTTY: false,
			args:      []string{"--progress", "always"},
			want:      ux.ProgressModeAlways,
		},
		{
			name:      "always overrides mixed ttys",
			stdoutTTY: true,
			stderrTTY: false,
			args:      []string{"--progress", "always"},
			want:      ux.ProgressModeAlways,
		},
		{
			name:      "never overrides",
			stdoutTTY: true,
			stderrTTY: true,
			args:      []string{"--progress", "never"},
			want:      ux.ProgressModeNever,
		},
		{
			name:      "never overrides pipes",
			stdoutTTY: false,
			stderrTTY: false,
			args:      []string{"--progress", "never"},
			want:      ux.ProgressModeNever,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pol := mustResolvePolicy(t, Options{
				StdinTTY:  func() bool { return true },
				StdoutTTY: func() bool { return tc.stdoutTTY },
				StderrTTY: func() bool { return tc.stderrTTY },
			}, tc.args)
			require.Equal(t, tc.want, pol.Progress)
		})
	}
}

// TestColorModePassedThrough does not pre-resolve AUTO.
func TestColorModePassedThrough(t *testing.T) {
	t.Setenv(envCI, "")

	pol := mustResolvePolicy(t, Options{
		StdinTTY:  func() bool { return true },
		StdoutTTY: func() bool { return true },
		StderrTTY: func() bool { return true },
	}, nil)
	require.Equal(t, ux.ColorModeAuto, pol.Color)

	pol = mustResolvePolicy(t, Options{
		StdinTTY:  func() bool { return true },
		StdoutTTY: func() bool { return true },
		StderrTTY: func() bool { return true },
	}, []string{"--color", "always"})
	require.Equal(t, ux.ColorModeAlways, pol.Color)
}

// TestVerboseEmitsDebugToStderr constructs the policy logger at debug when
// --verbose is set, and at warn-and-above by default so debug is silent.
func TestVerboseEmitsDebugToStderr(t *testing.T) {
	t.Setenv(envCI, "")

	var verboseErr bytes.Buffer
	verbose := mustResolvePolicy(t, Options{
		Err:       &verboseErr,
		StdinTTY:  func() bool { return true },
		StdoutTTY: func() bool { return true },
		StderrTTY: func() bool { return true },
	}, []string{"--verbose", "--color", "never"})
	logBuildPlan(verbose, "202508141200", "IncusOS_202508141200.iso", "out.iso", "")
	require.Contains(t, verboseErr.String(), "resolved version")
	require.Contains(t, verboseErr.String(), "202508141200")
	require.Contains(t, verboseErr.String(), "IncusOS_202508141200.iso")
	require.Contains(t, verboseErr.String(), "output paths")
	require.Contains(t, verboseErr.String(), "out.iso")

	var defaultErr bytes.Buffer
	def := mustResolvePolicy(t, Options{
		Err:       &defaultErr,
		StdinTTY:  func() bool { return true },
		StdoutTTY: func() bool { return true },
		StderrTTY: func() bool { return true },
	}, []string{"--color", "never"})
	logBuildPlan(def, "202508141200", "IncusOS_202508141200.iso", "out.iso", "")
	require.Empty(t, defaultErr.String())
}

// mustResolvePolicy runs the root command and returns the resolved policy.
func mustResolvePolicy(t *testing.T, opts Options, args []string) policy {
	t.Helper()
	if opts.Viper == nil {
		opts.Viper = viper.New()
	}
	if opts.In == nil {
		opts.In = strings.NewReader("")
	}
	root := NewRootCommand(opts)
	root.SetArgs(args)
	require.NoError(t, root.Execute())
	pol, err := resolvePolicy(root, opts.withDefaults(), opts.Viper)
	require.NoError(t, err)
	return pol
}
