package cli

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/componere/incusos-builder/internal/errdefs"
	"github.com/componere/incusos-builder/internal/ux"
)

const (
	testVersion = "0.1.0"
	testCommit  = "abc1234"
	testDate    = "2026-05-08T10:00:00Z"
	testPin     = "v0.0.0-20260815030500-0f5b8057f2fc"
	envServer   = "https://from.env.example/os"
	flagServerV = "https://from.flag.example/os"
	envCache    = "/tmp/from-env-cache"
	flagCache   = "/tmp/from-flag-cache"
	envColor    = "never"
	flagColorV  = "always"
)

// TestVersionFlagPrintsBuildMetadata verifies --version writes BuildInfo plus the pin line.
func TestVersionFlagPrintsBuildMetadata(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root := NewRootCommand(Options{
		Out:        &stdout,
		Err:        &stderr,
		IncusOSPin: testPin,
		Build: BuildInfo{
			Version: testVersion,
			Commit:  testCommit,
			Date:    testDate,
		},
	})
	root.SetArgs([]string{"--version"})

	require.NoError(t, root.ExecuteContext(context.Background()))
	want := "incusos-builder 0.1.0 (abc1234) built 2026-05-08T10:00:00Z\nincus-os API: v0.0.0-20260815030500-0f5b8057f2fc\n"
	require.Equal(t, want, stdout.String())
	require.Empty(t, stderr.String())
}

// TestViperPrecedenceFlagBeatsEnvBeatsDefault covers flags > INCUSOS_BUILDER_* > defaults
// for each of the six Viper-backed settings.
func TestViperPrecedenceFlagBeatsEnvBeatsDefault(t *testing.T) {
	tests := []struct {
		name     string
		envKey   string
		envVal   string
		args     []string
		get      func(*viper.Viper) any
		wantFlag any
		wantEnv  any
		wantDef  any
	}{
		{
			name:     "server",
			envKey:   "INCUSOS_BUILDER_SERVER",
			envVal:   envServer,
			args:     []string{"--server", flagServerV},
			get:      func(vp *viper.Viper) any { return vp.GetString(flagServer) },
			wantFlag: flagServerV,
			wantEnv:  envServer,
			wantDef:  defaultServer,
		},
		{
			name:     "cache-dir",
			envKey:   "INCUSOS_BUILDER_CACHE_DIR",
			envVal:   envCache,
			args:     []string{"--cache-dir", flagCache},
			get:      func(vp *viper.Viper) any { return vp.GetString(flagCacheDir) },
			wantFlag: flagCache,
			wantEnv:  envCache,
			wantDef:  defaultCacheDir(),
		},
		{
			name:     "json",
			envKey:   "INCUSOS_BUILDER_JSON",
			envVal:   "true",
			args:     []string{"--json=false"},
			get:      func(vp *viper.Viper) any { return vp.GetBool(flagJSON) },
			wantFlag: false,
			wantEnv:  true,
			wantDef:  false,
		},
		{
			name:     "color",
			envKey:   "INCUSOS_BUILDER_COLOR",
			envVal:   envColor,
			args:     []string{"--color", flagColorV},
			get:      func(vp *viper.Viper) any { return vp.GetString(flagColor) },
			wantFlag: flagColorV,
			wantEnv:  envColor,
			wantDef:  "auto",
		},
		{
			name:     "progress",
			envKey:   "INCUSOS_BUILDER_PROGRESS",
			envVal:   envColor,
			args:     []string{"--progress", flagColorV},
			get:      func(vp *viper.Viper) any { return vp.GetString(flagProgress) },
			wantFlag: flagColorV,
			wantEnv:  envColor,
			wantDef:  "auto",
		},
		{
			name:     "no-input",
			envKey:   "INCUSOS_BUILDER_NO_INPUT",
			envVal:   "true",
			args:     []string{"--no-input=false"},
			get:      func(vp *viper.Viper) any { return vp.GetBool(flagNoInput) },
			wantFlag: false,
			wantEnv:  true,
			wantDef:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name+"/flag", func(t *testing.T) {
			t.Setenv(tc.envKey, tc.envVal)
			vp := executeViper(t, tc.args)
			require.Equal(t, tc.wantFlag, tc.get(vp))
		})
		t.Run(tc.name+"/env", func(t *testing.T) {
			t.Setenv(tc.envKey, tc.envVal)
			vp := executeViper(t, nil)
			require.Equal(t, tc.wantEnv, tc.get(vp))
		})
		t.Run(tc.name+"/default", func(t *testing.T) {
			unsetEnv(t, tc.envKey)
			vp := executeViper(t, nil)
			require.Equal(t, tc.wantDef, tc.get(vp))
		})
	}
}

