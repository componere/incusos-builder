package seed

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	osdapi "github.com/lxc/incus-os/incus-osd/api"
	apicustomizer "github.com/lxc/incus-os/incus-osd/api/customizer"
	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
	incusapi "github.com/lxc/incus/v7/shared/api"

	"github.com/componere/incusos-builder/internal/build"
)

const (
	nineSectionGolden  = "testdata/nine-section.golden.tar"
	kernelGoldenYAML   = "testdata/kernel.golden.yaml"
	securityGoldenYAML = "testdata/security.golden.yaml"
)

// TestRenderNineSectionGolden asserts Render matches both the vendored
// writeSeed copy and the committed golden tar.
func TestRenderNineSectionGolden(t *testing.T) {
	t.Parallel()

	seeds := nineSectionSeeds()
	got, size, err := Render(seeds)
	require.NoError(t, err, "Render should succeed for a fully populated nine-section seed")
	require.Equal(t, int64(len(got)), size, "returned size should equal tar length")

	var buf bytes.Buffer
	n, err := writeSeed(&buf, webSeeds(seeds))
	require.NoError(t, err, "vendored writeSeed should succeed")
	upstream := buf.Bytes()
	require.Len(t, upstream, n, "writeSeed counter should match bytes written")

	assert.Equal(t, upstream, got, "Render must be byte-identical to vendored writeSeed")

	golden, err := os.ReadFile(nineSectionGolden)
	require.NoError(t, err, "committed nine-section golden must exist")
	assert.Equal(t, golden, got, "Render must match testdata/nine-section.golden.tar")
	assert.Equal(t, golden, upstream, "vendored writeSeed must match testdata/nine-section.golden.tar")

	entries := readEntries(t, got)
	require.Len(t, entries, 9, "nine-section tar should contain nine entries")
	assert.Equal(t, nineEntryNames(), entryNames(entries))
}

// TestRenderKernelGolden goldens kernel.yaml and strict-decodes it into apiseed.Kernel.
func TestRenderKernelGolden(t *testing.T) {
	t.Parallel()

	seeds := build.Seeds{Kernel: kernelSection()}
	got, size, err := Render(seeds)
	require.NoError(t, err)
	require.Equal(t, int64(len(got)), size)

	entries := readEntries(t, got)
	require.Len(t, entries, 1)
	assert.Equal(t, "kernel.yaml", entries[0].Name)

	golden, err := os.ReadFile(kernelGoldenYAML)
	require.NoError(t, err, "committed kernel golden must exist")
	assert.Equal(t, golden, entries[0].Body, "kernel.yaml body must match testdata/kernel.golden.yaml")

	var decoded apiseed.Kernel
	strictYAML(t, entries[0].Body, &decoded)
	assert.Equal(t, *kernelSection(), decoded, "strict-decoded kernel.yaml should round-trip")
}

// TestRenderSecurityGolden goldens security.yaml and strict-decodes it into apiseed.Security.
func TestRenderSecurityGolden(t *testing.T) {
	t.Parallel()

	seeds := build.Seeds{Security: securitySection()}
	got, size, err := Render(seeds)
	require.NoError(t, err)
	require.Equal(t, int64(len(got)), size)

	entries := readEntries(t, got)
	require.Len(t, entries, 1)
	assert.Equal(t, "security.yaml", entries[0].Name)

	golden, err := os.ReadFile(securityGoldenYAML)
	require.NoError(t, err, "committed security golden must exist")
	assert.Equal(t, golden, entries[0].Body, "security.yaml body must match testdata/security.golden.yaml")

	var decoded apiseed.Security
	strictYAML(t, entries[0].Body, &decoded)
	assert.Equal(t, *securitySection(), decoded, "strict-decoded security.yaml should round-trip")
}

// TestRenderEmptyAndSingleSection covers a zero-entry tar and a one-section tar.
func TestRenderEmptyAndSingleSection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		seeds     build.Seeds
		wantNames []string
	}{
		{
			name:      "empty seeds yields a valid zero-entry tar",
			seeds:     build.Seeds{},
			wantNames: []string{},
		},
		{
			name:      "single network section",
			seeds:     build.Seeds{Network: &apiseed.Network{Version: "1"}},
			wantNames: []string{"network.yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, size, err := Render(tt.seeds)
			require.NoError(t, err)
			require.Equal(t, int64(len(got)), size)
			require.NotEmpty(t, got, "even an empty tar includes end-of-archive blocks")

			entries := readEntries(t, got)
			assert.Equal(t, tt.wantNames, entryNames(entries))
		})
	}
}

