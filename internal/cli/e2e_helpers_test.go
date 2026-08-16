package cli_test

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	diskfs "github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/filesystem"
	"github.com/diskfs/go-diskfs/partition/gpt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	apiseed "github.com/lxc/incus-os/incus-osd/api/seed"
	incusapi "github.com/lxc/incus/v7/shared/api"

	"github.com/componere/incusos-builder/internal/build"
)

const (
	// envLiveE2E gates the live T3 suite. Ordinary go test ./... must skip.
	envLiveE2E = "INCUSOS_BUILDER_E2E"
	// liveDefaultServer is the production --server default.
	liveDefaultServer = "https://images.linuxcontainers.org/os"
	// seedPartName is the GPT partition the splice writes into.
	seedPartName = "seed-data"
	// rescueVolumeLabel is the ISO/FAT label and GPT partlabel.
	rescueVolumeLabel = "RESCUE_DATA"
	// seedTarMode is the writeSeed tar header mode.
	seedTarMode = int64(0o600)
	// gptSignature is the GPT header magic.
	gptSignature = "EFI PART"
	// gptHeaderLen is the GPT header size.
	gptHeaderLen = 92
	// gptPartLBAOff is the partition-entry LBA field.
	gptPartLBAOff = 72
	// gptNPartsOff is the partition-count field.
	gptNPartsOff = 80
	// gptESizeOff is the partition-entry size field.
	gptESizeOff = 84
	// gptMinEntrySize is the minimum GPT entry size.
	gptMinEntrySize = 128
	// gptMaxParts is a sanity bound on GPT entries.
	gptMaxParts = 4096
	// gptEntryTypeLen is the type-GUID width.
	gptEntryTypeLen = 16
	// gptEntryFirstOff is the first-LBA field.
	gptEntryFirstOff = 32
	// gptEntryLastOff is the last-LBA field.
	gptEntryLastOff = 40
	// gptEntryNameOff is the UTF-16LE name field.
	gptEntryNameOff = 56
	// gptEntryNameBytes is the GPT name field width.
	gptEntryNameBytes = 72
	// gptHeadLimit is the streamed GPT head used to locate seed-data.
	gptHeadLimit = 1 << 20
	// copyBufSize is the reused streaming buffer.
	copyBufSize = 1 << 20
	// isoPartition is GetFilesystem index 0.
	isoPartition = 0
	// fatPartition is the GPT partition number for raw rescue media.
	fatPartition = 1
	// gptHead is the 1 MiB gap before the FAT partition.
	gptHead = 1 << 20
	// rawSector is the GPT/FAT logical sector size.
	rawSector = 512
	// appSuffix is the application filename suffix.
	appSuffix = ".raw.gz"
)

// seedTarEntry is one regular file extracted from the spliced seed tar.
type seedTarEntry struct {
	// Name is the tar header name.
	Name string
	// Mode is the tar header mode.
	Mode int64
	// Body is the file contents.
	Body []byte
}

// seedPartition is the seed-data partition located by the GPT probe.
type seedPartition struct {
	// StartByte is the first byte of the partition.
	StartByte int64
	// Length is the partition length in bytes.
	Length int64
}

// requireLiveE2E skips unless INCUSOS_BUILDER_E2E=1.
func requireLiveE2E(t *testing.T) {
	t.Helper()

	if os.Getenv(envLiveE2E) != "1" {
		t.Skip("live T3 suite requires INCUSOS_BUILDER_E2E=1")
	}
	// Isolate from a developer-exported server. An empty value is explicit
	// under AllowEmptyEnv and would fail as --server "".
	unsetEnv(t, "INCUSOS_BUILDER_SERVER")
}

// unsetEnv clears key for the test without leaving an empty value that Viper would honor.
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "")
	require.NoError(t, os.Unsetenv(key))
}