// TestViperPrecedenceResolvedPolicy asserts flags > INCUSOS_BUILDER_* > defaults
// on the resolved policy for all six Viper-backed settings (18 rows).
func TestViperPrecedenceResolvedPolicy(t *testing.T) {
	tty := Options{
		StdinTTY:  func() bool { return true },
		StdoutTTY: func() bool { return true },
		StderrTTY: func() bool { return true },
	}

	tests := []struct {
		name   string
		envKey string
		envVal string
		args   []string
		get    func(policy) any
		want   any
	}{
		{
			name:   "server/flag",
			envKey: "INCUSOS_BUILDER_SERVER",
			envVal: envServer,
			args:   []string{"--server", flagServerV},
			get:    func(p policy) any { return p.Server },
			want:   flagServerV,
		},
		{
			name:   "server/env",
			envKey: "INCUSOS_BUILDER_SERVER",
			envVal: envServer,
			get:    func(p policy) any { return p.Server },
			want:   envServer,
		},
		{
			name:   "server/default",
			envKey: "INCUSOS_BUILDER_SERVER",
			get:    func(p policy) any { return p.Server },
			want:   defaultServer,
		},
		{
			name:   "cache-dir/flag",
			envKey: "INCUSOS_BUILDER_CACHE_DIR",
			envVal: envCache,
			args:   []string{"--cache-dir", flagCache},
			get:    func(p policy) any { return p.CacheDir },
			want:   flagCache,
		},
		{
			name:   "cache-dir/env",
			envKey: "INCUSOS_BUILDER_CACHE_DIR",
			envVal: envCache,
			get:    func(p policy) any { return p.CacheDir },
			want:   envCache,
		},
		{
			name:   "cache-dir/default",
			envKey: "INCUSOS_BUILDER_CACHE_DIR",
			get:    func(p policy) any { return p.CacheDir },
			want:   defaultCacheDir(),
		},
		{
			name:   "json/flag",
			envKey: "INCUSOS_BUILDER_JSON",
			envVal: "true",
			args:   []string{"--json=false"},
			get:    func(p policy) any { return p.JSON },
			want:   false,
		},
		{
			name:   "json/env",
			envKey: "INCUSOS_BUILDER_JSON",
			envVal: "true",
			get:    func(p policy) any { return p.JSON },
			want:   true,
		},
		{
			name:   "json/default",
			envKey: "INCUSOS_BUILDER_JSON",
			get:    func(p policy) any { return p.JSON },
			want:   false,
		},
		{
			name:   "color/flag",
			envKey: "INCUSOS_BUILDER_COLOR",
			envVal: envColor,
			args:   []string{"--color", flagColorV},
			get:    func(p policy) any { return p.Color },
			want:   ux.ColorModeAlways,
		},
		{
			name:   "color/env",
			envKey: "INCUSOS_BUILDER_COLOR",
			envVal: envColor,
			get:    func(p policy) any { return p.Color },
			want:   ux.ColorModeNever,
		},
		{
			name:   "color/default",
			envKey: "INCUSOS_BUILDER_COLOR",
			get:    func(p policy) any { return p.Color },
			want:   ux.ColorModeAuto,
		},
		{
			name:   "progress/flag",
			envKey: "INCUSOS_BUILDER_PROGRESS",
			envVal: envColor,
			args:   []string{"--progress", flagColorV},
			get:    func(p policy) any { return p.Progress },
			want:   ux.ProgressModeAlways,
		},
		{
			name:   "progress/env",
			envKey: "INCUSOS_BUILDER_PROGRESS",
			envVal: envColor,
			get:    func(p policy) any { return p.Progress },
			want:   ux.ProgressModeNever,
		},
		{
			name:   "progress/default",
			envKey: "INCUSOS_BUILDER_PROGRESS",
			get:    func(p policy) any { return p.Progress },
			want:   ux.ProgressModeAuto,
		},
		{
			name:   "no-input/flag",
			envKey: "INCUSOS_BUILDER_NO_INPUT",
			envVal: "true",
			args:   []string{"--no-input=false"},
			get:    func(p policy) any { return p.NoInput },
			want:   false,
		},
		{
			name:   "no-input/env",
			envKey: "INCUSOS_BUILDER_NO_INPUT",
			envVal: "true",
			get:    func(p policy) any { return p.NoInput },
			want:   true,
		},
		{
			name:   "no-input/default",
			envKey: "INCUSOS_BUILDER_NO_INPUT",
			get:    func(p policy) any { return p.NoInput },
			want:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envVal == "" {
				unsetEnv(t, tc.envKey)
			} else {
				t.Setenv(tc.envKey, tc.envVal)
			}
			t.Setenv(envCI, "")
			pol := mustResolvePolicy(t, tty, tc.args)
			require.Equal(t, tc.want, tc.get(pol))
		})
	}
}