// TestRenderKernelSecurityFollowNine asserts CLI-exclusive entries follow writeSeed's nine.
func TestRenderKernelSecurityFollowNine(t *testing.T) {
	t.Parallel()

	seeds := nineSectionSeeds()
	seeds.Kernel = kernelSection()
	seeds.Security = securitySection()

	got, _, err := Render(seeds)
	require.NoError(t, err)

	want := append(nineEntryNames(), "kernel.yaml", "security.yaml")
	assert.Equal(t, want, entryNames(readEntries(t, got)))
}

// tarEntry is one file extracted from a rendered seed tar.
type tarEntry struct {
	// Name is the tar header name.
	Name string
	// Mode is the tar header mode.
	Mode int64
	// Body is the file contents.
	Body []byte
}

// readEntries unpacks raw and asserts every member is mode 0600.
func readEntries(t *testing.T, raw []byte) []tarEntry {
	t.Helper()

	tr := tar.NewReader(bytes.NewReader(raw))
	var entries []tarEntry
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err, "tar should parse")
		body, err := io.ReadAll(tr)
		require.NoError(t, err, "tar entry body should read")
		require.Equal(t, int64(0o600), hdr.Mode, "every seed entry must be mode 0600")
		entries = append(entries, tarEntry{Name: hdr.Name, Mode: hdr.Mode, Body: body})
	}
	return entries
}

// entryNames returns tar member names in archive order.
func entryNames(entries []tarEntry) []string {
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name)
	}
	return names
}

// nineEntryNames is writeSeed's tar member order for the nine web sections.
func nineEntryNames() []string {
	return []string{
		"applications.yaml",
		"incus.yaml",
		"operations-center.yaml",
		"migration-manager.yaml",
		"install.yaml",
		"network.yaml",
		"provider.yaml",
		"services.yaml",
		"update.yaml",
	}
}

// strictYAML decodes body with yaml.WithKnownFields, matching incus-osd.
func strictYAML(t *testing.T, body []byte, target any) {
	t.Helper()

	loader, err := yaml.NewLoader(bytes.NewReader(body), yaml.WithKnownFields())
	require.NoError(t, err)
	require.NoError(t, loader.Load(target), "yaml strict-decode should succeed")
}

// webSeeds projects s onto the nine-section ImagesPostSeeds writeSeed accepts.
func webSeeds(s build.Seeds) apicustomizer.ImagesPostSeeds {
	return apicustomizer.ImagesPostSeeds{
		Applications:     s.Applications,
		Incus:            s.Incus,
		Install:          s.Install,
		MigrationManager: s.MigrationManager,
		Network:          s.Network,
		OperationsCenter: s.OperationsCenter,
		Provider:         s.Provider,
		Services:         s.Services,
		Update:           s.Update,
	}
}

