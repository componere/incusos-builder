package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
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
			t.Setenv(tc.envKey, "")
			vp := executeViper(t, nil)
			require.Equal(t, tc.wantDef, tc.get(vp))
		})
	}
}

// TestUsageErrorsExitTwo maps cobra flag-parse failures and invalid enums to exit 2.
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
			root := NewRootCommand(Options{
				Out: ioDiscard(),
				Err: ioDiscard(),
				In:  strings.NewReader(""),
			})
			root.SetArgs(tc.args)
			err := root.Execute()
			require.Error(t, err)
			require.True(t, IsUsage(err), "err=%v", err)
			require.Equal(t, exitUsage, exitCode(err))
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