// liveOnlineRawConfig is a fully populated eleven-section online raw config.
func liveOnlineRawConfig(image liveImage) string {
	return fmt.Sprintf(`version: 1
image:
  type: raw
  architecture: %s
  channel: stable
  release: %q
seeds:
  applications:
    version: "1"
    applications:
      - name: incus
      - name: operations-center
  incus:
    version: "1"
    apply_defaults: true
    preseed:
      config:
        core.https_address: ":8443"
  operations-center:
    version: "1"
    apply_defaults: true
    trusted_client_certificates:
      - |
        -----BEGIN CERTIFICATE-----
        OC
        -----END CERTIFICATE-----
  migration-manager:
    version: "1"
    apply_defaults: true
    trusted_client_certificates:
      - |
        -----BEGIN CERTIFICATE-----
        MM
        -----END CERTIFICATE-----
  install:
    version: "1"
    force_install: true
    force_install_confirmation: reinstall-incusos
    force_reboot: true
    security:
      missing_tpm: true
      missing_secure_boot: true
    target:
      bus: NVME
      id: disk-by-id-example
      min_size: 100GiB
      max_size: 2TiB
      sort_order: largest
  network:
    version: "1"
    confirmation_timeout: 30s
    dns:
      domain: example.test
      hostname: incusos
      nameservers:
        - 192.0.2.53
      search_domains:
        - example.test
      dns_over_tls: true
    time:
      ntp_servers:
        - time.cloudflare.com
      timezone: UTC
    interfaces:
      - name: eth0
        hwaddr: 00:16:3e:aa:bb:cc
        addresses:
          - 192.0.2.10/24
        mtu: 1500
        lldp: true
        roles:
          - management
        routes:
          - to: 0.0.0.0/0
            via: 192.0.2.1
  provider:
    version: "1"
    name: images
    config:
      server: https://images.linuxcontainers.org/os
  services:
    version: "1"
    iscsi:
      enabled: true
      targets:
        - target: iqn.2026-08.test:disk
          address: 192.0.2.20
          port: 3260
    lvm:
      enabled: true
      system_id: 1
    multipath:
      enabled: true
      wwns:
        - naa.60060160
    netbird:
      enabled: true
      setup_key: nb-setup
      management_url: https://netbird.example.test
      admin_url: https://netbird.example.test
      external_ip_map:
        - 192.0.2.10
      extra_dns_labels:
        - incusos
    nvme:
      enabled: true
      targets:
        - transport: tcp
          address: 192.0.2.30
          port: 4420
    ovn:
      enabled: true
      database: tcp:192.0.2.40:6641
      tunnel_address: 192.0.2.41
      tunnel_protocol: geneve
    tailscale:
      enabled: true
      login_server: https://login.tailscale.com
      auth_key: tskey-auth
      advertised_routes:
        - 192.0.2.0/24
    usbip:
      enabled: true
      targets:
        - address: 192.0.2.50
          bus_id: "1-1"
  update:
    version: "1"
    auto_reboot: true
    channel: stable
    check_frequency: 6h
    maintenance_windows:
      - start_day_of_week: Saturday
        start_hour: 2
        start_minute: 0
        end_day_of_week: Saturday
        end_hour: 4
        end_minute: 0
  kernel:
    version: "1"
    console:
      - device: /dev/ttyS0
        baud_rate: 115200
  security:
    version: "1"
    custom_ca_certs:
      - |
        -----BEGIN CERTIFICATE-----
        CA
        -----END CERTIFICATE-----
`, image.Architecture, image.Version)
}

// liveOfflineConfig is a minimal offline config for one application.
func liveOfflineConfig(imageType string, image liveImage, app string) string {
	return fmt.Sprintf(`version: 1
image:
  type: %s
  architecture: %s
  channel: stable
  release: %q
  offline: true
seeds:
  applications:
    applications:
      - name: %s
`, imageType, image.Architecture, image.Version, app)
}

// expectedSeedSections is every seed YAML the fully populated fixture emits.
func expectedSeedSections() []string {
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
		"kernel.yaml",
		"security.yaml",
	}
}

// assertSeedRoundTrip opens image without buffering it, extracts the seed tar,
// and compares every section to spec.
func assertSeedRoundTrip(t *testing.T, imagePath string, spec build.Seeds, seedBytes int64) {
	t.Helper()

	part := locateSeedPartition(t, imagePath)
	require.Positive(t, part.Length, "seed-data length")
	require.LessOrEqual(t, seedBytes, part.Length, "seed tar must fit in seed-data")

	entries := extractSeedTar(t, imagePath, part.StartByte, seedBytes)
	assertExactSeedEntries(t, entries)
	assertSeedSemantics(t, entries, spec)
}

// locateSeedPartition finds the seed-data GPT partition without reading the
// whole image.
func locateSeedPartition(t *testing.T, imagePath string) seedPartition {
	t.Helper()

	fh, err := os.Open(imagePath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = fh.Close() })

	part, err := parseSeedGPT(fh)
	require.NoError(t, err, "locate seed-data")
	return part
}