// nineSectionSeeds returns a fully populated nine-section seed fixture.
func nineSectionSeeds() build.Seeds {
	return build.Seeds{
		Applications: &apiseed.Applications{
			Version: "1",
			Applications: []apiseed.Application{
				{Name: "incus"},
				{Name: "operations-center"},
			},
		},
		Incus: &apiseed.Incus{
			Version:       "1",
			ApplyDefaults: true,
			Preseed: &incusapi.InitPreseed{
				InitLocalPreseed: incusapi.InitLocalPreseed{
					ServerPut: incusapi.ServerPut{
						Config: incusapi.ConfigMap{
							"core.https_address": ":8443",
						},
					},
				},
			},
		},
		OperationsCenter: &apiseed.OperationsCenter{
			Version:                   "1",
			ApplyDefaults:             true,
			TrustedClientCertificates: []string{"-----BEGIN CERTIFICATE-----\nOC\n-----END CERTIFICATE-----\n"},
		},
		MigrationManager: &apiseed.MigrationManager{
			Version:                   "1",
			ApplyDefaults:             true,
			TrustedClientCertificates: []string{"-----BEGIN CERTIFICATE-----\nMM\n-----END CERTIFICATE-----\n"},
		},
		Install: &apiseed.Install{
			Version:                  "1",
			ForceInstall:             true,
			ForceInstallConfirmation: "reinstall-incusos",
			ForceReboot:              true,
			Security: &apiseed.InstallSecurity{
				MissingTPM:        true,
				MissingSecureBoot: true,
			},
			Target: &apiseed.InstallTarget{
				Bus:       "NVME",
				ID:        "disk-by-id-example",
				MinSize:   "100GiB",
				MaxSize:   "2TiB",
				SortOrder: "largest",
			},
		},
		Network: &apiseed.Network{
			SystemNetworkConfig: osdapi.SystemNetworkConfig{
				ConfirmationTimeout: "30s",
				DNS: &osdapi.SystemNetworkDNS{
					Domain:        "example.test",
					Hostname:      "incusos",
					Nameservers:   []string{"192.0.2.53"},
					SearchDomains: []string{"example.test"},
					DNSOverTLS:    true,
				},
				Time: &osdapi.SystemNetworkTime{
					NTPServers: []string{"time.cloudflare.com"},
					Timezone:   "UTC",
				},
				Interfaces: []osdapi.SystemNetworkInterface{
					{
						Name:      "eth0",
						Hwaddr:    "00:16:3e:aa:bb:cc",
						Addresses: []string{"192.0.2.10/24"},
						MTU:       1500,
						LLDP:      true,
						Roles:     []string{osdapi.SystemNetworkInterfaceRoleManagement},
						Routes: []osdapi.SystemNetworkRoute{
							{To: "0.0.0.0/0", Via: "192.0.2.1"},
						},
					},
				},
			},
			Version: "1",
		},
		Provider: &apiseed.Provider{
			SystemProviderConfig: osdapi.SystemProviderConfig{
				Name: "images",
				Config: map[string]string{
					"server": "https://images.linuxcontainers.org/os",
				},
			},
			Version: "1",
		},
		Services: &apiseed.Services{
			ISCSI: &osdapi.ServiceISCSIConfig{
				Enabled: true,
				Targets: []osdapi.ServiceISCSITarget{
					{Target: "iqn.2026-08.test:disk", Address: "192.0.2.20", Port: 3260},
				},
			},
			LVM:       &osdapi.ServiceLVMConfig{Enabled: true, SystemID: 1},
			Multipath: &osdapi.ServiceMultipathConfig{Enabled: true, WWNs: []string{"naa.60060160"}},
			Netbird: &osdapi.ServiceNetbirdConfig{
				Enabled:        true,
				SetupKey:       "nb-setup",
				ManagementURL:  "https://netbird.example.test",
				AdminURL:       "https://netbird.example.test",
				ExternalIPMap:  []string{"192.0.2.10"},
				ExtraDNSLabels: []string{"incusos"},
			},
			NVME: &osdapi.ServiceNVMEConfig{
				Enabled: true,
				Targets: []osdapi.ServiceNVMETarget{
					{Transport: "tcp", Address: "192.0.2.30", Port: 4420},
				},
			},
			OVN: &osdapi.ServiceOVNConfig{
				Enabled:        true,
				Database:       "tcp:192.0.2.40:6641",
				TunnelAddress:  "192.0.2.41",
				TunnelProtocol: "geneve",
			},
			Tailscale: &osdapi.ServiceTailscaleConfig{
				Enabled:          true,
				LoginServer:      "https://login.tailscale.com",
				AuthKey:          "tskey-auth",
				AdvertisedRoutes: []string{"192.0.2.0/24"},
			},
			USBIP: &osdapi.ServiceUSBIPConfig{
				Enabled: true,
				Targets: []osdapi.ServiceUSBIPTarget{
					{Address: "192.0.2.50", BusID: "1-1"},
				},
			},
			Version: "1",
		},
		Update: &apiseed.Update{
			SystemUpdateConfig: osdapi.SystemUpdateConfig{
				AutoReboot:     true,
				Channel:        "stable",
				CheckFrequency: "6h",
				MaintenanceWindows: []osdapi.SystemUpdateMaintenanceWindow{
					{
						StartDayOfWeek: osdapi.Saturday,
						StartHour:      2,
						StartMinute:    0,
						EndDayOfWeek:   osdapi.Saturday,
						EndHour:        4,
						EndMinute:      0,
					},
				},
			},
			Version: "1",
		},
	}
}

// kernelSection is the CLI-exclusive kernel fixture.
func kernelSection() *apiseed.Kernel {
	return &apiseed.Kernel{
		Version: "1",
		Console: []osdapi.SystemKernelConfigConsole{
			{Device: "/dev/ttyS0", BaudRate: 115200},
		},
	}
}

