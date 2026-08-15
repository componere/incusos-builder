package config

import (
	"errors"
	"strings"
	"testing"

	"github.com/componere/incusos-builder/internal/build"
)

// TestParseValidationRules covers every §4 check, positive and negative.
func TestParseValidationRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		yaml    string
		wantErr error
		path    string
		contain string
		forbid  string
		check   func(*testing.T, build.Spec)
	}{
		{
			name: "iso x86_64",
			yaml: minimalYAML("iso", "x86_64"),
			check: func(t *testing.T, spec build.Spec) {
				t.Helper()
				if spec.Type != build.ImageTypeISO {
					t.Fatalf("Type = %q, want iso", spec.Type)
				}
				if spec.Architecture != build.ArchX8664 {
					t.Fatalf("Architecture = %q, want x86_64", spec.Architecture)
				}
				if spec.Channel != build.DefaultChannel {
					t.Fatalf("Channel = %q, want %q", spec.Channel, build.DefaultChannel)
				}
			},
		},
		{
			name: "raw aarch64",
			yaml: minimalYAML("raw", "aarch64"),
			check: func(t *testing.T, spec build.Spec) {
				t.Helper()
				if spec.Type != build.ImageTypeRaw {
					t.Fatalf("Type = %q, want raw", spec.Type)
				}
				if spec.Architecture != build.ArchAarch64 {
					t.Fatalf("Architecture = %q, want aarch64", spec.Architecture)
				}
			},
		},
		{
			name: "channel omitted defaults to stable",
			yaml: "version: 1\nimage:\n  type: iso\n  architecture: x86_64\n",
			check: func(t *testing.T, spec build.Spec) {
				t.Helper()
				if spec.Channel != build.DefaultChannel {
					t.Fatalf("Channel = %q, want stable", spec.Channel)
				}
			},
		},
		{
			name: "custom channel preserved",
			yaml: "version: 1\nimage:\n  type: iso\n  architecture: x86_64\n  channel: daily\n",
			check: func(t *testing.T, spec build.Spec) {
				t.Helper()
				if spec.Channel != "daily" {
					t.Fatalf("Channel = %q, want daily", spec.Channel)
				}
			},
		},
		{
			name:  "seed versions default to 1",
			yaml:  allSeedsYAML,
			check: assertDefaultSeedVersions,
		},
		{
			name:  "offline creates update seed with never",
			yaml:  offlineYAML,
			check: assertOfflineUpdateCreated,
		},
		{
			name:  "offline forces existing update check_frequency to never",
			yaml:  offlineUpdateYAML,
			check: assertOfflineUpdateForced,
		},
		{
			name: "install sort_order empty",
			yaml: installSortYAML(""),
		},
		{
			name: "install sort_order smallest",
			yaml: installSortYAML("smallest"),
		},
		{
			name: "install sort_order largest",
			yaml: installSortYAML("largest"),
		},
		{
			name: "security custom_ca_certs allowed",
			yaml: `version: 1
image:
  type: iso
  architecture: x86_64
seeds:
  security:
    custom_ca_certs:
      - dummy-ca
`,
		},
		{
			name:    "missing version",
			yaml:    "image:\n  type: iso\n  architecture: x86_64\n",
			wantErr: ErrConfig,
			path:    fieldVersion,
			contain: "required",
		},
		{
			name:    "unsupported version asks for newer CLI",
			yaml:    "version: 2\nimage:\n  type: iso\n  architecture: x86_64\n",
			wantErr: ErrConfig,
			path:    fieldVersion,
			contain: "a newer CLI is required",
		},
		{
			name:    "invalid type",
			yaml:    minimalYAML("disk", "x86_64"),
			wantErr: ErrConfig,
			path:    fieldImageType,
		},
		{
			name:    "invalid architecture",
			yaml:    minimalYAML("iso", "amd64"),
			wantErr: ErrConfig,
			path:    fieldImageArchitecture,
		},
		{
			name:    "offline without applications",
			yaml:    "version: 1\nimage:\n  type: iso\n  architecture: x86_64\n  offline: true\n",
			wantErr: ErrConfig,
			path:    fieldSeedsApplications,
		},
		{
			name: "offline with empty applications list",
			yaml: `version: 1
image:
  type: iso
  architecture: x86_64
  offline: true
seeds:
  applications:
    applications: []
`,
			wantErr: ErrConfig,
			path:    fieldSeedsApplications,
		},
		{
			name: "encryption recovery keys rejected without quoting the secret",
			yaml: `version: 1
image:
  type: iso
  architecture: x86_64
seeds:
  security:
    encryption_recovery_keys:
      - super-secret-recovery-key
`,
			wantErr: ErrConfig,
			path:    fieldSeedsSecurityRecoveryKeys,
			contain: upstreamRecoveryKeysRejected,
			forbid:  "super-secret-recovery-key",
		},
		{
			name:    "invalid install sort_order",
			yaml:    installSortYAML("medium"),
			wantErr: ErrConfig,
			path:    fieldSeedsInstallTargetSortOrder,
		},
		{
			name: "unknown top-level field uses pin wording",
			yaml: `version: 1
image:
  type: iso
  architecture: x86_64
mystery: true
`,
			wantErr: ErrConfig,
			path:    "mystery",
			contain: unknownFieldHint,
		},
		{
			name: "unknown image field uses pin wording",
			yaml: `version: 1
image:
  type: iso
  architecture: x86_64
  flavor: desktop
`,
			wantErr: ErrConfig,
			path:    "image.flavor",
			contain: unknownFieldHint,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec, err := Parse([]byte(tt.yaml))
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
			msg := err.Error()
			if tt.path != "" && !strings.Contains(msg, tt.path) {
				t.Fatalf("error %q does not name field path %q", msg, tt.path)
			}
			if tt.contain != "" && !strings.Contains(msg, tt.contain) {
				t.Fatalf("error %q does not contain %q", msg, tt.contain)
			}
			if tt.forbid != "" && strings.Contains(msg, tt.forbid) {
				t.Fatalf("error %q leaked secret value %q", msg, tt.forbid)
			}
			if errors.Is(err, ErrDecrypt) {
				t.Fatalf("validation error wrapped ErrDecrypt: %v", err)
			}
		})
	}
}