// extractSeedTar streams the seed-data prefix and unpacks regular 0600 entries.
func extractSeedTar(t *testing.T, imagePath string, offset, size int64) []seedTarEntry {
	t.Helper()

	fh, err := os.Open(imagePath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = fh.Close() })

	section := io.NewSectionReader(fh, offset, size)
	tr := tar.NewReader(section)
	var entries []seedTarEntry
	seen := make(map[string]int)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err, "seed tar should parse")
		name := strings.TrimPrefix(hdr.Name, "./")
		seen[name]++
		require.Equal(t, byte(tar.TypeReg), hdr.Typeflag, "unexpected tar type for %s", name)
		require.Equal(t, seedTarMode, hdr.Mode, "%s must be mode 0600", name)
		body, err := io.ReadAll(tr)
		require.NoError(t, err, "read %s", name)
		entries = append(entries, seedTarEntry{Name: name, Mode: hdr.Mode, Body: body})
	}
	for name, n := range seen {
		require.Equal(t, 1, n, "duplicate seed entry %s", name)
	}
	return entries
}

// assertExactSeedEntries rejects missing, extra, or unexpected seed names.
func assertExactSeedEntries(t *testing.T, entries []seedTarEntry) {
	t.Helper()

	want := expectedSeedSections()
	got := make([]string, 0, len(entries))
	for _, entry := range entries {
		got = append(got, entry.Name)
	}
	assert.Equal(t, want, got, "seed tar members")
}

// assertSeedSemantics strict-decodes each YAML section and compares it to spec.
func assertSeedSemantics(t *testing.T, entries []seedTarEntry, spec build.Seeds) {
	t.Helper()

	byName := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		byName[entry.Name] = entry.Body
	}

	var applications apiseed.Applications
	strictYAML(t, byName["applications.yaml"], &applications)
	assert.Equal(t, *spec.Applications, applications)

	var incus apiseed.Incus
	strictYAML(t, byName["incus.yaml"], &incus)
	assert.Equal(t, spec.Incus.Version, incus.Version)
	assert.Equal(t, spec.Incus.ApplyDefaults, incus.ApplyDefaults)
	require.NotNil(t, incus.Preseed)
	assert.Equal(t, incusapi.ConfigMap{"core.https_address": ":8443"}, incus.Preseed.Config)

	var operations apiseed.OperationsCenter
	strictYAML(t, byName["operations-center.yaml"], &operations)
	assert.Equal(t, *spec.OperationsCenter, operations)

	var migration apiseed.MigrationManager
	strictYAML(t, byName["migration-manager.yaml"], &migration)
	assert.Equal(t, *spec.MigrationManager, migration)

	var install apiseed.Install
	strictYAML(t, byName["install.yaml"], &install)
	assert.Equal(t, *spec.Install, install)

	var network apiseed.Network
	strictYAML(t, byName["network.yaml"], &network)
	assert.Equal(t, spec.Network.Version, network.Version)
	assert.Equal(t, spec.Network.ConfirmationTimeout, network.ConfirmationTimeout)
	assert.Equal(t, spec.Network.DNS, network.DNS)
	assert.Equal(t, spec.Network.Time, network.Time)
	assert.Equal(t, spec.Network.Interfaces, network.Interfaces)

	var provider apiseed.Provider
	strictYAML(t, byName["provider.yaml"], &provider)
	assert.Equal(t, *spec.Provider, provider)

	var services apiseed.Services
	strictYAML(t, byName["services.yaml"], &services)
	assert.Equal(t, *spec.Services, services)

	var updateSeed apiseed.Update
	strictYAML(t, byName["update.yaml"], &updateSeed)
	assert.Equal(t, *spec.Update, updateSeed)

	var kernel apiseed.Kernel
	strictYAML(t, byName["kernel.yaml"], &kernel)
	assert.Equal(t, *spec.Kernel, kernel)

	var security apiseed.Security
	strictYAML(t, byName["security.yaml"], &security)
	assert.Equal(t, spec.Security.Version, security.Version)
	assert.Equal(t, spec.Security.CustomCACerts, security.CustomCACerts)
	assert.Empty(t, security.EncryptionRecoveryKeys)
}