// securitySection is the CLI-exclusive security fixture.
func securitySection() *apiseed.Security {
	return &apiseed.Security{
		Version: "1",
		SystemSecurityConfig: osdapi.SystemSecurityConfig{
			CustomCACerts:          []string{"-----BEGIN CERTIFICATE-----\nCA\n-----END CERTIFICATE-----\n"},
			EncryptionRecoveryKeys: []string{},
		},
	}
}

// writeSeed is a verbatim copy of incus-osd/cmd/image-customizer writeSeed
// (github.com/lxc/incus-os/incus-osd @ v0.0.0-20260815030500-0f5b8057f2fc,
// incus-osd/cmd/image-customizer/main.go). It is the byte-compat oracle for
// the nine web-customizer seed sections.
//
//nolint:gocognit // vendored verbatim from upstream image-customizer for golden fidelity
func writeSeed(writer io.Writer, seeds apicustomizer.ImagesPostSeeds) (int, error) {
	archiveContents := [][]string{}

	// Create applications yaml contents.
	if seeds.Applications != nil {
		yamlContents, err := yaml.Dump(seeds.Applications, yaml.WithV2Defaults())
		if err != nil {
			return -1, err
		}

		archiveContents = append(archiveContents, []string{"applications.yaml", string(yamlContents)})
	}

	// Create incus yaml contents.
	if seeds.Incus != nil {
		yamlContents, err := yaml.Dump(seeds.Incus, yaml.WithV2Defaults())
		if err != nil {
			return -1, err
		}

		archiveContents = append(archiveContents, []string{"incus.yaml", string(yamlContents)})
	}

	// Create operations-center yaml contents.
	if seeds.OperationsCenter != nil {
		yamlContents, err := yaml.Dump(seeds.OperationsCenter, yaml.WithV2Defaults())
		if err != nil {
			return -1, err
		}

		archiveContents = append(archiveContents, []string{"operations-center.yaml", string(yamlContents)})
	}

	// Create migration-manager yaml contents.
	if seeds.MigrationManager != nil {
		yamlContents, err := yaml.Dump(seeds.MigrationManager, yaml.WithV2Defaults())
		if err != nil {
			return -1, err
		}

		archiveContents = append(archiveContents, []string{"migration-manager.yaml", string(yamlContents)})
	}

	// Create install yaml contents.
	if seeds.Install != nil {
		yamlContents, err := yaml.Dump(seeds.Install, yaml.WithV2Defaults())
		if err != nil {
			return -1, err
		}

		archiveContents = append(archiveContents, []string{"install.yaml", string(yamlContents)})
	}

	// Create network yaml contents.
	if seeds.Network != nil {
		yamlContents, err := yaml.Dump(seeds.Network, yaml.WithV2Defaults())
		if err != nil {
			return -1, err
		}

		archiveContents = append(archiveContents, []string{"network.yaml", string(yamlContents)})
	}

	// Create provider yaml contents.
	if seeds.Provider != nil {
		yamlContents, err := yaml.Dump(seeds.Provider, yaml.WithV2Defaults())
		if err != nil {
			return -1, err
		}

		archiveContents = append(archiveContents, []string{"provider.yaml", string(yamlContents)})
	}

	// Create services yaml contents.
	if seeds.Services != nil {
		yamlContents, err := yaml.Dump(seeds.Services, yaml.WithV2Defaults())
		if err != nil {
			return -1, err
		}

		archiveContents = append(archiveContents, []string{"services.yaml", string(yamlContents)})
	}

	// Create update yaml contents.
	if seeds.Update != nil {
		yamlContents, err := yaml.Dump(seeds.Update, yaml.WithV2Defaults())
		if err != nil {
			return -1, err
		}

		archiveContents = append(archiveContents, []string{"update.yaml", string(yamlContents)})
	}

	// Put a size counter in place.
	wc := &writeCounter{}

	// Create the tar archive.
	tw := tar.NewWriter(io.MultiWriter(wc, writer))

	for _, file := range archiveContents {
		hdr := &tar.Header{
			Name: file[0],
			Mode: 0o600,
			Size: int64(len(file[1])),
		}

		err := tw.WriteHeader(hdr)
		if err != nil {
			return -1, err
		}

		_, err = tw.Write([]byte(file[1]))
		if err != nil {
			return -1, err
		}
	}

	err := tw.Close()
	if err != nil {
		return -1, err
	}

	return wc.size, nil
}