// TestUsageErrorsExitTwo maps cobra flag-parse failures and invalid enums to exit 2.
// Without --json, stdout stays empty. With --json, stdout is exactly one error envelope.
func TestUsageErrorsExitTwo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "unknown flag", args: []string{"--not-a-real-flag"}},
		{name: "invalid color", args: []string{"--color", "rainbow"}},
		{name: "invalid progress", args: []string{"--progress", "sometimes"}},
		{name: "verbose and quiet", args: []string{"--verbose", "-q"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer
			root := NewRootCommand(Options{
				Out: &stdout,
				Err: ioDiscard(),
				In:  strings.NewReader(""),
			})
			root.SetArgs(tc.args)
			err := root.Execute()
			require.Error(t, err)
			require.True(t, IsUsage(err), "err=%v", err)
			require.Equal(t, exitUsage, exitCode(err))
			require.Empty(t, stdout.String())

			var jsonOut bytes.Buffer
			jsonRoot := NewRootCommand(Options{
				Out: &jsonOut,
				Err: ioDiscard(),
				In:  strings.NewReader(""),
			})
			jsonRoot.SetArgs(append([]string{"--json"}, tc.args...))
			err = jsonRoot.Execute()
			require.Error(t, err)
			require.True(t, IsUsage(err), "err=%v", err)
			require.Equal(t, exitUsage, exitCode(err))
			assertErrorEnvelope(t, jsonOut.Bytes(), exitUsage, err.Error())
			require.Equal(t, 1, strings.Count(jsonOut.String(), "\n"))
		})
	}
}

func executeViper(t *testing.T, args []string) *viper.Viper {
	t.Helper()
	vp := viper.New()
	root := NewRootCommand(Options{
		Viper:     vp,
		Out:       ioDiscard(),
		Err:       ioDiscard(),
		In:        strings.NewReader(""),
		StdinTTY:  func() bool { return true },
		StdoutTTY: func() bool { return true },
		StderrTTY: func() bool { return true },
	})
	root.SetArgs(args)
	require.NoError(t, root.Execute())
	return vp
}

func ioDiscard() *bytes.Buffer {
	return &bytes.Buffer{}
}

// unsetEnv clears key for the test without leaving an empty value that Viper would honor.
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "")
	require.NoError(t, os.Unsetenv(key))
}

// TestEmptyCacheDirEnvMatchesEmptyFlag pins F-CLI-4: an empty
// INCUSOS_BUILDER_CACHE_DIR is equivalent to --cache-dir "".
func TestEmptyCacheDirEnvMatchesEmptyFlag(t *testing.T) {
	const (
		want    = "acquisition failed: cache directory is required"
		httpsOK = "https://example.invalid/os"
	)

	run := func(t *testing.T, args []string) {
		t.Helper()
		var stdout bytes.Buffer
		root := NewRootCommand(Options{
			In:        strings.NewReader(""),
			Out:       &stdout,
			Err:       ioDiscard(),
			Viper:     viper.New(),
			StdinTTY:  func() bool { return true },
			StdoutTTY: func() bool { return true },
			StderrTTY: func() bool { return true },
		})
		root.SetArgs(args)
		err := root.Execute()
		require.Error(t, err)
		require.ErrorIs(t, err, errdefs.ErrFetch)
		require.Equal(t, want, err.Error())
		require.Equal(t, exitFetch, exitCode(err))
		assertErrorEnvelope(t, stdout.Bytes(), exitFetch, want)
	}

	t.Run("flag", func(t *testing.T) {
		unsetEnv(t, "INCUSOS_BUILDER_CACHE_DIR")
		run(t, []string{
			"versions", "--json", "--server", httpsOK,
			"--cache-dir", "", "--color", "never", "--progress", "never",
		})
	})
	t.Run("env", func(t *testing.T) {
		t.Setenv("INCUSOS_BUILDER_CACHE_DIR", "")
		run(t, []string{
			"versions", "--json", "--server", httpsOK,
			"--color", "never", "--progress", "never",
		})
	})
}

// TestOperandAndUnknownCommandExitTwo pins F-CLI-5: extra words and unknown
// commands are usage errors (exit 2) with a JSON envelope whose code is 2.
func TestOperandAndUnknownCommandExitTwo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "unknown command", args: []string{"frobnicate"}},
		{name: "build extra", args: []string{"build", "extra"}},
		{name: "validate x", args: []string{"validate", "x"}},
		{name: "versions x", args: []string{"versions", "x"}},
		{name: "init x", args: []string{"init", "x"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer
			root := NewRootCommand(Options{
				Out: &stdout,
				Err: ioDiscard(),
				In:  strings.NewReader(""),
			})
			root.SetArgs(tc.args)
			err := root.Execute()
			require.Error(t, err)
			require.True(t, IsUsage(err) || isUnknownCommandError(err), "err=%v", err)
			require.Equal(t, exitUsage, exitCode(err))
			require.Empty(t, stdout.String())

			var jsonOut bytes.Buffer
			jsonRoot := NewRootCommand(Options{
				Out: &jsonOut,
				Err: ioDiscard(),
				In:  strings.NewReader(""),
			})
			jsonRoot.SetArgs(append([]string{"--json"}, tc.args...))
			err = jsonRoot.Execute()
			require.Error(t, err)
			require.True(t, IsUsage(err) || isUnknownCommandError(err), "err=%v", err)
			require.Equal(t, exitUsage, exitCode(err))
			assertErrorEnvelope(t, jsonOut.Bytes(), exitUsage, err.Error())
			require.Equal(t, 1, strings.Count(jsonOut.String(), "\n"))
		})
	}
}