// strictYAML decodes body with yaml.WithKnownFields, matching incus-osd.
func strictYAML(t *testing.T, body []byte, target any) {
	t.Helper()

	loader, err := yaml.NewLoader(bytes.NewReader(body), yaml.WithKnownFields())
	require.NoError(t, err)
	require.NoError(t, loader.Load(target), "yaml strict-decode should succeed")
}

// readRescueTree opens media read-only and walks the update/ tree.
func readRescueTree(t *testing.T, mediaPath string, isISO bool) map[string][]byte {
	t.Helper()

	d, err := diskfs.Open(mediaPath, diskfs.WithOpenMode(diskfs.ReadOnly))
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	var fsys filesystem.FileSystem
	if isISO {
		fsys, err = d.GetFilesystem(isoPartition)
		require.NoError(t, err)
		require.Equal(t, filesystem.TypeISO9660, fsys.Type())
	} else {
		table, err := d.GetPartitionTable()
		require.NoError(t, err)
		g, ok := table.(*gpt.Table)
		require.True(t, ok, "want GPT, got %T", table)
		require.Len(t, g.Partitions, 1)
		p := g.Partitions[0]
		assert.Equal(t, rescueVolumeLabel, p.Name)
		assert.Equal(t, gpt.MicrosoftBasicData, p.Type)
		assert.Equal(t, uint64(gptHead/rawSector), p.Start)
		fsys, err = d.GetFilesystem(fatPartition)
		require.NoError(t, err)
		require.Equal(t, filesystem.TypeFat32, fsys.Type())
	}
	assert.Equal(t, rescueVolumeLabel, strings.TrimRight(fsys.Label(), "\x00 "))

	got := make(map[string][]byte)
	var walk func(dir string)
	walk = func(dir string) {
		ents, err := fsys.ReadDir(dir)
		require.NoError(t, err, "ReadDir(%s)", dir)
		for _, e := range ents {
			name := e.Name()
			if name == "." || name == ".." {
				continue
			}
			child := path.Join(dir, name)
			if e.IsDir() {
				walk(child)
				continue
			}
			f, err := fsys.OpenFile("/"+child, os.O_RDONLY)
			require.NoError(t, err, "OpenFile(/%s)", child)
			if child == "update/update.json" || child == "update/update.sjson" {
				got[child] = readRescueMetadata(t, f, child)
				continue
			}
			got[child] = hashRescueAsset(t, f, child)
		}
	}
	walk("update")
	return got
}

// readRescueMetadata stats child, allocates its logical size, and reads
// exactly those bytes with [io.ReadFull]. The file is always closed.
func readRescueMetadata(t *testing.T, f filesystem.File, child string) []byte {
	t.Helper()

	info, statErr := f.Stat()
	if statErr != nil {
		_ = f.Close()
		require.NoError(t, statErr, "stat %s", child)
	}
	size := info.Size()
	if size < 0 || size > int64(math.MaxInt) {
		_ = f.Close()
		require.GreaterOrEqual(t, size, int64(0), "%s logical size", child)
		require.LessOrEqual(
			t,
			size,
			int64(math.MaxInt),
			"%s logical size overflows addressable range",
			child,
		)
	}
	body := make([]byte, int(size))
	_, readErr := io.ReadFull(f, body)
	_ = f.Close()
	require.NoError(t, readErr, "read %s", child)
	return body
}

// hashRescueAsset streams child through a cluster-aligned SHA-256 digest
// without buffering the asset. The file is always closed.
func hashRescueAsset(t *testing.T, f filesystem.File, child string) []byte {
	t.Helper()

	sum := sha256.New()
	_, err := io.CopyBuffer(sum, f, make([]byte, copyBufSize))
	_ = f.Close()
	require.NoError(t, err, "hash %s", child)
	return []byte(hex.EncodeToString(sum.Sum(nil)))
}

// parseSeedGPT probes logical sector sizes 512/2048/4096 for seed-data.
func parseSeedGPT(r io.Reader) (seedPartition, error) {
	data, err := io.ReadAll(io.LimitReader(r, gptHeadLimit))
	if err != nil {
		return seedPartition{}, err
	}
	for _, secsz := range []int{512, 2048, 4096} {
		if secsz >= len(data) {
			continue
		}
		if string(data[secsz:secsz+len(gptSignature)]) != gptSignature {
			continue
		}
		return seedFromHeader(data, secsz)
	}
	return seedPartition{}, errors.New("no GPT header at 512/2048/4096")
}

