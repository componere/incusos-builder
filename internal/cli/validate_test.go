package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/componere/incusos-builder/internal/build"
	"github.com/componere/incusos-builder/internal/errdefs"
)

const (
	configTestdata      = "../config/testdata"
	validConfigFile     = configTestdata + "/valid.yaml"
	encryptedConfigFile = configTestdata + "/encrypted.yaml"
)

// TestValidateValidConfig prints the human success line for a committed fixture.
func TestValidateValidConfig(t *testing.T) {
	t.Parallel()

	stdout, stderr, err := executeValidate(t, nil, "-f", validConfigFile)
	require.NoError(t, err)
	assert.Equal(t, validateOK+"\n", stdout)
	assert.Empty(t, stderr)
}

// TestValidateQuietSuppressesSuccessLine honors -q for the human writer.
func TestValidateQuietSuppressesSuccessLine(t *testing.T) {
	t.Parallel()

	stdout, stderr, err := executeValidate(t, nil, "-f", validConfigFile, "-q")
	require.NoError(t, err)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
}

// TestValidateQuietKeepsJSONEnvelope leaves --json success output intact.
func TestValidateQuietKeepsJSONEnvelope(t *testing.T) {
	t.Parallel()

	stdout, stderr, err := executeValidate(t, nil, "-f", validConfigFile, "-q", "--json")
	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.JSONEq(t, `{"result":{"valid":true,"type":"iso","architecture":"x86_64","offline":false}}`, stdout)
}

// TestValidateJSONEnvelope locks the success document field names.
func TestValidateJSONEnvelope(t *testing.T) {
	t.Parallel()

	stdout, stderr, err := executeValidate(t, nil, "-f", validConfigFile, "--json")
	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.JSONEq(t, `{"result":{"valid":true,"type":"iso","architecture":"x86_64","offline":false}}`, stdout)

	var env struct {
		Result validateResult `json:"result"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &env))
	assert.True(t, env.Result.Valid)
	assert.Equal(t, build.ImageTypeISO, env.Result.Type)
	assert.Equal(t, build.ArchX8664, env.Result.Architecture)
	assert.False(t, env.Result.Offline)
}

// TestValidateInvalidConfigIsExitThree treats a schema failure as ErrConfig.
func TestValidateInvalidConfigIsExitThree(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "invalid.yaml")
	require.NoError(t, os.WriteFile(path, []byte("version: 1\nimage:\n  type: iso\n  architecture: riscv\n"), 0o644))

	stdout, _, err := executeValidate(t, nil, "-f", path)
	require.Error(t, err)
	require.ErrorIs(t, err, errdefs.ErrConfig)
	assert.Equal(t, exitConfig, exitCode(err))
	assert.Empty(t, stdout)
}

// TestValidateEncryptedWithoutKeyIsExitFour uses the committed encrypted fixture.
func TestValidateEncryptedWithoutKeyIsExitFour(t *testing.T) {
	isolateSOPSHome(t)

	stdout, _, err := executeValidate(t, nil, "-f", encryptedConfigFile)
	require.Error(t, err)
	require.ErrorIs(t, err, errdefs.ErrDecrypt)
	assert.Equal(t, exitDecrypt, exitCode(err))
	assert.Empty(t, stdout)
}

// TestValidateJSONErrorEnvelope writes exactly one error document.
func TestValidateJSONErrorEnvelope(t *testing.T) {
	isolateSOPSHome(t)

	stdout, stderr, err := executeValidate(t, nil, "-f", encryptedConfigFile, "--json")
	require.Error(t, err)
	require.ErrorIs(t, err, errdefs.ErrDecrypt)
	assert.Equal(t, exitDecrypt, exitCode(err))
	assert.Empty(t, stderr)

	var env errorEnvelope
	require.NoError(t, json.Unmarshal([]byte(stdout), &env))
	assert.Equal(t, exitDecrypt, env.Error.Code)
	assert.NotEmpty(t, env.Error.Message)
	assert.Equal(t, 1, strings.Count(stdout, "\n"))
}

// TestValidateStdinDashReadsConfig accepts -f - from opts.In.
func TestValidateStdinDashReadsConfig(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(validConfigFile)
	require.NoError(t, err)
	stdout, _, err := executeValidate(t, bytes.NewReader(raw), "-f", "-")
	require.NoError(t, err)
	assert.Equal(t, validateOK+"\n", stdout)
}

// TestValidateDotSlashDashReadsStdin treats -f ./- as the stdin sentinel.
func TestValidateDotSlashDashReadsStdin(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(validConfigFile)
	require.NoError(t, err)
	stdout, _, err := executeValidate(t, bytes.NewReader(raw), "-f", "./-")
	require.NoError(t, err)
	assert.Equal(t, validateOK+"\n", stdout)
}

// TestValidateMissingConfigFlagIsUsage requires -f.
func TestValidateMissingConfigFlagIsUsage(t *testing.T) {
	t.Parallel()

	_, _, err := executeValidate(t, nil)
	require.Error(t, err)
	require.True(t, IsUsage(err), "err=%v", err)
	assert.Equal(t, exitUsage, exitCode(err))
}

// executeValidate wires validate onto NewRootCommand and runs args.
func executeValidate(t *testing.T, stdin io.Reader, args ...string) (string, string, error) {
	t.Helper()
	if stdin == nil {
		stdin = bytes.NewReader(nil)
	}
	var stdout, stderr bytes.Buffer
	opts := Options{
		In:        stdin,
		Out:       &stdout,
		Err:       &stderr,
		Viper:     viper.New(),
		StdinTTY:  func() bool { return false },
		StdoutTTY: func() bool { return false },
		StderrTTY: func() bool { return false },
	}
	root := NewRootCommand(opts)
	root.SetArgs(append([]string{"validate"}, args...))
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

// isolateSOPSHome clears ambient SOPS key sources so encrypted fixtures fail.
func isolateSOPSHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GNUPGHOME", home)
	t.Setenv("SOPS_AGE_KEY", "")
	if orig, had := os.LookupEnv("SOPS_AGE_KEY_FILE"); had {
		t.Setenv("SOPS_AGE_KEY_FILE", orig)
	}
	if orig, had := os.LookupEnv("SOPS_AGE_KEY_CMD"); had {
		t.Setenv("SOPS_AGE_KEY_CMD", orig)
	}
	require.NoError(t, os.Unsetenv("SOPS_AGE_KEY_FILE"))
	require.NoError(t, os.Unsetenv("SOPS_AGE_KEY_CMD"))
}