// TestUnknownFieldWording asserts the exact pin string from ARCHITECTURE §4.
func TestUnknownFieldWording(t *testing.T) {
	t.Parallel()

	_, err := Parse([]byte("version: 1\nimage:\n  type: iso\n  architecture: x86_64\nextra: 1\n"))
	if err == nil {
		t.Fatal("Parse() error = nil, want unknown-field error")
	}
	if !errors.Is(err, ErrConfig) {
		t.Fatalf("Parse() error = %v, want ErrConfig", err)
	}
	want := "unknown to incus-os v0.0.0-20260815030500-0f5b8057f2fc; a newer incusos-builder may accept this"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not contain exact pin wording %q", err, want)
	}
}

const allSeedsYAML = `version: 1
image:
  type: iso
  architecture: x86_64
seeds:
  applications:
    applications:
      - name: incus
  install: {}
  incus: {}
  network: {}
  provider: {}
  services: {}
  update: {}
  kernel: {}
  security: {}
  migration-manager: {}
  operations-center: {}
`

const offlineYAML = `version: 1
image:
  type: iso
  architecture: x86_64
  offline: true
seeds:
  applications:
    applications:
      - name: incus
`

const offlineUpdateYAML = `version: 1
image:
  type: iso
  architecture: x86_64
  offline: true
seeds:
  applications:
    applications:
      - name: incus
  update:
    version: "1"
    check_frequency: 6h
`

func minimalYAML(imageType, arch string) string {
	return "version: 1\nimage:\n  type: " + imageType + "\n  architecture: " + arch + "\n"
}

func installSortYAML(order string) string {
	body := "version: 1\nimage:\n  type: iso\n  architecture: x86_64\nseeds:\n  install:\n    target:\n"
	if order == "" {
		return body + "      id: nvme\n"
	}
	return body + "      sort_order: " + order + "\n"
}

func assertDefaultSeedVersions(t *testing.T, spec build.Spec) {
	t.Helper()
	assertSeedVersion(t, "applications", spec.Seeds.Applications.Version)
	assertSeedVersion(t, "install", spec.Seeds.Install.Version)
	assertSeedVersion(t, "incus", spec.Seeds.Incus.Version)
	assertSeedVersion(t, "network", spec.Seeds.Network.Version)
	assertSeedVersion(t, "provider", spec.Seeds.Provider.Version)
	assertSeedVersion(t, "services", spec.Seeds.Services.Version)
	assertSeedVersion(t, "update", spec.Seeds.Update.Version)
	assertSeedVersion(t, "kernel", spec.Seeds.Kernel.Version)
	assertSeedVersion(t, "security", spec.Seeds.Security.Version)
	assertSeedVersion(t, "migration-manager", spec.Seeds.MigrationManager.Version)
	assertSeedVersion(t, "operations-center", spec.Seeds.OperationsCenter.Version)
}

func assertOfflineUpdateCreated(t *testing.T, spec build.Spec) {
	t.Helper()
	if spec.Seeds.Update == nil {
		t.Fatal("Update seed is nil")
	}
	if spec.Seeds.Update.Version != defaultSeedVersion {
		t.Fatalf("Update.Version = %q, want 1", spec.Seeds.Update.Version)
	}
	if spec.Seeds.Update.CheckFrequency != checkFrequencyNever {
		t.Fatalf("CheckFrequency = %q, want never", spec.Seeds.Update.CheckFrequency)
	}
}

func assertOfflineUpdateForced(t *testing.T, spec build.Spec) {
	t.Helper()
	if spec.Seeds.Update.CheckFrequency != checkFrequencyNever {
		t.Fatalf("CheckFrequency = %q, want never", spec.Seeds.Update.CheckFrequency)
	}
}

func assertSeedVersion(t *testing.T, name, got string) {
	t.Helper()
	if got != defaultSeedVersion {
		t.Fatalf("%s.Version = %q, want %q", name, got, defaultSeedVersion)
	}
}