// seedFromHeader decodes the seed-data entry from a GPT header at secsz.
func seedFromHeader(data []byte, secsz int) (seedPartition, error) {
	hdrOff := secsz
	if hdrOff+gptHeaderLen > len(data) {
		return seedPartition{}, errors.New("GPT header truncated")
	}
	hdr := data[hdrOff : hdrOff+gptHeaderLen]
	partLBA := binary.LittleEndian.Uint64(hdr[gptPartLBAOff : gptPartLBAOff+8])
	nparts := binary.LittleEndian.Uint32(hdr[gptNPartsOff : gptNPartsOff+4])
	esize := binary.LittleEndian.Uint32(hdr[gptESizeOff : gptESizeOff+4])
	if esize < gptMinEntrySize || nparts == 0 || nparts > gptMaxParts {
		return seedPartition{}, fmt.Errorf("implausible GPT: nparts=%d esize=%d", nparts, esize)
	}
	if partLBA > uint64(^uint(0)/uint(secsz)) {
		return seedPartition{}, errors.New("GPT entry array does not fit in streamed head")
	}
	entriesOff := int(partLBA) * secsz
	entriesLen := int(nparts) * int(esize)
	if entriesOff < 0 || entriesOff > len(data) || entriesLen > len(data)-entriesOff {
		return seedPartition{}, errors.New("GPT entry array does not fit in streamed head")
	}
	entries := data[entriesOff : entriesOff+entriesLen]
	for i := range int(nparts) {
		off := i * int(esize)
		entry := entries[off : off+int(esize)]
		if partitionTypeZero(entry[:gptEntryTypeLen]) {
			continue
		}
		name := gptName(entry[gptEntryNameOff : gptEntryNameOff+gptEntryNameBytes])
		if name != seedPartName {
			continue
		}
		first := binary.LittleEndian.Uint64(entry[gptEntryFirstOff : gptEntryFirstOff+8])
		last := binary.LittleEndian.Uint64(entry[gptEntryLastOff : gptEntryLastOff+8])
		start := int64(first) * int64(secsz)
		length := int64(last-first+1) * int64(secsz)
		return seedPartition{StartByte: start, Length: length}, nil
	}
	return seedPartition{}, errors.New("no seed-data partition in GPT")
}

// partitionTypeZero reports whether the 16-byte type GUID is unset.
func partitionTypeZero(typeGUID []byte) bool {
	return bytes.Equal(typeGUID, make([]byte, gptEntryTypeLen))
}

// gptName decodes a GPT partition name (36 UTF-16LE code units).
func gptName(b []byte) string {
	var runes []rune
	for i := 0; i+1 < len(b); i += 2 {
		unit := binary.LittleEndian.Uint16(b[i : i+2])
		if unit == 0 {
			break
		}
		runes = append(runes, rune(unit))
	}
	return string(runes)
}

// fileSHA256Hex streams path and returns the lowercase hex digest.
func fileSHA256Hex(t *testing.T, name string) string {
	t.Helper()

	fh, err := os.Open(name)
	require.NoError(t, err)
	defer fh.Close()
	sum := sha256.New()
	_, err = io.CopyBuffer(sum, fh, make([]byte, copyBufSize))
	require.NoError(t, err)
	return hex.EncodeToString(sum.Sum(nil))
}

// sha256Hex returns the lowercase hex digest of b.
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// cachedAssetPath returns the content-addressed cache path for digest.
func cachedAssetPath(cacheDir, digest string) string {
	return filepath.Join(cacheDir, "sha256", digest)
}

// sha256OfRescueFile returns the digest of a walked rescue file. Small
// files are hashed from the buffered body; larger files already store
// the hex digest in place of the payload.
func sha256OfRescueFile(t *testing.T, tree map[string][]byte, name string) string {
	t.Helper()

	body, ok := tree[name]
	require.True(t, ok, "missing %s", name)
	if len(body) == 64 && isHexDigest(string(body)) {
		return string(body)
	}
	return sha256Hex(body)
}

// applicationName returns the stem of an application filename.
func applicationName(filename string) string {
	base := path.Base(filename)
	if !strings.HasSuffix(base, appSuffix) {
		return ""
	}
	return strings.TrimSuffix(base, appSuffix)
}

// rescueUpdatePath stages filename under update/, preserving an arch prefix.
func rescueUpdatePath(filename string) string {
	return path.Join("update", path.Clean("/" + filename)[1:])
}
