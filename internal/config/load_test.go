package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/componere/incusos-builder/internal/build"
)

const ageKeyPath = "testdata/age.key"

// sopsCase is one row of the SOPS parse matrix.
type sopsCase struct {
	// name is the t.Run label.
	name string
	// file is the fixture path under testdata/.
	file string
	// setKey installs the test age key when true.
	setKey bool
	// wantErr is the expected sentinel, or nil on success.
	wantErr error
	// check inspects the decoded spec on success.
	check func(*testing.T, build.Spec)
}

// TestParseSOPSMatrix covers detection, round-trip, and every decrypt failure.
func TestParseSOPSMatrix(t *testing.T) {
	tests := []sopsCase{
		{
			name:   "valid encrypted round-trips",
			file:   "testdata/encrypted.yaml",
			setKey: true,
			check:  assertValidSpec,
		},
		{
			name:    "encrypted without a key",
			file:    "testdata/encrypted.yaml",
			setKey:  false,
			wantErr: ErrDecrypt,
		},
		{
			name:    "malformed sops metadata",
			file:    "testdata/malformed-sops.yaml",
			setKey:  true,
			wantErr: ErrDecrypt,
		},
		{
			name:    "no matching key",
			file:    "testdata/encrypted-wrong-key.yaml",
			setKey:  true,
			wantErr: ErrDecrypt,
		},
		{
			name:    "mac mismatch",
			file:    "testdata/encrypted-mac-mismatch.yaml",
			setKey:  true,
			wantErr: ErrDecrypt,
		},
		{
			name:    "plain config with stray sops key",
			file:    "testdata/stray-sops.yaml",
			setKey:  true,
			wantErr: ErrDecrypt,
		},
		{
			name:   "plain valid config",
			file:   "testdata/valid.yaml",
			setKey: false,
			check:  assertValidSpec,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runSOPSCase(t, tt)
		})
	}
}

// TestLoadStdin reads path "-" including encrypted bytes.
func TestLoadStdin(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		isolateSOPS(t)
		raw, err := os.ReadFile("testdata/valid.yaml")
		if err != nil {
			t.Fatalf("read fixture: %v", err)
		}
		spec, err := load(stdinPath, bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("load stdin: %v", err)
		}
		assertValidSpec(t, spec)
	})
	t.Run("encrypted", func(t *testing.T) {
		isolateSOPS(t)
		setAgeKey(t)
		raw, err := os.ReadFile("testdata/encrypted.yaml")
		if err != nil {
			t.Fatalf("read fixture: %v", err)
		}
		spec, err := load(stdinPath, bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("load encrypted stdin: %v", err)
		}
		assertValidSpec(t, spec)
	})
}

// TestLoadPath reads a filesystem path through the exported Load entrypoint.
func TestLoadPath(t *testing.T) {
	isolateSOPS(t)
	spec, err := Load("testdata/valid.yaml")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	assertValidSpec(t, spec)
}

// TestLoadMissingFile wraps a read failure in ErrConfig.
func TestLoadMissingFile(t *testing.T) {
	isolateSOPS(t)
	_, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if !errors.Is(err, ErrConfig) {
		t.Fatalf("Load() error = %v, want ErrConfig", err)
	}
}

// TestHasTopLevelSOPSDetectsKey treats a top-level sops key as encrypted.
func TestHasTopLevelSOPSDetectsKey(t *testing.T) {
	t.Parallel()

	encrypted, err := hasTopLevelSOPS([]byte("version: 1\nsops:\n  lastmodified: x\n"))
	if err != nil {
		t.Fatalf("hasTopLevelSOPS() error = %v", err)
	}
	if !encrypted {
		t.Fatal("hasTopLevelSOPS() = false, want true")
	}

	plain, err := hasTopLevelSOPS([]byte("version: 1\nimage: {}\n"))
	if err != nil {
		t.Fatalf("hasTopLevelSOPS() error = %v", err)
	}
	if plain {
		t.Fatal("hasTopLevelSOPS() = true, want false")
	}
}

// runSOPSCase executes one SOPS parse matrix row.
func runSOPSCase(t *testing.T, tt sopsCase) {
	t.Helper()
	isolateSOPS(t)
	if tt.setKey {
		setAgeKey(t)
	}
	raw, err := os.ReadFile(tt.file)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	spec, err := Parse(raw)
	if tt.wantErr == nil {
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		if tt.check != nil {
			tt.check(t, spec)
		}
		return
	}
	if err == nil {
		t.Fatal("Parse() error = nil, want error")
	}
	if !errors.Is(err, tt.wantErr) {
		t.Fatalf("Parse() error = %v, want %v", err, tt.wantErr)
	}
	if errors.Is(err, ErrConfig) {
		t.Fatalf("decrypt-path error fell through to ErrConfig: %v", err)
	}
}

