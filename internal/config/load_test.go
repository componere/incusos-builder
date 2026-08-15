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

// TestParseSOPSMatrix covers detection, round-trip, and every decrypt failure.
func TestParseSOPSMatrix(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		setKey  bool
		wantErr error
		check   func(*testing.T, build.Spec)
	}{
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

// TestHasTopLevelSOPSDetectsKey is the cheap probe used before decrypt.Data.
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

func unsetForTest(t *testing.T, key string) {
	t.Helper()
	orig, had := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		if !had {
			_ = os.Unsetenv(key)
			return
		}
		_ = os.Setenv(key, orig)
	})
}

func setAgeKey(t *testing.T) {
	t.Helper()
	key, err := os.ReadFile(ageKeyPath)
	if err != nil {
		t.Fatalf("read age key: %v", err)
	}
	t.Setenv("SOPS_AGE_KEY", strings.TrimSpace(string(key)))
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