func isolateSOPS(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GNUPGHOME", home)
	t.Setenv("SOPS_AGE_KEY", "")
	// Empty-but-set SOPS_AGE_KEY_FILE makes sops open path ""; unset instead.
	unsetForTest(t, "SOPS_AGE_KEY_FILE")
	unsetForTest(t, "SOPS_AGE_KEY_CMD")
}

// unsetForTest removes key for the rest of the test. [testing.T] has no
// Unsetenv in this toolchain; [testing.T.Setenv] registers restore of a
// previously present value, then [os.Unsetenv] drops it for the test body.
func unsetForTest(t *testing.T, key string) {
	t.Helper()
	if orig, had := os.LookupEnv(key); had {
		t.Setenv(key, orig)
	}
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
}

func setAgeKey(t *testing.T) {
	t.Helper()
	key, err := os.ReadFile(ageKeyPath)
	if err != nil {
		t.Fatalf("read age key: %v", err)
	}
	t.Setenv("SOPS_AGE_KEY", strings.TrimSpace(string(key)))
}

// TestParseDecryptStraySOPSSanitizesScalar redacts the stray sops scalar
// and keeps the decrypt diagnostic on one line.
func TestParseDecryptStraySOPSSanitizesScalar(t *testing.T) {
	isolateSOPS(t)
	raw, err := os.ReadFile("testdata/stray-sops.yaml")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	_, err = Parse(raw)
	if err == nil {
		t.Fatal("Parse() error = nil, want ErrDecrypt")
	}
	if !errors.Is(err, ErrDecrypt) {
		t.Fatalf("Parse() error = %v, want ErrDecrypt", err)
	}
	msg := err.Error()
	if strings.ContainsRune(msg, '\n') {
		t.Fatalf("decrypt error is multi-line: %q", msg)
	}
	if strings.Contains(msg, "not-a-s") {
		t.Fatalf("decrypt error leaked scalar fragment: %q", msg)
	}
}

// TestParseDecryptMissingKeyGolden asserts the missing-key decrypt wording.
func TestParseDecryptMissingKeyGolden(t *testing.T) {
	isolateSOPS(t)
	raw, err := os.ReadFile("testdata/encrypted.yaml")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	_, err = Parse(raw)
	if err == nil {
		t.Fatal("Parse() error = nil, want ErrDecrypt")
	}
	if !errors.Is(err, ErrDecrypt) {
		t.Fatalf("Parse() error = %v, want ErrDecrypt", err)
	}
	want := "decryption failed: Error getting data key: 0 successful groups required, got 0"
	if msg := err.Error(); msg != want {
		t.Fatalf("Parse() error = %q, want %q", msg, want)
	}
}

// TestSanitizeYAMLMessage redacts quoted scalars and collapses whitespace.
func TestSanitizeYAMLMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "redacts backtick scalar and flattens yaml unmarshal errors",
			in:   "Error unmarshalling input yaml: yaml: unmarshal errors:\n  line 5: cannot unmarshal !!str `not-a-s...` into stores.Metadata",
			want: "Error unmarshalling input yaml: yaml: unmarshal errors: line 5: cannot unmarshal !!str <value> into stores.Metadata",
		},
		{
			name: "preserves missing-key diagnostic",
			in:   "Error getting data key: 0 successful groups required, got 0",
			want: "Error getting data key: 0 successful groups required, got 0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sanitizeYAMLMessage(tt.in)
			if got != tt.want {
				t.Fatalf("sanitizeYAMLMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func assertValidSpec(t *testing.T, spec build.Spec) {
	t.Helper()
	if spec.Type != build.ImageTypeISO {
		t.Fatalf("Type = %q, want iso", spec.Type)
	}
	if spec.Architecture != build.ArchX8664 {
		t.Fatalf("Architecture = %q, want x86_64", spec.Architecture)
	}
	if spec.Channel != build.DefaultChannel {
		t.Fatalf("Channel = %q, want stable", spec.Channel)
	}
	if spec.Seeds.Applications == nil || len(spec.Seeds.Applications.Applications) != 1 {
		t.Fatalf("Applications = %+v, want one incus entry", spec.Seeds.Applications)
	}
	if spec.Seeds.Applications.Applications[0].Name != "incus" {
		t.Fatalf("Applications[0].Name = %q, want incus", spec.Seeds.Applications.Applications[0].Name)
	}
	if spec.Seeds.Applications.Version != defaultSeedVersion {
		t.Fatalf("Applications.Version = %q, want 1", spec.Seeds.Applications.Version)
	}
}
